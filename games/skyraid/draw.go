package main

import (
	"math"

	"golib"
)

// Drawing tuning.
const (
	gridSpacing   = 160 // pixels between the arena's grid lines
	starCount     = 90  // stars per layer
	starMargin    = 100 // pixels the star field reaches past each side of the screen
	nebulaCount   = 7
	nebulaDepth   = 0.5 // how far nebulae move for each pixel the camera moves
	shipNose      = 22  // pixels from the ship's center to its nose, where bullets start
	shotStreak    = 0.022
	ringSegments  = 24 // straight lines in a drawn ring
	blinkRate     = 16 // blinks per second of a hurt ship or an old repair kit
	enemyBarWidth = 36 // pixels, for the hull bar over a damaged enemy
)

// starDepths is how far each layer of stars moves for each pixel the camera
// moves: less is farther away.
var starDepths = [...]float32{0.12, 0.3, 0.55}

// backdrop is the space behind the arena: stars in layers and a few nebulae,
// scattered at random once for the whole session.
type backdrop struct {
	stars   [len(starDepths)][]star
	nebulae []nebula
}

type star struct {
	x, y float32 // in the star field, which repeats every screen plus margins
	size float32 // radius in pixels
}

type nebula struct {
	x, y   float32 // where it is when the camera is at 0, 0
	radius float32
	color  golib.Color
}

func newBackdrop() *backdrop {
	b := &backdrop{}
	fieldW, fieldH := float32(screenWidth+2*starMargin), float32(screenHeight+2*starMargin)
	for layer := range b.stars {
		for range starCount {
			b.stars[layer] = append(b.stars[layer], star{
				x:    golib.RandomFloat(0, fieldW),
				y:    golib.RandomFloat(0, fieldH),
				size: golib.RandomFloat(0.6, 1.2) * float32(layer+1) * 0.8,
			})
		}
	}
	// Nebulae move at half the camera's speed, so they spread over half the
	// arena plus a screen.
	spanW := float32((worldWidth-screenWidth)*nebulaDepth + screenWidth)
	spanH := float32((worldHeight-screenHeight)*nebulaDepth + screenHeight)
	for n := range nebulaCount {
		b.nebulae = append(b.nebulae, nebula{
			x:      golib.RandomFloat(0, spanW),
			y:      golib.RandomFloat(0, spanH),
			radius: golib.RandomFloat(180, 380),
			color:  nebulaColors[n%len(nebulaColors)],
		})
	}
	return b
}

// draw draws the backdrop as the camera sees it.
func (b *backdrop) draw(screen *golib.Screen, c camera) {
	screen.Clear(spaceColor)
	for _, n := range b.nebulae {
		x := n.x - c.x*nebulaDepth + c.shakeX*nebulaDepth
		y := n.y - c.y*nebulaDepth + c.shakeY*nebulaDepth
		if x+n.radius < 0 || x-n.radius > screenWidth || y+n.radius < 0 || y-n.radius > screenHeight {
			continue
		}
		// Three see-through circles make a soft blob.
		screen.DrawCircle(x, y, n.radius, n.color)
		screen.DrawCircle(x+n.radius*0.25, y-n.radius*0.1, n.radius*0.65, n.color)
		screen.DrawCircle(x-n.radius*0.2, y+n.radius*0.15, n.radius*0.4, n.color)
	}
	fieldW, fieldH := float32(screenWidth+2*starMargin), float32(screenHeight+2*starMargin)
	for layer, stars := range b.stars {
		depth := starDepths[layer]
		shade := withAlpha(starColor, 0.35+0.3*float32(layer))
		for _, s := range stars {
			x := wrap(s.x-c.x*depth, fieldW) - starMargin
			y := wrap(s.y-c.y*depth, fieldH) - starMargin
			screen.DrawCircle(x, y, s.size, shade)
		}
	}
}

