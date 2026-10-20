package golib

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

	"golib/internal/device"
)

// The 3D a Draw asks for is gathered between Screen.Begin3D and Screen.End3D
// and drawn at End3D, all at once: what is solid first, then the outlines and
// lines, then what is see-through, farthest first, so that it blends over
// whatever is behind it.

// item3D is one mesh drawn in a 3D pass.
type item3D struct {
	mesh  *Mesh
	model matrix
	color Color
	wire  bool
}

// pass3D is what a Screen gathers between Begin3D and End3D.
type pass3D struct {
	open        bool
	camera      *Camera3D
	items       []item3D
	lines       []float32 // device.LayoutColored
	lineIndices []uint32
}

// Begin3D starts drawing the 3D world seen through camera, over what Draw
// has drawn so far: call it after Screen.Clear, draw meshes with
// [Screen.DrawMesh] and the other 3D methods, and call [Screen.End3D] before
// drawing the score and the menus over the 3D:
//
//	screen.Clear(golib.SkyBlue)
//	screen.Begin3D(s.camera)
//	screen.DrawMesh(s.ground, golib.Transform{Scale: golib.Vector3{X: 40, Z: 40}}, golib.DarkGreen)
//	screen.DrawMesh(s.ball, golib.Transform{Position: s.ball.Position}, golib.Orange)
//	screen.End3D()
//	screen.DrawText(score, 10, 10, 20, golib.White)
//
// The 3D is drawn at End3D, so 2D drawn between Begin3D and End3D ends up
// under it. A Draw can have several 3D passes, each with its own camera; each
// starts with nothing in front of it. A sun from above and a soft light from
// the sky light every mesh.
func (s *Screen) Begin3D(camera *Camera3D) {
	if s.pass.open {
		s.End3D()
	}
	if camera == nil {
		reportError(errors.New("golib: Screen.Begin3D got a nil camera: make one with golib.NewCamera3D"))
		return
	}
	if s.fills {
		camera.width, camera.height = s.width, s.height // the screen changes with the window
	} else if camera.width != s.width || camera.height != s.height {
		reportError(fmt.Errorf("golib: Screen.Begin3D got a camera for a %g by %g screen, but the screen is %g by %g: pass the screen's size to NewCamera3D", camera.width, camera.height, s.width, s.height))
	}
	s.pass = pass3D{
		open:        true,
		camera:      camera,
		items:       s.pass.items[:0],
		lines:       s.pass.lines[:0],
		lineIndices: s.pass.lineIndices[:0],
	}
}

// in3D says whether a 3D pass is open, and reports the mistake when it isn't.
func (s *Screen) in3D(call string) bool {
	if !s.pass.open {
		reportError(fmt.Errorf("golib: Screen.%s was called outside a 3D pass: call screen.Begin3D(camera) first, and screen.End3D() after the 3D", call))
		return false
	}
	return true
}

// DrawMesh draws mesh placed by transform, in color, lit by the sun and the
// sky. A color that is partly see-through, with A below 255, lets what is
// behind the mesh show through.
func (s *Screen) DrawMesh(mesh *Mesh, transform Transform, color Color) {
	if !s.in3D("DrawMesh") {
		return
	}
	if mesh == nil {
		reportError(errors.New("golib: Screen.DrawMesh got a nil mesh: make one with golib.NewBoxMesh or another New...Mesh"))
		return
	}
	s.pass.items = append(s.pass.items, item3D{mesh: mesh, model: transform.matrix(), color: color})
}

// DrawWireframe draws the edges of mesh, placed by transform, as lines one
// pixel wide, unlit: to see where something is, such as the box a game
// checks collisions with. Edges inside a flat side, such as the diagonals of
// a box's faces, are left out.
func (s *Screen) DrawWireframe(mesh *Mesh, transform Transform, color Color) {
	if !s.in3D("DrawWireframe") {
		return
	}
	if mesh == nil {
		reportError(errors.New("golib: Screen.DrawWireframe got a nil mesh: make one with golib.NewBoxMesh or another New...Mesh"))
		return
	}
	s.pass.items = append(s.pass.items, item3D{mesh: mesh, model: transform.matrix(), color: color, wire: true})
}

// DrawLine3D draws a line one pixel wide from one point of the world to
// another, unlit, hidden by what is in front of it: a laser, a path, or a
// direction seen while testing.
func (s *Screen) DrawLine3D(from, to Vector3, color Color) {
	if !s.in3D("DrawLine3D") {
		return
	}
	s.line(from, to, color)
}

func (s *Screen) line(from, to Vector3, color Color) {
	r, g, b, a := float32(color.R)/255, float32(color.G)/255, float32(color.B)/255, float32(color.A)/255
	first := uint32(len(s.pass.lines) / device.LayoutFloats[device.LayoutColored])
	s.pass.lines = append(s.pass.lines, from.X, from.Y, from.Z, r, g, b, a, to.X, to.Y, to.Z, r, g, b, a)
	s.pass.lineIndices = append(s.pass.lineIndices, first, first+1)
}

