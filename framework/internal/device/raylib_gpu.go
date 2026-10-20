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
	glFramebuffer        = 0x8D40
	glFramebufferBinding = 0x8CA6
	glViewportParameter  = 0x0BA2
	glColorAttachment0   = 0x8CE0
	glDepthAttachment    = 0x8D00
	glRenderbuffer       = 0x8D41
	glDepthComponent24   = 0x81A6
	glRGBA16F            = 0x881A
	glHalfFloat          = 0x140B
	glDepthComponent     = 0x1902
	glCompareMode        = 0x884C
	glCompareRefToTex    = 0x884E
	glCompareFunc        = 0x884D
	glNone               = 0
	glTextureMaxLevel    = 0x813D
	glColor              = 0x1800
	glDepth              = 0x1801
)

// anisotropy is how many samples a texture takes along a surface seen at a
// slant; graphics cards that allow fewer use their most.
const anisotropy = 8

// scratchUnit is the texture unit textures are made on, which no program
// samples: making one mid-pass leaves the units bound for drawing alone.
const scratchUnit = 7

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
	GetIntegerv, BindFramebuffer, GenFramebuffers, DeleteFramebuffers        uintptr
	FramebufferTexture2D, GenRenderbuffers, BindRenderbuffer                 uintptr
	RenderbufferStorage, FramebufferRenderbuffer, DeleteRenderbuffers        uintptr
	Viewport, ClearBufferfv, BlendFuncSeparate, DrawArrays                   uintptr
	DrawBuffer, ReadBuffer                                                   uintptr

	// empty is a vertex array with nothing in it, for a triangle over the
	// whole view whose corners the vertex shader works out.
	empty uint32

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
		{&gl.GetIntegerv, "glGetIntegerv"}, {&gl.BindFramebuffer, "glBindFramebuffer"},
		{&gl.GenFramebuffers, "glGenFramebuffers"}, {&gl.DeleteFramebuffers, "glDeleteFramebuffers"},
		{&gl.FramebufferTexture2D, "glFramebufferTexture2D"}, {&gl.GenRenderbuffers, "glGenRenderbuffers"},
		{&gl.BindRenderbuffer, "glBindRenderbuffer"}, {&gl.RenderbufferStorage, "glRenderbufferStorage"},
		{&gl.FramebufferRenderbuffer, "glFramebufferRenderbuffer"}, {&gl.DeleteRenderbuffers, "glDeleteRenderbuffers"},
		{&gl.Viewport, "glViewport"}, {&gl.ClearBufferfv, "glClearBufferfv"},
		{&gl.BlendFuncSeparate, "glBlendFuncSeparate"}, {&gl.DrawArrays, "glDrawArrays"},
		{&gl.DrawBuffer, "glDrawBuffer"}, {&gl.ReadBuffer, "glReadBuffer"},
	} {
		*f.to = uintptr(rl.GetProcAddress(f.name))
		if *f.to == 0 {
			// raylib asked for an OpenGL 3.3 context, which has every one
			// of these: a missing one means there is no context yet.
			panic("golib: OpenGL has no " + f.name + ": 3D needs a window open")
		}
	}
	glCall(gl.GenBuffers, 1, uintptr(unsafe.Pointer(&gl.instances)))
	glCall(gl.GenVertexArrays, 1, uintptr(unsafe.Pointer(&gl.empty)))
	gl.loaded = true
}

// forgetGL drops the functions and the instance buffer found in a window's
// OpenGL context, when the window closes.
func forgetGL() {
	gl.loaded = false
	gl.instances, gl.empty = 0, 0
	target3D = struct {
		fbo, color, depth uint32
		width, height     int32
		previous          int32
		viewport          [4]int32
	}{}
}

// target3D is what a 3D pass draws into: colors in half floats, brighter
// than white where the light is, and a depth buffer, the size of the target
// the pass is drawn over. End3D puts it over that target, tonemapped.
var target3D struct {
	fbo, color, depth uint32
	width, height     int32
	previous          int32    // the framebuffer the pass is drawn over
	viewport          [4]int32 // and its viewport
}

