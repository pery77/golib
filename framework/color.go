package golib

import rl "github.com/gen2brain/raylib-go/raylib"

// Color is an RGBA color with 8 bits per channel. A is the opacity: 255 is
// fully opaque. It is the same type raylib uses, so a Color can be passed to
// raylib functions directly.
//
//	background := golib.Color{R: 20, G: 24, B: 32, A: 255}
type Color = rl.Color

// The raylib color palette.
var (
	LightGray  = rl.LightGray
	Gray       = rl.Gray
	DarkGray   = rl.DarkGray
	Yellow     = rl.Yellow
	Gold       = rl.Gold
	Orange     = rl.Orange
	Pink       = rl.Pink
	Red        = rl.Red
	Maroon     = rl.Maroon
	Green      = rl.Green
	Lime       = rl.Lime
	DarkGreen  = rl.DarkGreen
	SkyBlue    = rl.SkyBlue
	Blue       = rl.Blue
	DarkBlue   = rl.DarkBlue
	Purple     = rl.Purple
	Violet     = rl.Violet
	DarkPurple = rl.DarkPurple
	Beige      = rl.Beige
	Brown      = rl.Brown
	DarkBrown  = rl.DarkBrown
	White      = rl.White
	Black      = rl.Black
	Blank      = rl.Blank // Fully transparent.
	Magenta    = rl.Magenta
	RayWhite   = rl.RayWhite // The off-white raylib uses for backgrounds.
)
