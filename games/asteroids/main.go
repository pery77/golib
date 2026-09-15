// Asteroids is a GoLib game: fly a ship through a field of rocks and shoot
// them to pieces before they hit you.
//
// Besides the usual layout, it shows two framework features: post-processing
// shaders (shaders/glow.fs and shaders/crt.fs, turned on in scenes.go) and
// fullscreen. Read it in this order:
//
//   - main.go (this file) starts the game with the title scene, and holds the
//     colors and the screen effect settings.
//   - scenes.go has the scenes (title, play, pause, game over), the keys every
//     scene shares, and the drawing.
//   - world.go holds the rules as plain Go, with no input or drawing, so that
//     world_test.go can test them.
//   - DESIGN.md says what the game is, its controls and where its numbers live.
package main

import (
	_ "embed"
	"log"

	"golib"
)

// The screen effects, run over the whole picture after every Draw.
var (
	//go:embed shaders/glow.fs
	glowSource string

	//go:embed shaders/crt.fs
	crtSource string
)

// Screen effect settings, sent to the shaders as uniforms.
const (
	glowStrength = 2.4 // how bright the glow around lines is
	crtCurvature = 0.3 // how much the picture bulges, like an old tube; 0 is flat
)

// The game's colors, in one place so the look is easy to change.
var (
	spaceColor   = golib.Color{R: 4, G: 6, B: 16, A: 255}
	rockColor    = golib.Color{R: 220, G: 225, B: 240, A: 255}
	shipColor    = golib.Color{R: 110, G: 225, B: 255, A: 255}
	flameColor   = golib.Color{R: 255, G: 150, B: 40, A: 255}
	bulletColor  = golib.Color{R: 255, G: 240, B: 120, A: 255}
	sparkColor   = golib.Color{R: 255, G: 190, B: 90, A: 255}
	textColor    = golib.Color{R: 200, G: 210, B: 230, A: 255}
	titleColor   = golib.Color{R: 110, G: 225, B: 255, A: 255}
	overlayColor = golib.Color{R: 0, G: 0, B: 0, A: 160}
)

// lineWidth is the thickness of the ship and rock outlines, in pixels.
const lineWidth = 2

func main() {
	config := golib.Config{Title: "Asteroids", Width: worldWidth, Height: worldHeight}
	if err := golib.Run(newTitleScene(newOptions()), config); err != nil {
		log.Fatal(err)
	}
}
