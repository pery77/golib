//go:build !js

package device

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg" // pictures in glTF files
	_ "image/png"
	"runtime"
	"strings"
	"unsafe"

	"github.com/ebitengine/purego"
	rl "github.com/gen2brain/raylib-go/raylib"
)

// The 3D contract on the desktop calls OpenGL 3.3 directly, through the
// function pointers raylib's loader found (rlGetProcAddress), with purego.
// Not rlgl: its indexed draws take 16-bit indices only, the binding lacks its
// texture upload, and its calls go through libffi, which costs more (see
// docs/3d.md). raylib keeps drawing the 2D, into the same targets.

const (
	glArrayBuffer        = 0x8892
	glElementArrayBuffer = 0x8893
	glStaticDraw         = 0x88E4
	glFloat              = 0x1406
	glTriangles          = 0x0004
	glLines              = 0x0001
	glUnsignedInt        = 0x1405
	glTexture0           = 0x84C0
	glDepthTest          = 0x0B71
	glCullFace           = 0x0B44
	glBlend              = 0x0BE2
	glBack               = 0x0405
	glCCW                = 0x0901
	glSrcAlpha           = 0x0302
	glOneMinusSrcAlpha   = 0x0303
	glVertexShader       = 0x8B31
	glFragmentShader     = 0x8B30
	glCompileStatus      = 0x8B81
	glLinkStatus         = 0x8B82
	glLEqual             = 0x0203
	glDepthBufferBit     = 0x0100
	glStreamDraw         = 0x88E0
	glTexture2D          = 0x0DE1
	glRGBA               = 0x1908
	glRGBA8              = 0x8058
	glSRGB8Alpha8        = 0x8C43
	glUnsignedByte       = 0x1401
	glTextureMinFilter   = 0x2801
	glTextureMagFilter   = 0x2800
	glNearest            = 0x2600
	glLinear             = 0x2601
	glNearestMipNearest  = 0x2700
	glLinearMipLinear    = 0x2703
	glTextureWrapS       = 0x2802
	glTextureWrapT       = 0x2803
	glRepeat             = 0x2901
	glClampToEdge        = 0x812F
	glMirroredRepeat     = 0x8370
	glMaxAnisotropy      = 0x84FE
	glUnpackAlignment    = 0x0CF5
)

// anisotropy is how many samples a texture takes along a surface seen at a
// slant; graphics cards that allow fewer use their most.
const anisotropy = 8

// gl holds the OpenGL functions this file calls, found once a window, and
// so an OpenGL context, exists.
var gl struct {
	loaded bool

	GenVertexArrays, BindVertexArray, DeleteVertexArrays                     uintptr
	GenBuffers, BindBuffer, BufferData, DeleteBuffers                        uintptr
	EnableVertexAttribArray, VertexAttribPointer                             uintptr
	DrawElements                                                             uintptr
	ActiveTexture                                                            uintptr
	CreateShader, ShaderSource, CompileShader, GetShaderiv, GetShaderInfoLog uintptr
	DeleteShader                                                             uintptr
	CreateProgram, AttachShader, LinkProgram, GetProgramiv                   uintptr
	GetProgramInfoLog, DeleteProgram                                         uintptr
	UseProgram, GetUniformLocation, Uniform1i                                uintptr
	Uniform1fv, Uniform2fv, Uniform3fv, Uniform4fv, UniformMatrix4fv         uintptr
	Enable, Disable, DepthFunc, DepthMask, Clear, CullFace, FrontFace        uintptr
	BlendFunc                                                                uintptr
	VertexAttribDivisor, DrawElementsInstanced                               uintptr
	GenTextures, BindTexture, TexImage2D, TexParameteri, GenerateMipmap      uintptr
	DeleteTextures, PixelStorei                                              uintptr

	// instances is the buffer every instanced draw fills with its
	// instances' matrices and tints, attached to every lit mesh.
	instances uint32
}

