// topple is a game made with GoLib: knock the crates and barrels off the
// platforms of a yard with as few balls as you can. DESIGN.md says more.
//
// Read it in this order:
//
//   - main.go (this file) starts the game with the title scene.
//   - scenes.go has the scenes: title, play, pause, cleared and won.
//   - level.go reads the levels from assets/levels/*.json.
//   - world.go holds the rules: the yard as a golib.PhysicsWorld, the
//     crates, barrels and balls as rigid bodies, the player as a
//     golib.CharacterController, and when a level is cleared.
//   - controls.go turns the keyboard, the mouse and the gamepad into what
//     the player wants, and places the camera.
//   - draw.go draws the world and the HUD; models.go has the meshes, the
//     models and the materials; sounds.go the sounds.
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
	levels, err := loadLevels()
	if err != nil {
		log.Fatal(err)
	}
	config := golib.Config{
		Title: "Topple", Width: screenWidth, Height: screenHeight,
		PauseUnfocused: true,
	}
	if err := golib.Run(newTitleScene(levels), config); err != nil {
		log.Fatal(err)
	}
}
