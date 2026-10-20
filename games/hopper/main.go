// hopper is a small 3D platformer made with GoLib, from Kenney's Platformer
// Kit: a little astronaut hops across floating grass blocks, picks up coins
// and runs for the flag. DESIGN.md says what it is.
//
// Read it in this order:
//
//   - main.go (this file) starts the game: the screen's size, the colors, and
//     the level, read once before the game starts.
//   - level.go reads the level made in Blender, assets/models/level.glb: the
//     solid blocks, the coins, the checkpoints, the start and the goal, from
//     the empties placed in it (see sources/models/level.py).
//   - world.go holds the rules as plain Go, with no input or drawing, so that
//     world_test.go can test them: running, jumping, landing on blocks,
//     coins, lives and the clock.
//   - controls.go turns the keys and the sticks into a direction from where
//     the camera looks, and swings the camera behind the character.
//   - scenes.go has the title, play, pause, won and game over scenes.
//   - draw.go draws the world in 3D, and the coins, the time and the lives
//     over it.
//   - sounds.go has the sound effects, made in code.
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
	skyTopColor     = golib.Color{R: 82, G: 150, B: 230, A: 255}
	skyBottomColor  = golib.Color{R: 200, G: 232, B: 252, A: 255}
	cloudColor      = golib.Color{R: 255, G: 255, B: 255, A: 210}
	shadowColor     = golib.Color{R: 0, G: 0, B: 0, A: 80}
	checkpointOff   = golib.Color{R: 150, G: 150, B: 160, A: 255} // a checkpoint not reached yet
	textColor       = golib.White
	textShadowColor = golib.Color{R: 20, G: 30, B: 60, A: 200}
	accentColor     = golib.Gold
	coinColor       = golib.Color{R: 250, G: 200, B: 40, A: 255}
	coinDarkColor   = golib.Color{R: 200, G: 140, B: 20, A: 255}
	heartColor      = golib.Color{R: 236, G: 64, B: 82, A: 255}
	panelColor      = golib.Color{R: 10, G: 20, B: 50, A: 120}
	overlayColor    = golib.Color{R: 10, G: 20, B: 50, A: 150}
)

// theLevel is the level, read from assets/models/level.glb once, in main,
// before the game starts.
var theLevel *level

func main() {
	var err error
	theLevel, err = loadLevel(levelModel)
	if err != nil {
		log.Fatal(err)
	}
	config := golib.Config{
		Title: "Hopper", Width: screenWidth, Height: screenHeight,
		// Soft edges on the 3D.
		Antialias: true,
		// The game waits while the player is in another program.
		PauseUnfocused: true,
	}
	if err := golib.Run(newTitleScene(), config); err != nil {
		log.Fatal(err)
	}
}