// loadGL finds the OpenGL functions, the first time a 3D call needs them.
func loadGL() {
	if gl.loaded {
		return
	}
	for _, f := range []struct {
		to   *uintptr
		name string
	}{
		{&gl.GenVertexArrays, "glGenVertexArrays"}, {&gl.BindVertexArray, "glBindVertexArray"},
		{&gl.DeleteVertexArrays, "glDeleteVertexArrays"},
		{&gl.GenBuffers, "glGenBuffers"}, {&gl.BindBuffer, "glBindBuffer"},
		{&gl.BufferData, "glBufferData"}, {&gl.DeleteBuffers, "glDeleteBuffers"},
		{&gl.EnableVertexAttribArray, "glEnableVertexAttribArray"},
		{&gl.VertexAttribPointer, "glVertexAttribPointer"},
		{&gl.DrawElements, "glDrawElements"}, {&gl.ActiveTexture, "glActiveTexture"},
		{&gl.CreateShader, "glCreateShader"}, {&gl.ShaderSource, "glShaderSource"},
		{&gl.CompileShader, "glCompileShader"}, {&gl.GetShaderiv, "glGetShaderiv"},
		{&gl.GetShaderInfoLog, "glGetShaderInfoLog"}, {&gl.DeleteShader, "glDeleteShader"},
		{&gl.CreateProgram, "glCreateProgram"}, {&gl.AttachShader, "glAttachShader"},
		{&gl.LinkProgram, "glLinkProgram"}, {&gl.GetProgramiv, "glGetProgramiv"},
		{&gl.GetProgramInfoLog, "glGetProgramInfoLog"}, {&gl.DeleteProgram, "glDeleteProgram"},
		{&gl.UseProgram, "glUseProgram"}, {&gl.GetUniformLocation, "glGetUniformLocation"},
		{&gl.Uniform1i, "glUniform1i"}, {&gl.Uniform1fv, "glUniform1fv"},
		{&gl.Uniform2fv, "glUniform2fv"}, {&gl.Uniform3fv, "glUniform3fv"},
		{&gl.Uniform4fv, "glUniform4fv"}, {&gl.UniformMatrix4fv, "glUniformMatrix4fv"},
		{&gl.Enable, "glEnable"}, {&gl.Disable, "glDisable"}, {&gl.DepthFunc, "glDepthFunc"},
		{&gl.DepthMask, "glDepthMask"}, {&gl.Clear, "glClear"}, {&gl.CullFace, "glCullFace"},
		{&gl.FrontFace, "glFrontFace"}, {&gl.BlendFunc, "glBlendFunc"},
		{&gl.VertexAttribDivisor, "glVertexAttribDivisor"}, {&gl.DrawElementsInstanced, "glDrawElementsInstanced"},
		{&gl.GenTextures, "glGenTextures"}, {&gl.BindTexture, "glBindTexture"},
		{&gl.TexImage2D, "glTexImage2D"}, {&gl.TexParameteri, "glTexParameteri"},
		{&gl.GenerateMipmap, "glGenerateMipmap"}, {&gl.DeleteTextures, "glDeleteTextures"},
		{&gl.PixelStorei, "glPixelStorei"},
	} {
		*f.to = uintptr(rl.GetProcAddress(f.name))
		if *f.to == 0 {
			// raylib asked for an OpenGL 3.3 context, which has every one
			// of these: a missing one means there is no context yet.
			panic("golib: OpenGL has no " + f.name + ": 3D needs a window open")
		}
	}
	glCall(gl.GenBuffers, 1, uintptr(unsafe.Pointer(&gl.instances)))
	gl.loaded = true
}

// forgetGL drops the functions and the instance buffer found in a window's
// OpenGL context, when the window closes.
func forgetGL() {
	gl.loaded = false
	gl.instances = 0
}

