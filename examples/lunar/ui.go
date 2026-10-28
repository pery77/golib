package main

import (
	"fmt"
	"golib"
)

var (
	ink        = rgb(7, 13, 23)
	panelColor = rgb(13, 25, 38)
	lineColor  = rgb(39, 64, 78)
	cyan       = rgb(107, 240, 221)
	white      = rgb(227, 238, 237)
	muted      = rgb(129, 158, 171)
	orange     = rgb(255, 168, 101)
	red        = rgb(255, 108, 113)
)

func rgb(r, g, b uint8) golib.Color             { return golib.Color{R: r, G: g, B: b, A: 255} }
func fade(c golib.Color, a float32) golib.Color { return golib.WithOpacity(c, a) }
func rect(s *golib.Screen, x, y, w, h float32, c golib.Color) {
	s.DrawRectangle(golib.Rectangle{X: x, Y: y, Width: w, Height: h}, c)
}
func text(s *golib.Screen, v string, x, y, size float32, c golib.Color) { s.DrawText(v, x, y, size, c) }
func right(s *golib.Screen, v string, x, y, size float32, c golib.Color) {
	s.DrawText(v, x, y, size, c, golib.TextOptions{Align: golib.AlignRight})
}
func center(s *golib.Screen, v string, x, y, size float32, c golib.Color) {
	s.DrawText(v, x, y, size, c, golib.TextOptions{Align: golib.AlignCenter})
}
func box(s *golib.Screen, r golib.Rectangle) {
	s.DrawRectangle(r, fade(panelColor, 0.96))
	s.DrawRectangleOutline(r, 1, lineColor)
	s.DrawLine(r.X, r.Y, r.X+30, r.Y, 2, cyan)
}
func money(v int) string {
	if v < 1000 {
		return fmt.Sprintf("%d", v)
	}
	return fmt.Sprintf("%d,%03d", v/1000, v%1000)
}
func meter(s *golib.Screen, x, y, w, fraction float32, c golib.Color) {
	rect(s, x, y, w, 5, lineColor)
	rect(s, x, y, w*golib.Clamp(fraction, 0, 1), 5, c)
}

type button struct {
	r           golib.Rectangle
	title, hint string
	disabled    bool
}

func buttonAt(x, y, w, h float32, title, hint string) button {
	return button{r: golib.Rectangle{X: x, Y: y, Width: w, Height: h}, title: title, hint: hint}
}
func drawButton(s *golib.Screen, b button, selected bool) {
	fill, edge, fg := panelColor, lineColor, white
	if selected {
		fill = rgb(25, 54, 63)
		edge = cyan
	}
	if b.disabled {
		fg = muted
		fill = rgb(12, 20, 30)
	}
	s.DrawRectangle(b.r, fade(fill, 0.97))
	s.DrawRectangleOutline(b.r, 1, edge)
	if selected {
		rect(s, b.r.X, b.r.Y, 3, b.r.Height, cyan)
	}
	if b.hint == "" {
		center(s, b.title, b.r.Center().X, b.r.Y+(b.r.Height-20)/2, 20, fg)
	} else {
		text(s, b.title, b.r.X+18, b.r.Y+16, 20, fg)
		text(s, b.hint, b.r.X+18, b.r.Y+43, 16, muted)
	}
}
func menuInput(in *golib.Input, buttons []button, selected *int) int {
	if len(buttons) == 0 {
		return -1
	}
	*selected = (*selected + len(buttons)) % len(buttons)
	previous := *selected
	if in.KeyPressed(golib.KeyDown) || in.KeyPressed(golib.KeyRight) || in.GamepadPressed(0, golib.GamepadDown) || in.GamepadPressed(0, golib.GamepadRight) {
		*selected = (*selected + 1) % len(buttons)
	}
	if in.KeyPressed(golib.KeyUp) || in.KeyPressed(golib.KeyLeft) || in.GamepadPressed(0, golib.GamepadUp) || in.GamepadPressed(0, golib.GamepadLeft) {
		*selected = (*selected + len(buttons) - 1) % len(buttons)
	}
	mx, my := in.MousePosition()
	click := in.MousePressed(golib.MouseLeft)
	for i, b := range buttons {
		if b.r.Contains(mx, my) && (in.MouseMoved() || click) {
			*selected = i
			if click && !b.disabled {
				clickSound.Play()
				return i
			}
		}
	}
	if previous != *selected {
		clickSound.PlayWith(0.4, 1)
	}
	if confirm(in) && !buttons[*selected].disabled {
		clickSound.Play()
		return *selected
	}
	return -1
}
func footer(s *golib.Screen, a *session) {
	s.DrawLine(40, 679, 1240, 679, 1, lineColor)
	text(s, "SELENE  /  FRONTIER", 40, 694, 14, muted)
	fx, audio := "ON", "ON"
	if a.progress.EffectsOff {
		fx = "OFF"
	}
	if a.progress.Muted {
		audio = "OFF"
	}
	hint := fmt.Sprintf("F2  FX %s     M  AUDIO %s     F11  FULLSCREEN", fx, audio)
	if liveShaderEditing {
		hint = "F5  RELOAD SHADER     " + hint
	}
	right(s, hint, 1240, 694, 14, muted)
	if a.notice != "" {
		box(s, golib.Rectangle{X: 60, Y: 614, Width: 1160, Height: 56})
		for i, line := range wrapText(a.notice, 100) {
			text(s, line, 80, 625+float32(i)*19, 14, orange)
		}
	}
}
func badge(s *golib.Screen, v string, x, y float32, c golib.Color) {
	w := s.TextWidth(v, 14) + 24
	rect(s, x, y, w, 25, fade(c, 0.09))
	s.DrawRectangleOutline(golib.Rectangle{X: x, Y: y, Width: w, Height: 25}, 1, fade(c, 0.35))
	text(s, v, x+12, y+6, 14, c)
}

// A custom monoline wordmark keeps the main title clean at large sizes.
func wordmark(s *golib.Screen, x, y, h float32, c golib.Color) {
	strokes := map[rune][][]golib.Vector2{
		'S': {{{X: 0.8, Y: 0}, {X: 0, Y: 0}, {X: 0, Y: 0.5}, {X: 0.8, Y: 0.5}, {X: 0.8, Y: 1}, {X: 0, Y: 1}}},
		'E': {{{X: 0.8, Y: 0}, {X: 0, Y: 0}, {X: 0, Y: 1}, {X: 0.8, Y: 1}}, {{X: 0, Y: 0.5}, {X: 0.65, Y: 0.5}}},
		'L': {{{X: 0, Y: 0}, {X: 0, Y: 1}, {X: 0.8, Y: 1}}},
		'N': {{{X: 0, Y: 1}, {X: 0, Y: 0}, {X: 0.8, Y: 1}, {X: 0.8, Y: 0}}},
	}
	for _, r := range "SELENE" {
		for _, stroke := range strokes[r] {
			for i := 1; i < len(stroke); i++ {
				a, b := stroke[i-1], stroke[i]
				s.DrawLine(x+a.X*h, y+a.Y*h, x+b.X*h, y+b.Y*h, max(2, h*0.035), c)
			}
		}
		x += h * 1.07
	}
}