// DrawGrid draws a square grid of lines on the ground, centered on 0, 0, 0,
// with slices squares along each side, spacing meters apart: a floor to see
// where things are while a game has none.
//
//	screen.DrawGrid(20, 1, golib.Gray) // 20 by 20 meters, a line every meter
func (s *Screen) DrawGrid(slices int, spacing float32, color Color) {
	if !s.in3D("DrawGrid") {
		return
	}
	slices = max(1, slices)
	half := float32(slices) * spacing / 2
	for i := 0; i <= slices; i++ {
		at := -half + float32(i)*spacing
		s.line(Vector3{X: at, Z: -half}, Vector3{X: at, Z: half}, color)
		s.line(Vector3{X: -half, Z: at}, Vector3{X: half, Z: at}, color)
	}
}

// DrawAxes draws the world's three axes from position, length meters long:
// X in red, Y in green and Z in blue. Put them at 0, 0, 0, or at a model, to
// see which way is which.
func (s *Screen) DrawAxes(position Vector3, length float32) {
	if !s.in3D("DrawAxes") {
		return
	}
	s.line(position, position.Add(Vector3{X: length}), Red)
	s.line(position, position.Add(Vector3{Y: length}), Green)
	s.line(position, position.Add(Vector3{Z: length}), Blue)
}

// End3D draws the 3D gathered since Begin3D, and goes back to 2D: what is
// drawn after it goes over the 3D. Run ends a 3D pass the game left open at
// the end of Draw.
func (s *Screen) End3D() {
	if !s.pass.open {
		return
	}
	s.pass.open = false
	if err := renderer3D.load(); err != nil {
		reportError(err)
		return
	}
	// The 3D blends see-through things normally; the 2D's own mode comes
	// back after it.
	if s.blend != BlendNormal {
		device.EndBlend()
	}
	renderer3D.draw(&s.pass)
	if s.blend == BlendAdd {
		device.BeginBlendAdd()
	}
}

// The default light, until GoLib has lights a game places: a sun high behind
// and to the right of a camera facing +Z, as NewCamera3D's does and a
// third-person camera behind a player with no Yaw does, and the sky's soft
// light from above, all in linear color. A surface facing the sun shows its
// color as it is; the side away from the sun, at about half its brightness.
var (
	sunDirection = Vector3{X: 0.4, Y: -1, Z: 0.6}.Normalize()
	sunColor     = [4]float32{0.9, 0.9, 0.9}
	skyColor     = [4]float32{0.3, 0.3, 0.3}
	groundColor  = [4]float32{0.12, 0.12, 0.12}
)

// The shaders of the 3D, in GLSL 3.30; the web backend turns them into GLSL
// ES. Colors come in as the game gives them, in sRGB, and are lit in linear
// light.
const (
	litVertexShader = `#version 330
layout(location = 0) in vec3 vertexPosition;
layout(location = 1) in vec3 vertexNormal;
uniform mat4 model;
uniform mat4 normalMatrix;
uniform mat4 viewProjection;
out vec3 normal;

void main()
{
    normal = mat3(normalMatrix) * vertexNormal;
    gl_Position = viewProjection * model * vec4(vertexPosition, 1.0);
}
`
	litFragmentShader = `#version 330
in vec3 normal;
uniform vec4 color;
uniform vec3 sunDirection;
uniform vec3 sunColor;
uniform vec3 skyColor;
uniform vec3 groundColor;
out vec4 fragColor;

void main()
{
    vec3 albedo = pow(color.rgb, vec3(2.2));
    vec3 n = normalize(normal);
    if (!gl_FrontFacing) n = -n;
    float diffuse = max(dot(n, -sunDirection), 0.0);
    vec3 ambient = mix(groundColor, skyColor, n.y * 0.5 + 0.5);
    vec3 light = albedo * (sunColor * diffuse + ambient);
    fragColor = vec4(pow(min(light, vec3(1.0)), vec3(1.0 / 2.2)), color.a);
}
`
	lineVertexShader = `#version 330
layout(location = 0) in vec3 vertexPosition;
layout(location = 3) in vec4 vertexColor;
uniform mat4 model;
uniform mat4 viewProjection;
out vec4 tint;

void main()
{
    tint = vertexColor;
    gl_Position = viewProjection * model * vec4(vertexPosition, 1.0);
    // A hair nearer the camera, so a line on a surface, such as the grid on
    // the ground or a wireframe over its mesh, isn't half hidden by it.
    gl_Position.z -= 0.0001 * gl_Position.w;
}
`
	lineFragmentShader = `#version 330
in vec4 tint;
uniform vec4 color;
out vec4 fragColor;

void main()
{
    fragColor = tint * color;
}
`
)

// program3D is one of the 3D's programs, with where its uniforms sit.
type program3D struct {
	program                             device.Program
	model, normalMatrix, viewProjection int32
	color                               int32
	sunDirection, sunColor              int32
	skyColor, groundColor               int32
}

