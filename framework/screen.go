package golib

import rl "github.com/gen2brain/raylib-go/raylib"

// Screen is the window's drawing surface. Run passes it to Game.Draw; use it
// only there.
type Screen struct{}

// Width returns the screen width in pixels.
func (s *Screen) Width() int {
	return int(rl.GetScreenWidth())
}

// Height returns the screen height in pixels.
func (s *Screen) Height() int {
	return int(rl.GetScreenHeight())
}

// Clear fills the whole screen with color. Call it at the start of Draw.
func (s *Screen) Clear(color Color) {
	rl.ClearBackground(color)
}

// DrawText draws text in the default font. x and y are the top-left corner in
// pixels, and size is the text height in pixels.
func (s *Screen) DrawText(text string, x, y, size int, color Color) {
	rl.DrawText(text, int32(x), int32(y), int32(size), color)
}
