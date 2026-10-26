//go:build js

package device

import (
	"errors"
	"strings"
	"syscall/js"
	"unsafe"
)

// The 3D contract in a browser. Meshes and programs are made by direct calls
// into web.js; everything given between Begin3D and End3D is a command in the
// buffer the 2D goes through, so 2D and 3D are drawn in their order, and a
// frame's 3D crosses into JavaScript once instead of call by call: in stage 0
// of M7, 10,000 draws took 41 ms called one by one and 13 ms as commands
// (see docs/3d.md).

// float32Bytes returns the memory of values as bytes, without copying.
func float32Bytes(values []float32) []byte {
	if len(values) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&values[0])), len(values)*4)
}

func uint32Bytes(values []uint32) []byte {
	if len(values) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&values[0])), len(values)*4)
}

// jsBytes copies b into a new Uint8Array.
func jsBytes(b []byte) js.Value {
	array := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(array, b)
	return array
}

// NewMesh puts a mesh on the graphics card: vertices in layout, and indices
// into them, three a triangle, or two a line when lines is true.
func NewMesh(layout int, vertices []float32, indices []uint32, lines bool) Mesh {
	flush()
	id := js_().Call("newMesh", layout, jsBytes(float32Bytes(vertices)), jsBytes(uint32Bytes(indices)), lines).Int()
	return Mesh{ID: uint32(id), count: int32(len(indices)), lines: lines}
}

// UpdateMesh replaces what mesh holds, in the same layout, such as lines
// gathered again every frame.
func UpdateMesh(mesh *Mesh, vertices []float32, indices []uint32) {
	// Draws of the mesh already in the buffer must see what it held.
	flush()
	js_().Call("updateMesh", int(mesh.ID), jsBytes(float32Bytes(vertices)), jsBytes(uint32Bytes(indices)))
	mesh.count = int32(len(indices))
}

// UnloadMesh frees a mesh.
func UnloadMesh(mesh Mesh) {
	if mesh.ID == 0 {
		return
	}
	flush()
	js_().Call("unloadMesh", int(mesh.ID))
}

// NewProgram compiles a vertex and a fragment shader, written for GLSL 3.30
// as on the desktop, and links them, or says why it couldn't. Both are turned
// into GLSL ES 3.00 first, which is what browsers compile.
func NewProgram(vertex, fragment string) (Program, error) {
	result := js_().Call("newProgram3D", es3D(vertex), es3D(fragment))
	if id := result.Index(0).Int(); id >= 0 {
		return Program{ID: uint32(id)}, nil
	}
	return Program{}, shaderError(result.Index(1).String())
}

// es3D turns a 3D shader into GLSL ES as ESShader does, and gives shadow
// samplers the precision GLSL ES asks every sampler but sampler2D to declare.
func es3D(source string) string {
	es := ESShader(source)
	return strings.Replace(es, "precision highp float;\n", "precision highp float;\nprecision highp sampler2DShadow;\n", 1)
}

// shaderError is what the browser said about a shader that didn't compile.
type shaderError string

func (e shaderError) Error() string { return string(e) }

// UnloadProgram frees a program.
func UnloadProgram(program Program) {
	if program.ID == 0 {
		return
	}
	flush()
	js_().Call("unloadProgram3D", int(program.ID))
}

// UniformLocation returns where the uniform called name sits in program, as a
// number both halves of the backend keep, or -1 when the program doesn't have
// it: compilers drop the ones a program doesn't use.
func UniformLocation(program Program, name string) int32 {
	return int32(js_().Call("location3D", int(program.ID), name).Int())
}

// Begin3D starts a 3D pass over the target being drawn on: what follows goes
// into a target of its own, in half floats where the browser can render into
// them, the same size, cleared to nothing, with depth testing and back faces
// culled, blending off. End3D puts it over the 2D.
func Begin3D() {
	push(opBegin3D)
}

// End3D ends the pass Begin3D started: it draws the pass's picture over the
// target the pass was drawn over, through compose, a program whose vertex
// shader covers the view and whose fragment shader reads the picture from
// texture unit 0 and gives colors premultiplied by their opacity. Then it
// leaves WebGL as the 2D expects it.
func End3D(compose Program) {
	push(opEnd3D, float32(compose.ID))
}

// NewShadowMap makes a shadow map width by height texels, which a pass can
// draw several views into, side by side.
func NewShadowMap(width, height int) ShadowMap {
	flush()
	id := js_().Call("newShadowMap", width, height).Int()
	return ShadowMap{ID: uint32(id), Width: width, Height: height}
}

// UnloadShadowMap frees a shadow map.
func UnloadShadowMap(m ShadowMap) {
	if m.ID == 0 {
		return
	}
	flush()
	js_().Call("unloadShadowMap", int(m.ID))
}

