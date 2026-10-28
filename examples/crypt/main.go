// crypt is a small 3D dungeon game made with GoLib: Kenney's little
// astronaut sneaks through a crypt of torch-lit halls, past skeleton guards,
// to find three keys and the way out. DESIGN.md says what it is.
//
// Read it in this order:
//
//   - main.go (this file) starts the game: the screen's size and the colors.
//   - models.go names the models, meshes, textures and materials.
//   - level.go reads the crypt made in Blender (sources/models/level.py):
//     where the doors, crates, keys, guards, torches and the lift are, from
//     the level model's markers.
//   - world.go holds the rules as plain Go, with no input or drawing, so that
//     the tests can play them: running through the crypt, keys and doors,
//     checkpoints, hearts and the way out; crates.go has the crates, and
//     lift.go the stone lift.
//   - guards.go has the skeleton guards: their rounds, what they see, and
//     the chase along the crypt's paths.
//   - poses.go says which animation each character plays, from its state.
//   - controls.go turns the keys, the mouse and the gamepad into what the
//     player wants, and keeps the camera behind the astronaut.
//   - scenes.go has the title, play, pause, game over and win scenes, and
//     plays the sounds; sounds.go makes them.
//   - draw.go draws the crypt in 3D, and hud.go the hearts, keys and
//     messages over it.
package main

import (
	"log"

	"golib"
)

// The screen's size in pixels. It never changes: GoLib scales it to the window.
const (
	screenWidth  = 1280
	screenHeight = 720
)

// The game's colors, in one place so the look is easy to change.
var (
	skyColor        = golib.Color{R: 40, G: 42, B: 70, A: 255}
	horizonColor    = golib.Color{R: 14, G: 12, B: 20, A: 255}
	groundColor     = golib.Color{R: 34, G: 27, B: 22, A: 255}
	moonColor       = golib.Color{R: 120, G: 135, B: 190, A: 255}
	fogColor        = golib.Color{R: 8, G: 7, B: 11, A: 255}
	torchColor      = golib.Color{R: 255, G: 146, B: 64, A: 255}
	flameColor      = golib.Color{R: 255, G: 120, B: 30, A: 255}
	flameCoreColor  = golib.Color{R: 255, G: 214, B: 120, A: 255}
	exitColor       = golib.Color{R: 110, G: 200, B: 255, A: 255}
	keyLightColor   = golib.Color{R: 255, G: 210, B: 90, A: 255}
	liftColor       = golib.Color{R: 132, G: 124, B: 112, A: 255}
	ironColor       = golib.Color{R: 70, G: 66, B: 62, A: 255}
	woodChipColor   = golib.Color{R: 140, G: 96, B: 52, A: 255}
	dustColor       = golib.Color{R: 96, G: 70, B: 48, A: 255}
	guardTint       = golib.Color{R: 236, G: 230, B: 206, A: 255} // bone white
	textColor       = golib.White
	textShadowColor = golib.Color{R: 10, G: 6, B: 14, A: 220}
	accentColor     = golib.Color{R: 255, G: 196, B: 80, A: 255}
	heartColor      = golib.Color{R: 220, G: 60, B: 76, A: 255}
	alertColor      = golib.Color{R: 255, G: 70, B: 60, A: 255}
	panelColor      = golib.Color{R: 10, G: 8, B: 16, A: 150}
	overlayColor    = golib.Color{R: 8, G: 6, B: 12, A: 170}
)

func main() {
	var err error
	if theLevel, err = loadLevel(levelModel); err != nil {
		log.Fatal(err)
	}
	config := golib.Config{
		Title: "Crypt", Width: screenWidth, Height: screenHeight,
		// Soft edges on the 3D.
		Antialias: true,
		// The game waits while the player is in another program.
		PauseUnfocused: true,
	}
	if err := golib.Run(newTitleScene(), config); err != nil {
		log.Fatal(err)
	}
}
