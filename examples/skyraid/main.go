// Sky Raid is a top-down space shooter made with GoLib: fly around an arena
// several screens wide while waves of enemy ships hunt you down and shoot at
// you. DESIGN.md says what it is, its controls and where its numbers live.
//
// Read it in this order:
//
//   - main.go (this file) starts the game with the title scene, and holds the
//     screen size and the colors.
//   - scenes.go has the title, pause and game over scenes, and the keys every
//     scene shares.
//   - play.go is the scene where the game is played: it turns the keyboard,
//     the mouse and gamepad 0 into controls for the world, and points the
//     camera, a golib.Camera, ahead of the ship.
//   - world.go holds the rules for the ship, the bullets and the repair kits;
//     enemies.go the enemies; waves.go the waves. None of them read input or
//     draw, so the tests can play them. Positions are golib.Vector2, in the
//     arena's pixels, and angles are degrees.
//   - geometry.go has the little maths golib.Vector2 doesn't cover.
//   - draw.go draws the arena and everything in it, through the camera;
//     hud.go the score, the radar and the arrows to enemies off the screen,
//     in screen pixels.
//   - effects.go runs the screen effects over the finished picture: the bloom,
//     the lens and the glitch of a hit. Their shaders are files in
//     assets/shaders/, read as the game starts and again on F5, so they can be
//     tuned without building the game.
//   - sounds.go holds the sound effects, most of them files in
//     assets/sounds/, made with jfxr. music.go finds the tune in
//     assets/music/. assets.go puts the assets folder inside golib dist builds.
package main

import (
	"log"

	"golib"
)

// The screen, in pixels. The arena (world.go) is bigger; the camera shows
// this much of it.
const (
	screenWidth  = 1280
	screenHeight = 720
)

// The game's colors, in one place so the look is easy to change.
var (
	spaceColor     = golib.Color{R: 6, G: 8, B: 20, A: 255}
	gridColor      = golib.Color{R: 30, G: 40, B: 80, A: 110}
	borderColor    = golib.Color{R: 255, G: 70, B: 110, A: 255}
	borderGlow     = golib.Color{R: 255, G: 70, B: 110, A: 40}
	skyTint        = golib.Color{R: 40, G: 40, B: 52, A: 255} // how much of assets/textures/background.png shows
	starColor      = golib.Color{R: 200, G: 215, B: 255, A: 255}
	shipColor      = golib.Color{R: 90, G: 230, B: 255, A: 255}
	shipDarkColor  = golib.Color{R: 20, G: 80, B: 110, A: 255}
	flameColor     = golib.Color{R: 255, G: 160, B: 50, A: 255}
	dashTrailColor = golib.Color{R: 90, G: 230, B: 255, A: 90}
	shotColor      = golib.Color{R: 255, G: 250, B: 170, A: 255}
	sparkColor     = golib.Color{R: 255, G: 220, B: 140, A: 255}
	enemyColors    = [...]golib.Color{
		scout:   {R: 255, G: 90, B: 80, A: 255},
		gunship: {R: 255, G: 170, B: 40, A: 255},
		heavy:   {R: 200, G: 90, B: 255, A: 255},
	}
	enemyShotColor = golib.Color{R: 255, G: 60, B: 150, A: 255}
	enemyShotCore  = golib.Color{R: 255, G: 225, B: 240, A: 255}
	warpColor      = golib.Color{R: 200, G: 120, B: 255, A: 255}
	repairColor    = golib.Color{R: 90, G: 255, B: 140, A: 255}
	textColor      = golib.Color{R: 220, G: 230, B: 250, A: 255}
	dimTextColor   = golib.Color{R: 140, G: 150, B: 180, A: 255}
	titleColor     = golib.Color{R: 90, G: 230, B: 255, A: 255}
	warnColor      = golib.Color{R: 255, G: 90, B: 110, A: 255}
	panelColor     = golib.Color{R: 10, G: 14, B: 32, A: 200}
	radarColor     = golib.Color{R: 8, G: 10, B: 26, A: 240}
	overlayColor   = golib.Color{R: 0, G: 0, B: 0, A: 160}
)

func main() {
	config := golib.Config{Title: "Sky Raid", Width: screenWidth, Height: screenHeight}
	if err := golib.Run(newTitleScene(newSession()), config); err != nil {
		log.Fatal(err)
	}
}
