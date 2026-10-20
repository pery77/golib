//go:build js

package device

import (
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
	result := js_().Call("newProgram3D", ESShader(vertex), ESShader(fragment))
	if id := result.Index(0).Int(); id >= 0 {
		return Program{ID: uint32(id)}, nil
	}
	return Program{}, shaderError(result.Index(1).String())
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

// Begin3D starts drawing 3D into the target being drawn on, over the 2D
// drawn there so far: it clears the depth, and turns on depth testing and
// culling of back faces, with blending off.
func Begin3D() {
	push(opBegin3D)
}

// End3D ends the 3D Begin3D started, and leaves WebGL as the 2D expects it.
func End3D() {
	push(opEnd3D)
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

// DrawMesh draws mesh with the program in use.
func DrawMesh(mesh Mesh) {
	if mesh.ID == 0 || mesh.count == 0 {
		return
	}
	push(opDrawMesh, float32(mesh.ID))
}
