package golib

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"

	"golib/internal/device"
)

// The 3D a Draw asks for is gathered between Screen.Begin3D and Screen.End3D
// and drawn at End3D, all at once. What the camera can't see is left out.
// What is solid is drawn first, every copy of the same mesh with the same
// material in one instanced draw; then the outlines and lines; then what is
// see-through, farthest first, so that it blends over whatever is behind it.

// material3D is how a surface looks: its colors, its textures and how it
// lets light and what is behind it through. Colors are linear.
type material3D struct {
	baseColor       [4]float32
	baseTexture     *texture3D // nil for none
	emissive        [3]float32
	emissiveTexture *texture3D
	alpha           alphaMode
	alphaCutoff     float32
	twoSided        bool // its back faces are drawn too
	unlit           bool // its color shows as it is, without the sun
}

type alphaMode int

const (
	alphaOpaque alphaMode = iota // opacity ignored
	alphaMask                    // cut out where opacity is below alphaCutoff
	alphaBlend                   // see-through by its opacity
)

// The materials of the meshes GoLib makes: white, so DrawMesh's color is the
// color.
var (
	plainMaterial         = &material3D{baseColor: [4]float32{1, 1, 1, 1}}
	plainTwoSidedMaterial = &material3D{baseColor: [4]float32{1, 1, 1, 1}, twoSided: true}
)

// texture3D is a picture a material shows, put on the graphics card the
// first time something with it is drawn.
type texture3D struct {
	data    []byte // the PNG or JPEG file, until it is on the graphics card
	options device.TextureOptions
	decoded device.DecodedImage
	err     error
	gpu     device.Texture3D
}

// item3D is one mesh drawn in a 3D pass.
type item3D struct {
	gpu      device.Mesh
	material *material3D
	model    matrix
	tint     Color
	center   Vector3 // the middle of its bounds, in the world
	radius   float32 // how far its bounds reach from center, in the world
	wire     *Mesh   // for DrawWireframe: the mesh whose edges are drawn
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
	material := plainMaterial
	if mesh.twoSided {
		material = plainTwoSidedMaterial
	}
	s.addItem(mesh, material, transform.matrix(), color)
}

