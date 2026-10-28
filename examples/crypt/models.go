package main

import "golib"

// The models, meshes, textures and materials, made once. The crypt is one
// model, made in Blender by sources/models/level.py; the astronaut, and the
// skeleton guards, are Kenney's (see assets/ATTRIBUTION.md). Doors, crates,
// the lift, keys, flames and the way out are drawn from meshes made here.
var (
	levelModel     = golib.NewModel("models/level.glb")
	characterModel = golib.NewModel("models/kit/character-oopi.glb")

	boxMesh      = golib.NewBoxMesh()
	cylinderMesh = golib.NewCylinderMesh()
	sphereMesh   = golib.NewSphereMesh()
	coneMesh     = golib.NewConeMesh()

	crateTexture = golib.NewTexture("textures/crate.png")
	doorTexture  = golib.NewTexture("textures/door.png")
)

// The materials. Each draw of one Material has one color, so a crate under
// the mouse, which is drawn lighter, has a material of its own.
var (
	crateWood   = &golib.Material{Texture: crateTexture, Roughness: 0.85}
	crateLit    = &golib.Material{Texture: crateTexture, Roughness: 0.85, Emission: golib.Color{R: 36, G: 28, B: 12, A: 255}}
	doorWood    = &golib.Material{Texture: doorTexture, Roughness: 0.7}
	liftStone   = &golib.Material{Color: liftColor, Roughness: 0.8}
	liftRim     = &golib.Material{Color: ironColor, Metallic: 1, Roughness: 0.4}
	keyGold     = &golib.Material{Color: golib.Gold, Metallic: 1, Roughness: 0.25, Emission: golib.Color{R: 120, G: 90, B: 10, A: 255}, EmissionStrength: 1.5}
	flame       = &golib.Material{Color: golib.Black, Emission: flameColor, EmissionStrength: 5}
	flameCore   = &golib.Material{Color: golib.Black, Emission: flameCoreColor, EmissionStrength: 7}
	exitGlow    = &golib.Material{Color: golib.Black, Emission: exitColor, EmissionStrength: 3, TwoSided: true, Shader: exitShader}
	exitFloor   = &golib.Material{Color: golib.Black, Emission: exitColor, EmissionStrength: 0.8}
	debrisWood  = &golib.Material{Color: woodChipColor, Roughness: 0.9}
	debrisStone = &golib.Material{Color: dustColor, Roughness: 0.9}
	alertMarker = &golib.Material{Color: golib.Black, Emission: golib.Red, EmissionStrength: 3}
)

// exitShader makes the way out shimmer, rising like a heat haze.
const exitShader = `
void surface(inout Surface s) {
    float wave = 0.65 + 0.35 * sin(s.position.y * 6.0 - s.time * 4.0 + sin(s.position.x * 5.0) * 1.5);
    s.emission *= wave;
}`
