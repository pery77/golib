// Crates is a Sokoban puzzle game made with GoLib: push every crate onto a
// goal. DESIGN.md says what it is.
//
// Read it in this order:
//
//   - main.go (this file) starts the game: main loads the levels and the saved
//     progress, and calls golib.Run with the title scene, on a small pixel art
//     screen.
//   - world.go holds the rules of Sokoban as plain Go, with no input or
//     drawing, so that world_test.go can test them.
//   - levels.go lists the levels, which are Tiled maps in assets/maps/, and
//     reads them; levels_test.go solves every one.
//   - play.go is the scene where a level is played: it turns the keyboard and
//     gamepad into steps, slides the player and the crates, and draws the
//     room with its heading.
//   - scenes.go has the other scenes: title, level list, pause, level
//     complete and the end. session.go has what they share, and the keys that
//     work everywhere.
//   - controls.go reads the keys, buttons, stick and pointer; menu.go is the
//     column of buttons the title, pause and complete scenes use.
//   - draw.go has the drawing helpers: where the room goes, the warehouse
//     behind the menus, panels, text and stars. art.go names the frames of
//     the tileset picture.
//   - sounds.go holds the sound effects, and music.go the background tune,
//     both made in code.
//   - progress.go is how far the player got, which session.go loads and
//     saves with golib.LoadData and golib.SaveData.
//   - assets/ holds the levels, the tileset picture and one sound made in
//     jfxr; assets.go puts the folder inside golib dist builds.
//   - DESIGN.md says what the game is, its controls and where its numbers
//     live; game.json names the game in the file golib dist builds.
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
	tileSize     = 16 // pixels, the size of a cell and of the tileset's tiles
)

// The game's colors, in one place so the look is easy to change.
var (
	backgroundColor     = golib.Color{R: 22, G: 20, B: 32, A: 255} // outside the room
	textColor           = golib.Color{R: 238, G: 230, B: 210, A: 255}
	dimTextColor        = golib.Color{R: 140, G: 136, B: 160, A: 255}
	highlightColor      = golib.Color{R: 255, G: 196, B: 84, A: 255} // the goals' amber
	shadowColor         = golib.Color{R: 10, G: 8, B: 16, A: 255}
	buttonColor         = golib.Color{R: 52, G: 50, B: 76, A: 235}
	buttonSelectedColor = golib.Color{R: 196, G: 112, B: 52, A: 255} // the crates' wood
	lockedColor         = golib.Color{R: 36, G: 34, B: 50, A: 235}
	lockedSelectedColor = golib.Color{R: 100, G: 66, B: 50, A: 255}
	panelColor          = golib.Color{R: 30, G: 28, B: 44, A: 245}
	panelBorderColor    = golib.Color{R: 112, G: 106, B: 146, A: 255} // the walls' bricks
	solvedColor         = golib.Color{R: 46, G: 140, B: 88, A: 255}   // the crates on goals
	overlayColor        = golib.Color{R: 0, G: 0, B: 0, A: 160}       // darkens the room under menus and messages
)

func main() {
	levels, err := loadLevels()
	if err != nil {
		log.Fatal(err)
	}
	session, err := newSession(levels)
	if err != nil {
		log.Fatal(err)
	}
	config := golib.Config{Title: "Crates", Width: screenWidth, Height: screenHeight, PixelArt: true}
	if err := golib.Run(newTitleScene(session), config); err != nil {
		log.Fatal(err)
	}
}