// glCall calls an OpenGL function with integer and pointer arguments.
func glCall(fn uintptr, args ...uintptr) uintptr {
	r, _, _ := purego.SyscallN(fn, args...)
	return r
}

func glBool(b bool) uintptr {
	if b {
		return 1
	}
	return 0
}

// glString returns s ending in the zero byte C strings end with. Keep the
// slice alive until the call that reads it returns.
func glString(s string) []byte {
	return append([]byte(s), 0)
}

// NewMesh puts a mesh on the graphics card: vertices in layout, and indices
// into them, three a triangle, or two a line when lines is true.
func NewMesh(layout int, vertices []float32, indices []uint32, lines bool) Mesh {
	loadGL()
	var m Mesh
	glCall(gl.GenVertexArrays, 1, uintptr(unsafe.Pointer(&m.ID)))
	glCall(gl.BindVertexArray, uintptr(m.ID))
	glCall(gl.GenBuffers, 1, uintptr(unsafe.Pointer(&m.vbo)))
	glCall(gl.BindBuffer, glArrayBuffer, uintptr(m.vbo))
	glCall(gl.GenBuffers, 1, uintptr(unsafe.Pointer(&m.ibo)))
	glCall(gl.BindBuffer, glElementArrayBuffer, uintptr(m.ibo))
	stride := uintptr(LayoutFloats[layout] * 4)
	attribute := func(location, size, offset uintptr) {
		glCall(gl.EnableVertexAttribArray, location)
		glCall(gl.VertexAttribPointer, location, size, glFloat, 0, stride, offset*4)
	}
	switch layout {
	case LayoutLit:
		attribute(0, 3, 0)
		attribute(1, 3, 3)
		attribute(2, 2, 6)
		attribute(3, 4, 8)
		glCall(gl.BindBuffer, glArrayBuffer, uintptr(gl.instances))
		for i := uintptr(0); i < 5; i++ { // the matrix's four columns, then the tint
			glCall(gl.EnableVertexAttribArray, 4+i)
			glCall(gl.VertexAttribPointer, 4+i, 4, glFloat, 0, InstanceFloats*4, i*16)
			glCall(gl.VertexAttribDivisor, 4+i, 1)
		}
		glCall(gl.BindBuffer, glArrayBuffer, uintptr(m.vbo))
	case LayoutColored:
		attribute(0, 3, 0)
		attribute(3, 4, 3)
	}
	m.lines = lines
	fillMesh(&m, vertices, indices)
	glCall(gl.BindVertexArray, 0)
	return m
}

// fillMesh puts vertices and indices into the buffers of m, which is bound.
func fillMesh(m *Mesh, vertices []float32, indices []uint32) {
	var vertexData, indexData uintptr
	if len(vertices) > 0 {
		vertexData = uintptr(unsafe.Pointer(&vertices[0]))
	}
	if len(indices) > 0 {
		indexData = uintptr(unsafe.Pointer(&indices[0]))
	}
	glCall(gl.BufferData, glArrayBuffer, uintptr(len(vertices)*4), vertexData, glStaticDraw)
	glCall(gl.BufferData, glElementArrayBuffer, uintptr(len(indices)*4), indexData, glStaticDraw)
	runtime.KeepAlive(vertices)
	runtime.KeepAlive(indices)
	m.count = int32(len(indices))
}

// UpdateMesh replaces what mesh holds, in the same layout, such as lines
// gathered again every frame.
func UpdateMesh(mesh *Mesh, vertices []float32, indices []uint32) {
	glCall(gl.BindVertexArray, uintptr(mesh.ID))
	glCall(gl.BindBuffer, glArrayBuffer, uintptr(mesh.vbo))
	fillMesh(mesh, vertices, indices)
	glCall(gl.BindVertexArray, 0)
}

