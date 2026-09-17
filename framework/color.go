package golib

import (
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

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

// WithOpacity returns color with its opacity set to opacity, from 0,
// invisible, to 1, solid, such as to fade something in or out:
//
//	screen.DrawRectangle(whole, golib.WithOpacity(golib.White, s.flashLeft/flashTime))
func WithOpacity(color Color, opacity float32) Color {
	color.A = uint8(math.Round(float64(max(0, min(opacity, 1)) * 255)))
	return color
}
