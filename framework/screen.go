package golib

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

// Screen is the surface a game draws on. Run passes it to Game.Draw; use it
// only there. Coordinates are in pixels, from the top-left corner, with Y
// growing downwards. Later drawing covers earlier drawing.
type Screen struct {
	width, height float32
	time          float32 // seconds of game time, for animated map tiles
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

// DrawLine draws a straight line from x1, y1 to x2, y2, thickness pixels wide.
func (s *Screen) DrawLine(x1, y1, x2, y2, thickness float32, color Color) {
	rl.DrawLineEx(rl.Vector2{X: x1, Y: y1}, rl.Vector2{X: x2, Y: y2}, thickness, color)
}

// DrawTriangle fills the triangle with corners x1, y1, x2, y2 and x3, y3. The
// corners can come in any order.
func (s *Screen) DrawTriangle(x1, y1, x2, y2, x3, y3 float32, color Color) {
	// raylib only fills triangles whose corners go counterclockwise on screen.
	if (x2-x1)*(y3-y1)-(y2-y1)*(x3-x1) > 0 {
		x2, y2, x3, y3 = x3, y3, x2, y2
	}
	rl.DrawTriangle(rl.Vector2{X: x1, Y: y1}, rl.Vector2{X: x2, Y: y2}, rl.Vector2{X: x3, Y: y3}, color)
}

// DrawText draws text with its top-left corner at x, y, size pixels high, in
// GoLib's built-in pixel font unless options give a [Font]. Text is drawn at
// whole pixels, rounding x and y, so that its letters stay sharp. A line break
// starts a new line, 2 pixels below the last.
func (s *Screen) DrawText(text string, x, y, size float32, color Color, options ...TextOptions) {
	font, spacing := textFont("DrawText", text, size, options)
	rl.DrawTextEx(font, text, rl.Vector2{X: wholePixel(x), Y: wholePixel(y)}, size, spacing, color)
}

// TextWidth returns the width, in pixels, of text drawn by DrawText at size
// with the same options, or of its longest line.
// Use it to center or right-align text:
//
//	x := (screen.Width() - screen.TextWidth(message, 40)) / 2
func (s *Screen) TextWidth(text string, size float32, options ...TextOptions) float32 {
	font, spacing := textFont("TextWidth", text, size, options)
	return rl.MeasureTextEx(font, text, size, spacing).X
}
