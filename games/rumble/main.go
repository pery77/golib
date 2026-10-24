// rumble is a small 3D brawler made with GoLib: Kenney's little astronaut,
// stick in hand, holds a walled courtyard against waves of red astronauts.
// DESIGN.md says what it is.
//
// Read it in this order:
//
//   - main.go (this file) starts the game: the screen's size and the colors.
//   - models.go names the models from Kenney's Platformer Kit.
//   - world.go holds the rules as plain Go, with no input or drawing, so that
//     world_test.go can test them: running, jumping, swinging the stick, the
//     hearts, the score and the waves.
//   - enemies.go has the red astronauts: they wander, chase, punch, reel and
//     go down.
//   - poses.go says which animation each character's body plays, from its
//     own state; the swings, punches and the nod play over it on the torso.
//   - controls.go turns the keys and the sticks into a push from where the
//     camera looks, and keeps the camera behind the player.
//   - scenes.go has the title, play, pause and game over scenes.
//   - draw.go draws the courtyard and the characters in 3D, and hud.go the
//     hearts, the score and the banners over it.
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
	skyColor        = golib.Color{R: 86, G: 150, B: 232, A: 255}
	horizonColor    = golib.Color{R: 206, G: 228, B: 250, A: 255}
	meadowColor     = golib.Color{R: 96, G: 120, B: 70, A: 255}
	grassColor      = golib.Color{R: 110, G: 178, B: 72, A: 255}
	stoneColor      = golib.Color{R: 196, G: 188, B: 170, A: 255}
	darkStoneColor  = golib.Color{R: 138, G: 130, B: 120, A: 255}
	pavingColor     = golib.Color{R: 176, G: 166, B: 146, A: 255}
	roofColor       = golib.Color{R: 196, G: 76, B: 56, A: 255}
	doorColor       = golib.Color{R: 112, G: 74, B: 44, A: 255}
	stickColor      = golib.Color{R: 150, G: 104, B: 60, A: 255}
	waterColor      = golib.Color{R: 40, G: 100, B: 150, A: 255}
	bannerColor     = golib.Color{R: 178, G: 38, B: 46, A: 255}
	ironColor       = golib.Color{R: 90, G: 90, B: 96, A: 255}
	flameColor      = golib.Color{R: 255, G: 170, B: 60, A: 255}
	enemyTint       = golib.Color{R: 255, G: 92, B: 78, A: 255}
	whackedTint     = golib.Color{R: 255, G: 226, B: 120, A: 255} // an enemy flashes gold as the stick lands
	hurtTint        = golib.Color{R: 255, G: 150, B: 150, A: 255}
	textColor       = golib.White
	textShadowColor = golib.Color{R: 30, G: 20, B: 40, A: 200}
	accentColor     = golib.Gold
	heartColor      = golib.Color{R: 236, G: 64, B: 82, A: 255}
	panelColor      = golib.Color{R: 20, G: 16, B: 40, A: 120}
	overlayColor    = golib.Color{R: 20, G: 16, B: 40, A: 150}
)

func main() {
	config := golib.Config{
		Title: "Rumble", Width: screenWidth, Height: screenHeight,
		// Soft edges on the 3D.
		Antialias: true,
		// The game waits while the player is in another program.
		PauseUnfocused: true,
	}
	if err := golib.Run(newTitleScene(), config); err != nil {
		log.Fatal(err)
	}
}
