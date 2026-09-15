// Platformer is GoLib's example game: run and jump across the platforms and
// collect every coin.
//
// It is also the reference for making a game with GoLib. Read it in this order:
//
//   - main.go (this file) starts the game: main calls golib.Run with the first
//     scene.
//   - scenes.go splits the game into scenes (title, play, pause, won) and moves
//     between them with golib.SwitchScene. The play scene turns keys and
//     gamepad buttons into actions, the title menu works with the keyboard,
//     the mouse and a gamepad, and every scene draws what it shows.
//   - world.go holds the rules as plain Go, with no input or drawing, so that
//     world_test.go can test them.
//   - DESIGN.md says what the game is, its controls and where its numbers live.
package main

import (
	"log"

	"golib"
)

// The game's colors, in one place so the look is easy to change.
var (
	skyColor            = golib.Color{R: 135, G: 206, B: 235, A: 255}
	cloudColor          = golib.Color{R: 255, G: 255, B: 255, A: 200}
	platformColor       = golib.DarkGreen
	playerColor         = golib.Maroon
	coinColor           = golib.Gold
	textColor           = golib.DarkGray
	buttonColor         = golib.Color{R: 255, G: 255, B: 255, A: 160}
	buttonSelectedColor = golib.Gold
	overlayColor        = golib.Color{R: 0, G: 0, B: 0, A: 150} // darkens the level under the pause and win messages
	messageColor        = golib.RayWhite
)

func main() {
	config := golib.Config{Title: "Platformer", Width: levelWidth, Height: levelHeight}
	if err := golib.Run(newTitleScene(), config); err != nil {
		log.Fatal(err)
	}
}
