// tetris is a complete Tetris made with GoLib. DESIGN.md says what it is.
//
// Read it in this order:
//
//   - main.go (this file) starts the game: main makes the settings and the
//     saved records, then calls golib.Run with the title scene. The screen's
//     size and the game's colors are here.
//   - scenes.go holds the scenes (title, play, pause, game over) and the
//     settings and records every scene shares.
//   - play.go is the scene where the game is played: Update turns the keyboard
//     and gamepad into actions for the world, plays the sounds and runs the
//     effects, and Draw draws the world.
//   - world.go holds the rules as plain Go, with no input or drawing, so that
//     world_test.go can test them.
//   - draw.go draws the board, the blocks, the panels and the effects.
//   - sounds.go holds the music and the sound effects, all made in code.
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

// The board's size and place on the screen: the cell size in pixels, the size
// of the whole board, and its top-left corner. The board is centered, leaving
// room for the hold panel on the left and the next and score panels on the
// right.
const (
	boardCell   = 30
	boardPixelW = boardColumns * boardCell // 300
	boardPixelH = boardRows * boardCell    // 600
	boardX      = (screenWidth - boardPixelW) / 2
	boardY      = 60
)

// The game's colors, in one place so the look is easy to change.
var (
	backgroundColor = golib.Color{R: 16, G: 18, B: 26, A: 255} // behind everything
	panelColor      = golib.Color{R: 26, G: 30, B: 42, A: 255} // hold, next and score boxes
	panelEdgeColor  = golib.Color{R: 60, G: 68, B: 88, A: 255}
	boardColor      = golib.Color{R: 10, G: 12, B: 18, A: 255} // the playfield
	boardEdgeColor  = golib.Color{R: 70, G: 80, B: 104, A: 255}
	gridColor       = golib.Color{R: 28, G: 32, B: 44, A: 255} // faint cell lines
	textColor       = golib.Color{R: 226, G: 232, B: 242, A: 255}
	dimTextColor    = golib.Color{R: 120, G: 130, B: 150, A: 255}
	accentColor     = golib.Color{R: 120, G: 200, B: 255, A: 255}
)

// pieceColors is one color per tetromino, in pieceKind order, so each kind is
// easy to tell apart at a glance.
var pieceColors = [pieceKinds]golib.Color{
	pieceI: {R: 60, G: 200, B: 230, A: 255}, // cyan
	pieceO: {R: 240, G: 200, B: 60, A: 255}, // yellow
	pieceT: {R: 170, G: 90, B: 220, A: 255}, // purple
	pieceS: {R: 90, G: 200, B: 100, A: 255}, // green
	pieceZ: {R: 230, G: 80, B: 80, A: 255},  // red
	pieceJ: {R: 70, G: 110, B: 230, A: 255}, // blue
	pieceL: {R: 240, G: 150, B: 60, A: 255}, // orange
}

func main() {
	options := newOptions()
	records := newRecords()
	config := golib.Config{
		Title: "Tetris", Width: screenWidth, Height: screenHeight,
		// The game waits while the player is in another program: a falling
		// piece shouldn't drop while they are away from the window.
		PauseUnfocused: true,
	}
	if err := golib.Run(newTitleScene(options, records), config); err != nil {
		log.Fatal(err)
	}
}
