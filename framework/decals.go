package golib

import "golib/internal/device"

// Decal is a picture pressed onto what is solid inside a box: a footprint,
// a scorch mark, a pool of blood, a crack, a bullet hole, a rune that glows
// on the floor. The box is a meter each way, placed by the Transform
// DrawDecal gets, and the picture lies on its X and Z, pressed along its Y
// onto every surface in it: flat on the ground as it is, on a wall turned
// with Pitch 90. Its top is towards the box's +Z.
//
//	screen.DrawDecal(golib.Decal{Texture: scorch}, golib.Transform{
//		Position: hit.Position, Rotation: golib.RotationTowards(hit.Normal).Turn(golib.Rotation{Pitch: -90}),
//		Scale: golib.Vector3{X: 1, Y: 0.4, Z: 1},
//	})
//
// A decal darkens what it lies on, by its picture's colors where it is
// opaque, as dirt and stains do, after the solid things are drawn and
// before what is see-through; with Glow, it adds its light instead.
type Decal struct {
	// Texture is the picture: nil is a soft round dot, a stain.
	Texture *Texture

	// Color multiplies the picture: white when zero. Its opacity fades the
	// decal.
	Color Color

	// Glow adds the picture's light to what it lies on instead of darkening
	// it, and Brightness, 1 when zero, multiplies it, past white for a glow
	// that blooms.
	Glow       bool
	Brightness float32
}

// decalDraw is a decal as a pass draws it.
type decalDraw struct {
	texture    *texture3D
	tint       [4]float32
	place      matrix
	glow       bool
	brightness float32
}

// DrawDecal presses decal onto what is solid inside the meter box placed by
// transform.
func (s *Screen) DrawDecal(decal Decal, transform Transform) {
	if !s.in3D("DrawDecal") {
		return
	}
	texture := softDot
	if t := decal.Texture; t != nil {
		if err := t.prepare(); err != nil {
			reportError(err)
			return
		}
		texture = &t.texture
	}
	texture.upload()
	tint := White
	if decal.Color != (Color{}) {
		tint = decal.Color
	}
	brightness := decal.Brightness
	if brightness <= 0 {
		brightness = 1
	}
	if transform.Scale.X == 0 && transform.Scale.Y == 0 && transform.Scale.Z == 0 {
		transform.Scale = Vector3{X: 1, Y: 1, Z: 1}
	}
	l := linearColor(tint)
	s.pass.decals = append(s.pass.decals, decalDraw{
		texture: texture, tint: [4]float32{l[0], l[1], l[2], l[3]},
		place: transform.matrix(), glow: decal.Glow, brightness: brightness,
	})
}

// The program that presses decals: it draws the back of each decal's box,
// so a camera inside it still sees it, and for every pixel there finds
// where the solid thing seen there is, from the pass's depth, and in the
// box's space whether it is in the box, and where on the picture.
const (
	decalVertexShader = `#version 330
layout(location = 0) in vec3 vertexPosition;
uniform mat4 model;
uniform mat4 viewProjection;

void main()
{
    gl_Position = viewProjection * model * vec4(vertexPosition, 1.0);
}
`
	decalFragmentShader = `#version 330
uniform sampler2D depthCopy;
uniform sampler2D picture;
uniform mat4 inverseViewProjection;
uniform mat4 decalFromWorld;
uniform vec3 decalAxis;
uniform vec2 viewSize;
uniform vec4 tint;
uniform int glow;
uniform float brightness;
out vec4 fragColor;

void main()
{
    vec2 uv = gl_FragCoord.xy / viewSize;
    float depth = texture(depthCopy, uv).r;
    if (depth >= 1.0) discard;
    vec4 world = inverseViewProjection * vec4(uv * 2.0 - 1.0, depth * 2.0 - 1.0, 1.0);
    vec3 p = world.xyz / world.w;
    vec3 local = (decalFromWorld * vec4(p, 1.0)).xyz;
    if (any(greaterThan(abs(local), vec3(0.5)))) discard;
    // Faded where the surface turns from the way it is pressed, so it
    // doesn't smear down a wall, and towards the box's ends.
    vec3 n = normalize(cross(dFdx(p), dFdy(p)));
    float fade = smoothstep(0.25, 0.55, abs(dot(n, decalAxis))) * (1.0 - smoothstep(0.35, 0.5, abs(local.y)));
    vec4 c = texture(picture, vec2(local.x + 0.5, 0.5 - local.z)) * tint;
    float a = c.a * fade;
    if (glow == 1) {
        fragColor = vec4(c.rgb * brightness * a, 0.0);
    } else {
        fragColor = vec4(mix(vec3(1.0), c.rgb, a), 1.0);
    }
}
`
)