func newProgram3D(name, vertex, fragment string) (program3D, error) {
	program, err := device.NewProgram(vertex, fragment)
	if err != nil {
		return program3D{}, fmt.Errorf("golib: the 3D's %s shader doesn't compile on this graphics card: %w", name, err)
	}
	at := func(uniform string) int32 { return device.UniformLocation(program, uniform) }
	return program3D{
		program: program, model: at("model"), normalMatrix: at("normalMatrix"), viewProjection: at("viewProjection"),
		color: at("color"), sunDirection: at("sunDirection"), sunColor: at("sunColor"),
		skyColor: at("skyColor"), groundColor: at("groundColor"),
	}, nil
}

// state3D is what every 3D pass draws with: made when the first one ends,
// and freed with the window.
type state3D struct {
	loaded   bool
	lit      program3D
	lines    program3D
	lineMesh device.Mesh
}

var renderer3D state3D

// load makes the 3D's programs, the first time a 3D pass ends.
func (r *state3D) load() error {
	if r.loaded {
		return nil
	}
	var err error
	if r.lit, err = newProgram3D("lit", litVertexShader, litFragmentShader); err != nil {
		return err
	}
	if r.lines, err = newProgram3D("line", lineVertexShader, lineFragmentShader); err != nil {
		device.UnloadProgram(r.lit.program)
		return err
	}
	r.loaded = true
	return nil
}

// unload frees the programs and the lines' mesh. The window must still be
// open.
func (r *state3D) unload() {
	if !r.loaded {
		return
	}
	device.UnloadProgram(r.lit.program)
	device.UnloadProgram(r.lines.program)
	device.UnloadMesh(r.lineMesh)
	*r = state3D{}
}

// colorFloats returns c as four numbers from 0 to 1.
func colorFloats(c Color) [4]float32 {
	return [4]float32{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255, float32(c.A) / 255}
}

// draw draws a pass.
func (r *state3D) draw(pass *pass3D) {
	view, projection := pass.camera.matrices()
	viewProjection := [16]float32(projection.mul(view))
	eye, _ := pass.camera.view()

	device.Begin3D()

	lit := r.lit
	device.UseProgram(lit.program)
	device.SetUniformMatrix(lit.viewProjection, &viewProjection)
	device.SetUniform(lit.sunDirection, 3, [4]float32{sunDirection.X, sunDirection.Y, sunDirection.Z})
	device.SetUniform(lit.sunColor, 3, sunColor)
	device.SetUniform(lit.skyColor, 3, skyColor)
	device.SetUniform(lit.groundColor, 3, groundColor)
	culling := true
	drawLit := func(item *item3D) {
		if item.mesh.twoSided == culling {
			culling = !item.mesh.twoSided
			device.SetCulling(culling)
		}
		model := [16]float32(item.model)
		normal := [16]float32(item.model.normalMatrix())
		device.SetUniformMatrix(lit.model, &model)
		device.SetUniformMatrix(lit.normalMatrix, &normal)
		device.SetUniform(lit.color, 4, colorFloats(item.color))
		device.DrawMesh(item.mesh.upload())
	}

	var wires, seeThrough []int
	for i := range pass.items {
		item := &pass.items[i]
		switch {
		case item.wire:
			wires = append(wires, i)
		case item.color.A < 255:
			seeThrough = append(seeThrough, i)
		default:
			drawLit(item)
		}
	}

	if len(wires) > 0 || len(pass.lines) > 0 {
		lines := r.lines
		device.UseProgram(lines.program)
		device.SetUniformMatrix(lines.viewProjection, &viewProjection)
		for _, i := range wires {
			item := &pass.items[i]
			model := [16]float32(item.model)
			device.SetUniformMatrix(lines.model, &model)
			device.SetUniform(lines.color, 4, colorFloats(item.color))
			device.DrawMesh(item.mesh.uploadWire())
		}
		if len(pass.lines) > 0 {
			if r.lineMesh.ID == 0 {
				r.lineMesh = device.NewMesh(device.LayoutColored, pass.lines, pass.lineIndices, true)
			} else {
				device.UpdateMesh(&r.lineMesh, pass.lines, pass.lineIndices)
			}
			identity := [16]float32(identityMatrix())
			device.SetUniformMatrix(lines.model, &identity)
			device.SetUniform(lines.color, 4, [4]float32{1, 1, 1, 1})
			device.DrawMesh(r.lineMesh)
		}
	}

	if len(seeThrough) > 0 {
		distance := func(i int) float32 {
			m := pass.items[i].model
			return Vector3{X: m[12], Y: m[13], Z: m[14]}.Distance(eye)
		}
		slices.SortStableFunc(seeThrough, func(a, b int) int {
			return cmp.Compare(distance(b), distance(a)) // farthest first
		})
		device.UseProgram(lit.program)
		device.SetDepthWrite(false)
		device.SetBlend3D(true)
		for _, i := range seeThrough {
			drawLit(&pass.items[i])
		}
		device.SetDepthWrite(true)
		device.SetBlend3D(false)
	}

	device.End3D()
}
