package golib

import (
	"errors"
	"fmt"

	"golib/internal/device"
)

// Material is how a mesh's surface looks: its color, a texture, how
// metallic and how rough it is, the light it gives off, and, for anything
// else, a shader of the game's own. Make it once, like a mesh, and draw any
// mesh with it with Screen.DrawMeshWith:
//
//	gold := &golib.Material{Color: golib.Gold, Metallic: 1, Roughness: 0.3}
//	lava := &golib.Material{Color: golib.Red, Emission: golib.Orange, EmissionStrength: 3}
//	grass := &golib.Material{Texture: golib.NewTexture("textures/grass.png"), Tiling: 40}
//
// Fields left at zero keep a painted look: white, not metallic, roughness
// 0.6. Change a field and the next draw shows it.
type Material struct {
	// Color multiplies the texture, or is the color without one. The zero
	// Color counts as White; with A below 255 the surface is see-through.
	Color Color

	// Texture is a picture wrapped around the mesh, by its texture
	// coordinates; Tiling repeats it that many times across each way, for
	// a floor of tiles. 0 counts as 1.
	Texture *Texture
	Tiling  float32

	// Metallic is 0 for paint, wood, stone or plastic, and 1 for bare metal,
	// whose color tints what it reflects.
	Metallic float32

	// Roughness is how blurred reflections are: 0.05 for a mirror or wet
	// paint, 1 for chalk. 0 counts as 0.6.
	Roughness float32

	// Emission is the light the surface gives off, as lava, a screen or a
	// neon sign do; it shows even in the dark, and glows with
	// Environment.Bloom when EmissionStrength makes it brighter than white.
	// EmissionStrength multiplies it: 0 counts as 1.
	Emission         Color
	EmissionStrength float32

	// Unlit shows Color, or the texture, as it is, with no light or shadow:
	// a sky dome, a sign, a cartoon outline. As everywhere in the 3D,
	// Environment.Exposure brightens or darkens it, and only the brightest
	// colors, above about 225 of 255, are softened a little by the
	// tonemapper.
	Unlit bool

	// TwoSided draws the back of each triangle too, for leaves, flags and
	// paper.
	TwoSided bool

	// Shader is a function of the game's own, in GLSL, that changes the
	// surface before GoLib lights it:
	//
	//	void surface(inout Surface s)
	//
	// Surface holds color (vec4, linear), emission (vec3), metallic,
	// roughness (floats), normal (vec3, in the world), position (vec3, in
	// the world), texcoord (vec2, tiled) and time (seconds of game time).
	// Lighting, shadows, fog and the rest stay GoLib's:
	//
	//	water := &golib.Material{Color: golib.SkyBlue, Roughness: 0.1, Shader: `
	//	void surface(inout Surface s) {
	//	    s.normal = normalize(s.normal + 0.05 * vec3(sin(s.position.x * 3.0 + s.time * 2.0), 0.0, cos(s.position.z * 2.0 + s.time)));
	//	}`}
	//
	// A shader that doesn't compile stops Run with what the graphics card
	// said about it.
	Shader string

	made     *material3D      // what the renderer draws with
	twoSided *material3D      // the same, drawn on both sides, for a flat mesh
	from     materialSettings // the fields they were made from
	ready    bool
}

// materialSettings are a Material's fields, to tell when one changed.
type materialSettings struct {
	color            Color
	texture          *Texture
	tiling           float32
	metallic         float32
	roughness        float32
	emission         Color
	emissionStrength float32
	unlit, twoSided  bool
	shader           string
}

// material returns what the renderer draws m with, made again when a field
// changed.
func (m *Material) material() *material3D {
	settings := materialSettings{
		m.Color, m.Texture, m.Tiling, m.Metallic, m.Roughness,
		m.Emission, m.EmissionStrength, m.Unlit, m.TwoSided, m.Shader,
	}
	if m.ready && settings == m.from {
		return m.made
	}
	made := &material3D{baseColor: [4]float32{1, 1, 1, 1}, roughness: plainRoughness}
	if m.Color != (Color{}) {
		made.baseColor = linearColor(m.Color)
	}
	if made.baseColor[3] < 1 {
		made.alpha = alphaBlend
	}
	if m.Texture != nil {
		made.baseTexture = &m.Texture.texture
	}
	if m.Tiling > 0 {
		made.tiling = [2]float32{m.Tiling, m.Tiling}
	}
	made.metallic = min(max(m.Metallic, 0), 1)
	if m.Roughness > 0 {
		made.roughness = min(m.Roughness, 1)
	}
	if m.Emission != (Color{}) {
		strength := m.EmissionStrength
		if strength <= 0 {
			strength = 1
		}
		e := linearColor(m.Emission)
		made.emissive = [3]float32{e[0] * strength, e[1] * strength, e[2] * strength}
	}
	made.unlit, made.twoSided, made.surface = m.Unlit, m.TwoSided, m.Shader
	both := *made
	both.twoSided = true
	m.made, m.twoSided, m.from, m.ready = made, &both, settings, true
	return made
}

// DrawMeshWith draws mesh placed by transform, its surface as material says:
// a texture, metal, a glow, or a shader of the game's own. DrawMesh, with a
// color, is the same with a painted material of that color.
func (s *Screen) DrawMeshWith(mesh *Mesh, transform Transform, material *Material) {
	if !s.in3D("DrawMeshWith") {
		return
	}
	if mesh == nil {
		reportError(errors.New("golib: Screen.DrawMeshWith got a nil mesh: make one with golib.NewBoxMesh or another New...Mesh"))
		return
	}
	if material == nil {
		reportError(errors.New("golib: Screen.DrawMeshWith got a nil material: make one with &golib.Material{...}"))
		return
	}
	made := material.material()
	if t := material.Texture; t != nil {
		if err := t.prepare(); err != nil {
			reportError(err)
			return
		}
	}
	if mesh.twoSided {
		made = material.twoSided
	}
	s.addItem(mesh, made, transform.matrix(), White)
}

// Texture is a picture for 3D, from a PNG or JPEG file in the game's assets
// folder, wrapped around a mesh by a Material. It is smoothed, with mipmaps,
// and repeats past its edges.
//
//	var bricks = golib.NewTexture("textures/bricks.png")
type Texture struct {
	name string

	// Pixelated keeps a pixel art texture's pixels sharp up close, as
	// Config.PixelArt does for the screen. Set it before the texture is
	// first drawn.
	Pixelated bool

	read    bool
	err     error
	texture texture3D
}

// NewTexture returns the texture in the game's assets folder named name,
// such as "textures/bricks.png", read the first time it is drawn.
func NewTexture(name string) *Texture {
	return &Texture{name: name}
}

// Err reads the texture, if it has not been read yet, and returns what
// stopped it from loading. Drawing one that didn't load stops Run with the
// same mistake.
func (t *Texture) Err() error {
	return t.prepare()
}

func (t *Texture) prepare() error {
	if t.read {
		return t.err
	}
	t.read = true
	data, err := ReadAsset(t.name)
	if err != nil {
		t.err = fmt.Errorf("golib.NewTexture(%q): %w", t.name, err)
		return t.err
	}
	t.texture = texture3D{data: data, options: device.TextureOptions{Nearest: t.Pixelated, NearestMipmaps: t.Pixelated}}
	if t.texture.decoded, err = device.DecodeImage(data); err != nil {
		t.err = fmt.Errorf("golib.NewTexture(%q): it is not a PNG or JPEG picture GoLib can read: %w", t.name, err)
	}
	return t.err
}