// BeginShadow draws what follows into the part of m from x, y, width by
// height texels, depth only, clearing the whole map first when clear is
// true. EndShadow goes back to the pass.
func BeginShadow(m ShadowMap, x, y, width, height int, clear bool) {
	push(opBeginShadow, float32(m.ID), float32(x), float32(y), float32(width), float32(height), onOff(clear))
}

// EndShadow goes back to drawing the pass, after BeginShadow.
func EndShadow() {
	push(opEndShadow)
}

// NewWorkTarget makes a work target width by height.
func NewWorkTarget(width, height int) WorkTarget {
	flush()
	id := js_().Call("newWorkTarget", width, height).Int()
	return WorkTarget{ID: uint32(id), Width: width, Height: height}
}

// UnloadWorkTarget frees a work target.
func UnloadWorkTarget(t WorkTarget) {
	if t.ID == 0 {
		return
	}
	flush()
	js_().Call("unloadWorkTarget", int(t.ID))
}

// BeginWork draws what follows into t, with no depth testing, clearing it
// to nothing first when clear is true. EndWork goes back to the pass.
func BeginWork(t WorkTarget, clear bool) {
	push(opBeginWork, float32(t.ID), onOff(clear))
}

// EndWork goes back to drawing the pass, after BeginWork, with depth
// testing.
func EndWork() {
	push(opEndWork)
}

// BindWorkTarget makes t's picture the one a sampler set to unit reads.
func BindWorkTarget(unit int, t WorkTarget) {
	push(opBindWorkTarget, float32(unit), float32(t.ID))
}

// BindPassPicture makes what the pass has drawn so far the picture a
// sampler set to unit reads, while drawing into a work target.
func BindPassPicture(unit int) {
	push(opBindPassPicture, float32(unit))
}

// SetAdditive3D says whether what is drawn next adds its color to what is
// there, as glows pile up, rather than replacing it.
func SetAdditive3D(on bool) {
	push(opAdditive3D, onOff(on))
}

// BindShadowMap makes m the one a shadow sampler uniform set to unit reads.
func BindShadowMap(unit int, m ShadowMap) {
	push(opBindShadowMap, float32(unit), float32(m.ID))
}

// DrawFullscreen draws one triangle over the whole view with the program in
// use, whose vertex shader places its three corners from gl_VertexID.
func DrawFullscreen() {
	push(opDrawFullscreen)
}

// UseProgram draws what follows through program.
func UseProgram(program Program) {
	push(opUseProgram, float32(program.ID))
}

// SetUniformMatrix sets a mat4 uniform of the program in use, from 16 floats
// in column-major order.
func SetUniformMatrix(location int32, matrix *[16]float32) {
	if location < 0 {
		return
	}
	push(opUniformMatrix, float32(location))
	for _, v := range matrix {
		putf(v)
	}
}

// SetUniform sets a float, vec2, vec3 or vec4 uniform of the program in use,
// from the first count values.
func SetUniform(location int32, count int, values [4]float32) {
	if location < 0 || count < 1 || count > 4 {
		return
	}
	push(opUniform, float32(location), float32(count), values[0], values[1], values[2], values[3])
}

// SetUniformVec4s sets a uniform array of vec4 of the program in use, from
// four values for each, as many as values holds.
func SetUniformVec4s(location int32, values []float32) {
	count := len(values) / 4
	if location < 0 || count == 0 {
		return
	}
	push(opUniformVec4s, float32(location), float32(count))
	for _, v := range values[:count*4] {
		putf(v)
	}
}

// SetUniformInt sets an int or sampler uniform of the program in use.
func SetUniformInt(location, value int32) {
	if location < 0 {
		return
	}
	push(opUniformInt, float32(location), float32(value))
}

func onOff(on bool) float32 {
	if on {
		return 1
	}
	return 0
}

// SetDepthWrite says whether what is drawn next hides what is drawn behind
// it later. See-through things are drawn without, after the rest.
func SetDepthWrite(on bool) {
	push(opDepthWrite, onOff(on))
}

// SetColorWrite says whether what is drawn next changes the pass's colors;
// without, it changes only its depth, as a see-through model does first so
// that only its front shows.
func SetColorWrite(on bool) {
	push(opColorWrite, onOff(on))
}

// SetBlend3D says whether what is drawn next is blended over what is behind
// it by its opacity, as see-through things are.
func SetBlend3D(on bool) {
	push(opBlend3D, onOff(on))
}

// SetCulling says whether the back faces of what is drawn next are left out,
// as they are for closed shapes; flat things seen from both sides keep them.
func SetCulling(on bool) {
	push(opCulling, onOff(on))
}