// addItem adds mesh, drawn with material, placed by model.
func (s *Screen) addItem(mesh *Mesh, material *material3D, model matrix, tint Color) {
	gpu := mesh.upload()
	if gpu.ID == 0 {
		return // no triangles
	}
	mesh.bounds()
	s.pass.items = append(s.pass.items, item3D{
		gpu: gpu, material: material, model: model, tint: tint,
		center: model.transformPoint(mesh.center), radius: mesh.radius * model.largestScale(),
	})
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
	mesh.bounds()
	model := transform.matrix()
	s.pass.items = append(s.pass.items, item3D{
		wire: mesh, model: model, tint: color,
		center: model.transformPoint(mesh.center), radius: mesh.radius * model.largestScale(),
	})
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
// ES. Each instance brings its matrix and its tint; the tint comes in sRGB,
// as games give colors, and everything is lit in linear light.
const (
	litVertexShader = `#version 330
layout(location = 0) in vec3 vertexPosition;
layout(location = 1) in vec3 vertexNormal;
layout(location = 2) in vec2 vertexTexcoord;
layout(location = 3) in vec4 vertexColor;
layout(location = 4) in mat4 instanceModel;
layout(location = 8) in vec4 instanceTint;
uniform mat4 viewProjection;
out vec3 normal;
out vec2 texcoord;
out vec4 color;

void main()
{
    mat3 turn = mat3(instanceModel);
    normal = transpose(inverse(turn)) * vertexNormal;
    texcoord = vertexTexcoord;
    color = vertexColor * vec4(pow(instanceTint.rgb, vec3(2.2)), instanceTint.a);
    gl_Position = viewProjection * instanceModel * vec4(vertexPosition, 1.0);
}
`
	litFragmentShader = `#version 330
in vec3 normal;
in vec2 texcoord;
in vec4 color;
uniform vec4 baseColor;
uniform sampler2D baseTexture;
uniform int hasBaseTexture;
uniform vec3 emissive;
uniform sampler2D emissiveTexture;
uniform int hasEmissiveTexture;
uniform int alphaMode;
uniform float alphaCutoff;
uniform int unlit;
uniform vec3 sunDirection;
uniform vec3 sunColor;
uniform vec3 skyColor;
uniform vec3 groundColor;
out vec4 fragColor;

void main()
{
    vec4 albedo = baseColor * color;
    if (hasBaseTexture == 1) albedo *= texture(baseTexture, texcoord);
    if (alphaMode == 1 && albedo.a < alphaCutoff) discard;
    vec3 light = albedo.rgb;
    if (unlit == 0) {
        vec3 n = normalize(normal);
        if (!gl_FrontFacing) n = -n;
        float diffuse = max(dot(n, -sunDirection), 0.0);
        vec3 ambient = mix(groundColor, skyColor, n.y * 0.5 + 0.5);
        light *= sunColor * diffuse + ambient;
    }
    vec3 glow = emissive;
    if (hasEmissiveTexture == 1) glow *= texture(emissiveTexture, texcoord).rgb;
    light += glow;
    float alpha = alphaMode == 2 ? albedo.a : 1.0;
    fragColor = vec4(pow(min(light, vec3(1.0)), vec3(1.0 / 2.2)), alpha);
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

// litProgram is the program meshes are drawn with, and where its uniforms
// sit.
type litProgram struct {
	program                                  device.Program
	viewProjection                           int32
	baseColor, baseTexture, hasBaseTexture   int32
	emissive, emissiveTexture, hasEmissive   int32
	alphaMode, alphaCutoff, unlit            int32
	sunDirection, sunColor, skyColor, ground int32
}

// lineProgram is the program lines and wireframes are drawn with.
type lineProgram struct {
	program                      device.Program
	model, viewProjection, color int32
}

// state3D is what every 3D pass draws with: made when the first one ends,
// and freed with the window.
type state3D struct {
	loaded    bool
	lit       litProgram
	lines     lineProgram
	lineMesh  device.Mesh
	instances []float32 // the instances of the draw being built
}

var renderer3D state3D

func compile3D(name, vertex, fragment string) (device.Program, error) {
	program, err := device.NewProgram(vertex, fragment)
	if err != nil {
		return program, fmt.Errorf("golib: the 3D's %s shader doesn't compile on this graphics card: %w", name, err)
	}
	return program, nil
}

// load makes the 3D's programs, the first time a 3D pass ends.
func (r *state3D) load() error {
	if r.loaded {
		return nil
	}
	lit, err := compile3D("lit", litVertexShader, litFragmentShader)
	if err != nil {
		return err
	}
	lines, err := compile3D("line", lineVertexShader, lineFragmentShader)
	if err != nil {
		device.UnloadProgram(lit)
		return err
	}
	at := func(p device.Program, name string) int32 { return device.UniformLocation(p, name) }
	r.lit = litProgram{
		program: lit, viewProjection: at(lit, "viewProjection"),
		baseColor: at(lit, "baseColor"), baseTexture: at(lit, "baseTexture"), hasBaseTexture: at(lit, "hasBaseTexture"),
		emissive: at(lit, "emissive"), emissiveTexture: at(lit, "emissiveTexture"), hasEmissive: at(lit, "hasEmissiveTexture"),
		alphaMode: at(lit, "alphaMode"), alphaCutoff: at(lit, "alphaCutoff"), unlit: at(lit, "unlit"),
		sunDirection: at(lit, "sunDirection"), sunColor: at(lit, "sunColor"), skyColor: at(lit, "skyColor"), ground: at(lit, "groundColor"),
	}
	r.lines = lineProgram{program: lines, model: at(lines, "model"), viewProjection: at(lines, "viewProjection"), color: at(lines, "color")}
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

// colorFloats returns c as four numbers from 0 to 1, as it is: sRGB.
func colorFloats(c Color) [4]float32 {
	return [4]float32{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255, float32(c.A) / 255}
}

// frustum is the six planes around what a camera sees, each as a normal
// pointing inwards and a distance: a point p is inside a plane when
// normal.Dot(p) + distance >= 0.
type frustum [6]struct {
	normal   Vector3
	distance float32
}

// frustumOf takes the planes out of a view-projection matrix.
func frustumOf(m matrix) frustum {
	row := func(i int) [4]float32 { return [4]float32{m[i], m[4+i], m[8+i], m[12+i]} }
	r0, r1, r2, r3 := row(0), row(1), row(2), row(3)
	var f frustum
	for i, p := range [6][4]float32{
		{r3[0] + r0[0], r3[1] + r0[1], r3[2] + r0[2], r3[3] + r0[3]},
		{r3[0] - r0[0], r3[1] - r0[1], r3[2] - r0[2], r3[3] - r0[3]},
		{r3[0] + r1[0], r3[1] + r1[1], r3[2] + r1[2], r3[3] + r1[3]},
		{r3[0] - r1[0], r3[1] - r1[1], r3[2] - r1[2], r3[3] - r1[3]},
		{r3[0] + r2[0], r3[1] + r2[1], r3[2] + r2[2], r3[3] + r2[3]},
		{r3[0] - r2[0], r3[1] - r2[1], r3[2] - r2[2], r3[3] - r2[3]},
	} {
		normal := Vector3{X: p[0], Y: p[1], Z: p[2]}
		length := normal.Length()
		if length > 0 {
			f[i].normal, f[i].distance = normal.Scale(1/length), p[3]/length
		}
	}
	return f
}

// sees says whether any of the sphere at center, radius across, is inside.
func (f *frustum) sees(center Vector3, radius float32) bool {
	for _, p := range f {
		if p.normal.Dot(center)+p.distance < -radius {
			return false
		}
	}
	return true
}

// draw draws a pass.
func (r *state3D) draw(pass *pass3D) {
	view, projection := pass.camera.matrices()
	viewProjectionMatrix := projection.mul(view)
	viewProjection := [16]float32(viewProjectionMatrix)
	sight := frustumOf(viewProjectionMatrix)
	eye, _ := pass.camera.view()

	// What is solid, grouped by mesh and material in the order first drawn,
	// so every copy of a mesh is one draw; what is see-through, one by one.
	type groupKey struct {
		mesh     uint32
		material *material3D
	}
	type group struct {
		gpu      device.Mesh
		material *material3D
		items    []int
	}
	var groups []group
	byKey := map[groupKey]int{}
	var wires, seeThrough []int
	for i := range pass.items {
		item := &pass.items[i]
		if !sight.sees(item.center, item.radius) {
			continue
		}
		switch {
		case item.wire != nil:
			wires = append(wires, i)
		case item.material.alpha == alphaBlend || item.tint.A < 255:
			seeThrough = append(seeThrough, i)
		default:
			key := groupKey{item.gpu.ID, item.material}
			at, ok := byKey[key]
			if !ok {
				at = len(groups)
				byKey[key] = at
				groups = append(groups, group{gpu: item.gpu, material: item.material})
			}
			groups[at].items = append(groups[at].items, i)
		}
	}

	device.Begin3D()

	lit := r.lit
	device.UseProgram(lit.program)
	device.SetUniformMatrix(lit.viewProjection, &viewProjection)
	device.SetUniform(lit.sunDirection, 3, [4]float32{sunDirection.X, sunDirection.Y, sunDirection.Z})
	device.SetUniform(lit.sunColor, 3, sunColor)
	device.SetUniform(lit.skyColor, 3, skyColor)
	device.SetUniform(lit.ground, 3, groundColor)
	device.SetUniformInt(lit.baseTexture, 0)
	device.SetUniformInt(lit.emissiveTexture, 1)
	var current *material3D
	culling := true
	use := func(m *material3D) {
		if m == current {
			return
		}
		current = m
		device.SetUniform(lit.baseColor, 4, m.baseColor)
		device.SetUniform(lit.emissive, 3, [4]float32{m.emissive[0], m.emissive[1], m.emissive[2]})
		device.SetUniformInt(lit.alphaMode, int32(m.alpha))
		device.SetUniform(lit.alphaCutoff, 1, [4]float32{m.alphaCutoff})
		device.SetUniformInt(lit.unlit, boolInt(m.unlit))
		bind := func(unit int, texture *texture3D, has int32) {
			if gpu := texture.upload(); gpu.ID != 0 {
				device.BindTexture3D(unit, gpu)
				device.SetUniformInt(has, 1)
			} else {
				device.SetUniformInt(has, 0)
			}
		}
		bind(0, m.baseTexture, lit.hasBaseTexture)
		bind(1, m.emissiveTexture, lit.hasEmissive)
		if m.twoSided == culling {
			culling = !m.twoSided
			device.SetCulling(culling)
		}
	}
	instance := func(item *item3D) {
		r.instances = append(r.instances, item.model[:]...)
		tint := colorFloats(item.tint)
		r.instances = append(r.instances, tint[:]...)
	}
	for _, g := range groups {
		use(g.material)
		r.instances = r.instances[:0]
		for _, i := range g.items {
			instance(&pass.items[i])
		}
		device.DrawMeshInstanced(g.gpu, r.instances)
	}

	if len(wires) > 0 || len(pass.lines) > 0 {
		lines := r.lines
		device.UseProgram(lines.program)
		device.SetUniformMatrix(lines.viewProjection, &viewProjection)
		for _, i := range wires {
			item := &pass.items[i]
			model := [16]float32(item.model)
			device.SetUniformMatrix(lines.model, &model)
			device.SetUniform(lines.color, 4, colorFloats(item.tint))
			device.DrawMesh(item.wire.uploadWire())
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
		distance := func(i int) float32 { return pass.items[i].center.Distance(eye) }
		slices.SortStableFunc(seeThrough, func(a, b int) int {
			return cmp.Compare(distance(b), distance(a)) // farthest first
		})
		device.UseProgram(lit.program)
		current = nil
		device.SetDepthWrite(false)
		device.SetBlend3D(true)
		for _, i := range seeThrough {
			item := &pass.items[i]
			material := item.material
			if material.alpha != alphaBlend {
				// A solid material drawn see-through by its tint.
				see := *material
				see.alpha = alphaBlend
				material = &see
			}
			use(material)
			r.instances = r.instances[:0]
			instance(item)
			device.DrawMeshInstanced(item.gpu, r.instances)
		}
		device.SetDepthWrite(true)
		device.SetBlend3D(false)
	}

	device.End3D()
}

func boolInt(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

// largestScale returns how much m stretches lengths at most, along any of
// its axes: what a bounding sphere's radius grows by.
func (m matrix) largestScale() float32 {
	x := Vector3{X: m[0], Y: m[1], Z: m[2]}.Length()
	y := Vector3{X: m[4], Y: m[5], Z: m[6]}.Length()
	z := Vector3{X: m[8], Y: m[9], Z: m[10]}.Length()
	return float32(math.Max(float64(x), math.Max(float64(y), float64(z))))
}

// upload puts the texture on the graphics card, decoding it first if it
// hasn't been, the first time something with it is drawn. A nil texture, or
// one that didn't decode, gives none.
func (t *texture3D) upload() device.Texture3D {
	if t == nil || t.err != nil {
		return device.Texture3D{}
	}
	if t.gpu.ID == 0 {
		if t.decoded.Width == 0 {
			if t.decoded, t.err = device.DecodeImage(t.data); t.err != nil {
				return device.Texture3D{}
			}
		}
		t.gpu = device.NewTexture3D(t.decoded, t.options)
		t.decoded = device.DecodedImage{}
		loadedTextures.add(t)
	}
	return t.gpu
}

// unload frees the texture on the graphics card; the file stays, to decode
// it again if a game runs again.
func (t *texture3D) unload() {
	device.UnloadTexture3D(t.gpu)
	t.gpu = device.Texture3D{}
}

// loadedTextures are the 3D textures on the graphics card, for Run to free
// when it ends.
var loadedTextures textureList

type textureList struct {
	textures []*texture3D
}

func (l *textureList) add(t *texture3D) { l.textures = append(l.textures, t) }

func (l *textureList) unloadAll() {
	for _, t := range l.textures {
		t.unload()
	}
	l.textures = nil
}