// drawWorld draws the arena and everything in it, from back to front, as the
// camera sees it. It reads the state and never changes it.
func drawWorld(screen *golib.Screen, w *world, b *backdrop, c camera) {
	b.draw(screen, c)
	drawGrid(screen, c)
	drawBorder(screen, c)
	for _, wp := range w.warps {
		drawWarp(screen, wp, c)
	}
	for _, r := range w.repairs {
		drawRepair(screen, r, w.time, c)
	}
	for _, p := range w.particles {
		if !c.sees(p.x, p.y, p.size*4) {
			continue
		}
		share := p.life / p.lifetime
		x, y := c.toScreen(p.x, p.y)
		if p.blast {
			// A white flash that shrinks inside a ring that grows. GoLib has no
			// additive blending, so see-through colors can only darken: the
			// flash is white to read as light.
			grown := p.size * (1 + 2.5*(1-share))
			screen.DrawCircle(x, y, p.size*share*share, withAlpha(golib.White, share))
			drawRing(screen, x, y, grown, 1+4*share, withAlpha(p.color, share))
			continue
		}
		screen.DrawCircle(x, y, p.size*(0.3+0.7*share), withAlpha(p.color, share))
	}
	for _, b := range w.shots {
		if !c.sees(b.x, b.y, 20) {
			continue
		}
		x, y := c.toScreen(b.x, b.y)
		screen.DrawLine(x-b.vx*shotStreak, y-b.vy*shotStreak, x, y, b.radius, shotColor)
	}
	for _, e := range w.enemies {
		if c.sees(e.x, e.y, enemyKinds[e.kind].radius+4) {
			drawEnemy(screen, e, c)
		}
	}
	if w.ship.alive {
		drawShip(screen, &w.ship, w.time, c)
	}
	// Enemy bullets go on top of everything, so they are never hidden.
	for _, b := range w.enemyShots {
		if !c.sees(b.x, b.y, b.radius*2) {
			continue
		}
		x, y := c.toScreen(b.x, b.y)
		screen.DrawCircle(x, y, b.radius*1.9, withAlpha(enemyShotColor, 0.3))
		screen.DrawCircle(x, y, b.radius, enemyShotColor)
		screen.DrawCircle(x, y, b.radius*0.5, enemyShotCore)
	}
}

// drawGrid draws faint lines across the arena, so movement shows even where
// there is nothing else.
func drawGrid(screen *golib.Screen, c camera) {
	left, top := c.toScreen(0, 0)
	right, bottom := c.toScreen(worldWidth, worldHeight)
	firstX := float32(math.Floor(float64(c.x/gridSpacing))) * gridSpacing
	for x := firstX; x <= c.x+screenWidth+gridSpacing; x += gridSpacing {
		sx, _ := c.toScreen(x, 0)
		screen.DrawLine(sx, max(top, 0), sx, min(bottom, screenHeight), 1, gridColor)
	}
	firstY := float32(math.Floor(float64(c.y/gridSpacing))) * gridSpacing
	for y := firstY; y <= c.y+screenHeight+gridSpacing; y += gridSpacing {
		_, sy := c.toScreen(0, y)
		screen.DrawLine(max(left, 0), sy, min(right, screenWidth), sy, 1, gridColor)
	}
}

// drawBorder draws the arena's edge as a glowing line.
func drawBorder(screen *golib.Screen, c camera) {
	left, top := c.toScreen(0, 0)
	right, bottom := c.toScreen(worldWidth, worldHeight)
	for _, line := range [...][4]float32{
		{left, top, right, top},
		{right, top, right, bottom},
		{right, bottom, left, bottom},
		{left, bottom, left, top},
	} {
		screen.DrawLine(line[0], line[1], line[2], line[3], 24, borderGlow)
		screen.DrawLine(line[0], line[1], line[2], line[3], 10, borderGlow)
		screen.DrawLine(line[0], line[1], line[2], line[3], 3, borderColor)
	}
}

