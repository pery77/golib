package main

import (
	"fmt"
	"strings"

	"golib"
)

// HUD layout, in screen pixels.
const (
	hudMargin     = 20
	radarWidth    = 240 // the arena scaled down; its height follows the arena's shape
	radarScale    = float32(radarWidth) / worldWidth
	radarHeight   = worldHeight * radarScale
	arrowInset    = 26   // pixels from the screen's edge to the arrows at enemies off the screen
	arrowSize     = 11   // pixels from an arrow's tip to its back
	arrowFarAway  = 1600 // pixels: enemies this far or farther get the faintest arrow
	hullPipWidth  = 30
	hullPipHeight = 12
	dashBarWidth  = 120

	// Where what the game couldn't load goes, at the bottom left.
	noteLine    = 88 // letters of a note on a line, at 20 pixels on a 1280 pixel screen
	noteSpacing = 26 // pixels from one line of notes to the next
	noteBottom  = 76 // pixels from the bottom of the screen to the last line
)

// centered puts the middle of each line at the x that DrawText is given.
var centered = golib.TextOptions{Align: golib.AlignCenter}

// drawHUD draws everything that stays in place over the arena: the score, the
// hull, the wave, the dash charge, the radar and the arrows to enemies off the
// screen. It draws in screen pixels, so the camera must be off.
func drawHUD(screen *golib.Screen, w *world, camera *golib.Camera) {
	drawArrows(screen, w, camera)
	drawDanger(screen, w)

	// Score and hull, top left.
	screen.DrawText(fmt.Sprintf("SCORE %d", w.score), hudMargin, hudMargin, 30, textColor)
	for i := range shipHull {
		pip := golib.Rectangle{X: hudMargin + float32(i)*(hullPipWidth+6), Y: hudMargin + 40, Width: hullPipWidth, Height: hullPipHeight}
		color := panelColor
		if i < w.ship.hull {
			color = repairColor
			if w.ship.hull <= 2 {
				color = warnColor
			}
		}
		screen.DrawRectangle(pip, color)
	}

	// Wave and enemies left, top middle.
	// The break between waves has its own banner.
	if w.wave > 0 && w.fighting() {
		drawCentered(screen, fmt.Sprintf("WAVE %d", w.wave), hudMargin, 30, textColor)
		drawCentered(screen, fmt.Sprintf("%d enemies left   Esc: pause", w.enemiesLeft()), hudMargin+40, 20, dimTextColor)
	}

	// Dash charge, bottom left.
	charge := 1 - w.ship.dashCharge/dashCooldown
	labelColor := dimTextColor
	if charge >= 1 {
		labelColor = shipColor
	}
	screen.DrawText("DASH", hudMargin, screenHeight-hudMargin-30, 20, labelColor)
	bar := golib.Rectangle{X: hudMargin + 60, Y: screenHeight - hudMargin - 26, Width: dashBarWidth, Height: 12}
	screen.DrawRectangle(bar, panelColor)
	bar.Width *= clamp(charge, 0, 1)
	screen.DrawRectangle(bar, labelColor)

	drawRadar(screen, w, camera.View())
	drawBanner(screen, w)
}

// drawBanner shows the wave that is coming during the break between waves.
func drawBanner(screen *golib.Screen, w *world) {
	if w.fighting() || w.over {
		return
	}
	// Above the ship, which is in the middle of the screen.
	const top = 110
	if w.wave > 0 {
		drawCentered(screen, fmt.Sprintf("Wave %d cleared", w.wave), top, 30, repairColor)
		if w.lastBonus > 0 {
			drawCentered(screen, fmt.Sprintf("Flawless! +%d", w.lastBonus), top+40, 20, textColor)
		}
	}
	drawCentered(screen, fmt.Sprintf("WAVE %d", w.wave+1), top+80, 60, titleColor)
	drawCentered(screen, fmt.Sprintf("%d enemies incoming", len(waveEnemies(w.wave+1))), top+150, 20, dimTextColor)
}

// drawRadar draws the whole arena, scaled down, in the bottom-right corner,
// with a frame around the part of it the camera shows.
func drawRadar(screen *golib.Screen, w *world, view golib.Rectangle) {
	left := float32(screenWidth - hudMargin - radarWidth)
	top := float32(screenHeight-hudMargin) - radarHeight
	panel := golib.Rectangle{X: left, Y: top, Width: radarWidth, Height: radarHeight}
	screen.DrawRectangle(panel, radarColor)
	screen.DrawRectangleOutline(panel, 1, withAlpha(borderColor, 0.7))
	screen.DrawRectangleOutline(golib.Rectangle{
		X: left + view.X*radarScale, Y: top + view.Y*radarScale,
		Width: view.Width * radarScale, Height: view.Height * radarScale,
	}, 1, withAlpha(textColor, 0.35))

	dot := func(at golib.Vector2, radius float32, color golib.Color) {
		screen.DrawCircle(left+at.X*radarScale, top+at.Y*radarScale, radius, color)
	}
	for _, r := range w.repairs {
		dot(r.position, 3, repairColor)
	}
	if int(w.time*4)%2 == 0 {
		for _, wp := range w.warps {
			dot(wp.position, 2, warpColor)
		}
	}
	for _, e := range w.enemies {
		dot(e.position, 1.5+float32(e.kind), enemyColors[e.kind])
	}
	if w.ship.alive {
		dot(w.ship.position, 3, shipColor)
	}
}