// UnloadMesh frees a mesh.
func UnloadMesh(mesh Mesh) {
	if mesh.ID == 0 {
		return
	}
	buffers := [2]uint32{mesh.vbo, mesh.ibo}
	glCall(gl.DeleteBuffers, 2, uintptr(unsafe.Pointer(&buffers[0])))
	glCall(gl.DeleteVertexArrays, 1, uintptr(unsafe.Pointer(&mesh.ID)))
}

// NewProgram compiles a vertex and a fragment shader, written for GLSL 3.30,
// and links them, or says why it couldn't.
func NewProgram(vertex, fragment string) (Program, error) {
	loadGL()
	vs, err := compileShader(glVertexShader, vertex)
	if err != nil {
		return Program{}, fmt.Errorf("vertex shader: %w", err)
	}
	defer glCall(gl.DeleteShader, vs)
	fs, err := compileShader(glFragmentShader, fragment)
	if err != nil {
		return Program{}, fmt.Errorf("fragment shader: %w", err)
	}
	defer glCall(gl.DeleteShader, fs)
	id := glCall(gl.CreateProgram)
	glCall(gl.AttachShader, id, vs)
	glCall(gl.AttachShader, id, fs)
	glCall(gl.LinkProgram, id)
	var ok int32
	glCall(gl.GetProgramiv, id, glLinkStatus, uintptr(unsafe.Pointer(&ok)))
	if ok == 0 {
		log := infoLog(gl.GetProgramInfoLog, id)
		glCall(gl.DeleteProgram, id)
		return Program{}, errors.New(log)
	}
	return Program{ID: uint32(id)}, nil
}

func compileShader(kind uintptr, source string) (uintptr, error) {
	shader := glCall(gl.CreateShader, kind)
	text := glString(source)
	pointer := uintptr(unsafe.Pointer(&text[0]))
	glCall(gl.ShaderSource, shader, 1, uintptr(unsafe.Pointer(&pointer)), 0)
	runtime.KeepAlive(text)
	glCall(gl.CompileShader, shader)
	var ok int32
	glCall(gl.GetShaderiv, shader, glCompileStatus, uintptr(unsafe.Pointer(&ok)))
	if ok == 0 {
		log := infoLog(gl.GetShaderInfoLog, shader)
		glCall(gl.DeleteShader, shader)
		return 0, errors.New(log)
	}
	return shader, nil
}

// infoLog returns what the graphics card said about a shader or a program.
func infoLog(get uintptr, id uintptr) string {
	log := make([]byte, 4096)
	var length int32
	glCall(get, id, uintptr(len(log)), uintptr(unsafe.Pointer(&length)), uintptr(unsafe.Pointer(&log[0])))
	if text := strings.TrimSpace(string(log[:length])); text != "" {
		return text
	}
	return "the graphics card gave no reason"
}

// UnloadProgram frees a program.
func UnloadProgram(program Program) {
	if program.ID != 0 {
		glCall(gl.DeleteProgram, uintptr(program.ID))
	}
}

// UniformLocation returns where the uniform called name sits in program, or
// -1 when the program doesn't have it: compilers drop the ones a program
// doesn't use.
func UniformLocation(program Program, name string) int32 {
	text := glString(name)
	location := int32(glCall(gl.GetUniformLocation, uintptr(program.ID), uintptr(unsafe.Pointer(&text[0]))))
	runtime.KeepAlive(text)
	return location
}

// Begin3D starts drawing 3D into the target being drawn on, over the 2D
// drawn there so far: it sends raylib's waiting 2D first, clears the depth,
// and turns on depth testing and culling of back faces, with blending off.
func Begin3D() {
	loadGL()
	rl.DrawRenderBatchActive()
	glCall(gl.Enable, glDepthTest)
	glCall(gl.DepthFunc, glLEqual)
	glCall(gl.DepthMask, 1)
	glCall(gl.Clear, glDepthBufferBit)
	glCall(gl.Enable, glCullFace)
	glCall(gl.CullFace, glBack)
	glCall(gl.FrontFace, glCCW)
	glCall(gl.Disable, glBlend)
}

