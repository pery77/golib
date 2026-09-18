package golib

import (
	"math"

	"golib/internal/device"
)

// Color is an RGBA color with 8 bits per channel. A is the opacity: 255 is
// fully opaque. It is the same type raylib uses, so a Color can be passed to
// be passed to raylib functions directly.
//
//	background := golib.Color{R: 20, G: 24, B: 32, A: 255}
type Color = device.Color

// The raylib color palette.
var (
	LightGray  = device.LightGray
	Gray       = device.Gray
	DarkGray   = device.DarkGray
	Yellow     = device.Yellow
	Gold       = device.Gold
	Orange     = device.Orange
	Pink       = device.Pink
	Red        = device.Red
	Maroon     = device.Maroon
	Green      = device.Green
	Lime       = device.Lime
	DarkGreen  = device.DarkGreen
	SkyBlue    = device.SkyBlue
	Blue       = device.Blue
	DarkBlue   = device.DarkBlue
	Purple     = device.Purple
	Violet     = device.Violet
	DarkPurple = device.DarkPurple
	Beige      = device.Beige
	Brown      = device.Brown
	DarkBrown  = device.DarkBrown
	White      = device.White
	Black      = device.Black
	Blank      = device.Blank // Fully transparent.
	Magenta    = device.Magenta
	RayWhite   = device.RayWhite // The off-white raylib uses for backgrounds.
)

// WithOpacity returns color with its opacity set to opacity, from 0,
// invisible, to 1, solid, such as to fade something in or out:
//
//	screen.DrawRectangle(whole, golib.WithOpacity(golib.White, s.flashLeft/flashTime))
//
// It sets the opacity rather than scaling the color's own, so fading a color
// that is already see-through means multiplying: WithOpacity(c,
// float32(c.A)/255*fade).
func WithOpacity(color Color, opacity float32) Color {
	color.A = uint8(math.Round(float64(max(0, min(opacity, 1)) * 255)))
	return color
}
