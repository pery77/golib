package main

import (
	"fmt"
	"math"

	"golib"
)

var (
	centered    = golib.TextOptions{Align: golib.AlignCenter}
	rightJusted = golib.TextOptions{Align: golib.AlignRight}
)

// drawText draws text with a dark shadow under it, so it reads over the sky
// and the grass.
func drawText(screen *golib.Screen, text string, x, y, size float32, color golib.Color, options ...golib.TextOptions) {
	offset := size / 10
	screen.DrawText(text, x+offset, y+offset, size, textShadowColor, options...)
	screen.DrawText(text, x, y, size, color, options...)
}

// clockText writes seconds as minutes and seconds, rounded up, as a
// countdown shows them: 0:01 until the very end.
func clockText(seconds float32) string {
	whole := int(math.Ceil(float64(seconds)))
	return fmt.Sprintf("%d:%02d", whole/60, whole%60)
}

// hud is what the play scene shows over the world besides the world itself:
// the effects that are only for the eye, kept by the scene.
type hud struct {
	coinPop golib.Timer // runs after a coin is picked up: the counter swells
	timePop golib.Timer // runs after the clock changes by more than time: "+1.5 s" or "-3 s"
	timeBy  float32     // seconds the clock last changed by
}

// drawHUD draws the coins in the top left, the clock in the top right, and a
// marker over the flag.
func drawHUD(screen *golib.Screen, w *world, camera *golib.Camera3D, h *hud) {
	// The coins.
	screen.DrawRectangle(golib.Rectangle{X: 16, Y: 16, Width: 210, Height: 74}, panelColor)
	swell := 1 + 0.35*(1-golib.EaseOut(h.coinPop.Progress()))
	drawCoinIcon(screen, 54, 53, 20*swell)
	drawText(screen, fmt.Sprintf("x %d", w.taken), 86, 34, 40*swell, textColor)

	// The clock: red and beating in its last ten seconds.
	right := screen.Width() - 16
	screen.DrawRectangle(golib.Rectangle{X: right - 190, Y: 16, Width: 190, Height: 74}, panelColor)
	color, size := textColor, float32(50)
	if w.clock <= 10 && w.outcome == running {
		color = warningColor
		beat := float32(math.Ceil(float64(w.clock))) - w.clock // from 0 just after a second ticks, to 1
		size += 10 * (1 - golib.EaseOut(golib.Clamp(beat*3, 0, 1)))
	}
	drawText(screen, clockText(w.clock), right-20, 53-size/2, size, color, rightJusted)
	if h.timePop.Running() {
		text, c := fmt.Sprintf("+%.1f s", h.timeBy), accentColor
		if h.timeBy < 0 {
			text, c = fmt.Sprintf("%.0f s", h.timeBy), warningColor
		}
		rise := 30 * golib.EaseOut(h.timePop.Progress())
		drawText(screen, text, right-20, 100-rise+30, 20, golib.WithOpacity(c, 1-h.timePop.Progress()), rightJusted)
	}

	// A marker over the flag, so the player knows where to go.
	if w.outcome == running {
		top := w.island.flag.Add(golib.Vector3{Y: 0.9*flagScale + 0.5})
		if at, ok := camera.ToScreen(top); ok && at.X > 0 && at.X < screen.Width() && at.Y > 0 && at.Y < screen.Height() {
			distance := w.player.position.Distance(w.island.flag)
			bob := 4 * float32(math.Sin(float64(w.time*4)))
			screen.DrawTriangle(at.X-12, at.Y-24+bob, at.X+12, at.Y-24+bob, at.X, at.Y-6+bob, accentColor)
			drawText(screen, fmt.Sprintf("%.0f m", distance), at.X, at.Y-50+bob, 20, textColor, centered)
		}
	}
}

// drawCoinIcon draws a coin seen from the front, centered on x, y.
func drawCoinIcon(screen *golib.Screen, x, y, radius float32) {
	screen.DrawCircle(x, y, radius, golib.Color{R: 214, G: 150, B: 30, A: 255})
	screen.DrawCircle(x, y, radius*0.78, golib.Gold)
	screen.DrawRectangle(golib.Rectangle{X: x - radius*0.12, Y: y - radius*0.45, Width: radius * 0.24, Height: radius * 0.9}, golib.Color{R: 214, G: 150, B: 30, A: 255})
}

// drawBanner writes a big line in the middle of the screen, with a smaller
// one under it.
func drawBanner(screen *golib.Screen, title, line string, opacity float32) {
	if opacity <= 0 {
		return
	}
	x, y := screen.Width()/2, screen.Height()*0.22
	drawText(screen, title, x, y, 60, golib.WithOpacity(accentColor, opacity), centered)
	if line != "" {
		drawText(screen, line, x, y+76, 20, golib.WithOpacity(textColor, opacity), centered)
	}
}

// drawMessage darkens the lower part of the screen and writes a title and
// lines there, leaving the astronaut in sight above.
func drawMessage(screen *golib.Screen, title string, titleColor golib.Color, lines ...string) {
	height := float32(130 + 34*len(lines))
	top := screen.Height() - height - 30
	screen.DrawRectangle(golib.Rectangle{Y: top, Width: screen.Width(), Height: height + 30}, overlayColor)
	drawText(screen, title, screen.Width()/2, top+24, 60, titleColor, centered)
	for i, line := range lines {
		drawText(screen, line, screen.Width()/2, top+104+float32(i)*34, 20, textColor, centered)
	}
}
