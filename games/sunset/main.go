// sunset is a 3D game made with GoLib: a little hovercraft collecting
// glowing crystals on a lagoon at sunset, before a 90-second timer runs out.
// DESIGN.md says what it is.
//
// Read it in this order:
//
//   - main.go (this file) starts the game: the screen's size, the sun and
//     the game's colors.
//   - world.go holds the rules as plain Go, with no input or drawing, so that
//     world_test.go can test them.
//   - controls.go turns the keys, the d-pad, the stick and the triggers into
//     a throttle and a steer.
//   - scenes.go has the title, play, pause and time-up scenes, and the camera.
//   - look.go makes the meshes and materials once; draw.go draws the world
//     with them, under the HDRI sky; effects.go has the spray and sparkles;
//     hud.go draws the score and the time over it.
//   - sounds.go has the sound effects, made in code.
//   - assets/skies holds the sky, from Poly Haven (see assets/ATTRIBUTION.md).
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

// sunDirection is the way the sunlight travels. The HDRI's sun is 3 degrees
// over the horizon, towards -X and +Z (GoLib puts the middle of a panorama
// at +Z, and its left quarter at +X); this one comes from the same way,
// raised to 9 degrees, so its shadows stay long but don't stretch across the
// whole lagoon.
var sunDirection = golib.Vector3{X: 0.581, Y: -0.156, Z: -0.799}

// The game's colors, in one place so the look is easy to change.
var (
	sunColor      = golib.Color{R: 255, G: 168, B: 104, A: 255}
	waterColor    = golib.Color{R: 34, G: 72, B: 80, A: 255}
	steelColor    = golib.Color{R: 214, G: 218, B: 226, A: 255}
	darkSteel     = golib.Color{R: 70, G: 74, B: 82, A: 255}
	paintColor    = golib.Color{R: 236, G: 92, B: 34, A: 255}
	rubberColor   = golib.Color{R: 28, G: 28, B: 32, A: 255}
	glassColor    = golib.Color{R: 40, G: 70, B: 96, A: 170}
	helmetColor   = golib.Color{R: 245, G: 240, B: 230, A: 255}
	rockColor     = golib.Color{R: 120, G: 108, B: 96, A: 255}
	sandColor     = golib.Color{R: 214, G: 190, B: 150, A: 255}
	grassColor    = golib.Color{R: 92, G: 118, B: 58, A: 255}
	pineColor     = golib.Color{R: 44, G: 74, B: 48, A: 255}
	barkColor     = golib.Color{R: 90, G: 62, B: 44, A: 255}
	poleColor     = golib.Color{R: 82, G: 60, B: 44, A: 255}
	buoyRed       = golib.Color{R: 200, G: 40, B: 36, A: 255}
	buoyWhite     = golib.Color{R: 236, G: 232, B: 224, A: 255}
	crystalColor  = golib.Color{R: 70, G: 230, B: 255, A: 255}
	goldenColor   = golib.Color{R: 255, G: 190, B: 60, A: 255}
	lanternColor  = golib.Color{R: 255, G: 190, B: 120, A: 255}
	sprayColor    = golib.Color{R: 255, G: 250, B: 244, A: 255}
	textColor     = golib.Color{R: 255, G: 246, B: 232, A: 255}
	textShadow    = golib.Color{R: 40, G: 16, B: 24, A: 200}
	accentColor   = golib.Color{R: 255, G: 196, B: 100, A: 255}
	warningColor  = golib.Color{R: 255, G: 90, B: 70, A: 255}
	panelColor    = golib.Color{R: 30, G: 14, B: 30, A: 120}
	overlayColor  = golib.Color{R: 30, G: 12, B: 36, A: 140}
	markerColor   = golib.Color{R: 120, G: 236, B: 255, A: 230}
	goldMarkColor = golib.Color{R: 255, G: 206, B: 90, A: 240}
)

func main() {
	config := golib.Config{
		Title: "Sunset Lagoon", Width: screenWidth, Height: screenHeight,
		// The game waits while the player is in another program.
		PauseUnfocused: true,
	}
	if err := golib.Run(newTitleScene(), config); err != nil {
		log.Fatal(err)
	}
}
