// Lantern: a night in a flooded temple. A hooded explorer walks three stone
// halls, lit by torches and the moon through a broken roof, takes six
// glowing gems and leaves by the gate they open. DESIGN.md says more.
//
// Read it in this order:
//
//   - main.go (this file) starts the game on the title.
//   - scenes.go: the title, play, pause and won scenes.
//   - world.go: the rules, with no input or drawing, which world_test.go
//     tests on the real level.
//   - controls.go: the keys and the camera behind the explorer.
//   - draw.go: the temple, its light, haze, water, fire, gems and runes.
//   - art.go and sounds.go: the level, meshes, materials, particles and
//     sounds, made once.
//   - sources/models/level.py builds the level in Blender, and
//     tools/blender/bake_lightmaps.py bakes its light (see its top);
//     sources/textures/rune.go paints the rune.
package main

import (
	"log"

	"golib"
)

// The screen's size in pixels. GoLib scales it to the window.
const (
	screenWidth  = 1280
	screenHeight = 720
)

func main() {
	config := golib.Config{
		Title: "Lantern", Width: screenWidth, Height: screenHeight,
		PauseUnfocused: true,
	}
	if err := golib.Run(newTitleScene(), config); err != nil {
		log.Fatal(err)
	}
}