// prepareTarget3D makes target3D width by height, or makes it again at a new
// size.
func prepareTarget3D(width, height int32) {
	if target3D.fbo != 0 && target3D.width == width && target3D.height == height {
		return
	}
	if target3D.fbo != 0 {
		glCall(gl.DeleteFramebuffers, 1, uintptr(unsafe.Pointer(&target3D.fbo)))
		glCall(gl.DeleteTextures, 1, uintptr(unsafe.Pointer(&target3D.color)))
		glCall(gl.DeleteRenderbuffers, 1, uintptr(unsafe.Pointer(&target3D.depth)))
	}
	target3D.width, target3D.height = width, height
	glCall(gl.GenTextures, 1, uintptr(unsafe.Pointer(&target3D.color)))
	glCall(gl.ActiveTexture, glTexture0+scratchUnit)
	glCall(gl.BindTexture, glTexture2D, uintptr(target3D.color))
	glCall(gl.TexImage2D, glTexture2D, 0, glRGBA16F, uintptr(width), uintptr(height), 0, glRGBA, glHalfFloat, 0)
	// Smoothed, for what reads between its texels: the glow's first step,
	// and the compose shader smoothing edges.
	glCall(gl.TexParameteri, glTexture2D, glTextureMinFilter, glLinear)
	glCall(gl.TexParameteri, glTexture2D, glTextureMagFilter, glLinear)
	glCall(gl.TexParameteri, glTexture2D, glTextureWrapS, glClampToEdge)
	glCall(gl.TexParameteri, glTexture2D, glTextureWrapT, glClampToEdge)
	glCall(gl.BindTexture, glTexture2D, 0)
	glCall(gl.ActiveTexture, glTexture0)
	glCall(gl.GenRenderbuffers, 1, uintptr(unsafe.Pointer(&target3D.depth)))
	glCall(gl.BindRenderbuffer, glRenderbuffer, uintptr(target3D.depth))
	glCall(gl.RenderbufferStorage, glRenderbuffer, glDepthComponent24, uintptr(width), uintptr(height))
	glCall(gl.GenFramebuffers, 1, uintptr(unsafe.Pointer(&target3D.fbo)))
	glCall(gl.BindFramebuffer, glFramebuffer, uintptr(target3D.fbo))
	glCall(gl.FramebufferTexture2D, glFramebuffer, glColorAttachment0, glTexture2D, uintptr(target3D.color), 0)
	glCall(gl.FramebufferRenderbuffer, glFramebuffer, glDepthAttachment, glRenderbuffer, uintptr(target3D.depth))
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

// Begin3D starts a 3D pass over the target being drawn on: it sends
// raylib's waiting 2D first, and draws what follows into a target of its own,
// in half floats, the same size, cleared to nothing, with depth testing and
// back faces culled, blending off. End3D puts it over the 2D.
func Begin3D() {
	loadGL()
	rl.DrawRenderBatchActive()
	glCall(gl.GetIntegerv, glFramebufferBinding, uintptr(unsafe.Pointer(&target3D.previous)))
	glCall(gl.GetIntegerv, glViewportParameter, uintptr(unsafe.Pointer(&target3D.viewport[0])))
	prepareTarget3D(max(1, target3D.viewport[2]), max(1, target3D.viewport[3]))
	glCall(gl.BindFramebuffer, glFramebuffer, uintptr(target3D.fbo))
	glCall(gl.Viewport, 0, 0, uintptr(target3D.width), uintptr(target3D.height))
	glCall(gl.DepthMask, 1)
	nothing, far := [4]float32{}, float32(1)
	glCall(gl.ClearBufferfv, glColor, 0, uintptr(unsafe.Pointer(&nothing[0])))
	glCall(gl.ClearBufferfv, glDepth, 0, uintptr(unsafe.Pointer(&far)))
	glCall(gl.Enable, glDepthTest)
	glCall(gl.DepthFunc, glLEqual)
	glCall(gl.Enable, glCullFace)
	glCall(gl.CullFace, glBack)
	glCall(gl.FrontFace, glCCW)
	glCall(gl.Disable, glBlend)
}

// End3D ends the pass Begin3D started: it draws the pass's picture over the
// target the pass was drawn over, through compose, a program whose vertex
// shader covers the view (see DrawFullscreen) and whose fragment shader reads
// the picture from texture unit 0 and gives colors premultiplied by their
// opacity. Then it leaves OpenGL as raylib expects it for the 2D drawn after:
// no depth testing, back faces culled, normal blending on, texture unit 0,
// and no program or vertex array of the 3D.
func End3D(compose Program) {
	glCall(gl.BindFramebuffer, glFramebuffer, uintptr(target3D.previous))
	v := target3D.viewport
	glCall(gl.Viewport, uintptr(v[0]), uintptr(v[1]), uintptr(v[2]), uintptr(v[3]))
	glCall(gl.Disable, glDepthTest)
	glCall(gl.DepthMask, 1)
	glCall(gl.Disable, glCullFace)
	glCall(gl.BlendFunc, glOne, glOneMinusSrcAlpha)
	glCall(gl.Enable, glBlend)
	glCall(gl.UseProgram, uintptr(compose.ID))
	glCall(gl.ActiveTexture, glTexture0)
	glCall(gl.BindTexture, glTexture2D, uintptr(target3D.color))
	DrawFullscreen()
	glCall(gl.BindTexture, glTexture2D, 0)
	glCall(gl.BindVertexArray, 0)
	glCall(gl.Enable, glCullFace)
	glCall(gl.BlendFunc, glSrcAlpha, glOneMinusSrcAlpha)
	glCall(gl.UseProgram, 0)
}

// NewShadowMap makes a shadow map width by height texels, which a pass can
// draw several views into, side by side.
func NewShadowMap(width, height int) ShadowMap {
	loadGL()
	m := ShadowMap{Width: width, Height: height}
	glCall(gl.GenTextures, 1, uintptr(unsafe.Pointer(&m.texture)))
	glCall(gl.ActiveTexture, glTexture0+scratchUnit)
	glCall(gl.BindTexture, glTexture2D, uintptr(m.texture))
	glCall(gl.TexImage2D, glTexture2D, 0, glDepthComponent24, uintptr(width), uintptr(height), 0, glDepthComponent, glUnsignedInt, 0)
	glCall(gl.TexParameteri, glTexture2D, glTextureMinFilter, glLinear)
	glCall(gl.TexParameteri, glTexture2D, glTextureMagFilter, glLinear)
	glCall(gl.TexParameteri, glTexture2D, glTextureWrapS, glClampToEdge)
	glCall(gl.TexParameteri, glTexture2D, glTextureWrapT, glClampToEdge)
	glCall(gl.TexParameteri, glTexture2D, glCompareMode, glCompareRefToTex)
	glCall(gl.TexParameteri, glTexture2D, glCompareFunc, glLEqual)
	glCall(gl.BindTexture, glTexture2D, 0)
	glCall(gl.ActiveTexture, glTexture0)
	glCall(gl.GenFramebuffers, 1, uintptr(unsafe.Pointer(&m.ID)))
	glCall(gl.BindFramebuffer, glFramebuffer, uintptr(m.ID))
	glCall(gl.FramebufferTexture2D, glFramebuffer, glDepthAttachment, glTexture2D, uintptr(m.texture), 0)
	glCall(gl.DrawBuffer, glNone)
	glCall(gl.ReadBuffer, glNone)
	glCall(gl.BindFramebuffer, glFramebuffer, uintptr(target3D.fbo))
	return m
}

// UnloadShadowMap frees a shadow map.
func UnloadShadowMap(m ShadowMap) {
	if m.ID == 0 {
		return
	}
	glCall(gl.DeleteFramebuffers, 1, uintptr(unsafe.Pointer(&m.ID)))
	glCall(gl.DeleteTextures, 1, uintptr(unsafe.Pointer(&m.texture)))
}

// BeginShadow draws what follows into the part of m from x, y, width by
// height texels, depth only, clearing the whole map first when clear is
// true. EndShadow goes back to the pass.
func BeginShadow(m ShadowMap, x, y, width, height int, clear bool) {
	glCall(gl.BindFramebuffer, glFramebuffer, uintptr(m.ID))
	glCall(gl.Viewport, uintptr(x), uintptr(y), uintptr(width), uintptr(height))
	glCall(gl.DepthMask, 1)
	if clear {
		far := float32(1)
		glCall(gl.ClearBufferfv, glDepth, 0, uintptr(unsafe.Pointer(&far)))
	}
}

// EndShadow goes back to drawing the pass, after BeginShadow.
func EndShadow() {
	glCall(gl.BindFramebuffer, glFramebuffer, uintptr(target3D.fbo))
	glCall(gl.Viewport, 0, 0, uintptr(target3D.width), uintptr(target3D.height))
}

// NewWorkTarget makes a work target width by height.
func NewWorkTarget(width, height int) WorkTarget {
	loadGL()
	t := WorkTarget{Width: width, Height: height}
	glCall(gl.GenTextures, 1, uintptr(unsafe.Pointer(&t.texture)))
	glCall(gl.ActiveTexture, glTexture0+scratchUnit)
	glCall(gl.BindTexture, glTexture2D, uintptr(t.texture))
	glCall(gl.TexImage2D, glTexture2D, 0, glRGBA16F, uintptr(width), uintptr(height), 0, glRGBA, glHalfFloat, 0)
	glCall(gl.TexParameteri, glTexture2D, glTextureMinFilter, glLinear)
	glCall(gl.TexParameteri, glTexture2D, glTextureMagFilter, glLinear)
	glCall(gl.TexParameteri, glTexture2D, glTextureWrapS, glClampToEdge)
	glCall(gl.TexParameteri, glTexture2D, glTextureWrapT, glClampToEdge)
	glCall(gl.BindTexture, glTexture2D, 0)
	glCall(gl.ActiveTexture, glTexture0)
	glCall(gl.GenFramebuffers, 1, uintptr(unsafe.Pointer(&t.ID)))
	glCall(gl.BindFramebuffer, glFramebuffer, uintptr(t.ID))
	glCall(gl.FramebufferTexture2D, glFramebuffer, glColorAttachment0, glTexture2D, uintptr(t.texture), 0)
	glCall(gl.BindFramebuffer, glFramebuffer, uintptr(target3D.fbo))
	return t
}

// UnloadWorkTarget frees a work target.
func UnloadWorkTarget(t WorkTarget) {
	if t.ID == 0 {
		return
	}
	glCall(gl.DeleteFramebuffers, 1, uintptr(unsafe.Pointer(&t.ID)))
	glCall(gl.DeleteTextures, 1, uintptr(unsafe.Pointer(&t.texture)))
}

// BeginWork draws what follows into t, with no depth testing, clearing it
// to nothing first when clear is true. EndWork goes back to the pass.
func BeginWork(t WorkTarget, clear bool) {
	glCall(gl.BindFramebuffer, glFramebuffer, uintptr(t.ID))
	glCall(gl.Viewport, 0, 0, uintptr(t.Width), uintptr(t.Height))
	glCall(gl.Disable, glDepthTest)
	if clear {
		nothing := [4]float32{}
		glCall(gl.ClearBufferfv, glColor, 0, uintptr(unsafe.Pointer(&nothing[0])))
	}
}

// EndWork goes back to drawing the pass, after BeginWork, with depth
// testing.
func EndWork() {
	glCall(gl.BindFramebuffer, glFramebuffer, uintptr(target3D.fbo))
	glCall(gl.Viewport, 0, 0, uintptr(target3D.width), uintptr(target3D.height))
	glCall(gl.Enable, glDepthTest)
}

// BindWorkTarget makes t's picture the one a sampler set to unit reads.
func BindWorkTarget(unit int, t WorkTarget) {
	glCall(gl.ActiveTexture, glTexture0+uintptr(unit))
	glCall(gl.BindTexture, glTexture2D, uintptr(t.texture))
}

// BindPassPicture makes what the pass has drawn so far the picture a
// sampler set to unit reads, while drawing into a work target.
func BindPassPicture(unit int) {
	glCall(gl.ActiveTexture, glTexture0+uintptr(unit))
	glCall(gl.BindTexture, glTexture2D, uintptr(target3D.color))
}

// SetAdditive3D says whether what is drawn next adds its color to what is
// there, as glows pile up, rather than replacing it.
func SetAdditive3D(on bool) {
	if on {
		glCall(gl.BlendFunc, glOne, glOne)
		glCall(gl.Enable, glBlend)
	} else {
		glCall(gl.Disable, glBlend)
	}
}

// BindShadowMap makes m the one a shadow sampler uniform set to unit reads.
func BindShadowMap(unit int, m ShadowMap) {
	glCall(gl.ActiveTexture, glTexture0+uintptr(unit))
	glCall(gl.BindTexture, glTexture2D, uintptr(m.texture))
}

// DrawFullscreen draws one triangle over the whole view with the program in
// use, whose vertex shader places its three corners from gl_VertexID: a sky
// behind everything, or a picture put over another.
func DrawFullscreen() {
	glCall(gl.BindVertexArray, uintptr(gl.empty))
	glCall(gl.DrawArrays, glTriangles, 0, 3)
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

// SetUniformVec4s sets a uniform array of vec4 of the program in use, from
// four values for each, as many as values holds.
func SetUniformVec4s(location int32, values []float32) {
	if location < 0 || len(values) < 4 {
		return
	}
	glCall(gl.Uniform4fv, uintptr(location), uintptr(len(values)/4), uintptr(unsafe.Pointer(&values[0])))
	runtime.KeepAlive(values)
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
		// Opacity piles up as coverage, so End3D can put the pass over the
		// 2D with what shows through.
		glCall(gl.BlendFuncSeparate, glSrcAlpha, glOneMinusSrcAlpha, glOne, glOneMinusSrcAlpha)
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
	// A unit no program samples, so the textures bound for drawing stay.
	glCall(gl.ActiveTexture, glTexture0+scratchUnit)
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
	glCall(gl.ActiveTexture, glTexture0)
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

// NewHDRTexture puts a picture brighter than white on the graphics card, in
// half floats, RGBA, with the mipmaps given: levels[0] is width by height,
// each next one half as large, down to one texel, the top row first. It is
// smoothed between texels and mipmaps, repeated across and clamped down,
// as a panorama of the sky wants.
func NewHDRTexture(width, height int, levels [][]uint16) Texture3D {
	if len(levels) == 0 {
		return Texture3D{}
	}
	loadGL()
	var id uint32
	glCall(gl.GenTextures, 1, uintptr(unsafe.Pointer(&id)))
	glCall(gl.ActiveTexture, glTexture0+scratchUnit)
	glCall(gl.BindTexture, glTexture2D, uintptr(id))
	glCall(gl.PixelStorei, glUnpackAlignment, 1)
	w, h := width, height
	for level, data := range levels {
		glCall(gl.TexImage2D, glTexture2D, uintptr(level), glRGBA16F, uintptr(w), uintptr(h), 0, glRGBA, glHalfFloat, uintptr(unsafe.Pointer(&data[0])))
		runtime.KeepAlive(data)
		w, h = max(1, w/2), max(1, h/2)
	}
	glCall(gl.TexParameteri, glTexture2D, glTextureMaxLevel, uintptr(len(levels)-1))
	glCall(gl.TexParameteri, glTexture2D, glTextureMinFilter, glLinearMipLinear)
	glCall(gl.TexParameteri, glTexture2D, glTextureMagFilter, glLinear)
	glCall(gl.TexParameteri, glTexture2D, glTextureWrapS, glRepeat)
	glCall(gl.TexParameteri, glTexture2D, glTextureWrapT, glClampToEdge)
	glCall(gl.BindTexture, glTexture2D, 0)
	glCall(gl.ActiveTexture, glTexture0)
	return Texture3D{ID: id, Width: width, Height: height}
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
