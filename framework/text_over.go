package golib

import (
	"math"
	"strings"

	"golib/internal/device"
)

// overText is text drawn with TextOptions.FullResolution, waiting to go over
// the screen at the window's resolution. Its position and size are in
// screen pixels, through the camera it was drawn with.
type overText struct {
	text  string
	x, y  float32
	size  float32
	color Color
	font  *Font // nil for the built-in font
	align TextAlign
}

// overScale is how many window pixels a screen pixel took the last time the
// screen went into the window, so that TextWidth measures full-resolution
// text as it is drawn; 0 before.
var overScale float32

// overText returns text, drawn at x, y and seen through the screen's camera
// unless there is none, as it waits to be drawn over the screen.
func (s *Screen) overText(text string, x, y, size float32, color Color, options TextOptions) *overText {
	if s.camera != nil {
		at := s.camera.ToScreen(Vector2{X: x, Y: y})
		x, y, size = at.X, at.Y, size*s.camera.zoom()
	}
	return &overText{text: text, x: x, y: y, size: size, color: color, font: options.Font, align: options.Align}
}

// draw draws the text over the screen in fit, scale window pixels a screen
// pixel, each line at a whole pixel of the window.
func (t *overText) draw(fit device.Rectangle, scale float32) {
	size := t.size * scale
	font, spacing := device.DefaultFont(), textSpacing(t.size)*scale
	if t.font != nil {
		var err error
		if font, err = t.font.atlas(size, t.text); err != nil {
			reportError(err)
			return
		}
		spacing = 0
	}
	for i, line := range strings.Split(t.text, "\n") {
		left := t.x * scale
		switch t.align {
		case AlignCenter:
			left -= device.TextWidth(font, line, size, spacing) / 2
		case AlignRight:
			left -= device.TextWidth(font, line, size, spacing)
		}
		top := (t.y + float32(i)*(t.size+textLineGap)) * scale
		x := float32(math.Round(float64(fit.X + left)))
		y := float32(math.Round(float64(fit.Y + top)))
		device.DrawText(font, line, x, y, size, spacing, t.color)
	}
}