// Shapes, pointing right, in pixels from their center. Each is drawn as a fan
// of triangles from the center, so every corner must be visible from it.
var (
	shipShape    = []golib.Vector2{{X: shipNose, Y: 0}, {X: -8, Y: 7}, {X: -14, Y: 15}, {X: -10, Y: 0}, {X: -14, Y: -15}, {X: -8, Y: -7}}
	scoutShape   = []golib.Vector2{{X: 17, Y: 0}, {X: -10, Y: 11}, {X: -5, Y: 0}, {X: -10, Y: -11}}
	gunshipShape = []golib.Vector2{{X: 22, Y: 0}, {X: 6, Y: 7}, {X: -4, Y: 21}, {X: -15, Y: 10}, {X: -10, Y: 0}, {X: -15, Y: -10}, {X: -4, Y: -21}, {X: 6, Y: -7}}
	heavyShape   = []golib.Vector2{{X: 34, Y: 0}, {X: 22, Y: 18}, {X: 4, Y: 32}, {X: -20, Y: 28}, {X: -32, Y: 10}, {X: -32, Y: -10}, {X: -20, Y: -28}, {X: 4, Y: -32}, {X: 22, Y: -18}}
	enemyShapes  = [...][]golib.Vector2{scout: scoutShape, gunship: gunshipShape, heavy: heavyShape}
)

// drawShape draws a shape centered at the screen point x, y, turned by angle,
// filled with one color and outlined with another.
func drawShape(screen *golib.Screen, shape []golib.Vector2, x, y, angle float32, fill, outline golib.Color) {
	corners := make([]golib.Vector2, len(shape))
	for i, p := range shape {
		rx, ry := rotate(p.X, p.Y, angle)
		corners[i] = golib.Vector2{X: x + rx, Y: y + ry}
	}
	for i, a := range corners {
		b := corners[(i+1)%len(corners)]
		screen.DrawTriangle(x, y, a.X, a.Y, b.X, b.Y, fill)
	}
	for i, a := range corners {
		b := corners[(i+1)%len(corners)]
		screen.DrawLine(a.X, a.Y, b.X, b.Y, 2, outline)
	}
}

// drawShip draws the player's ship, blinking while it can't be hurt.
func drawShip(screen *golib.Screen, s *ship, time float32, c camera) {
	x, y := c.toScreen(s.x, s.y)
	if s.dashLeft > 0 {
		screen.DrawCircle(x, y, shipRadius+10, withAlpha(shipColor, 0.25))
	}
	if s.hurt > 0 && int(time*blinkRate)%2 == 0 {
		return
	}
	if s.thrusting && length(s.vx, s.vy) > 40 {
		// The flame flickers against the way the ship flies, which isn't
		// always where it aims.
		flame := 12 + 5*float32(math.Sin(float64(s.flameFlicker)*50))
		back := angleOf(s.vx, s.vy)
		ax, ay := rotate(-8, -5, back)
		bx, by := rotate(-8, 5, back)
		tx, ty := rotate(-8-flame, 0, back)
		screen.DrawTriangle(x+ax, y+ay, x+bx, y+by, x+tx, y+ty, flameColor)
	}
	drawShape(screen, shipShape, x, y, s.angle, shipDarkColor, shipColor)
	screen.DrawCircle(x+2*float32(math.Cos(float64(s.angle))), y+2*float32(math.Sin(float64(s.angle))), 4, shipColor)
	if s.flash > 0 {
		nx, ny := direction(s.angle)
		screen.DrawCircle(x+nx*(shipNose+4), y+ny*(shipNose+4), 7, shotColor)
	}
}

