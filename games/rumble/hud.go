package main

import (
	"fmt"

	"golib"
)

var (
	centered    = golib.TextOptions{Align: golib.AlignCenter}
	rightJusted = golib.TextOptions{Align: golib.AlignRight}
)

// drawText draws text with a dark shadow under it, so it reads over the sky
// and the lawn.
func drawText(screen *golib.Screen, text string, x, y, size float32, color golib.Color, options ...golib.TextOptions) {
	offset := size / 10
	screen.DrawText(text, x+offset, y+offset, size, textShadowColor, options...)
	screen.DrawText(text, x, y, size, color, options...)
}

// drawHUD draws the hearts, the score and the wave in the top corners.
func drawHUD(screen *golib.Screen, w *world, best int) {
	screen.DrawRectangle(golib.Rectangle{X: 16, Y: 16, Width: 250, Height: 104}, panelColor)
	for i := range playerHealth {
		drawHeart(screen, float32(44+i*42), 46, i < w.player.health)
	}
	drawText(screen, fmt.Sprintf("Score %d", w.score), 32, 76, 30, textColor)

	right := screen.Width() - 16
	screen.DrawRectangle(golib.Rectangle{X: right - 220, Y: 16, Width: 220, Height: 128}, panelColor)
	wave := "Wave -"
	if w.wave > 0 {
		wave = fmt.Sprintf("Wave %d", w.wave)
	}
	drawText(screen, wave, right-16, 28, 40, accentColor, rightJusted)
	left := 0
	for _, e := range w.enemies {
		if !e.down() {
			left++
		}
	}
	drawText(screen, fmt.Sprintf("Enemies %d", left), right-16, 76, 20, textColor, rightJusted)
	drawText(screen, fmt.Sprintf("Best %d", best), right-16, 100, 20, golib.WithOpacity(textColor, 0.8), rightJusted)
}

// drawHeart draws a heart centered on x, y, filled or empty.
func drawHeart(screen *golib.Screen, x, y float32, full bool) {
	color := heartColor
	if !full {
		color = golib.WithOpacity(golib.White, 0.25)
	}
	screen.DrawCircle(x-7, y-5, 9, color)
	screen.DrawCircle(x+7, y-5, 9, color)
	screen.DrawTriangle(x-15.8, y-2, x+15.8, y-2, x, y+15, color)
}

// drawBanner writes a big line in the middle of the screen, with a smaller
// one under it, fading in and out over its time.
func drawBanner(screen *golib.Screen, title, line string, opacity float32) {
	if opacity <= 0 {
		return
	}
	x, y := screen.Width()/2, screen.Height()*0.26
	drawText(screen, title, x, y, 60, golib.WithOpacity(accentColor, opacity), centered)
	if line != "" {
		drawText(screen, line, x, y+76, 30, golib.WithOpacity(textColor, opacity), centered)
	}
}

// drawMessage darkens the screen and writes a title and lines under it, in the middle.
func drawMessage(screen *golib.Screen, title string, lines ...string) {
	screen.DrawRectangle(golib.Rectangle{Width: screen.Width(), Height: screen.Height()}, overlayColor)
	y := screen.Height()/2 - float32(40+30*len(lines))/2 - 30
	drawText(screen, title, screen.Width()/2, y, 60, accentColor, centered)
	for i, line := range lines {
		drawText(screen, line, screen.Width()/2, y+90+float32(i)*36, 20, textColor, centered)
	}
}