// DrawMeshInstanced draws a lit mesh once for each instance, with the program
// in use: instances holds InstanceFloats numbers for each. They travel in the
// command, and web.js hands them to the graphics card.
func DrawMeshInstanced(mesh Mesh, instances []float32) {
	count := len(instances) / InstanceFloats
	if mesh.ID == 0 || mesh.count == 0 || count == 0 {
		return
	}
	push(opDrawInstanced, float32(mesh.ID), float32(count))
	putFloats(instances[:count*InstanceFloats])
}

// NewDataTexture puts values on the graphics card as a DataTexture, four to
// a texel, padded with zeros to whole rows.
func NewDataTexture(values []float32) DataTexture {
	flush()
	id := js_().Call("newDataTexture", jsBytes(float32Bytes(values))).Int()
	return DataTexture{ID: uint32(id)}
}

// UpdateDataTexture replaces what t holds with values, which may be more
// than it held: what is drawn after it reads the new values. The values
// travel in the command, so draws already given still read the old ones.
func UpdateDataTexture(t *DataTexture, values []float32) {
	count := len(values) / 4 * 4
	if t.ID == 0 || count == 0 {
		return
	}
	push(opUpdateData, float32(t.ID), float32(count))
	putFloats(values[:count])
}

// UnloadDataTexture frees a data texture.
func UnloadDataTexture(t DataTexture) {
	if t.ID == 0 {
		return
	}
	flush()
	js_().Call("unloadDataTexture", int(t.ID))
}

// BindDataTexture makes t the one a sampler uniform set to unit reads.
func BindDataTexture(unit int, t DataTexture) {
	push(opBindData, float32(unit), float32(t.ID))
}

// DecodedImage is a picture the browser has decoded, ready for NewTexture3D.
type DecodedImage struct {
	id            int // the page's number for it
	Width, Height int
}

// DecodeImage hands a PNG or JPEG file to the browser, which decodes it off
// the page's thread, and waits for it: in WebAssembly, which has one thread,
// Go decoded Sponza's pictures eighteen times slower (see docs/3d.md).
// Pictures asked for from several goroutines decode at the same time.
func DecodeImage(data []byte) (DecodedImage, error) {
	done := make(chan DecodedImage, 1)
	var answer js.Func
	answer = js.FuncOf(func(_ js.Value, args []js.Value) any {
		answer.Release()
		done <- DecodedImage{id: args[0].Int(), Width: args[1].Int(), Height: args[2].Int()}
		return nil
	})
	js_().Call("decodeImage", jsBytes(data), answer)
	img := <-done
	if img.id < 0 {
		return DecodedImage{}, errors.New("the browser could not decode the picture")
	}
	return img, nil
}

// NewTexture3D puts a decoded picture on the graphics card, with mipmaps.
// Its top row is where texture coordinates have V 0, as in glTF.
func NewTexture3D(img DecodedImage, options TextureOptions) Texture3D {
	if img.Width == 0 {
		return Texture3D{}
	}
	flush()
	id := js_().Call("newTexture3D", img.id, options.Linear, options.Nearest, options.NearestMipmaps,
		wrapMode(options.ClampS, options.MirrorS), wrapMode(options.ClampT, options.MirrorT)).Int()
	return Texture3D{ID: uint32(id), Width: img.Width, Height: img.Height}
}

// wrapMode is how web.js names a way to wrap: 0 repeat, 1 clamp, 2 mirror.
func wrapMode(clamp, mirror bool) int {
	switch {
	case clamp:
		return 1
	case mirror:
		return 2
	}
	return 0
}

// NewHDRTexture puts a picture brighter than white on the graphics card, in
// half floats, RGBA, with the mipmaps given: levels[0] is width by height,
// each next one half as large, the top row first. It is smoothed between
// texels and mipmaps. A panorama of the sky repeats across and is clamped
// down; anything else, such as a lightmap, is clamped both ways.
func NewHDRTexture(width, height int, levels [][]uint16, panorama bool) Texture3D {
	if len(levels) == 0 {
		return Texture3D{}
	}
	flush()
	var all []uint16
	for _, level := range levels {
		all = append(all, level...)
	}
	bytes := unsafe.Slice((*byte)(unsafe.Pointer(&all[0])), len(all)*2)
	id := js_().Call("newHDRTexture", width, height, jsBytes(bytes), len(levels), panorama).Int()
	return Texture3D{ID: uint32(id), Width: width, Height: height}
}

// UnloadTexture3D frees a texture.
func UnloadTexture3D(texture Texture3D) {
	if texture.ID == 0 {
		return
	}
	flush()
	js_().Call("unloadTexture", int(texture.ID))
}

// BindTexture3D makes texture the one a sampler uniform set to unit reads.
func BindTexture3D(unit int, texture Texture3D) {
	push(opBindTexture3D, float32(unit), float32(texture.ID))
}

// DrawMesh draws mesh with the program in use.
func DrawMesh(mesh Mesh) {
	if mesh.ID == 0 || mesh.count == 0 {
		return
	}
	push(opDrawMesh, float32(mesh.ID))
}
