// Platformer is GoLib's example game: run and jump through a forest at night,
// slash the snakes and open every chest.
//
// It is also the reference for making a game with GoLib. Read it in this order:
//
//   - main.go (this file) starts the game: main calls golib.Run with the first
//     scene, on a small pixel art screen.
//   - scenes.go splits the game into scenes (title, play, pause, won) and moves
//     between them with golib.SwitchScene. The play scene turns keys and
//     gamepad buttons into actions, and the title menu works with the
//     keyboard, the mouse and a gamepad.
//   - world.go holds the rules as plain Go, with no input or drawing, so that
//     world_test.go can test them. The level comes from a Tiled map.
//   - draw.go draws the world through a golib.Camera that follows the player:
//     the map layer by layer, with sprites between them.
//   - art.go names the sprite sheets, their frames and animations, and the map.
//   - sounds.go holds the sound effects, which GoLib makes in code.
//   - assets/ holds the pictures and the level, assets/maps/forest.tmx, which
//     opens in Tiled; assets.go puts the folder inside golib dist builds.
//   - DESIGN.md says what the game is, its controls and where its numbers live.
//   - game.json and icon.png name the game and give it an icon in the
//     executables golib builds on Windows.
package main

import (
	"log"

	"golib"
)

// The screen, in pixels. golib.Run scales it up by whole numbers, so the
// pixel art stays sharp.
const (
	screenWidth  = 320
	screenHeight = 180
)

// The game's colors, in one place so the look is easy to change.
var (
	skyColor            = golib.Color{R: 29, G: 35, B: 64, A: 255} // the map's background color
	cloudColor          = golib.Color{R: 64, G: 76, B: 118, A: 170}
	textColor           = golib.Color{R: 240, G: 232, B: 214, A: 255}
	shadowColor         = golib.Color{R: 18, G: 16, B: 30, A: 255}
	buttonColor         = golib.Color{R: 58, G: 50, B: 84, A: 230}
	buttonSelectedColor = golib.Color{R: 214, G: 110, B: 48, A: 255} // the orange of the ground
	overlayColor        = golib.Color{R: 0, G: 0, B: 0, A: 150}      // darkens the level under menus and messages
)

func main() {
	config := golib.Config{Title: "Platformer", Width: screenWidth, Height: screenHeight, PixelArt: true}
	if err := golib.Run(newTitleScene(), config); err != nil {
		log.Fatal(err)
	}
}
