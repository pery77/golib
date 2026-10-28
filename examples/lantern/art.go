package main

import (
	"math"

	"golib"
)

// The level, baked in Blender (sources/models/level.py, then
// tools/blender/bake_lightmaps.py), and the rune a taken gem leaves
// (sources/textures/rune.go).
var (
	level     = golib.NewModel("models/level.glb")
	runeImage = golib.NewTexture("textures/rune.png")
)

// The meshes, made once.
var (
	boxMesh     = golib.NewBoxMesh()
	sphereMesh  = golib.NewSphereMesh()
	coneMesh    = golib.NewConeMesh()
	capsuleMesh = golib.NewCapsuleMesh()
	gemMesh     = newGemMesh()
)

// gemColors are the six gems' colors, which their runes glow in too.
var gemColors = []golib.Color{
	{R: 60, G: 220, B: 255, A: 255},  // aquamarine
	{R: 255, G: 70, B: 200, A: 255},  // rose
	{R: 90, G: 255, B: 120, A: 255},  // emerald
	{R: 255, G: 200, B: 40, A: 255},  // topaz
	{R: 170, G: 110, B: 255, A: 255}, // amethyst
	{R: 255, G: 90, B: 60, A: 255},   // ruby
}

// gemMaterials are cartoon gems: lit in three flat bands, with a black
// outline, glowing a little in their own color.
var gemMaterials = func() []*golib.Material {
	var ms []*golib.Material
	for _, c := range gemColors {
		ms = append(ms, &golib.Material{
			Color: c, Toon: 3, Outline: golib.Black, OutlineWidth: 6,
			Emission: c, EmissionStrength: 0.35, Roughness: 0.3,
		})
	}
	return ms
}()

// The explorer's look: a hooded cloak, a face in its shadow, and a lantern.
var (
	cloakMaterial   = &golib.Material{Color: golib.Color{R: 120, G: 40, B: 34, A: 255}, Roughness: 0.85}
	hoodMaterial    = &golib.Material{Color: golib.Color{R: 96, G: 30, B: 28, A: 255}, Roughness: 0.85}
	faceMaterial    = &golib.Material{Color: golib.Color{R: 40, G: 28, B: 24, A: 255}, Roughness: 0.7}
	bootMaterial    = &golib.Material{Color: golib.Color{R: 50, G: 36, B: 28, A: 255}, Roughness: 0.7}
	lanternMaterial = &golib.Material{Color: golib.Black, Emission: golib.Color{R: 255, G: 190, B: 110, A: 255}, EmissionStrength: 3}
	ironMaterial    = &golib.Material{Color: golib.Color{R: 30, G: 28, B: 26, A: 255}, Metallic: 0.9, Roughness: 0.4}
	flameMaterial   = &golib.Material{Color: golib.Black, Emission: golib.Color{R: 255, G: 170, B: 60, A: 255}, EmissionStrength: 5}
	gateMaterial    = &golib.Material{Color: golib.Color{R: 92, G: 84, B: 74, A: 255}, Roughness: 0.8}
	socketMaterial  = &golib.Material{Color: golib.Color{R: 20, G: 18, B: 16, A: 255}, Roughness: 0.9}
)

// The torches' fire: flames that glow and rise, smoke that covers and
// grows, and a few sparks.
var (
	flames = golib.NewParticles(golib.ParticleSpec{
		Color: golib.Color{R: 255, G: 190, B: 70, A: 255}, EndColor: golib.Color{R: 200, G: 40, B: 10, A: 0},
		Brightness: 3, Size: 0.22, EndSize: 0.05, Life: 0.45,
		Speed: 0.7, Spread: 18, Gravity: golib.Vector3{Y: 2}, Drag: 0.2, Glow: true,
	})
	smoke = golib.NewParticles(golib.ParticleSpec{
		Color: golib.Color{R: 40, G: 36, B: 34, A: 150}, EndColor: golib.Color{R: 70, G: 68, B: 66, A: 0},
		Size: 0.25, EndSize: 1.1, Life: 2.6,
		Speed: 0.5, Spread: 12, Gravity: golib.Vector3{Y: 0.35}, Drag: 0.4,
	})
	sparks = golib.NewParticles(golib.ParticleSpec{
		Color: golib.Color{R: 255, G: 220, B: 120, A: 255}, EndColor: golib.Color{R: 255, G: 80, B: 20, A: 0},
		Brightness: 4, Size: 0.04, Life: 0.9,
		Speed: 1.6, Spread: 35, Gravity: golib.Vector3{Y: 0.6}, Drag: 0.5, Glow: true,
	})
	// ripples spread on the water round the explorer's legs as they wade.
	ripples = golib.NewParticles(golib.ParticleSpec{
		Color: golib.Color{R: 120, G: 150, B: 160, A: 40}, Size: 0.15, EndSize: 0.7, Life: 0.8,
		Speed: 0.6, Spread: 80, Drag: 0.6,
	})
)

// glimmers burst out of a gem as it is taken, one set in each gem's color.
var glimmers = func() []*golib.Particles {
	var ps []*golib.Particles
	for _, c := range gemColors {
		ps = append(ps, golib.NewParticles(golib.ParticleSpec{
			Color: c, EndColor: golib.Color{R: c.R, G: c.G, B: c.B, A: 0},
			Brightness: 3, Size: 0.12, EndSize: 0.02, Life: 0.9,
			Speed: 3, Spread: 180, Gravity: golib.Vector3{Y: -2}, Drag: 0.8, Glow: true,
		}))
	}
	return ps
}()

// newGemMesh is a cut gem: a ring of eight corners, a short crown above
// and a long point below, sharing their corners so the outline goes all
// round.
func newGemMesh() *golib.Mesh {
	const sides = 8
	const radius, crown, point = 0.24, 0.16, 0.36
	vertices := []golib.Vertex{
		{Position: golib.Vector3{Y: crown}},
		{Position: golib.Vector3{Y: -point}},
	}
	for i := range sides {
		a := float64(i) * 2 * math.Pi / sides
		vertices = append(vertices, golib.Vertex{Position: golib.Vector3{
			X: radius * float32(math.Cos(a)), Z: radius * float32(math.Sin(a)),
		}})
	}
	var indices []int
	for i := range sides {
		a, b := 2+i, 2+(i+1)%sides
		indices = append(indices, 0, b, a) // the crown, counterclockwise seen from above
		indices = append(indices, 1, a, b) // the point, seen from below
	}
	return golib.NewMesh(vertices, indices)
}
