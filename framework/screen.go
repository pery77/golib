package golib

import (
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Screen is the surface a game draws on. Run passes it to Game.Draw; use it
// only there. Coordinates are in pixels, from the top-left corner, with Y
// growing downwards. Later drawing covers earlier drawing.
type Screen struct {
	width, height float32
}

// Width returns the screen width in pixels.
func (s *Screen) Width() float32 {
	return s.width
}

// Height returns the screen height in pixels.
func (s *Screen) Height() float32 {
	return s.height
}

// Clear fills the whole screen with color. Call it at the start of Draw.
func (s *Screen) Clear(color Color) {
	rl.ClearBackground(color)
}

// DrawRectangle fills rect with color.
func (s *Screen) DrawRectangle(rect Rectangle, color Color) {
	rl.DrawRectangleRec(rl.Rectangle{X: rect.X, Y: rect.Y, Width: rect.Width, Height: rect.Height}, color)
}

// DrawCircle fills the circle centered at x, y with color.
func (s *Screen) DrawCircle(x, y, radius float32, color Color) {
	rl.DrawCircleV(rl.Vector2{X: x, Y: y}, radius, color)
}

// DrawText draws text in the default font. x and y are the top-left corner and
// size is the text height, all in pixels.
func (s *Screen) DrawText(text string, x, y, size float32, color Color) {
	rl.DrawTextEx(rl.GetFontDefault(), text, rl.Vector2{X: x, Y: y}, size, textSpacing(size), color)
}

// TextWidth returns the width, in pixels, of text drawn by DrawText at size.
// Use it to center or right-align text:
//
//	x := (screen.Width() - screen.TextWidth(message, 40)) / 2
func (s *Screen) TextWidth(text string, size float32) float32 {
	return rl.MeasureTextEx(rl.GetFontDefault(), text, size, textSpacing(size)).X
}

// textSpacing matches raylib's DrawText: one pixel between letters for every
// 10 pixels of text height.
func textSpacing(size float32) float32 {
	return float32(math.Floor(float64(max(size, 10)) / 10))
}
