// The JavaScript half of GoLib's web backend: a WebGL 2 renderer, the
// keyboard, the mouse and the gamepads, and the frame the browser asks for.
// web.go and web_draw.go are the Go half; the command numbers below and the
// ones in web_draw.go are the same list, so change one and change the other.
//
// A game never calls into this file. Package golib writes its drawing into a
// buffer of numbers and hands the whole frame over at once, because a call
// per shape would cost more than the drawing.

window.golib = (function () {
	'use strict';

	// The drawing commands, as web_draw.go writes them.
	const CLEAR = 1, RECTANGLE = 2, RECTANGLE_OUTLINE = 3, CIRCLE = 4, RING = 5,
		LINE = 6, TRIANGLE = 7, TEXTURE = 8, BEGIN_TARGET = 9, END_TARGET = 10,
		BEGIN_CAMERA = 11, END_CAMERA = 12, BLEND = 13, BEGIN_SHADER = 14,
		END_SHADER = 15, SHADER_VALUES = 16, BEGIN_FRAME = 17, END_FRAME = 18;

	const BLEND_NORMAL = 0, BLEND_ADD = 1, BLEND_COPY = 2;

	// How many vertices a batch holds before it goes to the graphics card.
	const BATCH_VERTICES = 24576;
	const FLOATS_PER_VERTEX = 8; // x, y, u, v, r, g, b, a

	const DEG = Math.PI / 180;

	let canvas = null, gl = null, program = null, screenLocation = null;
	let vertices = null, vertexBuffer = null, vertexArray = null, used = 0;
	let white = null; // a single white pixel, for shapes

	// What is being drawn on now.
	let boundTarget = 0, boundTexture = 0, boundBlend = BLEND_NORMAL;
	let viewWidth = 0, viewHeight = 0;

	// The camera, as an offset and a zoom: BeginCamera sets it, and every
	// point is moved by it as it is added to the batch.
	let camera = null;

	const textures = new Map(); // id -> WebGLTexture
	const targets = new Map();  // id -> { framebuffer, texture, textureID, width, height }
	let nextID = 1;

	let frameTime = 0, wake = null, closed = false;
	let wantFullscreen = false, fullscreenAsked = false;
	let cursorShown = true;

	let commandBytes = null, commandFloats = null;

	// ---------------------------------------------------------------- setup

	function open(width, height, title) {
		if (title) document.title = title;
		canvas = document.getElementById('game');
		if (!canvas) throw new Error('golib: the page has no <canvas id="game">');
		gl = canvas.getContext('webgl2', {
			alpha: false,
			antialias: false,
			depth: false,
			stencil: false,
			premultipliedAlpha: false,
			preserveDrawingBuffer: true,
		});
		if (!gl) throw new Error('golib: this browser has no WebGL 2');
		resize();
		program = buildProgram();
		gl.useProgram(program);
		screenLocation = gl.getUniformLocation(program, 'screen');
		gl.uniform1i(gl.getUniformLocation(program, 'image'), 0);
		setUpBatch();
		white = gl.createTexture();
		gl.bindTexture(gl.TEXTURE_2D, white);
		gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, 1, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE,
			new Uint8Array([255, 255, 255, 255]));
		clampAndFilter(gl.NEAREST);
		gl.enable(gl.BLEND);
		setBlend(BLEND_NORMAL);
		bindCanvas();
		listen();
		frameTime = performance.now() / 1000;
	}

	// Where the corners' parts sit, the same in every program, so one mesh
	// feeds the drawing and a game's own shader alike.
	const POSITION = 0, TEXCOORD = 1, COLOR = 2;

	// The vertex shader for the drawing, which hands the fragment shader its
	// place in the picture and its color.
	const SHAPE_VERTEX = `#version 300 es
in vec2 position;
in vec2 texcoord;
in vec4 color;
uniform vec2 screen;
out vec2 vTexcoord;
out vec4 vColor;
void main() {
	vec2 clip = position / screen * 2.0 - 1.0;
	gl_Position = vec4(clip.x, -clip.y, 0.0, 1.0);
	vTexcoord = texcoord;
	vColor = color;
}`;

	const SHAPE_FRAGMENT = `#version 300 es
precision highp float;
in vec2 vTexcoord;
in vec4 vColor;
uniform sampler2D image;
out vec4 result;
void main() {
	result = texture(image, vTexcoord) * vColor;
}`;

	// The vertex shader a game's post-processing shader runs with. Its
	// outputs are named as raylib names them, because that is what the
	// shaders games write read.
	const POST_VERTEX = `#version 300 es
in vec2 position;
in vec2 texcoord;
in vec4 color;
uniform vec2 screen;
out vec2 fragTexCoord;
out vec4 fragColor;
void main() {
	vec2 clip = position / screen * 2.0 - 1.0;
	gl_Position = vec4(clip.x, -clip.y, 0.0, 1.0);
	fragTexCoord = texcoord;
	fragColor = color;
}`;

	function buildProgram() {
		const built = linkProgram(SHAPE_VERTEX, SHAPE_FRAGMENT);
		if (built.error) throw new Error('golib: ' + built.error);
		return built.program;
	}

	// linkProgram builds a program, or says what the graphics card said about
	// it, which is what a game with a shader that doesn't compile is told.
	function linkProgram(vertexSource, fragmentSource) {
		const vertex = compile(gl.VERTEX_SHADER, vertexSource);
		if (vertex.error) return { error: vertex.error };
		const fragment = compile(gl.FRAGMENT_SHADER, fragmentSource);
		if (fragment.error) return { error: fragment.error };
		const program = gl.createProgram();
		gl.attachShader(program, vertex.shader);
		gl.attachShader(program, fragment.shader);
		gl.bindAttribLocation(program, POSITION, 'position');
		gl.bindAttribLocation(program, TEXCOORD, 'texcoord');
		gl.bindAttribLocation(program, COLOR, 'color');
		gl.linkProgram(program);
		gl.deleteShader(vertex.shader);
		gl.deleteShader(fragment.shader);
		if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
			const log = gl.getProgramInfoLog(program);
			gl.deleteProgram(program);
			return { error: log };
		}
		return { program: program };
	}

	function compile(kind, source) {
		const shader = gl.createShader(kind);
		gl.shaderSource(shader, source);
		gl.compileShader(shader);
		if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
			const log = gl.getShaderInfoLog(shader);
			gl.deleteShader(shader);
			return { error: log };
		}
		return { shader: shader };
	}

	function setUpBatch() {
		vertices = new Float32Array(BATCH_VERTICES * FLOATS_PER_VERTEX);
		vertexArray = gl.createVertexArray();
		gl.bindVertexArray(vertexArray);
		vertexBuffer = gl.createBuffer();
		gl.bindBuffer(gl.ARRAY_BUFFER, vertexBuffer);
		gl.bufferData(gl.ARRAY_BUFFER, vertices.byteLength, gl.DYNAMIC_DRAW);
		const stride = FLOATS_PER_VERTEX * 4;
		bindAttribute(POSITION, 2, 0, stride);
		bindAttribute(TEXCOORD, 2, 8, stride);
		bindAttribute(COLOR, 4, 16, stride);
	}

	function bindAttribute(at, size, offset, stride) {
		gl.enableVertexAttribArray(at);
		gl.vertexAttribPointer(at, size, gl.FLOAT, false, stride, offset);
	}

	function clampAndFilter(filter) {
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, filter);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, filter);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
	}

	// resize keeps the drawing buffer the size of the canvas on the screen,
	// counting the pixels the display really has.
	function resize() {
		const scale = window.devicePixelRatio || 1;
		const width = Math.max(1, Math.round(canvas.clientWidth * scale));
		const height = Math.max(1, Math.round(canvas.clientHeight * scale));
		if (canvas.width !== width || canvas.height !== height) {
			canvas.width = width;
			canvas.height = height;
			if (boundTarget === 0) bindCanvas();
		}
	}

	// ------------------------------------------------------------- drawing

	function drawBuffer(size) {
		commandBytes = new Uint8Array(size);
		commandFloats = new Float32Array(commandBytes.buffer);
		return commandBytes;
	}

	// draw runs the commands package golib wrote, in order.
	function draw(count) {
		const c = commandFloats;
		let i = 0;
		while (i < count) {
			switch (c[i++]) {
				case BEGIN_FRAME:
					bindCanvas();
					break;
				case END_FRAME:
					flush();
					break;
				case CLEAR: {
					flush();
					gl.clearColor(c[i] / 255, c[i + 1] / 255, c[i + 2] / 255, c[i + 3] / 255);
					gl.clear(gl.COLOR_BUFFER_BIT);
					i += 4;
					break;
				}
				case RECTANGLE:
					rectangle(c[i], c[i + 1], c[i + 2], c[i + 3], c, i + 4);
					i += 8;
					break;
				case RECTANGLE_OUTLINE:
					rectangleOutline(c[i], c[i + 1], c[i + 2], c[i + 3], c[i + 4], c, i + 5);
					i += 9;
					break;
				case CIRCLE:
					circle(c[i], c[i + 1], c[i + 2], c, i + 3);
					i += 7;
					break;
				case RING:
					ring(c[i], c[i + 1], c[i + 2], c[i + 3], c, i + 4);
					i += 8;
					break;
				case LINE:
					line(c[i], c[i + 1], c[i + 2], c[i + 3], c[i + 4], c, i + 5);
					i += 9;
					break;
				case TRIANGLE:
					useTexture(0);
					triangle(c[i], c[i + 1], c[i + 2], c[i + 3], c[i + 4], c[i + 5], c, i + 6);
					i += 10;
					break;
				case TEXTURE:
					texture(c, i);
					i += 18;
					break;
				case BEGIN_TARGET:
					bindTarget(c[i]);
					i += 1;
					break;
				case END_TARGET:
					bindCanvas();
					break;
				case BEGIN_CAMERA:
					flush();
					camera = { offsetX: c[i], offsetY: c[i + 1], targetX: c[i + 2], targetY: c[i + 3], zoom: c[i + 4] };
					i += 5;
					break;
				case END_CAMERA:
					flush();
					camera = null;
					break;
				case BLEND:
					setBlend(c[i]);
					i += 1;
					break;
				case BEGIN_SHADER:
					beginShader(c[i]);
					i += 1;
					break;
				case END_SHADER:
					endShader();
					break;
				case SHADER_VALUES:
					setShaderValues(c[i], c[i + 1], c[i + 2], c[i + 3], c[i + 4], c[i + 5], c[i + 6]);
					i += 7;
					break;
				default:
					// A command this file doesn't know means the two halves
					// have drifted apart. Stopping says so loudly.
					throw new Error('golib: unknown drawing command ' + c[i - 1]);
			}
		}
	}

	// where moves a point through the camera, as raylib's Camera2D does.
	function whereX(x, y) {
		return camera ? (x - camera.targetX) * camera.zoom + camera.offsetX : x;
	}

	function whereY(x, y) {
		return camera ? (y - camera.targetY) * camera.zoom + camera.offsetY : y;
	}

	// vertex adds one corner to the batch.
	function vertex(x, y, u, v, c, at) {
		const i = used * FLOATS_PER_VERTEX;
		vertices[i] = whereX(x, y);
		vertices[i + 1] = whereY(x, y);
		vertices[i + 2] = u;
		vertices[i + 3] = v;
		vertices[i + 4] = c[at] / 255;
		vertices[i + 5] = c[at + 1] / 255;
		vertices[i + 6] = c[at + 2] / 255;
		vertices[i + 7] = c[at + 3] / 255;
		used++;
		if (used >= BATCH_VERTICES - 3) flush();
	}

	function triangle(x1, y1, x2, y2, x3, y3, c, at) {
		// The graphics card fills a triangle whatever way its corners turn,
		// so, unlike raylib, this needs no winding check.
		vertex(x1, y1, 0, 0, c, at);
		vertex(x2, y2, 0, 0, c, at);
		vertex(x3, y3, 0, 0, c, at);
	}

	function rectangle(x, y, width, height, c, at) {
		useTexture(0);
		triangle(x, y, x, y + height, x + width, y + height, c, at);
		triangle(x, y, x + width, y + height, x + width, y, c, at);
	}

	// rectangleOutline draws the four edges inside the rectangle, as raylib
	// does, thinning them when the rectangle is smaller than twice the edge.
	function rectangleOutline(x, y, width, height, thick, c, at) {
		if (thick > width / 2) thick = width / 2;
		if (thick > height / 2) thick = height / 2;
		rectangle(x, y, width, thick, c, at);
		rectangle(x, y - thick + height, width, thick, c, at);
		rectangle(x, y + thick, thick, height - thick * 2, c, at);
		rectangle(x - thick + width, y + thick, thick, height - thick * 2, c, at);
	}

	// circle draws raylib's 36 slices, so a circle has the same corners here.
	function circle(x, y, radius, c, at) {
		useTexture(0);
		const step = 360 / 36;
		for (let i = 0; i < 36; i++) {
			const a = i * step, b = a + step;
			triangle(x, y,
				x + Math.cos(b * DEG) * radius, y + Math.sin(b * DEG) * radius,
				x + Math.cos(a * DEG) * radius, y + Math.sin(a * DEG) * radius, c, at);
		}
	}

	// ring draws the space between two circles, with as many slices as raylib
	// picks for a circle that size.
	function ring(x, y, inner, outer, c, at) {
		useTexture(0);
		if (outer <= 0) return;
		if (inner > outer) { const swap = inner; inner = outer; outer = swap; }
		// raylib's own count, from how far a straight edge may stray.
		const th = Math.acos(2 * Math.pow(1 - 0.5 / outer, 2) - 1);
		let segments = Math.floor(360 * Math.ceil(2 * Math.PI / th) / 360);
		if (!isFinite(segments) || segments <= 0) segments = 4;
		const step = 360 / segments;
		for (let i = 0; i < segments; i++) {
			const a = i * step, b = a + step;
			const ca = Math.cos(a * DEG), sa = Math.sin(a * DEG);
			const cb = Math.cos(b * DEG), sb = Math.sin(b * DEG);
			triangle(x + ca * inner, y + sa * inner, x + ca * outer, y + sa * outer,
				x + cb * outer, y + sb * outer, c, at);
			triangle(x + ca * inner, y + sa * inner, x + cb * outer, y + sb * outer,
				x + cb * inner, y + sb * inner, c, at);
		}
	}

	// line draws a straight line as a quad, the way raylib's DrawLineEx does.
	function line(x1, y1, x2, y2, thick, c, at) {
		const dx = x2 - x1, dy = y2 - y1;
		const length = Math.sqrt(dx * dx + dy * dy);
		if (length <= 0 || thick <= 0) return;
		const scale = thick / (2 * length);
		const rx = -scale * dy, ry = scale * dx;
		useTexture(0);
		triangle(x1 - rx, y1 - ry, x1 + rx, y1 + ry, x2 - rx, y2 - ry, c, at);
		triangle(x1 + rx, y1 + ry, x2 + rx, y2 + ry, x2 - rx, y2 - ry, c, at);
	}

	// texture draws a part of a picture into a rectangle, turned around an
	// origin: the corners and the flipping are raylib's DrawTexturePro.
	function texture(c, i) {
		const id = c[i], width = c[i + 1], height = c[i + 2];
		let sx = c[i + 3], sy = c[i + 4], sw = c[i + 5], sh = c[i + 6];
		const dx = c[i + 7], dy = c[i + 8], dw = c[i + 9], dh = c[i + 10];
		const ox = c[i + 11], oy = c[i + 12], rotation = c[i + 13];
		const at = i + 14;
		if (width <= 0 || height <= 0) return;

		let flipX = false;
		if (sw < 0) { flipX = true; sw = -sw; }
		if (sh < 0) sy -= sh;

		let u0 = sx / width, u1 = (sx + sw) / width;
		if (flipX) { const swap = u0; u0 = u1; u1 = swap; }
		const v0 = sy / height, v1 = (sy + sh) / height;

		let ax, ay, bx, by, cx, cy, ex, ey; // the four corners, clockwise
		if (rotation === 0) {
			const x = dx - ox, y = dy - oy;
			ax = x; ay = y;
			bx = x; by = y + dh;
			cx = x + dw; cy = y + dh;
			ex = x + dw; ey = y;
		} else {
			const s = Math.sin(rotation * DEG), k = Math.cos(rotation * DEG);
			const nx = -ox, ny = -oy;
			ax = dx + nx * k - ny * s; ay = dy + nx * s + ny * k;
			bx = dx + nx * k - (ny + dh) * s; by = dy + nx * s + (ny + dh) * k;
			cx = dx + (nx + dw) * k - (ny + dh) * s; cy = dy + (nx + dw) * s + (ny + dh) * k;
			ex = dx + (nx + dw) * k - ny * s; ey = dy + (nx + dw) * s + ny * k;
		}
		useTexture(id);
		vertex(ax, ay, u0, v0, c, at);
		vertex(bx, by, u0, v1, c, at);
		vertex(cx, cy, u1, v1, c, at);
		vertex(ax, ay, u0, v0, c, at);
		vertex(cx, cy, u1, v1, c, at);
		vertex(ex, ey, u1, v0, c, at);
	}

	// useTexture picks the picture the next corners are cut from; 0 is the
	// single white pixel that shapes use.
	function useTexture(id) {
		if (boundTexture === id) return;
		flush();
		boundTexture = id;
		gl.bindTexture(gl.TEXTURE_2D, id === 0 ? white : textures.get(id));
	}

	function setBlend(mode) {
		if (boundBlend === mode) return;
		flush();
		boundBlend = mode;
		gl.blendEquation(gl.FUNC_ADD);
		if (mode === BLEND_ADD) gl.blendFunc(gl.SRC_ALPHA, gl.ONE);
		else if (mode === BLEND_COPY) gl.blendFunc(gl.ONE, gl.ZERO);
		else gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);
	}

	function bindCanvas() {
		if (boundTarget === 0 && viewWidth === canvas.width && viewHeight === canvas.height) return;
		flush();
		boundTarget = 0;
		gl.bindFramebuffer(gl.FRAMEBUFFER, null);
		setView(canvas.width, canvas.height);
	}

	function bindTarget(id) {
		if (boundTarget === id) return;
		flush();
		const target = targets.get(id);
		if (!target) return;
		boundTarget = id;
		gl.bindFramebuffer(gl.FRAMEBUFFER, target.framebuffer);
		setView(target.width, target.height);
	}

	function setView(width, height) {
		viewWidth = width;
		viewHeight = height;
		gl.viewport(0, 0, width, height);
		useCurrentProgram();
	}

	// A game's post-processing shader is a program of its own, with GoLib's
	// vertex shader and the game's fragment shader. Between BEGIN_SHADER and
	// END_SHADER the picture is drawn through it instead of the plain one.
	const shaders = new Map(); // id -> { program, screen, uniforms, byName }
	let boundShader = 0;

	function newShader(source) {
		const built = linkProgram(POST_VERTEX, source);
		if (built.error) return [-1, (built.error || 'the graphics card gave no reason').trim()];
		const id = nextID++;
		gl.useProgram(built.program);
		const image = gl.getUniformLocation(built.program, 'texture0');
		if (image) gl.uniform1i(image, 0);
		shaders.set(id, {
			program: built.program,
			screen: gl.getUniformLocation(built.program, 'screen'),
			uniforms: [],
			byName: new Map(),
		});
		useCurrentProgram();
		return [id, ''];
	}

	function unloadShader(id) {
		const shader = shaders.get(id);
		if (!shader) return;
		if (boundShader === id) { boundShader = 0; useCurrentProgram(); }
		gl.deleteProgram(shader.program);
		shaders.delete(id);
	}

	function beginShader(id) {
		if (boundShader === id) return;
		flush();
		boundShader = shaders.has(id) ? id : 0;
		useCurrentProgram();
	}

	function endShader() {
		if (boundShader === 0) return;
		flush();
		boundShader = 0;
		useCurrentProgram();
	}

	// useCurrentProgram picks the program the next corners go through, and
	// tells it how large what it draws on is.
	function useCurrentProgram() {
		const shader = shaders.get(boundShader);
		if (shader) {
			gl.useProgram(shader.program);
			if (shader.screen) gl.uniform2f(shader.screen, viewWidth, viewHeight);
			return;
		}
		gl.useProgram(program);
		gl.uniform2f(screenLocation, viewWidth, viewHeight);
	}

	// shaderLocation returns where a uniform sits in a shader, as a number
	// package golib can keep, or -1 when the shader doesn't declare it: GLSL
	// compilers drop the ones a shader doesn't use.
	function shaderLocation(id, name) {
		const shader = shaders.get(id);
		if (!shader) return -1;
		if (shader.byName.has(name)) return shader.byName.get(name);
		const found = gl.getUniformLocation(shader.program, name);
		let at = -1;
		if (found) {
			at = shader.uniforms.length;
			shader.uniforms.push(found);
		}
		shader.byName.set(name, at);
		return at;
	}

	function setShaderValues(id, at, count, a, b, c, d) {
		const shader = shaders.get(id);
		if (!shader || at < 0 || at >= shader.uniforms.length) return;
		const where = shader.uniforms[at];
		if (count === 1) gl.uniform1f(where, a);
		else if (count === 2) gl.uniform2f(where, a, b);
		else if (count === 3) gl.uniform3f(where, a, b, c);
		else if (count === 4) gl.uniform4f(where, a, b, c, d);
	}

	// flush sends the corners built so far to the graphics card.
	function flush() {
		if (used === 0) return;
		gl.bufferSubData(gl.ARRAY_BUFFER, 0, vertices, 0, used * FLOATS_PER_VERTEX);
		gl.drawArrays(gl.TRIANGLES, 0, used);
		used = 0;
	}

	// -------------------------------------------------------------- pictures

	function newTexture(width, height, pixels) {
		const id = nextID++;
		const made = gl.createTexture();
		gl.bindTexture(gl.TEXTURE_2D, made);
		gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, width, height, 0, gl.RGBA, gl.UNSIGNED_BYTE, pixels);
		clampAndFilter(gl.NEAREST); // raylib's own default for a new picture
		textures.set(id, made);
		boundTexture = -1;
		return id;
	}

	function unloadTexture(id) {
		const made = textures.get(id);
		if (!made) return;
		gl.deleteTexture(made);
		textures.delete(id);
		if (boundTexture === id) boundTexture = -1;
	}

	function newTarget(width, height, smooth) {
		const textureID = nextID++, id = nextID++;
		const made = gl.createTexture();
		gl.bindTexture(gl.TEXTURE_2D, made);
		gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, width, height, 0, gl.RGBA, gl.UNSIGNED_BYTE, null);
		clampAndFilter(smooth ? gl.LINEAR : gl.NEAREST);
		textures.set(textureID, made);
		const framebuffer = gl.createFramebuffer();
		gl.bindFramebuffer(gl.FRAMEBUFFER, framebuffer);
		gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, made, 0);
		gl.bindFramebuffer(gl.FRAMEBUFFER, null);
		targets.set(id, { framebuffer: framebuffer, texture: made, textureID: textureID, width: width, height: height });
		boundTexture = -1;
		boundTarget = -1;
		bindCanvas();
		return [id, textureID];
	}

	function unloadTarget(id) {
		const target = targets.get(id);
		if (!target) return;
		gl.deleteFramebuffer(target.framebuffer);
		gl.deleteTexture(target.texture);
		textures.delete(target.textureID);
		targets.delete(id);
		if (boundTarget === id) { boundTarget = -1; bindCanvas(); }
	}

	// readTarget copies what was drawn into a target back into pixels, the
	// bottom row first, as the graphics card holds it.
	function readTarget(id, width, height, pixels) {
		const target = targets.get(id);
		if (!target) return;
		flush();
		gl.bindFramebuffer(gl.FRAMEBUFFER, target.framebuffer);
		gl.readPixels(0, 0, width, height, gl.RGBA, gl.UNSIGNED_BYTE, pixels);
		gl.bindFramebuffer(gl.FRAMEBUFFER, null);
		boundTarget = -1;
		bindCanvas();
	}

	// ----------------------------------------------------------------- input

	// The keys GoLib names, by what the browser calls them. The numbers are
	// GLFW's, which device.go fixes for every backend.
	const KEYS = {
		Space: 32, Enter: 257, NumpadEnter: 257, Escape: 256, Tab: 258, Backspace: 259,
		ArrowLeft: 263, ArrowRight: 262, ArrowUp: 265, ArrowDown: 264,
		ShiftLeft: 340, ShiftRight: 344, ControlLeft: 341, ControlRight: 345,
		AltLeft: 342, AltRight: 346,
	};
	for (let i = 0; i < 26; i++) KEYS['Key' + String.fromCharCode(65 + i)] = 65 + i;
	for (let i = 0; i < 10; i++) { KEYS['Digit' + i] = 48 + i; KEYS['Numpad' + i] = 48 + i; }
	for (let i = 1; i <= 12; i++) KEYS['F' + i] = 289 + i;

	// The browser's gamepad buttons, in GoLib's order. -1 is a button GoLib
	// doesn't name.
	const PAD_BUTTONS = [7, 6, 8, 5, 9, 11, 10, 12, 13, 15, 16, 17, 1, 3, 4, 2, 14];
	const PAD_BUTTON_COUNT = 18, MAX_PADS = 4;

	const keysDown = new Uint8Array(349), keysPressed = new Uint8Array(349);
	const mouseDown = new Uint8Array(3), mousePressed = new Uint8Array(3);
	let mouseX = 0, mouseY = 0, wheel = 0;
	const padWas = []; // what each pad's buttons were last frame, for presses

	let inputBytes = null, inputFloats = null;

	function inputBuffer(size) {
		inputBytes = new Uint8Array(size);
		inputFloats = new Float32Array(inputBytes.buffer, 0, 19);
		return inputBytes;
	}

	function listen() {
		window.addEventListener('keydown', function (e) {
			const key = KEYS[e.code];
			if (key !== undefined) {
				if (!keysDown[key]) keysPressed[key] = 1;
				keysDown[key] = 1;
				// Arrows, space and tab would otherwise scroll the page or
				// move the focus out of the game.
				if (key === 32 || key === 258 || (key >= 262 && key <= 265)) e.preventDefault();
			}
			takeFullscreen();
			wakeAudio();
		});
		window.addEventListener('keyup', function (e) {
			const key = KEYS[e.code];
			if (key !== undefined) keysDown[key] = 0;
		});
		window.addEventListener('blur', function () {
			// A game that never sees the key go up would walk on for ever.
			keysDown.fill(0);
			mouseDown.fill(0);
		});
		canvas.addEventListener('mousemove', function (e) {
			const box = canvas.getBoundingClientRect();
			const scale = canvas.width / Math.max(1, box.width);
			mouseX = (e.clientX - box.left) * scale;
			mouseY = (e.clientY - box.top) * (canvas.height / Math.max(1, box.height));
		});
		canvas.addEventListener('mousedown', function (e) {
			const button = domButton(e.button);
			if (button >= 0) {
				if (!mouseDown[button]) mousePressed[button] = 1;
				mouseDown[button] = 1;
			}
			takeFullscreen();
			wakeAudio();
			e.preventDefault();
		});
		window.addEventListener('mouseup', function (e) {
			const button = domButton(e.button);
			if (button >= 0) mouseDown[button] = 0;
		});
		canvas.addEventListener('wheel', function (e) {
			wheel += e.deltaY > 0 ? -1 : (e.deltaY < 0 ? 1 : 0);
			e.preventDefault();
		}, { passive: false });
		canvas.addEventListener('contextmenu', function (e) { e.preventDefault(); });
	}

	// domButton turns the browser's button number into GoLib's.
	function domButton(button) {
		if (button === 0) return 0; // left
		if (button === 1) return 2; // middle
		if (button === 2) return 1; // right
		return -1;
	}

	// snapshotInput writes the state the next frame reads and clears the
	// presses, so each press reaches exactly one frame.
	function snapshotInput() {
		if (!inputBytes) return;
		inputFloats[0] = mouseX;
		inputFloats[1] = mouseY;
		inputFloats[2] = wheel;
		wheel = 0;

		const keysAt = 76; // where the bytes start, as web_input.go says
		inputBytes.set(keysDown, keysAt);
		inputBytes.set(keysPressed, keysAt + 349);
		inputBytes.set(mouseDown, keysAt + 698);
		inputBytes.set(mousePressed, keysAt + 701);
		keysPressed.fill(0);
		mousePressed.fill(0);

		const connectedAt = keysAt + 704;
		const downAt = connectedAt + MAX_PADS;
		const pressedAt = downAt + MAX_PADS * PAD_BUTTON_COUNT;
		const pads = navigator.getGamepads ? navigator.getGamepads() : [];
		for (let pad = 0; pad < MAX_PADS; pad++) {
			const found = pads[pad];
			inputBytes[connectedAt + pad] = found ? 1 : 0;
			if (!padWas[pad]) padWas[pad] = new Uint8Array(PAD_BUTTON_COUNT);
			const was = padWas[pad];
			for (let button = 0; button < PAD_BUTTON_COUNT; button++) {
				inputBytes[downAt + pad * PAD_BUTTON_COUNT + button] = 0;
				inputBytes[pressedAt + pad * PAD_BUTTON_COUNT + button] = 0;
			}
			if (!found) { was.fill(0); continue; }
			for (let i = 0; i < PAD_BUTTONS.length && i < found.buttons.length; i++) {
				const button = PAD_BUTTONS[i];
				if (button < 0 || button >= PAD_BUTTON_COUNT) continue;
				const down = found.buttons[i].pressed ? 1 : 0;
				inputBytes[downAt + pad * PAD_BUTTON_COUNT + button] = down;
				if (down && !was[button]) inputBytes[pressedAt + pad * PAD_BUTTON_COUNT + button] = 1;
				was[button] = down;
			}
			const sticks = 3 + pad * 4;
			inputFloats[sticks] = found.axes[0] || 0;
			inputFloats[sticks + 1] = found.axes[1] || 0;
			inputFloats[sticks + 2] = found.axes[2] || 0;
			inputFloats[sticks + 3] = found.axes[3] || 0;
		}
	}

	function gamepadName(pad) {
		const pads = navigator.getGamepads ? navigator.getGamepads() : [];
		return pads[pad] ? pads[pad].id : '';
	}

	// ------------------------------------------------------------------ text

	// A font from a .ttf or .otf file is read by the browser, which then draws
	// its letters onto a canvas that becomes one picture, the way raylib
	// draws them onto an atlas. The browser shapes letters a little
	// differently from raylib, so the text says the same thing in the same
	// place, give or take a pixel.
	const FONT_PAD = 2;      // space around each letter, so they don't bleed
	const FONT_ATLAS = 1024; // how wide the picture may be
	let nextFamily = 1;

	// readFont reads a font file and calls done with the letters drawn, or
	// with a reason it couldn't. The game waits for it, so done is always
	// called.
	function readFont(file, size, letters, done) {
		let face;
		try {
			const data = file.buffer.slice(file.byteOffset, file.byteOffset + file.byteLength);
			face = new FontFace('golibfont' + (nextFamily++), data);
		} catch (e) {
			done({ error: String(e && e.message ? e.message : e) });
			return;
		}
		face.load().then(function (loaded) {
			document.fonts.add(loaded);
			try {
				done(drawLetters(loaded.family, size, letters));
			} catch (e) {
				done({ error: String(e && e.message ? e.message : e) });
			}
		}).catch(function (e) {
			done({ error: String(e && e.message ? e.message : e) });
		});
	}

	// drawLetters draws every letter once onto a picture, and says where each
	// one sits and how far it moves the pen, as raylib's font does.
	function drawLetters(family, size, letters) {
		const measuring = document.createElement('canvas').getContext('2d');

		const seen = new Set();
		const wanted = [];
		for (const letter of letters) {
			if (!seen.has(letter)) { seen.add(letter); wanted.push(letter); }
		}

		// A browser reads a size as the height of the font's em square, while
		// raylib reads it as the height from the top of the letters to the
		// bottom of the tails. Asking the browser for the size that makes
		// those two agree puts the letters where raylib puts them.
		let asked = size;
		measuring.font = asked + 'px "' + family + '"';
		let line = measuring.measureText('Hy');
		const tall = (line.fontBoundingBoxAscent || 0) + (line.fontBoundingBoxDescent || 0);
		if (tall > 0) {
			asked = size * size / tall;
			measuring.font = asked + 'px "' + family + '"';
			line = measuring.measureText('Hy');
		}
		const font = measuring.font;

		// Where the letters sit on their line, which is where a letter's
		// offset is measured from.
		let ascent = Math.round(line.fontBoundingBoxAscent || 0);
		const measured = [];
		for (const letter of wanted) {
			const m = measuring.measureText(letter);
			const left = Math.ceil(m.actualBoundingBoxLeft || 0);
			const right = Math.ceil(m.actualBoundingBoxRight || 0);
			const up = Math.ceil(m.actualBoundingBoxAscent || 0);
			const down = Math.ceil(m.actualBoundingBoxDescent || 0);
			if (!ascent) ascent = Math.max(ascent, up);
			measured.push({
				letter: letter,
				width: Math.max(0, left + right),
				height: Math.max(0, up + down),
				left: left, up: up,
				advance: Math.round(m.width),
			});
		}
		if (!ascent) ascent = Math.round(size * 0.75);

		// Lay them out in rows, and make the picture as tall as they need.
		let x = FONT_PAD, y = FONT_PAD, row = 0, width = FONT_PAD;
		for (const glyph of measured) {
			if (x + glyph.width + FONT_PAD > FONT_ATLAS) {
				x = FONT_PAD;
				y += row + FONT_PAD;
				row = 0;
			}
			glyph.x = x;
			glyph.y = y;
			x += glyph.width + FONT_PAD;
			width = Math.max(width, x);
			row = Math.max(row, glyph.height);
		}
		const height = y + row + FONT_PAD;

		const canvas = document.createElement('canvas');
		canvas.width = Math.max(1, width);
		canvas.height = Math.max(1, height);
		const ctx = canvas.getContext('2d', { willReadFrequently: true });
		ctx.font = font;
		ctx.textBaseline = 'alphabetic';
		ctx.fillStyle = '#ffffff';
		const glyphs = [];
		for (const glyph of measured) {
			if (glyph.width > 0 && glyph.height > 0) {
				ctx.fillText(glyph.letter, glyph.x + glyph.left, glyph.y + glyph.up);
			}
			glyphs.push(
				glyph.letter.codePointAt(0), glyph.x, glyph.y, glyph.width, glyph.height,
				-glyph.left, ascent - glyph.up, glyph.advance);
		}
		const picture = ctx.getImageData(0, 0, canvas.width, canvas.height);
		return {
			width: canvas.width,
			height: canvas.height,
			pixels: new Uint8Array(picture.data.buffer),
			glyphs: glyphs,
		};
	}

	// ----------------------------------------------------------------- sound

	// Web Audio. Package golib hands over the bytes of a sound file, the
	// browser decodes them, and every play is a fresh source node, which is
	// how a sound overlaps itself. A voice is one of the copies package golib
	// keeps of a sound, with its own volume and pitch.
	let audio = null, master = null;
	const decoded = new Map(); // id -> AudioBuffer, waiting to become a sound
	const voices = new Map();  // id -> { buffer, volume, pitch, node, gain }
	const musics = new Map();  // id -> a sound that plays on, and may loop
	let nextSoundID = 1;

	function openAudio() {
		if (audio) return true;
		const Context = window.AudioContext || window.webkitAudioContext;
		if (!Context) return false;
		audio = new Context();
		master = audio.createGain();
		master.gain.value = 1;
		master.connect(audio.destination);
		return true;
	}

	// wakeAudio starts the sound device at the first key or click. Browsers
	// keep it asleep until then, whatever the game asks for.
	function wakeAudio() {
		if (audio && audio.state === 'suspended') audio.resume().catch(function () { });
	}

	function closeAudio() {
		voices.forEach(function (voice, id) { stopSound(id); });
		musics.forEach(stopMusicNode);
		voices.clear();
		musics.clear();
		decoded.clear();
		if (audio) {
			audio.close().catch(function () { });
			audio = null;
			master = null;
		}
	}

	function setMasterVolume(volume) {
		if (master) master.gain.value = volume;
	}

	// decodeSound hands a sound file to the browser and calls done with the
	// number of the decoded sound, or -1 when the browser can't read it: .xm,
	// .mod and .qoa are formats it has never heard of. The game waits for
	// this, so done is always called.
	function decodeSound(bytes, done) {
		if (!audio) { done(-1); return; }
		// decodeAudioData empties the buffer it is given, so it gets a copy.
		const data = bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength);
		audio.decodeAudioData(data, function (buffer) {
			const id = nextSoundID++;
			decoded.set(id, buffer);
			done(id);
		}, function () { done(-1); });
	}

	function unloadWave(id) { decoded.delete(id); }

	function newSound(waveID) {
		const buffer = decoded.get(waveID);
		if (!buffer) return [-1, 0];
		return [newVoice(buffer), buffer.length];
	}

	function newSoundAlias(soundID) {
		const voice = voices.get(soundID);
		if (!voice) return [-1, 0];
		return [newVoice(voice.buffer), voice.buffer.length];
	}

	function newVoice(buffer) {
		const id = nextSoundID++;
		voices.set(id, { buffer: buffer, volume: 1, pitch: 1, node: null, gain: null });
		return id;
	}

	function unloadSound(id) {
		stopSound(id);
		voices.delete(id);
	}

	// playSound plays a voice from its start, over whatever it was playing,
	// as raylib does.
	function playSound(id) {
		const voice = voices.get(id);
		if (!voice || !audio) return;
		stopSound(id);
		const node = audio.createBufferSource();
		node.buffer = voice.buffer;
		node.playbackRate.value = voice.pitch;
		const gain = audio.createGain();
		gain.gain.value = voice.volume;
		node.connect(gain);
		gain.connect(master);
		node.onended = function () {
			if (voice.node === node) { voice.node = null; voice.gain = null; }
		};
		node.start();
		voice.node = node;
		voice.gain = gain;
	}

	function stopSound(id) {
		const voice = voices.get(id);
		if (!voice || !voice.node) return;
		try { voice.node.stop(); } catch (e) { /* it had already ended */ }
		voice.node = null;
		voice.gain = null;
	}

	function setSoundVolume(id, volume) {
		const voice = voices.get(id);
		if (!voice) return;
		voice.volume = volume;
		if (voice.gain) voice.gain.gain.value = volume;
	}

	function setSoundPitch(id, pitch) {
		const voice = voices.get(id);
		if (!voice) return;
		voice.pitch = pitch;
		if (voice.node) voice.node.playbackRate.value = pitch;
	}

	function soundPlaying(id) {
		const voice = voices.get(id);
		return !!(voice && voice.node);
	}

	// A music is a decoded sound that plays on its own, and loops without a
	// gap when asked to: package golib loops a sound effect this way too.
	function newMusic(waveID, looping) {
		const buffer = decoded.get(waveID);
		if (!buffer) return -1;
		decoded.delete(waveID); // the music holds it now
		const id = nextSoundID++;
		musics.set(id, {
			buffer: buffer, volume: 1, looping: !!looping,
			node: null, gain: null, offset: 0, startedAt: 0, playing: false,
		});
		return id;
	}

	function playMusic(id) {
		const music = musics.get(id);
		if (!music || !audio) return;
		stopMusicNode(music);
		music.offset = 0;
		startMusic(music);
	}

	function startMusic(music) {
		const node = audio.createBufferSource();
		node.buffer = music.buffer;
		node.loop = music.looping;
		const gain = audio.createGain();
		gain.gain.value = music.volume;
		node.connect(gain);
		gain.connect(master);
		node.onended = function () {
			if (music.node === node && !music.looping) {
				music.node = null;
				music.playing = false;
			}
		};
		const length = Math.max(music.buffer.duration, 0.001);
		node.start(0, music.offset % length);
		music.node = node;
		music.gain = gain;
		music.startedAt = audio.currentTime;
		music.playing = true;
	}

	function stopMusicNode(music) {
		if (!music.node) return;
		try { music.node.stop(); } catch (e) { /* it had already ended */ }
		music.node = null;
		music.gain = null;
	}

	// pauseMusic remembers how far the music got, because a source node can
	// only be started and stopped, never held.
	function pauseMusic(id) {
		const music = musics.get(id);
		if (!music || !music.playing) return;
		music.offset += audio.currentTime - music.startedAt;
		stopMusicNode(music);
		music.playing = false;
	}

	function resumeMusic(id) {
		const music = musics.get(id);
		if (!music || music.playing || !audio) return;
		startMusic(music);
	}

	function stopMusic(id) {
		const music = musics.get(id);
		if (!music) return;
		stopMusicNode(music);
		music.offset = 0;
		music.playing = false;
	}

	function setMusicVolume(id, volume) {
		const music = musics.get(id);
		if (!music) return;
		music.volume = volume;
		if (music.gain) music.gain.gain.value = volume;
	}

	function musicPlaying(id) {
		const music = musics.get(id);
		return !!(music && music.playing);
	}

	function unloadMusic(id) {
		const music = musics.get(id);
		if (music) stopMusicNode(music);
		musics.delete(id);
	}

	// ----------------------------------------------------------------- frames

	function onFrame(fn) { wake = fn; }

	function askForFrame() {
		requestAnimationFrame(function (stamp) {
			frameTime = stamp / 1000;
			resize();
			if (wake) wake();
		});
	}

	function frame() {
		return [frameTime, canvas ? canvas.width : 0, canvas ? canvas.height : 0, document.hasFocus(), closed];
	}

	// takeFullscreen makes the request golib.SetFullscreen asked for. A
	// browser only allows it while it is handling a key or a click, which is
	// why it waits here for one.
	function takeFullscreen() {
		if (wantFullscreen === fullscreenAsked) return;
		fullscreenAsked = wantFullscreen;
		if (wantFullscreen) {
			if (canvas.requestFullscreen) canvas.requestFullscreen().catch(function () { });
		} else if (document.exitFullscreen && document.fullscreenElement) {
			document.exitFullscreen().catch(function () { });
		}
	}

	function setFullscreen(on) { wantFullscreen = !!on; }

	function setCursorVisible(visible) {
		cursorShown = !!visible;
		if (canvas) canvas.style.cursor = cursorShown ? '' : 'none';
	}

	function cursorVisible() { return cursorShown; }

	function close() { closed = true; }

	// showError puts a message over the game. A player in a browser never
	// opens its console, so a game that stops says why here.
	function showError(title, message) {
		let box = document.getElementById('message');
		if (!box) {
			box = document.createElement('div');
			box.id = 'message';
			document.body.appendChild(box);
		}
		box.textContent = message;
		if (title) document.title = title;
		console.error(title + ': ' + message);
	}

	return {
		open: open, close: close, frame: frame,
		draw: draw, drawBuffer: drawBuffer,
		newTexture: newTexture, unloadTexture: unloadTexture,
		newTarget: newTarget, unloadTarget: unloadTarget, readTarget: readTarget,
		inputBuffer: inputBuffer, snapshotInput: snapshotInput, gamepadName: gamepadName,
		onFrame: onFrame, askForFrame: askForFrame,
		setFullscreen: setFullscreen, setCursorVisible: setCursorVisible, cursorVisible: cursorVisible,
		showError: showError,
		newShader: newShader, unloadShader: unloadShader, shaderLocation: shaderLocation,
		readFont: readFont,
		openAudio: openAudio, closeAudio: closeAudio, setMasterVolume: setMasterVolume,
		decodeSound: decodeSound, unloadWave: unloadWave,
		newSound: newSound, newSoundAlias: newSoundAlias, unloadSound: unloadSound,
		playSound: playSound, stopSound: stopSound, soundPlaying: soundPlaying,
		setSoundVolume: setSoundVolume, setSoundPitch: setSoundPitch,
		newMusic: newMusic, playMusic: playMusic, pauseMusic: pauseMusic,
		resumeMusic: resumeMusic, stopMusic: stopMusic, unloadMusic: unloadMusic,
		setMusicVolume: setMusicVolume, musicPlaying: musicPlaying,
	};
})();
