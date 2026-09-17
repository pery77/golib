package main

import (
	"fmt"
	"math"

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
)

// drawHUD draws everything that stays in place over the arena: the score, the
// hull, the wave, the dash charge, the radar and the arrows to enemies off the
// screen.
func drawHUD(screen *golib.Screen, w *world, c camera) {
	drawArrows(screen, w, c)
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

	drawRadar(screen, w, c)
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

// drawRadar draws the whole arena, scaled down, in the bottom-right corner.
func drawRadar(screen *golib.Screen, w *world, c camera) {
	left := float32(screenWidth - hudMargin - radarWidth)
	top := float32(screenHeight-hudMargin) - radarHeight
	screen.DrawRectangle(golib.Rectangle{X: left, Y: top, Width: radarWidth, Height: radarHeight}, radarColor)
	outline(screen, golib.Rectangle{X: left, Y: top, Width: radarWidth, Height: radarHeight}, 1, withAlpha(borderColor, 0.7))
	outline(screen, golib.Rectangle{
		X: left + c.x*radarScale, Y: top + c.y*radarScale,
		Width: screenWidth * radarScale, Height: screenHeight * radarScale,
	}, 1, withAlpha(textColor, 0.35))

	dot := func(x, y, radius float32, color golib.Color) {
		screen.DrawCircle(left+x*radarScale, top+y*radarScale, radius, color)
	}
	for _, r := range w.repairs {
		dot(r.x, r.y, 3, repairColor)
	}
	if int(w.time*4)%2 == 0 {
		for _, wp := range w.warps {
			dot(wp.x, wp.y, 2, warpColor)
		}
	}
	for _, e := range w.enemies {
		dot(e.x, e.y, 1.5+float32(e.kind), enemyColors[e.kind])
	}
	if w.ship.alive {
		dot(w.ship.x, w.ship.y, 3, shipColor)
	}
}

// drawArrows points at every enemy and repair kit off the screen, from the
// screen's edge. Nearer ones have stronger arrows.
func drawArrows(screen *golib.Screen, w *world, c camera) {
	if !w.ship.alive {
		return
	}
	radarLeft := float32(screenWidth-hudMargin-radarWidth) - arrowSize*2
	radarTop := float32(screenHeight-hudMargin) - radarHeight - arrowSize*2
	arrow := func(x, y float32, color golib.Color) {
		ex, ey, ok := c.edgePoint(x, y, arrowInset)
		if !ok {
			return
		}
		if ex > radarLeft && ey > radarTop {
			// Keep arrows off the radar: slide them along the edge to its side.
			if ex-radarLeft < ey-radarTop {
				ex = radarLeft
			} else {
				ey = radarTop
			}
		}
		distance := float32(math.Sqrt(float64(distanceSquared(x, y, w.ship.x, w.ship.y))))
		strength := clamp(1-distance/arrowFarAway, 0.25, 1)
		sx, sy := c.toScreen(x, y)
		angle := angleOf(sx-ex, sy-ey)
		tipX, tipY := rotate(arrowSize, 0, angle)
		leftX, leftY := rotate(-arrowSize, arrowSize*0.8, angle)
		rightX, rightY := rotate(-arrowSize, -arrowSize*0.8, angle)
		screen.DrawTriangle(ex+tipX, ey+tipY, ex+leftX, ey+leftY, ex+rightX, ey+rightY, withAlpha(color, strength))
	}
	for _, e := range w.enemies {
		arrow(e.x, e.y, enemyColors[e.kind])
	}
	for _, r := range w.repairs {
		arrow(r.x, r.y, repairColor)
	}
}

// drawDanger pulses red at the screen's edges while the ship has one hull
// point left.
func drawDanger(screen *golib.Screen, w *world) {
	if !w.ship.alive || w.ship.hull > 1 {
		return
	}
	pulse := 0.5 + 0.5*float32(math.Sin(float64(w.time)*6))
	color := withAlpha(warnColor, 0.15+0.2*pulse)
	const edge = 14
	screen.DrawRectangle(golib.Rectangle{Width: screenWidth, Height: edge}, color)
	screen.DrawRectangle(golib.Rectangle{Y: screenHeight - edge, Width: screenWidth, Height: edge}, color)
	screen.DrawRectangle(golib.Rectangle{Y: edge, Width: edge, Height: screenHeight - 2*edge}, color)
	screen.DrawRectangle(golib.Rectangle{X: screenWidth - edge, Y: edge, Width: edge, Height: screenHeight - 2*edge}, color)
}

// outline draws a rectangle's edges, which GoLib doesn't have, as lines.
func outline(screen *golib.Screen, r golib.Rectangle, thickness float32, color golib.Color) {
	right, bottom := r.X+r.Width, r.Y+r.Height
	screen.DrawLine(r.X, r.Y, right, r.Y, thickness, color)
	screen.DrawLine(right, r.Y, right, bottom, thickness, color)
	screen.DrawLine(right, bottom, r.X, bottom, thickness, color)
	screen.DrawLine(r.X, bottom, r.X, r.Y, thickness, color)
}

// drawCentered draws text centered across the screen, with its top at y.
func drawCentered(screen *golib.Screen, text string, y, size float32, color golib.Color) {
	screen.DrawText(text, (screen.Width()-screen.TextWidth(text, size))/2, y, size, color)
}
