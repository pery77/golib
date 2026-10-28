package main

import "golib"

// The meshes and models, made once.
var (
	boxMesh      = golib.NewBoxMesh()
	cylinderMesh = golib.NewCylinderMesh()
	sphereMesh   = golib.NewSphereMesh()
	planeMesh    = golib.NewPlaneMesh()

	hero     = golib.NewModel("models/kit/character-oopi.glb")
	tree     = golib.NewModel("models/kit/tree.glb")
	pineTree = golib.NewModel("models/kit/tree-pine.glb")
	rocks    = golib.NewModel("models/kit/rocks.glb")
	flowers  = golib.NewModel("models/kit/flowers.glb")
)

// The materials of the yard.
var (
	grassLook  = &golib.Material{Color: golib.Color{R: 96, G: 150, B: 72, A: 255}, Roughness: 0.95}
	yardLook   = &golib.Material{Color: golib.Color{R: 176, G: 150, B: 108, A: 255}, Roughness: 0.95}
	fenceLook  = &golib.Material{Color: golib.Color{R: 120, G: 84, B: 52, A: 255}, Roughness: 0.8}
	plinthLook = &golib.Material{Color: golib.Color{R: 150, G: 146, B: 138, A: 255}, Roughness: 0.85}
	plinthTop  = &golib.Material{Color: golib.Color{R: 112, G: 86, B: 60, A: 255}, Roughness: 0.7}
	stoneLook  = &golib.Material{Color: golib.Color{R: 92, G: 96, B: 104, A: 255}, Roughness: 0.9}
	crateLook  = &golib.Material{Color: golib.Color{R: 196, G: 140, B: 74, A: 255}, Roughness: 0.75}
	crateBand  = &golib.Material{Color: golib.Color{R: 128, G: 82, B: 40, A: 255}, Roughness: 0.75}
	barrelLook = &golib.Material{Color: golib.Color{R: 150, G: 88, B: 48, A: 255}, Roughness: 0.7}
	hoopLook   = &golib.Material{Color: golib.Color{R: 70, G: 70, B: 76, A: 255}, Metallic: 0.8, Roughness: 0.4}
	ballLook   = &golib.Material{Color: golib.Color{R: 220, G: 60, B: 50, A: 255}, Roughness: 0.45}
	downLook   = &golib.Material{Color: golib.Color{R: 150, G: 120, B: 90, A: 255}, Roughness: 0.8}
)
