package main

import "golib"

// The models, from Kenney's Platformer Kit (see assets/ATTRIBUTION.md), made
// once: every character, the player and the enemies, is a copy of the same
// astronaut, each posed by an Animator of its own.
var (
	characterModel = golib.NewModel("models/kit/character-oopi.glb")
	grassModel     = golib.NewModel("models/kit/grass.glb")
	flowersModel   = golib.NewModel("models/kit/flowers.glb")
	rocksModel     = golib.NewModel("models/kit/rocks.glb")
	treeModel      = golib.NewModel("models/kit/tree.glb")
	pineModel      = golib.NewModel("models/kit/tree-pine.glb")
)