// End3D ends the 3D Begin3D started, and leaves OpenGL as raylib expects it
// for the 2D drawn after: no depth testing, back faces culled, normal
// blending on, texture unit 0, and no program or vertex array of the 3D.
func End3D() {
	glCall(gl.BindVertexArray, 0)
	glCall(gl.Disable, glDepthTest)
	glCall(gl.DepthMask, 1)
	glCall(gl.Enable, glCullFace)
	glCall(gl.BlendFunc, glSrcAlpha, glOneMinusSrcAlpha)
	glCall(gl.Enable, glBlend)
	glCall(gl.ActiveTexture, glTexture0)
	glCall(gl.UseProgram, 0)
}

// UseProgram draws what follows through program.
func UseProgram(program Program) {
	glCall(gl.UseProgram, uintptr(program.ID))
}

// SetUniformMatrix sets a mat4 uniform of the program in use, from 16
// floats in column-major order.
func SetUniformMatrix(location int32, matrix *[16]float32) {
	if location < 0 {
		return
	}
	glCall(gl.UniformMatrix4fv, uintptr(location), 1, 0, uintptr(unsafe.Pointer(matrix)))
	runtime.KeepAlive(matrix)
}

// SetUniform sets a float, vec2, vec3 or vec4 uniform of the program in use,
// from the first count values.
func SetUniform(location int32, count int, values [4]float32) {
	if location < 0 || count < 1 || count > 4 {
		return
	}
	fn := [...]uintptr{gl.Uniform1fv, gl.Uniform2fv, gl.Uniform3fv, gl.Uniform4fv}[count-1]
	glCall(fn, uintptr(location), 1, uintptr(unsafe.Pointer(&values[0])))
	runtime.KeepAlive(&values)
}

// SetUniformInt sets an int or sampler uniform of the program in use.
func SetUniformInt(location, value int32) {
	if location < 0 {
		return
	}
	glCall(gl.Uniform1i, uintptr(location), uintptr(value))
}

// SetDepthWrite says whether what is drawn next hides what is drawn behind
// it later. See-through things are drawn without, after the rest.
func SetDepthWrite(on bool) {
	glCall(gl.DepthMask, glBool(on))
}

// SetBlend3D says whether what is drawn next is blended over what is behind
// it by its opacity, as see-through things are.
func SetBlend3D(on bool) {
	if on {
		glCall(gl.BlendFunc, glSrcAlpha, glOneMinusSrcAlpha)
		glCall(gl.Enable, glBlend)
	} else {
		glCall(gl.Disable, glBlend)
	}
}

// SetCulling says whether the back faces of what is drawn next are left out,
// as they are for closed shapes; flat things seen from both sides keep them.
func SetCulling(on bool) {
	if on {
		glCall(gl.Enable, glCullFace)
	} else {
		glCall(gl.Disable, glCullFace)
	}
}

// DrawMeshInstanced draws a lit mesh once for each instance, with the program
// in use: instances holds InstanceFloats numbers for each.
func DrawMeshInstanced(mesh Mesh, instances []float32) {
	count := len(instances) / InstanceFloats
	if mesh.ID == 0 || mesh.count == 0 || count == 0 {
		return
	}
	glCall(gl.BindBuffer, glArrayBuffer, uintptr(gl.instances))
	glCall(gl.BufferData, glArrayBuffer, uintptr(count*InstanceFloats*4), uintptr(unsafe.Pointer(&instances[0])), glStreamDraw)
	runtime.KeepAlive(instances)
	glCall(gl.BindVertexArray, uintptr(mesh.ID))
	glCall(gl.DrawElementsInstanced, glTriangles, uintptr(mesh.count), glUnsignedInt, 0, uintptr(count))
}