// decalProgram presses decals.
type decalProgram struct {
	program                                 device.Program
	model, viewProjection                   int32
	depthCopy, picture                      int32
	inverseViewProjection, decalFromWorld   int32
	decalAxis, viewSize, tint, glow, bright int32
}

// The texture units the pass's copy is read from, past those a lit program
// uses: a shader reads at most 16, but the units go to 32 and more.
const (
	depthCopyUnit = 16
	colorCopyUnit = 17
)

// decalProgram returns the program decals are pressed with, made the first
// time a pass has one, or nil when it doesn't compile.
func (r *state3D) decalProgram() *decalProgram {
	if r.decals == nil {
		program, err := compile3D("decal", decalVertexShader, decalFragmentShader)
		if err != nil {
			reportError(err)
			r.decals = &decalProgram{}
		} else {
			at := func(name string) int32 { return device.UniformLocation(program, name) }
			r.decals = &decalProgram{
				program: program, model: at("model"), viewProjection: at("viewProjection"),
				depthCopy: at("depthCopy"), picture: at("picture"),
				inverseViewProjection: at("inverseViewProjection"), decalFromWorld: at("decalFromWorld"),
				decalAxis: at("decalAxis"), viewSize: at("viewSize"), tint: at("tint"), glow: at("glow"), bright: at("brightness"),
			}
		}
	}
	if r.decals.program.ID == 0 {
		return nil
	}
	return r.decals
}

// drawDecals presses the pass's decals onto what the pass has drawn so far,
// which CopyPass has copied.
func (r *state3D) drawDecals(pass *pass3D, viewProjection, inverse *[16]float32) {
	d := r.decalProgram()
	if d == nil {
		return
	}
	box := decalBox.upload()
	if box.ID == 0 {
		return
	}
	device.UseProgram(d.program)
	device.SetUniformMatrix(d.viewProjection, viewProjection)
	device.SetUniformMatrix(d.inverseViewProjection, inverse)
	device.SetUniformInt(d.depthCopy, depthCopyUnit)
	device.SetUniformInt(d.picture, 0)
	device.SetUniform(d.viewSize, 2, [4]float32{float32(pass.width), float32(pass.height)})
	device.BindPassCopy(colorCopyUnit, depthCopyUnit)
	device.SetDepthWrite(false)
	device.SetDepthTest(false)
	device.SetCullFront(true)
	for _, decal := range pass.decals {
		model := [16]float32(decal.place)
		fromWorld := [16]float32(decal.place.inverse())
		axis := decal.place.transformDirection(Vector3{Y: 1}).Normalize()
		device.SetUniformMatrix(d.model, &model)
		device.SetUniformMatrix(d.decalFromWorld, &fromWorld)
		device.SetUniform(d.decalAxis, 3, vec3(axis))
		device.SetUniform(d.tint, 4, decal.tint)
		device.SetUniformInt(d.glow, boolInt(decal.glow))
		device.SetUniform(d.bright, 1, [4]float32{decal.brightness})
		device.BindTexture3D(0, decal.texture.upload())
		if decal.glow {
			device.SetAdditive3D(true)
		} else {
			device.SetMultiply3D(true)
		}
		device.DrawMesh(box)
	}
	device.SetAdditive3D(false)
	device.SetCullFront(false)
	device.SetDepthTest(true)
	device.SetDepthWrite(true)
}

// decalBox is the meter box every decal is drawn as.
var decalBox = NewBoxMesh()
