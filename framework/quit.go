package golib

import "sync/atomic"

// quitRequested is set by Quit. Run clears it when it starts and checks it
// after every update.
var quitRequested atomic.Bool

// Quit ends the game: Run returns right after the update that called Quit,
// without drawing another frame. Call it from Update, for example when the
// player picks Quit in a menu:
//
//	if input.KeyPressed(golib.KeyEscape) {
//		golib.Quit()
//	}
//
// Closing the window also ends the game. No key quits on its own, not even
// Esc: the game decides.
func Quit() {
	quitRequested.Store(true)
}