// DecodedImage is a picture read from a PNG or JPEG file, ready for
// NewTexture3D.
type DecodedImage struct {
	pixels        []byte // RGBA, not premultiplied, the top row first
	Width, Height int
}

// DecodeImage reads a PNG or JPEG file. It may run on any goroutine, so a
// model's pictures decode at the same time.
func DecodeImage(data []byte) (DecodedImage, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return DecodedImage{}, err
	}
	b := img.Bounds()
	nrgba, ok := img.(*image.NRGBA)
	if !ok || nrgba.Stride != b.Dx()*4 || b.Min != (image.Point{}) {
		nrgba = image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(nrgba, nrgba.Bounds(), img, b.Min, draw.Src)
	}
	return DecodedImage{pixels: nrgba.Pix, Width: b.Dx(), Height: b.Dy()}, nil
}

// NewTexture3D puts a decoded picture on the graphics card, with mipmaps.
// Its top row is where texture coordinates have V 0, as in glTF.
func NewTexture3D(img DecodedImage, options TextureOptions) Texture3D {
	if img.Width == 0 || img.Height == 0 {
		return Texture3D{}
	}
	loadGL()
	var id uint32
	glCall(gl.GenTextures, 1, uintptr(unsafe.Pointer(&id)))
	glCall(gl.BindTexture, glTexture2D, uintptr(id))
	glCall(gl.PixelStorei, glUnpackAlignment, 1)
	format := uintptr(glSRGB8Alpha8)
	if options.Linear {
		format = glRGBA8
	}
	glCall(gl.TexImage2D, glTexture2D, 0, format, uintptr(img.Width), uintptr(img.Height), 0, glRGBA, glUnsignedByte, uintptr(unsafe.Pointer(&img.pixels[0])))
	runtime.KeepAlive(img.pixels)
	glCall(gl.GenerateMipmap, glTexture2D)
	minify, magnify := uintptr(glLinearMipLinear), uintptr(glLinear)
	if options.NearestMipmaps {
		minify = glNearestMipNearest
	}
	if options.Nearest {
		magnify = glNearest
	}
	glCall(gl.TexParameteri, glTexture2D, glTextureMinFilter, minify)
	glCall(gl.TexParameteri, glTexture2D, glTextureMagFilter, magnify)
	glCall(gl.TexParameteri, glTexture2D, glTextureWrapS, wrap(options.ClampS, options.MirrorS))
	glCall(gl.TexParameteri, glTexture2D, glTextureWrapT, wrap(options.ClampT, options.MirrorT))
	glCall(gl.TexParameteri, glTexture2D, glMaxAnisotropy, anisotropy)
	glCall(gl.BindTexture, glTexture2D, 0)
	return Texture3D{ID: id, Width: img.Width, Height: img.Height}
}

func wrap(clamp, mirror bool) uintptr {
	switch {
	case clamp:
		return glClampToEdge
	case mirror:
		return glMirroredRepeat
	}
	return glRepeat
}

// UnloadTexture3D frees a texture.
func UnloadTexture3D(texture Texture3D) {
	if texture.ID != 0 {
		glCall(gl.DeleteTextures, 1, uintptr(unsafe.Pointer(&texture.ID)))
	}
}

// BindTexture3D makes texture the one a sampler uniform set to unit reads.
func BindTexture3D(unit int, texture Texture3D) {
	glCall(gl.ActiveTexture, glTexture0+uintptr(unit))
	glCall(gl.BindTexture, glTexture2D, uintptr(texture.ID))
}

// DrawMesh draws mesh with the program in use.
func DrawMesh(mesh Mesh) {
	if mesh.ID == 0 || mesh.count == 0 {
		return
	}
	mode := uintptr(glTriangles)
	if mesh.lines {
		mode = glLines
	}
	glCall(gl.BindVertexArray, uintptr(mesh.ID))
	glCall(gl.DrawElements, mode, uintptr(mesh.count), glUnsignedInt, 0)
}
