package main

import (
	"fmt"
	"math"

	"golib"
)

var (
	centered     = golib.TextOptions{Align: golib.AlignCenter}
	rightAligned = golib.TextOptions{Align: golib.AlignRight}
)

// drawText draws text with a dark shadow under it, so it reads over the
// bright sky and the dark water alike.
func drawText(screen *golib.Screen, text string, x, y, size float32, color golib.Color, options ...golib.TextOptions) {
	offset := max(2, size/10)
	screen.DrawText(text, x+offset, y+offset, size, textShadow, options...)
	screen.DrawText(text, x, y, size, color, options...)
}

// clockText is seconds as minutes and seconds, rounded up: 1:30, 0:09.
func clockText(seconds float32) string {
	whole := int(math.Ceil(float64(seconds)))
	return fmt.Sprintf("%d:%02d", whole/60, whole%60)
}

// drawHUD draws the score and the time in the top-left corner, the best
// score in the top-right one, markers at the screen's edge for crystals out
// of sight, and the points of each collected crystal floating up.
func drawHUD(screen *golib.Screen, w *world, fx *effects, camera *golib.Camera3D, best int) {
	drawMarkers(screen, w, camera)
	for _, f := range fx.flashes {
		at, ok := camera.ToScreen(f.position.Add(golib.Vector3{Y: 2.2 + 2*f.age}))
		if !ok {
			continue
		}
		color := markerColor
		if f.golden {
			color = goldMarkColor
		}
		fade := 1 - golib.EaseIn(f.age/popupLife)
		drawText(screen, fmt.Sprintf("+%d", f.points), at.X, at.Y, 50, golib.WithOpacity(color, fade), centered)
	}

	screen.DrawRectangle(golib.Rectangle{X: 16, Y: 16, Width: 270, Height: 112}, panelColor)
	drawText(screen, "CRYSTALS", 32, 28, 20, accentColor)
	drawText(screen, fmt.Sprint(w.score), 32, 54, 60, textColor)
	left := w.timeLeft()
	timeColor := textColor
	size := float32(40)
	if left <= 10 && !w.over {
		timeColor = warningColor
		// The last ten seconds swell at each tick.
		fraction := left - float32(math.Floor(float64(left)))
		size = 40 + 10*golib.EaseIn(fraction)
		size = float32(int(size/10) * 10)
	}
	drawText(screen, "TIME", 180, 28, 20, accentColor)
	drawText(screen, clockText(left), 180, 58, size, timeColor)

	drawText(screen, "BEST", screen.Width()-32, 28, 20, accentColor, rightAligned)
	drawText(screen, fmt.Sprint(max(best, w.score)), screen.Width()-32, 54, 40, textColor, rightAligned)
}

// drawMarkers draws a small diamond at the screen's edge towards each crystal
// the camera doesn't see, so the player knows which way to turn.
func drawMarkers(screen *golib.Screen, w *world, camera *golib.Camera3D) {
	// The markers keep inside this box, below the score and the best.
	const margin, top = 36, 150
	box := golib.Rectangle{X: margin, Y: top, Width: screen.Width() - 2*margin, Height: screen.Height() - top - margin}
	middle := box.Center()
	for _, k := range w.crystals {
		at, inFront := camera.ToScreen(k.position)
		if inFront && at.X > margin && at.X < screen.Width()-margin && at.Y > margin && at.Y < screen.Height()-margin {
			continue
		}
		// Its bearing from where the camera looks, on the water, in
		// degrees: 0 ahead, 90 to the right, 180 behind.
		toward := flat(k.position.Sub(camera.Position))
		right := toward.Dot(camera.Right())
		ahead := toward.Dot(flat(camera.Forward()).Normalize())
		bearing := float32(math.Atan2(float64(right), float64(ahead)) * 180 / math.Pi)
		side, turn := float32(1), bearing
		if bearing < 0 {
			side, turn = -1, -bearing
		}
		// Ahead and to the side, on that side's edge, lower the further
		// round it is; behind, along the bottom edge.
		var spot golib.Vector2
		if turn <= 135 {
			x := middle.X + side*box.Width/2
			spot = golib.Vector2{X: x, Y: box.Y + box.Height*golib.Clamp((turn-30)/105, 0, 1)}
		} else {
			spot = golib.Vector2{X: middle.X + side*box.Width/2*(180-turn)/45, Y: box.Y + box.Height}
		}
		color := markerColor
		if k.golden {
			color = goldMarkColor
		}
		const size = 12
		diamond := []golib.Vector2{{X: spot.X, Y: spot.Y - size}, {X: spot.X + size*0.7, Y: spot.Y}, {X: spot.X, Y: spot.Y + size}, {X: spot.X - size*0.7, Y: spot.Y}}
		screen.DrawPolygon(diamond, color)
		screen.DrawPolygonOutline(diamond, 2, textShadow)
	}
}

// drawMessage darkens the screen and writes a title and lines under it, in
// the middle.
func drawMessage(screen *golib.Screen, title string, lines ...string) {
	screen.DrawRectangle(golib.Rectangle{Width: screen.Width(), Height: screen.Height()}, overlayColor)
	y := screen.Height()/2 - float32(60+36*len(lines))/2 - 20
	drawText(screen, title, screen.Width()/2, y, 60, accentColor, centered)
	for i, line := range lines {
		drawText(screen, line, screen.Width()/2, y+100+float32(i)*36, 20, textColor, centered)
	}
}
