// marbles is a 3D game made with GoLib: a marble rolling on a floating
// platform, collecting gems and keeping away from red cubes. DESIGN.md says
// what it is.
//
// Read it in this order:
//
//   - main.go (this file) starts the game: the screen's size and the colors.
//   - world.go holds the rules as plain Go, with no input or drawing, so that
//     world_test.go can test them; controls.go turns the keys into a direction
//     from where the camera looks, and swings the camera behind the marble.
//   - scenes.go has the title, play, pause and game over scenes.
//   - draw.go draws the world in 3D, and the score over it.
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
	skyTopColor     = golib.Color{R: 88, G: 148, B: 222, A: 255}
	skyBottomColor  = golib.Color{R: 196, G: 226, B: 250, A: 255}
	tileColor       = golib.Color{R: 236, G: 232, B: 220, A: 255}
	tileOtherColor  = golib.Color{R: 200, G: 196, B: 186, A: 255}
	platformColor   = golib.Color{R: 150, G: 140, B: 128, A: 255}
	rockColor       = golib.Color{R: 120, G: 98, B: 80, A: 255}
	rockDarkColor   = golib.Color{R: 92, G: 74, B: 62, A: 255}
	grassColor      = golib.Color{R: 96, G: 168, B: 88, A: 255}
	cloudColor      = golib.Color{R: 255, G: 255, B: 255, A: 200}
	marbleColor     = golib.Color{R: 40, G: 92, B: 210, A: 255}
	marbleMarkColor = golib.Color{R: 250, G: 214, B: 70, A: 255}
	gemColor        = golib.Color{R: 60, G: 240, B: 190, A: 255}
	gemGlowColor    = golib.Color{R: 120, G: 255, B: 220, A: 70}
	cubeColor       = golib.Color{R: 220, G: 44, B: 44, A: 255}
	eyeColor        = golib.White
	pupilColor      = golib.Color{R: 30, G: 20, B: 20, A: 255}
	textColor       = golib.White
	textShadowColor = golib.Color{R: 20, G: 30, B: 60, A: 200}
	accentColor     = golib.Gold
	overlayColor    = golib.Color{R: 10, G: 20, B: 50, A: 150}
)

func main() {
	config := golib.Config{
		Title: "Marbles", Width: screenWidth, Height: screenHeight,
		// Soft edges on the 3D shapes.
		Antialias: true,
		// The game waits while the player is in another program.
		PauseUnfocused: true,
	}
	if err := golib.Run(newTitleScene(), config); err != nil {
		log.Fatal(err)
	}
}