// drawEnemy draws an enemy facing the ship, white for a moment when hit, with
// a hull bar once it is damaged.
func drawEnemy(screen *golib.Screen, e enemy, c camera) {
	k := enemyKinds[e.kind]
	x, y := c.toScreen(e.x, e.y)
	color := enemyColors[e.kind]
	fill := darker(color, 0.35)
	if e.flash > 0 {
		fill, color = golib.White, golib.White
	}
	drawShape(screen, enemyShapes[e.kind], x, y, e.angle, fill, color)
	if e.kind == heavy {
		screen.DrawCircle(x, y, 9, color)
	}
	if e.hull < k.hull && k.hull > 1 {
		top := y - k.radius - 12
		bar := golib.Rectangle{X: x - enemyBarWidth/2, Y: top, Width: enemyBarWidth, Height: 4}
		screen.DrawRectangle(bar, panelColor)
		bar.Width *= float32(e.hull) / float32(k.hull)
		screen.DrawRectangle(bar, color)
	}
}

// drawWarp draws the ring that shows where an enemy is about to arrive.
func drawWarp(screen *golib.Screen, wp warp, c camera) {
	k := enemyKinds[wp.kind]
	if !c.sees(wp.x, wp.y, k.radius+80) {
		return
	}
	x, y := c.toScreen(wp.x, wp.y)
	share := wp.left / warpTime // 1 when it starts, 0 when the enemy arrives
	drawRing(screen, x, y, k.radius+70*share, 3, withAlpha(warpColor, 1-share*0.7))
	screen.DrawCircle(x, y, k.radius*(1-share), withAlpha(warpColor, 0.4))
}

// drawRepair draws a repair kit: a green cross in a ring that pulses, and
// blinks when it is about to go.
func drawRepair(screen *golib.Screen, r repairKit, time float32, c camera) {
	if !c.sees(r.x, r.y, repairRadius+6) {
		return
	}
	if r.life < repairLifetime-repairBlinkAfter && int(time*blinkRate/2)%2 == 0 {
		return
	}
	x, y := c.toScreen(r.x, r.y)
	pulse := 1 + 0.12*float32(math.Sin(float64(time)*8))
	radius := repairRadius * pulse
	screen.DrawCircle(x, y, radius, withAlpha(repairColor, 0.2))
	drawRing(screen, x, y, radius, 2, repairColor)
	arm, thick := radius*0.6, radius*0.28
	screen.DrawRectangle(golib.Rectangle{X: x - arm, Y: y - thick/2, Width: 2 * arm, Height: thick}, repairColor)
	screen.DrawRectangle(golib.Rectangle{X: x - thick/2, Y: y - arm, Width: thick, Height: 2 * arm}, repairColor)
}

// drawRing draws a circle's outline, which GoLib doesn't have, as straight
// lines.
func drawRing(screen *golib.Screen, x, y, radius, thickness float32, color golib.Color) {
	lastX, lastY := x+radius, y
	for i := 1; i <= ringSegments; i++ {
		dx, dy := direction(2 * math.Pi * float32(i) / ringSegments)
		nextX, nextY := x+dx*radius, y+dy*radius
		screen.DrawLine(lastX, lastY, nextX, nextY, thickness, color)
		lastX, lastY = nextX, nextY
	}
}

// drawCrosshair draws the mouse aim at the screen point x, y.
func drawCrosshair(screen *golib.Screen, x, y float32) {
	drawRing(screen, x, y, 11, 2, textColor)
	for _, d := range [...][2]float32{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		screen.DrawLine(x+d[0]*6, y+d[1]*6, x+d[0]*17, y+d[1]*17, 2, textColor)
	}
	screen.DrawCircle(x, y, 1.5, textColor)
}

// withAlpha returns color made see-through: alpha 0 is invisible, 1 unchanged.
func withAlpha(color golib.Color, alpha float32) golib.Color {
	color.A = uint8(float32(color.A) * clamp(alpha, 0, 1))
	return color
}

// darker returns color with its red, green and blue scaled by share.
func darker(color golib.Color, share float32) golib.Color {
	color.R = uint8(float32(color.R) * share)
	color.G = uint8(float32(color.G) * share)
	color.B = uint8(float32(color.B) * share)
	return color
}
