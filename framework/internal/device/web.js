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
		END_SHADER = 15, SHADER_VALUES = 16, BEGIN_FRAME = 17, END_FRAME = 18,
		DRAW_SIZE = 19;

	// The 3D commands, as web_gpu.go writes them.
	const BEGIN_3D = 20, END_3D = 21, USE_PROGRAM = 22, UNIFORM_MATRIX = 23,
		UNIFORM = 24, UNIFORM_INT = 25, DEPTH_WRITE = 26, BLEND_3D = 27,
		CULLING = 28, DRAW_MESH = 29, DRAW_INSTANCED = 30, BIND_TEXTURE_3D = 31,
		DRAW_FULLSCREEN = 32, UNIFORM_VEC4S = 33, BEGIN_SHADOW = 34, END_SHADOW = 35,
		BIND_SHADOW_MAP = 36, BEGIN_WORK = 37, END_WORK = 38, BIND_WORK_TARGET = 39,
		BIND_PASS_PICTURE = 40, ADDITIVE_3D = 41, UPDATE_DATA = 42, BIND_DATA = 43,
		COLOR_WRITE = 44;

	// The vertex layouts of 3D meshes, as gpu.go defines them, and how many
	// floats an instance of a lit mesh takes: its matrix and its tint.
	const LAYOUT_LIT = 0, LAYOUT_COLORED = 1, LAYOUT_SKINNED = 2, LAYOUT_LIGHTMAPPED = 3, INSTANCE_FLOATS = 20;

	// How many texels a row of a data texture holds, as gpu.go says.
	const DATA_WIDTH = 1024;

	const BLEND_NORMAL = 0, BLEND_ADD = 1, BLEND_COPY = 2, BLEND_PREMULTIPLIED = 3;

	// How many vertices a batch holds before it goes to the graphics card.
	const BATCH_VERTICES = 24576;
	const FLOATS_PER_VERTEX = 8; // x, y, u, v, r, g, b, a

	const DEG = Math.PI / 180;

	let canvas = null, gl = null, program = null, screenLocation = null;
	let vertices = null, vertexBuffer = null, vertexArray = null, used = 0;
	let white = null; // a single white pixel, for shapes

	// What is being drawn on now.
	// boundBlend starts as no mode at all, so that the first setBlend really
	// sets it: a graphics card starts by copying what is drawn instead of
	// blending it, which would show the transparent parts of a letter as
	// boxes until something changed the mode.
	let boundTarget = 0, boundTexture = 0, boundBlend = -1;
	let viewWidth = 0, viewHeight = 0;
	// The area drawing measures in: the view's own pixels, unless DRAW_SIZE
	// stretches a smaller one over it, as Config.Antialias does.
	let drawWidth = 0, drawHeight = 0;

	// The camera, as an offset and a zoom: BeginCamera sets it, and every
	// point is moved by it as it is added to the batch.
	let camera = null;

	const textures = new Map(); // id -> WebGLTexture
	const targets = new Map();  // id -> { framebuffer, texture, textureID, width, height }
	let nextID = 1;

	let frameTime = 0, wake = null, closed = false;
	// frameGap is the seconds at least between two frames the game draws, as
	// setFrameRate asks; 0 draws at every frame the browser offers.
	let frameGap = 0, lastWake = -1;
	let wantFullscreen = false, fullscreenAsked = false;
	let fullscreenTarget = null; // the element the game asked to show fullscreen
	let fullscreenHeld = false;  // the browser granted that ask
	let fullscreenLeft = false;  // and the player has left it since
	let sizedBox = false;        // the canvas is the size this file gave it
	let cursorShown = true;

	let commandBytes = null, commandFloats = null;

	// ---------------------------------------------------------------- setup

	function open(width, height, title) {
		if (title) document.title = title;
		canvas = document.getElementById('game');
		if (!canvas) throw new Error('golib: the page has no <canvas id="game">');
		// A canvas holds the keyboard focus only once it can be focused at
		// all; -1 keeps it out of the page's tab order, where Tab is the
		// game's key. A page that sets tabindex itself is left alone.
		if (!canvas.hasAttribute('tabindex')) canvas.tabIndex = -1;
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
		takeFocus();
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
	//
	// While the game itself asked for fullscreen, the canvas is measured
	// against the screen and given that size in CSS pixels, instead of being
	// measured against its own box: a phone turned from portrait to landscape
	// leaves a fullscreen element the shape it had until the browser lays the
	// page out again, and the game would sit in a corner of the screen meanwhile
	// instead of filling it. A page that put the game fullscreen by itself, as
	// itch.io's own button does, is left alone: the canvas's box there is the
	// page's to decide, and it already follows the screen.
	function resize() {
		const scale = window.devicePixelRatio || 1;
		let boxWidth = canvas.clientWidth, boxHeight = canvas.clientHeight;
		if (ownFullscreen()) {
			boxWidth = window.innerWidth || boxWidth;
			boxHeight = window.innerHeight || boxHeight;
			sizeCanvasBox(boxWidth, boxHeight);
		} else if (sizedBox) {
			sizeCanvasBox(0, 0); // back out of the page, which sizes it again
			boxWidth = canvas.clientWidth;
			boxHeight = canvas.clientHeight;
		}
		const width = Math.max(1, Math.round(boxWidth * scale));
		const height = Math.max(1, Math.round(boxHeight * scale));
		if (canvas.width !== width || canvas.height !== height) {
			canvas.width = width;
			canvas.height = height;
			if (boundTarget === 0) bindCanvas();
		}
	}

	// sizeCanvasBox says how large the canvas is on the page, in CSS pixels. A
	// width or height of 0 takes that back, so the page's own stylesheet sizes
	// it again.
	function sizeCanvasBox(width, height) {
		if (width > 0 && height > 0) {
			canvas.style.width = width + 'px';
			canvas.style.height = height + 'px';
			sizedBox = true;
			return;
		}
		canvas.style.width = '';
		canvas.style.height = '';
		sizedBox = false;
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
				case DRAW_SIZE:
					flush();
					drawWidth = c[i];
					drawHeight = c[i + 1];
					useCurrentProgram();
					i += 2;
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
				case BEGIN_3D:
					begin3D();
					break;
				case END_3D:
					end3D(c[i]);
					i += 1;
					break;
				case UNIFORM_VEC4S: {
					const count = c[i + 1];
					gl.uniform4fv(locations3D[c[i]], c, i + 2, count * 4);
					i += 2 + count * 4;
					break;
				}
				case BEGIN_SHADOW: {
					const map = shadowMaps.get(c[i]);
					if (map) {
						gl.bindFramebuffer(gl.FRAMEBUFFER, map.framebuffer);
						gl.viewport(c[i + 1], c[i + 2], c[i + 3], c[i + 4]);
						gl.depthMask(true);
						if (c[i + 5] !== 0) gl.clearBufferfv(gl.DEPTH, 0, [1]);
					}
					i += 6;
					break;
				}
				case END_SHADOW:
					gl.bindFramebuffer(gl.FRAMEBUFFER, pass3D.framebuffer);
					gl.viewport(0, 0, pass3D.width, pass3D.height);
					break;
				case BIND_SHADOW_MAP: {
					const map = shadowMaps.get(c[i + 1]);
					gl.activeTexture(gl.TEXTURE0 + c[i]);
					gl.bindTexture(gl.TEXTURE_2D, map ? map.texture : null);
					i += 2;
					break;
				}
				case BEGIN_WORK: {
					const work = workTargets.get(c[i]);
					if (work) {
						gl.bindFramebuffer(gl.FRAMEBUFFER, work.framebuffer);
						gl.viewport(0, 0, work.width, work.height);
						gl.disable(gl.DEPTH_TEST);
						if (c[i + 1] !== 0) gl.clearBufferfv(gl.COLOR, 0, [0, 0, 0, 0]);
					}
					i += 2;
					break;
				}
				case END_WORK:
					gl.bindFramebuffer(gl.FRAMEBUFFER, pass3D.framebuffer);
					gl.viewport(0, 0, pass3D.width, pass3D.height);
					gl.enable(gl.DEPTH_TEST);
					break;
				case BIND_WORK_TARGET: {
					const work = workTargets.get(c[i + 1]);
					gl.activeTexture(gl.TEXTURE0 + c[i]);
					gl.bindTexture(gl.TEXTURE_2D, work ? work.texture : null);
					i += 2;
					break;
				}
				case BIND_PASS_PICTURE:
					gl.activeTexture(gl.TEXTURE0 + c[i]);
					gl.bindTexture(gl.TEXTURE_2D, pass3D.color);
					i += 1;
					break;
				case ADDITIVE_3D:
					if (c[i] !== 0) {
						gl.blendFunc(gl.ONE, gl.ONE);
						gl.enable(gl.BLEND);
					} else {
						gl.disable(gl.BLEND);
					}
					i += 1;
					break;
				case DRAW_FULLSCREEN:
					gl.bindVertexArray(emptyArray);
					gl.drawArrays(gl.TRIANGLES, 0, 3);
					break;
				case USE_PROGRAM:
					gl.useProgram(programs3D.get(c[i]) || null);
					i += 1;
					break;
				case UNIFORM_MATRIX:
					gl.uniformMatrix4fv(locations3D[c[i]], false, c, i + 1, 16);
					i += 17;
					break;
				case UNIFORM: {
					const where = locations3D[c[i]], count = c[i + 1];
					if (count === 1) gl.uniform1f(where, c[i + 2]);
					else if (count === 2) gl.uniform2f(where, c[i + 2], c[i + 3]);
					else if (count === 3) gl.uniform3f(where, c[i + 2], c[i + 3], c[i + 4]);
					else gl.uniform4f(where, c[i + 2], c[i + 3], c[i + 4], c[i + 5]);
					i += 6;
					break;
				}
				case UNIFORM_INT:
					gl.uniform1i(locations3D[c[i]], c[i + 1]);
					i += 2;
					break;
				case COLOR_WRITE: {
					const on = c[i] !== 0;
					gl.colorMask(on, on, on, on);
					i += 1;
					break;
				}
				case DEPTH_WRITE:
					gl.depthMask(c[i] !== 0);
					i += 1;
					break;
				case BLEND_3D:
					if (c[i] !== 0) {
						// The lit shader's colors come premultiplied by their
						// opacity; opacity piles up as coverage, so end3D
						// can put the pass over the 2D with what shows
						// through.
						gl.blendFunc(gl.ONE, gl.ONE_MINUS_SRC_ALPHA);
						gl.enable(gl.BLEND);
					} else {
						gl.disable(gl.BLEND);
					}
					i += 1;
					break;
				case CULLING:
					if (c[i] !== 0) gl.enable(gl.CULL_FACE); else gl.disable(gl.CULL_FACE);
					i += 1;
					break;
				case DRAW_INSTANCED: {
					const mesh = meshes.get(c[i]), count = c[i + 1];
					const floats = count * INSTANCE_FLOATS;
					if (mesh) {
						gl.bindBuffer(gl.ARRAY_BUFFER, instanceBuffer);
						gl.bufferData(gl.ARRAY_BUFFER, c.subarray(i + 2, i + 2 + floats), gl.STREAM_DRAW);
						gl.bindVertexArray(mesh.vao);
						gl.drawElementsInstanced(gl.TRIANGLES, mesh.count, gl.UNSIGNED_INT, 0, count);
					}
					i += 2 + floats;
					break;
				}
				case BIND_TEXTURE_3D:
					gl.activeTexture(gl.TEXTURE0 + c[i]);
					gl.bindTexture(gl.TEXTURE_2D, textures.get(c[i + 1]) || null);
					i += 2;
					break;
				case UPDATE_DATA: {
					const made = dataTextures.get(c[i]), count = c[i + 1];
					if (made) {
						gl.activeTexture(gl.TEXTURE7);
						gl.bindTexture(gl.TEXTURE_2D, made.texture);
						fillData(made, c, i + 2, count);
						gl.bindTexture(gl.TEXTURE_2D, null);
						gl.activeTexture(gl.TEXTURE0);
					}
					i += 2 + count;
					break;
				}
				case BIND_DATA: {
					const made = dataTextures.get(c[i + 1]);
					gl.activeTexture(gl.TEXTURE0 + c[i]);
					gl.bindTexture(gl.TEXTURE_2D, made ? made.texture : null);
					i += 2;
					break;
				}
				case DRAW_MESH: {
					const mesh = meshes.get(c[i]);
					if (mesh) {
						gl.bindVertexArray(mesh.vao);
						gl.drawElements(mesh.lines ? gl.LINES : gl.TRIANGLES, mesh.count, gl.UNSIGNED_INT, 0);
					}
					i += 1;
					break;
				}
				default:
					// A command this file doesn't know means the two halves
					// have drifted apart. Stopping says so loudly.
					throw new Error('golib: unknown drawing command ' + c[i - 1]);
			}
		}
		// Nothing is left waiting when this returns: package golib hands the
		// drawing over before it makes or frees a texture, a target or a
		// shader, and those change what the graphics card is holding. Corners
		// left in the batch would be drawn with the new state instead of the
		// one they were built for.
		flush();
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
		else if (mode === BLEND_PREMULTIPLIED) gl.blendFunc(gl.ONE, gl.ONE_MINUS_SRC_ALPHA);
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
		drawWidth = width;
		drawHeight = height;
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
			if (shader.screen) gl.uniform2f(shader.screen, drawWidth, drawHeight);
			return;
		}
		gl.useProgram(program);
		gl.uniform2f(screenLocation, drawWidth, drawHeight);
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

	// -------------------------------------------------------------------- 3D

	// The 3D's own objects. Meshes and programs share nextID with the rest;
	// uniform locations are numbered in one list for every program.
	const meshes = new Map();     // id -> { vao, vbo, ibo, count, lines }
	const programs3D = new Map(); // id -> WebGLProgram
	const locations3D = [];       // number -> WebGLUniformLocation
	const locationsByName = new Map(); // "program name" -> number, or -1
	let instanceBuffer = null;    // what instanced draws fill, attached to every lit mesh
	const bitmaps = new Map();    // id -> ImageBitmap, decoded and waiting for newTexture3D
	let anisotropy = undefined;   // the extension, once looked for

	// decodeImage has the browser decode a PNG or JPEG file off the page's
	// thread, as it is, and calls done with its number and size, or -1.
	function decodeImage(bytes, done) {
		const blob = new Blob([bytes]);
		createImageBitmap(blob, { premultiplyAlpha: 'none', colorSpaceConversion: 'none', imageOrientation: 'none' })
			.then(function (bitmap) {
				const id = nextID++;
				bitmaps.set(id, bitmap);
				done(id, bitmap.width, bitmap.height);
			}, function () { done(-1, 0, 0); });
	}

	// newHDRTexture puts a picture brighter than white, a panorama or a
	// lightmap, on the graphics card, in half floats, with the mipmaps
	// package golib made, one after another in bytes: WebGL can't make them
	// for a format it can't render into without an extension.
	function newHDRTexture(width, height, bytes, levels, panorama) {
		const halves = new Uint16Array(bytes.buffer, bytes.byteOffset, bytes.byteLength / 2);
		const id = nextID++;
		const made = gl.createTexture();
		gl.activeTexture(gl.TEXTURE7);
		gl.bindTexture(gl.TEXTURE_2D, made);
		gl.pixelStorei(gl.UNPACK_ALIGNMENT, 1);
		let w = width, h = height, at = 0;
		for (let level = 0; level < levels; level++) {
			const size = w * h * 4;
			gl.texImage2D(gl.TEXTURE_2D, level, gl.RGBA16F, w, h, 0, gl.RGBA, gl.HALF_FLOAT, halves.subarray(at, at + size));
			at += size;
			w = Math.max(1, w >> 1);
			h = Math.max(1, h >> 1);
		}
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAX_LEVEL, levels - 1);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, panorama ? gl.REPEAT : gl.CLAMP_TO_EDGE);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
		gl.bindTexture(gl.TEXTURE_2D, null);
		gl.activeTexture(gl.TEXTURE0);
		textures.set(id, made);
		boundTexture = -1;
		return id;
	}

	// newTexture3D puts a decoded picture on the graphics card, with
	// mipmaps, its top row first, as WebGL and glTF both want it.
	function newTexture3D(bitmapID, linear, nearest, nearestMipmaps, wrapS, wrapT) {
		const bitmap = bitmaps.get(bitmapID);
		if (!bitmap) return 0;
		bitmaps.delete(bitmapID);
		const id = nextID++;
		const made = gl.createTexture();
		// A unit no program samples, so the textures bound for drawing stay.
		gl.activeTexture(gl.TEXTURE7);
		gl.bindTexture(gl.TEXTURE_2D, made);
		gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, false);
		gl.pixelStorei(gl.UNPACK_PREMULTIPLY_ALPHA_WEBGL, false);
		gl.pixelStorei(gl.UNPACK_COLORSPACE_CONVERSION_WEBGL, gl.NONE);
		gl.texImage2D(gl.TEXTURE_2D, 0, linear ? gl.RGBA8 : gl.SRGB8_ALPHA8, gl.RGBA, gl.UNSIGNED_BYTE, bitmap);
		gl.pixelStorei(gl.UNPACK_COLORSPACE_CONVERSION_WEBGL, gl.BROWSER_DEFAULT_WEBGL);
		bitmap.close();
		gl.generateMipmap(gl.TEXTURE_2D);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, nearestMipmaps ? gl.NEAREST_MIPMAP_NEAREST : gl.LINEAR_MIPMAP_LINEAR);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, nearest ? gl.NEAREST : gl.LINEAR);
		const wraps = [gl.REPEAT, gl.CLAMP_TO_EDGE, gl.MIRRORED_REPEAT];
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, wraps[wrapS]);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, wraps[wrapT]);
		if (anisotropy === undefined) anisotropy = gl.getExtension('EXT_texture_filter_anisotropic');
		if (anisotropy) gl.texParameterf(gl.TEXTURE_2D, anisotropy.TEXTURE_MAX_ANISOTROPY_EXT, 8);
		gl.bindTexture(gl.TEXTURE_2D, null);
		gl.activeTexture(gl.TEXTURE0);
		textures.set(id, made);
		boundTexture = -1;
		return id;
	}

	function newMesh(layout, vertexBytes, indexBytes, lines) {
		const id = nextID++;
		const vao = gl.createVertexArray();
		gl.bindVertexArray(vao);
		const vbo = gl.createBuffer();
		gl.bindBuffer(gl.ARRAY_BUFFER, vbo);
		const ibo = gl.createBuffer();
		gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, ibo);
		if (layout === LAYOUT_LIT || layout === LAYOUT_SKINNED || layout === LAYOUT_LIGHTMAPPED) {
			const stride = layout === LAYOUT_SKINNED ? 80 : (layout === LAYOUT_LIGHTMAPPED ? 56 : 48);
			bindAttribute(0, 3, 0, stride);
			bindAttribute(1, 3, 12, stride);
			bindAttribute(2, 2, 24, stride);
			bindAttribute(3, 4, 32, stride);
			if (layout === LAYOUT_SKINNED) {
				bindAttribute(9, 4, 48, stride);
				bindAttribute(10, 4, 64, stride);
			}
			if (layout === LAYOUT_LIGHTMAPPED) {
				bindAttribute(11, 2, 48, stride);
			}
			if (!instanceBuffer) instanceBuffer = gl.createBuffer();
			gl.bindBuffer(gl.ARRAY_BUFFER, instanceBuffer);
			for (let k = 0; k < 5; k++) { // the matrix's four columns, then the tint
				bindAttribute(4 + k, 4, k * 16, INSTANCE_FLOATS * 4);
				gl.vertexAttribDivisor(4 + k, 1);
			}
		} else {
			bindAttribute(0, 3, 0, 28);
			bindAttribute(3, 4, 12, 28);
		}
		const mesh = { vao: vao, vbo: vbo, ibo: ibo, count: 0, lines: lines };
		fillMesh(mesh, vertexBytes, indexBytes);
		meshes.set(id, mesh);
		backTo2D();
		return id;
	}

	// fillMesh puts the bytes into a mesh whose vertex array is bound.
	function fillMesh(mesh, vertexBytes, indexBytes) {
		gl.bindBuffer(gl.ARRAY_BUFFER, mesh.vbo);
		gl.bufferData(gl.ARRAY_BUFFER, vertexBytes, gl.STATIC_DRAW);
		gl.bufferData(gl.ELEMENT_ARRAY_BUFFER, indexBytes, gl.STATIC_DRAW);
		mesh.count = indexBytes.length / 4;
	}

	function updateMesh(id, vertexBytes, indexBytes) {
		const mesh = meshes.get(id);
		if (!mesh) return;
		gl.bindVertexArray(mesh.vao);
		fillMesh(mesh, vertexBytes, indexBytes);
		backTo2D();
	}

	function unloadMesh(id) {
		const mesh = meshes.get(id);
		if (!mesh) return;
		gl.deleteVertexArray(mesh.vao);
		gl.deleteBuffer(mesh.vbo);
		gl.deleteBuffer(mesh.ibo);
		meshes.delete(id);
		backTo2D();
	}

	// backTo2D binds what the 2D's batch draws with again.
	function backTo2D() {
		gl.bindVertexArray(vertexArray);
		gl.bindBuffer(gl.ARRAY_BUFFER, vertexBuffer);
	}

	function newProgram3D(vertexSource, fragmentSource) {
		const built = linkProgram(vertexSource, fragmentSource);
		if (built.error) return [-1, (built.error || 'the graphics card gave no reason').trim()];
		const id = nextID++;
		programs3D.set(id, built.program);
		return [id, ''];
	}

	function unloadProgram3D(id) {
		const made = programs3D.get(id);
		if (!made) return;
		gl.deleteProgram(made);
		programs3D.delete(id);
		useCurrentProgram();
	}

	function location3D(id, name) {
		const key = id + ' ' + name;
		if (locationsByName.has(key)) return locationsByName.get(key);
		const made = programs3D.get(id);
		const found = made ? gl.getUniformLocation(made, name) : null;
		let at = -1;
		if (found) {
			at = locations3D.length;
			locations3D.push(found);
		}
		locationsByName.set(key, at);
		return at;
	}

	// pass3D is what a 3D pass draws into: colors in half floats where the
	// browser can render into them, a depth buffer, the size of the target
	// the pass is drawn over. end3D puts it over that target, tonemapped.
	const pass3D = { framebuffer: null, color: null, depth: null, width: 0, height: 0, float: undefined };
	let emptyArray = null; // a vertex array with nothing, for a triangle over the whole view

	function preparePass3D(width, height) {
		if (pass3D.framebuffer && pass3D.width === width && pass3D.height === height) return;
		if (pass3D.float === undefined) pass3D.float = !!gl.getExtension('EXT_color_buffer_float');
		if (!emptyArray) emptyArray = gl.createVertexArray();
		if (pass3D.framebuffer) {
			gl.deleteFramebuffer(pass3D.framebuffer);
			gl.deleteTexture(pass3D.color);
			gl.deleteRenderbuffer(pass3D.depth);
		}
		pass3D.width = width;
		pass3D.height = height;
		pass3D.color = gl.createTexture();
		gl.activeTexture(gl.TEXTURE7);
		gl.bindTexture(gl.TEXTURE_2D, pass3D.color);
		if (pass3D.float) gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA16F, width, height, 0, gl.RGBA, gl.HALF_FLOAT, null);
		else gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, width, height, 0, gl.RGBA, gl.UNSIGNED_BYTE, null);
		// Smoothed, for what reads between its texels: the glow's first
		// step, and the compose shader smoothing edges.
		clampAndFilter(gl.LINEAR);
		gl.bindTexture(gl.TEXTURE_2D, null);
		gl.activeTexture(gl.TEXTURE0);
		pass3D.depth = gl.createRenderbuffer();
		gl.bindRenderbuffer(gl.RENDERBUFFER, pass3D.depth);
		gl.renderbufferStorage(gl.RENDERBUFFER, gl.DEPTH_COMPONENT24, width, height);
		pass3D.framebuffer = gl.createFramebuffer();
		gl.bindFramebuffer(gl.FRAMEBUFFER, pass3D.framebuffer);
		gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, pass3D.color, 0);
		gl.framebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.RENDERBUFFER, pass3D.depth);
		boundTexture = -1;
	}

	// Shadow maps: depth textures sampled with comparison, each with a
	// framebuffer that draws only depth into it.
	const shadowMaps = new Map(); // id -> { framebuffer, texture }

	function newShadowMap(width, height) {
		const id = nextID++;
		const texture = gl.createTexture();
		gl.activeTexture(gl.TEXTURE7);
		gl.bindTexture(gl.TEXTURE_2D, texture);
		gl.texImage2D(gl.TEXTURE_2D, 0, gl.DEPTH_COMPONENT24, width, height, 0, gl.DEPTH_COMPONENT, gl.UNSIGNED_INT, null);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_COMPARE_MODE, gl.COMPARE_REF_TO_TEXTURE);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_COMPARE_FUNC, gl.LEQUAL);
		gl.bindTexture(gl.TEXTURE_2D, null);
		gl.activeTexture(gl.TEXTURE0);
		const framebuffer = gl.createFramebuffer();
		gl.bindFramebuffer(gl.FRAMEBUFFER, framebuffer);
		gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.TEXTURE_2D, texture, 0);
		gl.drawBuffers([gl.NONE]);
		gl.readBuffer(gl.NONE);
		const target = boundTarget === 0 ? null : targets.get(boundTarget);
		gl.bindFramebuffer(gl.FRAMEBUFFER, target ? target.framebuffer : null);
		shadowMaps.set(id, { framebuffer: framebuffer, texture: texture });
		boundTexture = -1;
		return id;
	}

	function unloadShadowMap(id) {
		const map = shadowMaps.get(id);
		if (!map) return;
		gl.deleteFramebuffer(map.framebuffer);
		gl.deleteTexture(map.texture);
		shadowMaps.delete(id);
	}

	// Data textures: numbers for shaders, such as the bones of a pass, four
	// 32-bit floats a texel, DATA_WIDTH texels a row, read with texelFetch.
	const dataTextures = new Map(); // id -> { texture, rows }

	function newDataTexture(bytes) {
		const values = new Float32Array(bytes.buffer, bytes.byteOffset, bytes.byteLength / 4);
		const id = nextID++;
		const made = { texture: gl.createTexture(), rows: 0 };
		gl.activeTexture(gl.TEXTURE7);
		gl.bindTexture(gl.TEXTURE_2D, made.texture);
		clampAndFilter(gl.NEAREST);
		fillData(made, values, 0, values.length);
		gl.bindTexture(gl.TEXTURE_2D, null);
		gl.activeTexture(gl.TEXTURE0);
		dataTextures.set(id, made);
		boundTexture = -1;
		return id;
	}

	// fillData puts count numbers of values, from at, into a data texture
	// that is bound: into the rows it has when they fit, into new ones when
	// they don't. A last row that isn't full is written as far as it goes.
	function fillData(made, values, at, count) {
		const texels = Math.floor(count / 4);
		const rows = Math.max(1, Math.ceil(texels / DATA_WIDTH));
		if (rows > made.rows) {
			gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA32F, DATA_WIDTH, rows, 0, gl.RGBA, gl.FLOAT, null);
			made.rows = rows;
		}
		const full = Math.floor(texels / DATA_WIDTH), rest = texels - full * DATA_WIDTH;
		if (full > 0) gl.texSubImage2D(gl.TEXTURE_2D, 0, 0, 0, DATA_WIDTH, full, gl.RGBA, gl.FLOAT, values, at);
		if (rest > 0) gl.texSubImage2D(gl.TEXTURE_2D, 0, 0, full, rest, 1, gl.RGBA, gl.FLOAT, values, at + full * DATA_WIDTH * 4);
	}

	function unloadDataTexture(id) {
		const made = dataTextures.get(id);
		if (!made) return;
		gl.deleteTexture(made.texture);
		dataTextures.delete(id);
	}

	// ---------------------------------------------------------------- physics

	// Rigid bodies, from JoltPhysics.js, Jolt Physics built for
	// WebAssembly, which web builds of games that use them carry beside
	// this file as jolt-physics.wasm.js and its .wasm. It loads the first
	// time a game makes a world.
	let Jolt = null, joltLoading = null;
	const physicsWorlds = new Map(); // id -> { jolt, system, bodies, ids, v, w, r, q }

	function loadJolt(done) {
		if (Jolt) { done(''); return; }
		if (!joltLoading) {
			joltLoading = import(new URL('jolt-physics.wasm.js', document.baseURI).href)
				.then(function (m) { return m.default(); })
				.then(function (j) { Jolt = j; });
		}
		joltLoading.then(function () { done(''); }, function (e) {
			done('the page could not load jolt-physics.wasm.js, which rigid bodies need: build the game again with golib web or golib dist --web (' + e + ')');
		});
	}

	function newPhysics(maxBodies) {
		const settings = new Jolt.JoltSettings();
		const most = Math.max(1024, maxBodies);
		settings.mMaxBodies = most;
		settings.mMaxBodyPairs = Math.min(most, 16384);
		settings.mMaxContactConstraints = Math.min(most, 8192);
		// Two layers, as golibnative has them: what never moves, and what
		// does; still things don't collide with each other.
		const pairs = new Jolt.ObjectLayerPairFilterTable(2);
		pairs.EnableCollision(0, 1);
		pairs.EnableCollision(1, 1);
		const broad = new Jolt.BroadPhaseLayerInterfaceTable(2, 2);
		broad.MapObjectToBroadPhaseLayer(0, new Jolt.BroadPhaseLayer(0));
		broad.MapObjectToBroadPhaseLayer(1, new Jolt.BroadPhaseLayer(1));
		settings.mObjectLayerPairFilter = pairs;
		settings.mBroadPhaseLayerInterface = broad;
		settings.mObjectVsBroadPhaseLayerFilter = new Jolt.ObjectVsBroadPhaseLayerFilterTable(broad, 2, pairs, 2);
		const jolt = new Jolt.JoltInterface(settings);
		Jolt.destroy(settings);
		const system = jolt.GetPhysicsSystem();
		system.SetGravity(new Jolt.Vec3(0, -9.81, 0));
		const id = nextID++;
		physicsWorlds.set(id, {
			jolt: jolt, system: system, bodies: system.GetBodyInterface(), ids: new Map(),
			v: new Jolt.Vec3(), w: new Jolt.Vec3(), r: new Jolt.RVec3(), q: new Jolt.Quat(),
		});
		return id;
	}

	function freePhysics(id) {
		const world = physicsWorlds.get(id);
		if (!world) return;
		Jolt.destroy(world.jolt);
		physicsWorlds.delete(id);
	}

	function physicsGravity(id, x, y, z) {
		const world = physicsWorlds.get(id);
		if (world) world.system.SetGravity(new Jolt.Vec3(x, y, z));
	}

	// floatsOf reads bytes from Go as the floats they are.
	function floatsOf(bytes) {
		return new Float32Array(bytes.buffer, bytes.byteOffset, bytes.byteLength / 4);
	}

	// keepBody keeps a new body's id, and returns its number, as the
	// desktop's library numbers bodies.
	function keepBody(world, bodyID) {
		const number = bodyID.GetIndexAndSequenceNumber();
		world.ids.set(number, new Jolt.BodyID(number));
		return number;
	}

	// addPhysicsMesh adds a body of triangles in its own space, placed as
	// moreBytes says: friction, restitution, position, rotation, and 1 for a
	// body the game moves.
	function addPhysicsMesh(id, bytes, moreBytes) {
		const world = physicsWorlds.get(id);
		const c = floatsOf(bytes), m = floatsOf(moreBytes);
		const triangles = new Jolt.TriangleList();
		const a = new Jolt.Vec3(), b = new Jolt.Vec3(), d = new Jolt.Vec3();
		for (let i = 0; i + 8 < c.length; i += 9) {
			a.Set(c[i], c[i + 1], c[i + 2]);
			b.Set(c[i + 3], c[i + 4], c[i + 5]);
			d.Set(c[i + 6], c[i + 7], c[i + 8]);
			const t = new Jolt.Triangle(a, b, d);
			triangles.push_back(t);
			Jolt.destroy(t);
		}
		const result = new Jolt.MeshShapeSettings(triangles).Create();
		Jolt.destroy(triangles);
		if (result.HasError()) return 0xffffffff;
		const moved = m[9] === 1;
		const settings = new Jolt.BodyCreationSettings(result.Get(), new Jolt.RVec3(m[2], m[3], m[4]), new Jolt.Quat(m[5], m[6], m[7], m[8]),
			moved ? Jolt.EMotionType_Kinematic : Jolt.EMotionType_Static, moved ? 1 : 0);
		settings.mFriction = m[0];
		settings.mRestitution = m[1];
		const number = keepBody(world, world.bodies.CreateAndAddBody(settings, moved ? Jolt.EActivation_Activate : Jolt.EActivation_DontActivate));
		Jolt.destroy(settings);
		return number;
	}

	// addPhysicsBody reads a body's description, in the order of
	// physics.go's Body constants.
	function addPhysicsBody(id, bytes) {
		const world = physicsWorlds.get(id);
		const d = floatsOf(bytes);
		let result = null;
		switch (d[0]) {
			case 0:
				result = new Jolt.BoxShapeSettings(new Jolt.Vec3(d[1], d[2], d[3]), Math.min(0.05, 0.5 * Math.min(d[1], d[2], d[3]))).Create();
				break;
			case 1:
				result = new Jolt.SphereShapeSettings(d[1]).Create();
				break;
			case 2:
				result = new Jolt.CapsuleShapeSettings(d[2], d[1]).Create();
				break;
			case 3: {
				const hull = new Jolt.ConvexHullShapeSettings();
				for (let i = 0; i < d[19]; i++) {
					hull.mPoints.push_back(new Jolt.Vec3(d[20 + i * 3], d[21 + i * 3], d[22 + i * 3]));
				}
				hull.mMaxConvexRadius = 0.02;
				result = hull.Create();
				break;
			}
			case 4:
				result = new Jolt.CylinderShapeSettings(d[2], d[1], Math.min(0.05, 0.5 * Math.min(d[1], d[2]))).Create();
				break;
		}
		if (!result || result.HasError()) return 0xffffffff;
		const motion = d[11] === 2 ? Jolt.EMotionType_Dynamic : (d[11] === 1 ? Jolt.EMotionType_Kinematic : Jolt.EMotionType_Static);
		const settings = new Jolt.BodyCreationSettings(result.Get(), new Jolt.RVec3(d[4], d[5], d[6]),
			new Jolt.Quat(d[7], d[8], d[9], d[10]), motion, motion === Jolt.EMotionType_Static ? 0 : 1);
		if (d[12] > 0) {
			settings.mOverrideMassProperties = Jolt.EOverrideMassProperties_CalculateInertia;
			settings.mMassPropertiesOverride.mMass = d[12];
		}
		settings.mFriction = d[13];
		settings.mRestitution = d[14];
		settings.mLinearDamping = d[15];
		settings.mAngularDamping = d[16];
		settings.mGravityFactor = d[17];
		if (d[18] !== 0) settings.mMotionQuality = Jolt.EMotionQuality_LinearCast;
		const activation = motion === Jolt.EMotionType_Static ? Jolt.EActivation_DontActivate : Jolt.EActivation_Activate;
		const number = keepBody(world, world.bodies.CreateAndAddBody(settings, activation));
		Jolt.destroy(settings);
		return number;
	}

	function removePhysicsBody(id, number) {
		const world = physicsWorlds.get(id);
		const body = world && world.ids.get(number);
		if (!body) return;
		world.bodies.RemoveBody(body);
		world.bodies.DestroyBody(body);
		Jolt.destroy(body);
		world.ids.delete(number);
	}

	function stepPhysics(id, dt, steps) {
		const world = physicsWorlds.get(id);
		if (world) world.jolt.Step(dt, Math.max(1, steps));
	}

	// readPhysicsBodies writes 14 floats for each body numbered in idBytes
	// into out: position, rotation, velocity, spin, awake.
	function readPhysicsBodies(id, idBytes, out) {
		const world = physicsWorlds.get(id);
		const numbers = new Uint32Array(idBytes.buffer, idBytes.byteOffset, idBytes.byteLength / 4);
		const o = new Float32Array(numbers.length * 14);
		for (let i = 0; i < numbers.length; i++) {
			const body = world.ids.get(numbers[i]);
			if (!body) continue;
			world.bodies.GetPositionAndRotation(body, world.r, world.q);
			const v = world.bodies.GetLinearVelocity(body);
			const k = i * 14;
			o[k] = world.r.GetX(); o[k + 1] = world.r.GetY(); o[k + 2] = world.r.GetZ();
			o[k + 3] = world.q.GetX(); o[k + 4] = world.q.GetY(); o[k + 5] = world.q.GetZ(); o[k + 6] = world.q.GetW();
			o[k + 7] = v.GetX(); o[k + 8] = v.GetY(); o[k + 9] = v.GetZ();
			const s = world.bodies.GetAngularVelocity(body);
			o[k + 10] = s.GetX(); o[k + 11] = s.GetY(); o[k + 12] = s.GetZ();
			o[k + 13] = world.bodies.IsActive(body) ? 1 : 0;
		}
		out.set(new Uint8Array(o.buffer));
	}

	// setPhysicsBody changes a body as physics.go's Set constants say.
	function setPhysicsBody(id, number, what, bytes) {
		const world = physicsWorlds.get(id);
		const body = world && world.ids.get(number);
		if (!body) return;
		const v = floatsOf(bytes);
		const b = world.bodies;
		switch (what) {
			case 0:
				world.r.Set(v[0], v[1], v[2]);
				world.q.Set(v[3], v[4], v[5], v[6]);
				b.SetPositionAndRotation(body, world.r, world.q, Jolt.EActivation_Activate);
				break;
			case 1:
				world.v.Set(v[0], v[1], v[2]);
				world.w.Set(v[3], v[4], v[5]);
				b.SetLinearAndAngularVelocity(body, world.v, world.w);
				break;
			case 2:
				world.v.Set(v[0], v[1], v[2]);
				world.r.Set(v[3], v[4], v[5]);
				b.AddImpulse(body, world.v, world.r);
				break;
			case 3:
				world.r.Set(v[0], v[1], v[2]);
				world.q.Set(v[3], v[4], v[5], v[6]);
				b.MoveKinematic(body, world.r, world.q, v[7]);
				break;
			case 4:
				world.v.Set(v[0], v[1], v[2]);
				b.AddForce(body, world.v, Jolt.EActivation_Activate);
				break;
		}
	}

	// Work targets: pictures a pass draws into and reads back on the way to
	// the screen, in half floats where the browser can render into them.
	const workTargets = new Map(); // id -> { framebuffer, texture, width, height }

	function newWorkTarget(width, height) {
		if (pass3D.float === undefined) pass3D.float = !!gl.getExtension('EXT_color_buffer_float');
		const id = nextID++;
		const texture = gl.createTexture();
		gl.activeTexture(gl.TEXTURE7);
		gl.bindTexture(gl.TEXTURE_2D, texture);
		if (pass3D.float) gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA16F, width, height, 0, gl.RGBA, gl.HALF_FLOAT, null);
		else gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, width, height, 0, gl.RGBA, gl.UNSIGNED_BYTE, null);
		clampAndFilter(gl.LINEAR);
		gl.bindTexture(gl.TEXTURE_2D, null);
		gl.activeTexture(gl.TEXTURE0);
		const framebuffer = gl.createFramebuffer();
		gl.bindFramebuffer(gl.FRAMEBUFFER, framebuffer);
		gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, texture, 0);
		const target = boundTarget === 0 ? null : targets.get(boundTarget);
		gl.bindFramebuffer(gl.FRAMEBUFFER, target ? target.framebuffer : null);
		workTargets.set(id, { framebuffer: framebuffer, texture: texture, width: width, height: height });
		boundTexture = -1;
		return id;
	}

	function unloadWorkTarget(id) {
		const work = workTargets.get(id);
		if (!work) return;
		gl.deleteFramebuffer(work.framebuffer);
		gl.deleteTexture(work.texture);
		workTargets.delete(id);
	}

	function begin3D() {
		flush();
		preparePass3D(Math.max(1, viewWidth), Math.max(1, viewHeight));
		gl.bindFramebuffer(gl.FRAMEBUFFER, pass3D.framebuffer);
		gl.viewport(0, 0, pass3D.width, pass3D.height);
		gl.enable(gl.DEPTH_TEST);
		gl.depthFunc(gl.LEQUAL);
		gl.depthMask(true);
		gl.clearBufferfv(gl.COLOR, 0, [0, 0, 0, 0]);
		gl.clearBufferfv(gl.DEPTH, 0, [1]);
		gl.enable(gl.CULL_FACE);
		gl.cullFace(gl.BACK);
		gl.frontFace(gl.CCW);
		gl.disable(gl.BLEND);
	}

	// end3D draws the pass's picture over the target the pass was drawn
	// over, through the compose program, which reads it from texture unit 0
	// and gives colors premultiplied by their opacity. Then it leaves WebGL as
	// the 2D expects it: no depth testing or culling, blending as it was, and
	// the 2D's program, batch and texture unit.
	function end3D(composeID) {
		const target = boundTarget === 0 ? null : targets.get(boundTarget);
		gl.bindFramebuffer(gl.FRAMEBUFFER, target ? target.framebuffer : null);
		gl.viewport(0, 0, viewWidth, viewHeight);
		gl.disable(gl.DEPTH_TEST);
		gl.depthMask(true);
		gl.disable(gl.CULL_FACE);
		gl.enable(gl.BLEND);
		gl.blendFunc(gl.ONE, gl.ONE_MINUS_SRC_ALPHA);
		gl.useProgram(programs3D.get(composeID) || null);
		gl.activeTexture(gl.TEXTURE0);
		gl.bindTexture(gl.TEXTURE_2D, pass3D.color);
		gl.bindVertexArray(emptyArray);
		gl.drawArrays(gl.TRIANGLES, 0, 3);
		const mode = boundBlend;
		boundBlend = -1;
		setBlend(mode < 0 ? BLEND_NORMAL : mode);
		gl.activeTexture(gl.TEXTURE0);
		boundTexture = -1;
		backTo2D();
		useCurrentProgram();
	}


	// -------------------------------------------------------------- pictures

	// newTexture loads a picture, unsmoothed as raylib loads one, or smoothed
	// with mipmaps, as raylib's trilinear filter does, so a large picture drawn
	// small keeps its detail.
	function newTexture(width, height, pixels, smooth) {
		const id = nextID++;
		const made = gl.createTexture();
		gl.bindTexture(gl.TEXTURE_2D, made);
		gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, width, height, 0, gl.RGBA, gl.UNSIGNED_BYTE, pixels);
		if (smooth) {
			clampAndFilter(gl.LINEAR);
			gl.generateMipmap(gl.TEXTURE_2D);
			gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR);
		} else {
			clampAndFilter(gl.NEAREST); // raylib's own default for a new picture
		}
		textures.set(id, made);
		boundTexture = -1;
		return id;
	}

	function updateTexture(id, width, height, pixels) {
		const made = textures.get(id);
		if (!made) return;
		gl.bindTexture(gl.TEXTURE_2D, made);
		gl.texSubImage2D(gl.TEXTURE_2D, 0, 0, 0, width, height, gl.RGBA, gl.UNSIGNED_BYTE, pixels);
		boundTexture = -1;
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
		// A depth buffer, for the 3D drawn into the target, as raylib's
		// render textures have one.
		const depth = gl.createRenderbuffer();
		gl.bindRenderbuffer(gl.RENDERBUFFER, depth);
		gl.renderbufferStorage(gl.RENDERBUFFER, gl.DEPTH_COMPONENT24, width, height);
		gl.framebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.RENDERBUFFER, depth);
		gl.bindFramebuffer(gl.FRAMEBUFFER, null);
		targets.set(id, { framebuffer: framebuffer, depth: depth, texture: made, textureID: textureID, width: width, height: height });
		boundTexture = -1;
		boundTarget = -1;
		bindCanvas();
		return [id, textureID];
	}

	function unloadTarget(id) {
		const target = targets.get(id);
		if (!target) return;
		gl.deleteFramebuffer(target.framebuffer);
		gl.deleteRenderbuffer(target.depth);
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
		Delete: 261, Home: 268, End: 269, PageUp: 266, PageDown: 267,
		ShiftLeft: 340, ShiftRight: 344, ControlLeft: 341, ControlRight: 345,
		AltLeft: 342, AltRight: 346,
		Quote: 39, Comma: 44, Minus: 45, Period: 46, Slash: 47, Semicolon: 59, Equal: 61,
		BracketLeft: 91, Backslash: 92, BracketRight: 93, Backquote: 96,
	};
	for (let i = 0; i < 26; i++) KEYS['Key' + String.fromCharCode(65 + i)] = 65 + i;
	for (let i = 0; i < 10; i++) { KEYS['Digit' + i] = 48 + i; KEYS['Numpad' + i] = 48 + i; }
	for (let i = 1; i <= 12; i++) KEYS['F' + i] = 289 + i;

	// The browser's gamepad buttons, in GoLib's order. -1 is a button GoLib
	// doesn't name.
	const PAD_BUTTONS = [7, 6, 8, 5, 9, 11, 10, 12, 13, 15, 16, 17, 1, 3, 4, 2, 14];
	const PAD_BUTTON_COUNT = 18, MAX_PADS = 4;

	// Where each part of the input buffer is, the same list as in web_input.go:
	// change one and change the other. The floats come first, counted in
	// floats, and the keys and buttons after them, counted in bytes.
	const MAX_TOUCHES = 8;
	const IN_MOUSE_X = 0, IN_MOUSE_Y = 1, IN_WHEEL = 2, IN_STICKS = 3;
	const IN_TOUCH_COUNT = IN_STICKS + 4 * MAX_PADS;
	const IN_TOUCHES = IN_TOUCH_COUNT + 1; // MAX_TOUCHES fingers of id, x, y
	const IN_FLOATS = IN_TOUCHES + 3 * MAX_TOUCHES;
	const KEYS_AT = IN_FLOATS * 4;

	const keysDown = new Uint8Array(349), keysPressed = new Uint8Array(349);
	const mouseDown = new Uint8Array(3), mousePressed = new Uint8Array(3);
	let mouseX = 0, mouseY = 0, wheel = 0;
	// Whether the mouse pointer is over the canvas: from its first move there
	// until it leaves.
	let mouseOver = false;
	// The characters typed since the last frame, which Input.TypedText gives.
	let typed = '';
	const MAX_TYPED = 256;
	const padWas = []; // what each pad's buttons were last frame, for presses

	// The fingers on the screen, oldest first, each with an id of GoLib's own
	// that lasts as long as the finger is down: { id, pointer, x, y, fresh,
	// gone }. A game reads them through Input.Touches.
	let touches = [];
	let nextTouch = 1;

	let inputBytes = null, inputFloats = null;

	function inputBuffer(size) {
		inputBytes = new Uint8Array(size);
		inputFloats = new Float32Array(inputBytes.buffer, 0, IN_FLOATS);
		return inputBytes;
	}

	// touchScreen reports whether a finger is how this machine is pointed at: a
	// phone or a tablet. A computer with a touch screen and a mouse says no,
	// because its player has a keyboard and a pointer and the game should not
	// cover itself with on-screen controls; package golib turns them on there
	// as soon as a finger really lands on the game.
	//
	// "pointer: coarse" is the browser's own answer to that question: it is the
	// main pointer being a finger rather than a mouse.
	function touchScreen() {
		const fingers = navigator && navigator.maxTouchPoints !== undefined
			? navigator.maxTouchPoints > 0
			: 'ontouchstart' in window;
		if (!fingers || !window.matchMedia) return fingers;
		return window.matchMedia('(pointer: coarse)').matches;
	}

	// takeFocus brings the keyboard to the game. A site that puts the game in
	// an <iframe>, as itch.io does, sends key events to whichever document
	// has the focus, and a click on the game doesn't move it by itself: the
	// handlers below cancel the click's default action, which is what would
	// have moved it. Without this, a game on itch.io draws and plays its
	// sounds but never sees a key, and document.hasFocus stays false, so
	// Config.PauseUnfocused holds it as well.
	function takeFocus() {
		try {
			window.focus();
			if (canvas && canvas.focus) canvas.focus({ preventScroll: true });
		} catch (e) { /* a browser that refuses is no reason to stop the game */ }
	}

	function listen() {
		// Every click and touch, wherever it lands, brings the keyboard with
		// it: the game opens without the focus when a page embeds it. It is
		// also all a phone has, so it is what fullscreen and the sound device
		// wait for there, where there is no key to press.
		window.addEventListener('pointerdown', function () {
			takeFocus();
			takeFullscreen();
			wakeAudio();
		});
		// A phone turned on its side, a window resized and fullscreen coming
		// or going all change the size the game is drawn at. Every frame
		// measures it anyway; these arrive first, so the frame being drawn
		// already has the new size.
		window.addEventListener('resize', resize);
		window.addEventListener('orientationchange', resize);
		document.addEventListener('fullscreenchange', onFullscreenChange);
		document.addEventListener('webkitfullscreenchange', onFullscreenChange);
		window.addEventListener('keydown', function (e) {
			const key = KEYS[e.code];
			if (key !== undefined) {
				if (!keysDown[key]) keysPressed[key] = 1;
				keysDown[key] = 1;
				// Arrows, space, tab, Home, End and the page keys would
				// otherwise scroll the page or move the focus out of the game.
				if (key === 32 || key === 258 || (key >= 262 && key <= 269)) e.preventDefault();
			}
			// A key that types a character gives it in e.key, with the
			// layout, Shift and AltGr applied; the others give their name,
			// such as "Enter". Ctrl without Alt is a shortcut, not typing
			// (AltGr comes as Ctrl and Alt on Windows).
			if ([...e.key].length === 1 && !e.metaKey && (!e.ctrlKey || e.altKey)) {
				if (typed.length < MAX_TYPED) typed += e.key;
				e.preventDefault(); // "/" and "'" open Firefox's quick find
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
			touches = [];
		});
		canvas.addEventListener('mousemove', function (e) {
			const box = canvas.getBoundingClientRect();
			const scale = canvas.width / Math.max(1, box.width);
			mouseX = (e.clientX - box.left) * scale;
			mouseY = (e.clientY - box.top) * (canvas.height / Math.max(1, box.height));
			mouseOver = true;
		});
		canvas.addEventListener('mouseleave', function () { mouseOver = false; });
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
		// A touch screen. Every finger is followed by an id of its own, so a
		// game can tell one from another, and the default action of a touch is
		// cancelled: a finger on the game is the game's, not a scroll, a zoom
		// or a mouse event the browser makes up afterwards. Package golib
		// moves the mouse pointer with the first finger itself, so that a tap
		// works a menu written for a mouse.
		canvas.addEventListener('pointerdown', function (e) {
			if (e.pointerType === 'mouse') return;
			addTouch(e);
			e.preventDefault();
		}, { passive: false });
		canvas.addEventListener('pointermove', function (e) {
			if (e.pointerType === 'mouse') return;
			moveTouch(e);
			e.preventDefault();
		}, { passive: false });
		window.addEventListener('pointerup', function (e) {
			if (e.pointerType !== 'mouse') dropTouch(e);
		});
		window.addEventListener('pointercancel', function (e) {
			if (e.pointerType !== 'mouse') dropTouch(e);
		});
		canvas.addEventListener('wheel', function (e) {
			wheel += e.deltaY > 0 ? -1 : (e.deltaY < 0 ? 1 : 0);
			e.preventDefault();
		}, { passive: false });
		canvas.addEventListener('contextmenu', function (e) { e.preventDefault(); });
	}

	function onFullscreenChange() {
		followFullscreen();
		resize();
	}

	// pointAt returns where a touch or a click is, in canvas pixels, as the
	// mouse pointer's position is.
	function pointAt(event) {
		const box = canvas.getBoundingClientRect();
		return {
			x: (event.clientX - box.left) * (canvas.width / Math.max(1, box.width)),
			y: (event.clientY - box.top) * (canvas.height / Math.max(1, box.height)),
		};
	}

	// foundTouch returns where in touches the browser's pointer is, or -1.
	function foundTouch(pointer) {
		for (let i = 0; i < touches.length; i++) {
			if (touches[i].pointer === pointer) return i;
		}
		return -1;
	}

	function addTouch(event) {
		if (touches.length >= MAX_TOUCHES || foundTouch(event.pointerId) >= 0) return;
		const at = pointAt(event);
		touches.push({ id: nextTouch++, pointer: event.pointerId, x: at.x, y: at.y, fresh: true, gone: false });
	}

	function moveTouch(event) {
		const found = foundTouch(event.pointerId);
		if (found < 0) return;
		const at = pointAt(event);
		touches[found].x = at.x;
		touches[found].y = at.y;
	}

	function dropTouch(event) {
		const found = foundTouch(event.pointerId);
		if (found < 0) return;
		// A finger that lands and lifts between two frames is still one the
		// game sees: it stays until the frame that saw it land has passed. Its
		// pointer is forgotten, because the next finger may be given the same
		// one before then.
		if (touches[found].fresh) {
			touches[found].gone = true;
			touches[found].pointer = -1;
			return;
		}
		touches.splice(found, 1);
	}

	// domButton turns the browser's button number into GoLib's.
	function domButton(button) {
		if (button === 0) return 0; // left
		if (button === 1) return 2; // middle
		if (button === 2) return 1; // right
		return -1;
	}

	// takeTyped gives the characters typed since it was last called.
	function takeTyped() {
		const text = typed;
		typed = '';
		return text;
	}

	// snapshotInput writes the state the next frame reads and clears the
	// presses, so each press reaches exactly one frame.
	function snapshotInput() {
		if (!inputBytes) return;
		inputFloats[IN_MOUSE_X] = mouseX;
		inputFloats[IN_MOUSE_Y] = mouseY;
		inputFloats[IN_WHEEL] = wheel;
		wheel = 0;

		const keysAt = KEYS_AT;
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
			const sticks = IN_STICKS + pad * 4;
			inputFloats[sticks] = found.axes[0] || 0;
			inputFloats[sticks + 1] = found.axes[1] || 0;
			inputFloats[sticks + 2] = found.axes[2] || 0;
			inputFloats[sticks + 3] = found.axes[3] || 0;
		}

		// The fingers on the screen, oldest first, and which of them landed
		// since the last frame. A finger that lifted before any frame saw it
		// is reported this once and then let go.
		const touchesNewAt = pressedAt + MAX_PADS * PAD_BUTTON_COUNT;
		const count = Math.min(touches.length, MAX_TOUCHES);
		inputFloats[IN_TOUCH_COUNT] = count;
		for (let i = 0; i < MAX_TOUCHES; i++) {
			const touch = i < count ? touches[i] : null;
			const at = IN_TOUCHES + i * 3;
			inputFloats[at] = touch ? touch.id : 0;
			inputFloats[at + 1] = touch ? touch.x : 0;
			inputFloats[at + 2] = touch ? touch.y : 0;
			inputBytes[touchesNewAt + i] = touch && touch.fresh ? 1 : 0;
		}
		for (let i = touches.length - 1; i >= 0; i--) {
			if (touches[i].gone) touches.splice(i, 1);
			else touches[i].fresh = false;
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
		const reader = decoder();
		if (!reader) { done(-1); return; }
		// decodeAudioData empties the buffer it is given, so it gets a copy.
		const data = bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength);
		reader.decodeAudioData(data, function (buffer) {
			const id = nextSoundID++;
			decoded.set(id, buffer);
			done(id);
		}, function () { done(-1); });
	}

	// decoder returns something that reads sound files: the sound device when
	// there is one, and otherwise a context that only decodes. golib shot
	// opens no sound device, and a game still has to know whether its sound
	// files are good, as it does on the desktop.
	let silentDecoder = null;
	function decoder() {
		if (audio) return audio;
		if (!silentDecoder) {
			const Offline = window.OfflineAudioContext || window.webkitOfflineAudioContext;
			if (Offline) silentDecoder = new Offline(1, 1, 44100);
		}
		return silentDecoder;
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
		voices.set(id, { buffer: buffer, volume: 1, pitch: 1, pan: 0, node: null, gain: null, panner: null });
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
		voice.panner = connectPanned(gain, voice.pan);
		node.onended = function () {
			if (voice.node === node) { voice.node = null; voice.gain = null; voice.panner = null; }
		};
		node.start();
		voice.node = node;
		voice.gain = gain;
	}

	// connectPanned connects gain to the master volume through a stereo
	// panner when pan isn't 0, and returns the panner. A sound in the
	// middle goes without one, since a panner lowers a mono sound in the
	// middle by a third, as no sound before panning was.
	function connectPanned(gain, pan) {
		if (pan === 0) {
			gain.connect(master);
			return null;
		}
		const panner = audio.createStereoPanner();
		panner.pan.value = pan;
		gain.connect(panner);
		panner.connect(master);
		return panner;
	}

	function setSoundPan(id, pan) {
		const voice = voices.get(id);
		if (!voice) return;
		voice.pan = pan;
		if (voice.panner) voice.panner.pan.value = pan;
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
		music.panner = connectPanned(gain, music.pan || 0);
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

	function setMusicPan(id, pan) {
		const music = musics.get(id);
		if (!music) return;
		music.pan = pan;
		if (music.panner) {
			music.panner.pan.value = pan;
		} else if (music.gain && pan !== 0) {
			// Playing in the middle: a panner joins it now.
			music.gain.disconnect();
			music.panner = connectPanned(music.gain, pan);
		}
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

	// postPicture sends a screenshot back to the golib that opened the page,
	// which writes it: golib shot --web listens for these. A page cannot
	// write files itself, and a game carrying its own HTTP would be megabytes
	// heavier for a post only golib shot makes.
	function postPicture(name, bytes, done) {
		fetch('/golib-shot?name=' + encodeURIComponent(name), {
			method: 'POST',
			headers: { 'Content-Type': 'image/png' },
			body: bytes,
		}).then(function (answer) {
			done(answer.ok);
		}).catch(function () {
			done(false);
		});
	}

	// ----------------------------------------------------------------- frames

	function onFrame(fn) { wake = fn; }

	function askForFrame() {
		requestAnimationFrame(function (stamp) {
			// Too soon for the frame rate asked for: wait for the next frame.
			// A millisecond's slack keeps 60 on a 60 Hz screen from halving.
			if (frameGap > 0 && lastWake >= 0 && stamp / 1000 - lastWake < frameGap - 0.001) {
				askForFrame();
				return;
			}
			lastWake = stamp / 1000;
			frameTime = stamp / 1000;
			followFullscreen();
			resize();
			if (wake) wake();
		});
	}

	// setFrameRate draws fps frames a second at most, or at every frame the
	// browser offers for 0.
	function setFrameRate(fps) {
		frameGap = fps > 0 ? 1 / fps : 0;
	}

	function frame() {
		return [frameTime, canvas ? canvas.width : 0, canvas ? canvas.height : 0, document.hasFocus(), closed];
	}

	// fullscreenNow returns the element the browser is showing fullscreen, or
	// null. The webkit name is Safari's, which has no other.
	function fullscreenNow() {
		return document.fullscreenElement || document.webkitFullscreenElement || null;
	}

	// ownFullscreen reports whether the game is fullscreen because it asked
	// for it, rather than because the page around it did.
	function ownFullscreen() {
		return fullscreenTarget !== null && fullscreenNow() === fullscreenTarget;
	}

	// followFullscreen follows what the browser does with the fullscreen the game
	// asked for, so that a player who leaves it with Esc or a phone's gesture is
	// not put back into it, and the game's own idea of fullscreen can follow
	// theirs. Every frame asks, as well as every fullscreenchange: a browser that
	// grants fullscreen without telling the page is then no reason to miss it.
	function followFullscreen() {
		if (ownFullscreen()) {
			fullscreenHeld = true;
			return;
		}
		if (fullscreenHeld) {
			fullscreenHeld = false;
			fullscreenLeft = true;
			// The ask is over: the player answered it. Nothing is wanted until
			// the game asks again, and then that is a new ask.
			wantFullscreen = false;
			fullscreenAsked = false;
		}
	}

	// fullscreenLost is what package golib asks once a frame. It stays true
	// until the game asks for fullscreen again.
	function fullscreenLost() {
		return fullscreenLeft;
	}

	// takeFullscreen makes the request golib.SetFullscreen asked for. A
	// browser only grants fullscreen for a moment after a key, a click or a
	// touch, so the handlers of those call it, and setFullscreen does when one
	// came a moment ago. Leaving needs none.
	//
	// The whole page goes fullscreen, not the canvas. A fullscreen canvas on a
	// phone keeps the shape its box had when the phone turns, so the game ends
	// up in a corner of the screen; the page's own root element follows the
	// screen, and a message over the game stays visible instead of being left
	// behind the canvas. The canvas fills the page, so the game covers the
	// screen either way.
	function takeFullscreen() {
		if (wantFullscreen === fullscreenAsked) return;
		fullscreenAsked = wantFullscreen;
		if (!wantFullscreen) {
			fullscreenTarget = null;
			fullscreenHeld = false; // the game leaves, not the player
			const leave = document.exitFullscreen || document.webkitExitFullscreen;
			if (leave && fullscreenNow()) call(leave, document);
			return;
		}
		const whole = document.documentElement || canvas;
		const ask = whole.requestFullscreen || whole.webkitRequestFullscreen;
		// An iPhone shows nothing but video fullscreen: the game plays on in
		// the page, which is the whole screen there anyway.
		if (!ask) return;
		// A browser that says no leaves fullscreenNow as it was, so it is not
		// the element asked for, and nothing here thinks the game is fullscreen;
		// the next key, click or touch asks again.
		fullscreenTarget = whole;
		call(ask, whole, function () {
			if (fullscreenAsked && fullscreenNow() !== whole) fullscreenAsked = false;
		});
	}

	// call runs a method a browser may not have and may refuse, and swallows
	// both, after calling refused when there is one: a browser that says no to
	// fullscreen is no reason to stop the game.
	function call(method, on, refused) {
		try {
			const answer = method.call(on);
			if (answer && answer.catch) answer.catch(function () { if (refused) refused(); });
		} catch (e) {
			if (refused) refused();
		}
	}

	// setFullscreen is what golib.SetFullscreen asks for, a frame after the key
	// or the click the game read it on, when that event's handler is over. The
	// browser still counts that key or click for a moment, so the request is
	// made at once, as it is to leave; otherwise it waits for the next one.
	function setFullscreen(on) {
		wantFullscreen = !!on;
		// Asked again, whatever the player did before is past.
		if (wantFullscreen) fullscreenLeft = false;
		if (!wantFullscreen || justActed()) takeFullscreen();
	}

	// justActed reports whether the player pressed a key, clicked or touched a
	// moment ago, which is when a browser grants fullscreen. A browser that
	// can't say is answered by waiting for the next one.
	function justActed() {
		const activation = navigator.userActivation;
		return !!(activation && activation.isActive);
	}

	function setCursorVisible(visible) {
		cursorShown = !!visible;
		if (canvas) canvas.style.cursor = cursorShown ? '' : 'none';
	}

	function cursorVisible() { return cursorShown; }

	function mouseInside() { return mouseOver; }

	function close() { closed = true; }

	// showError puts a message over the game. A player in a browser never
	// opens its console, so a game that stops says why here.
	function showError(title, message) {
		// golib shot --web opened this page and is waiting: tell it, so that
		// it says why the game stopped instead of only that it never
		// finished. The page shows the message too, for whoever is looking.
		if (location.search.indexOf('GOLIB_SHOT_') >= 0) {
			fetch('/golib-shot-error', { method: 'POST', body: message }).catch(function () { });
		}
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
		newTexture: newTexture, updateTexture: updateTexture, unloadTexture: unloadTexture,
		newTarget: newTarget, unloadTarget: unloadTarget, readTarget: readTarget,
		inputBuffer: inputBuffer, snapshotInput: snapshotInput, takeTyped: takeTyped, gamepadName: gamepadName,
		touchScreen: touchScreen,
		onFrame: onFrame, askForFrame: askForFrame, setFrameRate: setFrameRate,
		setFullscreen: setFullscreen, fullscreenLost: fullscreenLost,
		setCursorVisible: setCursorVisible, cursorVisible: cursorVisible, mouseInside: mouseInside,
		showError: showError,
		postPicture: postPicture,
		newShader: newShader, unloadShader: unloadShader, shaderLocation: shaderLocation,
		newMesh: newMesh, updateMesh: updateMesh, unloadMesh: unloadMesh,
		decodeImage: decodeImage, newTexture3D: newTexture3D, newHDRTexture: newHDRTexture,
		newShadowMap: newShadowMap, unloadShadowMap: unloadShadowMap,
		newWorkTarget: newWorkTarget, unloadWorkTarget: unloadWorkTarget,
		newDataTexture: newDataTexture, unloadDataTexture: unloadDataTexture,
		loadJolt: loadJolt, newPhysics: newPhysics, freePhysics: freePhysics, physicsGravity: physicsGravity,
		addPhysicsMesh: addPhysicsMesh, addPhysicsBody: addPhysicsBody, removePhysicsBody: removePhysicsBody,
		stepPhysics: stepPhysics, readPhysicsBodies: readPhysicsBodies, setPhysicsBody: setPhysicsBody,
		newProgram3D: newProgram3D, unloadProgram3D: unloadProgram3D, location3D: location3D,
		readFont: readFont,
		openAudio: openAudio, closeAudio: closeAudio, setMasterVolume: setMasterVolume,
		decodeSound: decodeSound, unloadWave: unloadWave,
		newSound: newSound, newSoundAlias: newSoundAlias, unloadSound: unloadSound,
		playSound: playSound, stopSound: stopSound, soundPlaying: soundPlaying,
		setSoundVolume: setSoundVolume, setSoundPitch: setSoundPitch, setSoundPan: setSoundPan,
		newMusic: newMusic, playMusic: playMusic, pauseMusic: pauseMusic,
		resumeMusic: resumeMusic, stopMusic: stopMusic, unloadMusic: unloadMusic,
		setMusicVolume: setMusicVolume, setMusicPan: setMusicPan, musicPlaying: musicPlaying,
	};
})();
