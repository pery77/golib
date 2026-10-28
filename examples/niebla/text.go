package main

import "golib"

// The fonts every text in the game is drawn with, and the options that
// choose them: uiText for plain text, uiTextBold for emphasis.
var (
	uiFont     = golib.NewFont("fonts/FiraSans-Regular.ttf")
	uiFontBold = golib.NewFont("fonts/FiraSans-Medium.ttf")
	uiText     = golib.TextOptions{Font: uiFont}
	uiTextBold = golib.TextOptions{Font: uiFontBold}
)
