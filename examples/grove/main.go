// grove is a small 3D collect-and-run game made with GoLib: Kenney's little
// astronaut crosses a forest island of grass blocks, picks up coins, punches
// rocks open for more, and races the clock to the flag at the far end.
// DESIGN.md says what it is.
//
// Read it in this order:
//
//   - main.go (this file) starts the game: the screen's size and the colors.
//   - models.go names the FBX models of the pack and how long each of its
//     animations really lasts (see the note there).
//   - island.go reads the island from assets/levels/island.txt: the ground's
//     height cell by cell, and where the coins, rocks, trees and flag are.
//   - world.go holds the rules as plain Go, with no input or drawing, so that
//     world_test.go and island_test.go can test them: walking, sprinting,
//     jumping, the coins, the rocks, the clock and the flag.
//   - poses.go says which animation the astronaut plays, from its own state.
//   - controls.go turns the keys and the sticks into a push from where the
//     camera looks, and keeps the camera behind the astronaut.
//   - scenes.go has the title, play, pause and end scenes.
//   - draw.go draws the island in 3D, and hud.go the coins, the clock and the
//     messages over it.
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
	skyColor        = golib.Color{R: 92, G: 162, B: 238, A: 255}
	horizonColor    = golib.Color{R: 208, G: 232, B: 250, A: 255}
	seaColor        = golib.Color{R: 46, G: 132, B: 178, A: 255}
	seaBedColor     = golib.Color{R: 40, G: 90, B: 110, A: 255}
	textColor       = golib.White
	textShadowColor = golib.Color{R: 20, G: 30, B: 50, A: 200}
	accentColor     = golib.Gold
	warningColor    = golib.Color{R: 255, G: 96, B: 80, A: 255}
	panelColor      = golib.Color{R: 16, G: 28, B: 48, A: 130}
	overlayColor    = golib.Color{R: 16, G: 28, B: 48, A: 150}
	hitTint         = golib.Color{R: 255, G: 210, B: 160, A: 255} // a rock flashes as a punch lands
)

func main() {
	config := golib.Config{
		Title: "Grove", Width: screenWidth, Height: screenHeight,
		// Soft edges on the 3D.
		Antialias: true,
		// The game waits while the player is in another program.
		PauseUnfocused: true,
	}
	l, err := loadIsland()
	if err != nil {
		log.Fatal(err)
	}
	if err := golib.Run(newTitleScene(l), config); err != nil {
		log.Fatal(err)
	}
}