// drawArrows points at every enemy and repair kit off the screen, from the
// screen's edge. Nearer ones have stronger arrows.
func drawArrows(screen *golib.Screen, w *world, camera *golib.Camera) {
	if !w.ship.alive {
		return
	}
	view := camera.View()
	radarLeft := float32(screenWidth-hudMargin-radarWidth) - arrowSize*2
	radarTop := float32(screenHeight-hudMargin) - radarHeight - arrowSize*2
	arrow := func(at golib.Vector2, color golib.Color) {
		if view.Contains(at.X, at.Y) {
			return
		}
		on := camera.ToScreen(at) // outside the screen, so the arrow has a direction
		edge := edgePoint(on, arrowInset)
		if edge.X > radarLeft && edge.Y > radarTop {
			// Keep arrows off the radar: slide them along the edge to its side.
			if edge.X-radarLeft < edge.Y-radarTop {
				edge.X = radarLeft
			} else {
				edge.Y = radarTop
			}
		}
		strength := clamp(1-at.Distance(w.ship.position)/arrowFarAway, 0.25, 1)
		angle := on.Sub(edge).Angle()
		tip := golib.Vector2{X: arrowSize}.Rotate(angle).Add(edge)
		left := golib.Vector2{X: -arrowSize, Y: arrowSize * 0.8}.Rotate(angle).Add(edge)
		right := golib.Vector2{X: -arrowSize, Y: -arrowSize * 0.8}.Rotate(angle).Add(edge)
		screen.DrawTriangle(tip.X, tip.Y, left.X, left.Y, right.X, right.Y, withAlpha(color, strength))
	}
	for _, e := range w.enemies {
		arrow(e.position, enemyColors[e.kind])
	}
	for _, r := range w.repairs {
		arrow(r.position, repairColor)
	}
}

// edgePoint returns where an arrow to a point off the screen goes: on a frame
// inset pixels inside the screen's edge, on the line from the screen's middle
// to the point. GoLib has no such edge maths, so this stays the game's.
func edgePoint(on golib.Vector2, inset float32) golib.Vector2 {
	middle := golib.Vector2{X: screenWidth / 2, Y: screenHeight / 2}
	away := on.Sub(middle)
	half := middle.Sub(golib.Vector2{X: inset, Y: inset})
	// Scale the line down until it touches the frame.
	scale := min(half.X/max(abs(away.X), 0.001), half.Y/max(abs(away.Y), 0.001))
	return middle.Add(away.Scale(scale))
}

// drawDanger pulses red at the screen's edges while the ship has one hull
// point left.
func drawDanger(screen *golib.Screen, w *world) {
	if !w.ship.alive || w.ship.hull > 1 {
		return
	}
	color := withAlpha(warnColor, 0.15+0.2*w.danger())
	const edge = 14
	screen.DrawRectangle(golib.Rectangle{Width: screenWidth, Height: edge}, color)
	screen.DrawRectangle(golib.Rectangle{Y: screenHeight - edge, Width: screenWidth, Height: edge}, color)
	screen.DrawRectangle(golib.Rectangle{Y: edge, Width: edge, Height: screenHeight - 2*edge}, color)
	screen.DrawRectangle(golib.Rectangle{X: screenWidth - edge, Y: edge, Width: edge, Height: screenHeight - 2*edge}, color)
}

// drawNotes says on screen what the game couldn't load: a shader that isn't
// where it should be, or music GoLib can't play. It shows nothing when
// everything loaded, so a finished game never shows it.
func drawNotes(screen *golib.Screen, notes ...string) {
	var lines []string
	for _, note := range notes {
		if note != "" {
			lines = append(lines, wrapText(note, noteLine)...)
		}
	}
	top := screenHeight - noteBottom - float32(len(lines))*noteSpacing
	for i, line := range lines {
		screen.DrawText(line, hudMargin, top+float32(i)*noteSpacing, 20, warnColor)
	}
}

// wrapText breaks text into lines of at most width letters, on the spaces.
func wrapText(text string, width int) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		switch {
		case line == "":
			line = word
		case len(line)+1+len(word) <= width:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
	}
	return append(lines, line)
}

// drawCentered draws text centered across the screen, with its top at y.
func drawCentered(screen *golib.Screen, text string, y, size float32, color golib.Color) {
	screen.DrawText(text, screen.Width()/2, y, size, color, centered)
}
