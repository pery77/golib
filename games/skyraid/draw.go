package main

import (
	"math"

	"golib"
)

// Drawing tuning.
const (
	gridSpacing   = 160  // pixels between the arena's grid lines
	starCount     = 90   // stars per layer
	starMargin    = 100  // pixels the star field reaches past each side of the screen
	skyDepth      = 0.08 // how far the sky picture moves for each pixel the camera moves
	shipNose      = 22   // pixels from the ship's center to its nose, where bullets start
	shotStreak    = 0.022
	blinkRate     = 16 // blinks per second of a hurt ship or an old repair kit
	enemyBarWidth = 36 // pixels, for the hull bar over a damaged enemy
)

// starDepths is how far each layer of stars moves for each pixel the camera
// moves: less is farther away.
var starDepths = [...]float32{0.12, 0.3, 0.55}

// sky is the picture behind everything, from the game's assets folder, so it
// can be swapped without building the game. It is made once, and read the
// first time it is drawn.
var sky = golib.NewSprite("textures/background.png")

// backdrop is the space behind the arena: the sky picture, with stars in
// layers over it, scattered at random once for the whole session.
type backdrop struct {
	stars [len(starDepths)][]star
}

type star struct {
	x, y float32 // in the star field, which repeats every screen plus margins
	size float32 // radius in pixels
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
	return b
}

// draw draws the backdrop in screen pixels, behind the arena. Parallax is the
// game's own: each layer moves by a share of the camera's view, so the
// nearest stars keep up with the arena and the farthest barely move.
func (b *backdrop) draw(screen *golib.Screen, camera *golib.Camera) {
	view := camera.View()
	screen.Clear(spaceColor)
	drawSky(screen, view)
	fieldW, fieldH := float32(screenWidth+2*starMargin), float32(screenHeight+2*starMargin)
	for layer, stars := range b.stars {
		depth := starDepths[layer]
		shade := withAlpha(starColor, 0.35+0.3*float32(layer))
		for _, s := range stars {
			x := wrap(s.x-view.X*depth, fieldW) - starMargin
			y := wrap(s.y-view.Y*depth, fieldH) - starMargin
			screen.DrawCircle(x, y, s.size, shade)
		}
	}
}

// drawSky draws the picture behind everything, scaled so that it covers the
// screen wherever the camera looks, and moved a little with the camera, so the
// arena feels deep. It is drawn much darker than the file (skyTint in
// main.go): at its own brightness the clouds are as bright as the ships and
// the bullets, and the game is hard to read over them.
func drawSky(screen *golib.Screen, view golib.Rectangle) {
	width, height := sky.Width(), sky.Height()
	if width == 0 || height == 0 {
		return // the picture couldn't be read; golib.Run stops and says why
	}
	// Wide and tall enough that the drift never reaches its edges.
	scale := max(
		(screenWidth+(worldWidth-screenWidth)*skyDepth)/width,
		(screenHeight+(worldHeight-screenHeight)*skyDepth)/height)
	screen.DrawSprite(sky, 0, -view.X*skyDepth, -view.Y*skyDepth,
		golib.DrawOptions{Scale: scale, Tint: skyTint})
}

// drawWorld draws the arena and everything in it, from back to front, through
// the camera: every position below is in the arena, not on the screen. It
// reads the state and never changes it.
func drawWorld(screen *golib.Screen, w *world, b *backdrop, camera *golib.Camera) {
	b.draw(screen, camera) // in screen pixels, behind the arena
	view := camera.View()
	screen.SetCamera(camera)
	drawGrid(screen, view)
	drawBorder(screen)
	for _, wp := range w.warps {
		drawWarp(screen, wp, view)
	}
	for _, r := range w.repairs {
		drawRepair(screen, r, w.time, view)
	}
	for _, p := range w.particles {
		if !inView(view, p.position, p.size*4) {
			continue
		}
		share := p.life / p.lifetime
		if p.blast {
			// A white flash that shrinks inside a ring that grows. GoLib has no
			// additive blending, so see-through colors can only darken: the
			// flash is white to read as light.
			grown := p.size * (1 + 2.5*(1-share))
			screen.DrawCircle(p.position.X, p.position.Y, p.size*share*share, withAlpha(golib.White, share))
			screen.DrawCircleOutline(p.position.X, p.position.Y, grown, 1+4*share, withAlpha(p.color, share))
			continue
		}
		screen.DrawCircle(p.position.X, p.position.Y, p.size*(0.3+0.7*share), withAlpha(p.color, share))
	}
	for _, b := range w.shots {
		if !inView(view, b.position, 20) {
			continue
		}
		back := b.position.Sub(b.velocity.Scale(shotStreak))
		screen.DrawLine(back.X, back.Y, b.position.X, b.position.Y, b.radius, shotColor)
	}
	for _, e := range w.enemies {
		if inView(view, e.position, enemyKinds[e.kind].radius+4) {
			drawEnemy(screen, e)
		}
	}
	if w.ship.alive {
		drawShip(screen, &w.ship, w.time)
	}
	// Enemy bullets go on top of everything, so they are never hidden.
	for _, b := range w.enemyShots {
		if !inView(view, b.position, b.radius*2) {
			continue
		}
		screen.DrawCircle(b.position.X, b.position.Y, b.radius*1.9, withAlpha(enemyShotColor, 0.3))
		screen.DrawCircle(b.position.X, b.position.Y, b.radius, enemyShotColor)
		screen.DrawCircle(b.position.X, b.position.Y, b.radius*0.5, enemyShotCore)
	}
	screen.SetCamera(nil) // screen pixels again, for the HUD
}

// inView reports whether a circle in the arena is at least partly in the
// camera's view, so drawing can skip what isn't.
func inView(view golib.Rectangle, at golib.Vector2, radius float32) bool {
	return view.Overlaps(golib.Rectangle{X: at.X - radius, Y: at.Y - radius, Width: 2 * radius, Height: 2 * radius})
}

// drawGrid draws faint lines across the arena, so movement shows even where
// there is nothing else. Only the lines in view are drawn.
func drawGrid(screen *golib.Screen, view golib.Rectangle) {
	left, right := max(view.X, 0), min(view.X+view.Width, worldWidth)
	top, bottom := max(view.Y, 0), min(view.Y+view.Height, worldHeight)
	for x := gridBefore(view.X); x <= view.X+view.Width; x += gridSpacing {
		screen.DrawLine(x, top, x, bottom, 1, gridColor)
	}
	for y := gridBefore(view.Y); y <= view.Y+view.Height; y += gridSpacing {
		screen.DrawLine(left, y, right, y, 1, gridColor)
	}
}

// gridBefore returns the last grid line at or before value.
func gridBefore(value float32) float32 {
	return float32(math.Floor(float64(value/gridSpacing))) * gridSpacing
}

// drawBorder draws the arena's edge as a glowing line: three lines of
// different widths, centered on the edge.
func drawBorder(screen *golib.Screen) {
	for _, line := range [...]struct {
		thickness float32
		color     golib.Color
	}{{24, borderGlow}, {10, borderGlow}, {3, borderColor}} {
		out := line.thickness / 2 // an outline is drawn inside its rectangle
		edge := golib.Rectangle{X: -out, Y: -out, Width: worldWidth + line.thickness, Height: worldHeight + line.thickness}
		screen.DrawRectangleOutline(edge, line.thickness, line.color)
	}
}

// Shapes, pointing right, in pixels from their center. Screen.DrawPolygon
// fills each from its middle, so every corner must be visible from there.
var (
	shipShape    = []golib.Vector2{{X: shipNose, Y: 0}, {X: -8, Y: 7}, {X: -14, Y: 15}, {X: -10, Y: 0}, {X: -14, Y: -15}, {X: -8, Y: -7}}
	scoutShape   = []golib.Vector2{{X: 17, Y: 0}, {X: -10, Y: 11}, {X: -5, Y: 0}, {X: -10, Y: -11}}
	gunshipShape = []golib.Vector2{{X: 22, Y: 0}, {X: 6, Y: 7}, {X: -4, Y: 21}, {X: -15, Y: 10}, {X: -10, Y: 0}, {X: -15, Y: -10}, {X: -4, Y: -21}, {X: 6, Y: -7}}
	heavyShape   = []golib.Vector2{{X: 34, Y: 0}, {X: 22, Y: 18}, {X: 4, Y: 32}, {X: -20, Y: 28}, {X: -32, Y: 10}, {X: -32, Y: -10}, {X: -20, Y: -28}, {X: 4, Y: -32}, {X: 22, Y: -18}}
	enemyShapes  = [...][]golib.Vector2{scout: scoutShape, gunship: gunshipShape, heavy: heavyShape}
)

// drawShape draws a shape centered at a point, turned by angle degrees,
// filled with one color and outlined with another.
func drawShape(screen *golib.Screen, shape []golib.Vector2, at golib.Vector2, angle float32, fill, outline golib.Color) {
	corners := make([]golib.Vector2, len(shape))
	for i, corner := range shape {
		corners[i] = corner.Rotate(angle).Add(at)
	}
	screen.DrawPolygon(corners, fill)
	screen.DrawPolygonOutline(corners, 2, outline)
}

// drawShip draws the player's ship, blinking while it can't be hurt.
func drawShip(screen *golib.Screen, s *ship, time float32) {
	at := s.position
	if s.dashLeft > 0 {
		screen.DrawCircle(at.X, at.Y, shipRadius+10, withAlpha(shipColor, 0.25))
	}
	if s.hurt > 0 && int(time*blinkRate)%2 == 0 {
		return
	}
	if s.thrusting && s.velocity.Length() > 40 {
		// The flame flickers against the way the ship flies, which isn't
		// always where it aims.
		flame := 12 + 5*float32(math.Sin(float64(s.flameFlicker)*50))
		back := s.velocity.Angle()
		a := golib.Vector2{X: -8, Y: -5}.Rotate(back).Add(at)
		b := golib.Vector2{X: -8, Y: 5}.Rotate(back).Add(at)
		tip := golib.Vector2{X: -8 - flame}.Rotate(back).Add(at)
		screen.DrawTriangle(a.X, a.Y, b.X, b.Y, tip.X, tip.Y, flameColor)
	}
	drawShape(screen, shipShape, at, s.angle, shipDarkColor, shipColor)
	aim := golib.Vector2FromAngle(s.angle)
	eye := at.Add(aim.Scale(2))
	screen.DrawCircle(eye.X, eye.Y, 4, shipColor)
	if s.flash > 0 {
		muzzle := at.Add(aim.Scale(shipNose + 4))
		screen.DrawCircle(muzzle.X, muzzle.Y, 7, shotColor)
	}
}

// drawEnemy draws an enemy facing the ship, white for a moment when hit, with
// a hull bar once it is damaged.
func drawEnemy(screen *golib.Screen, e enemy) {
	k := enemyKinds[e.kind]
	color := enemyColors[e.kind]
	fill := darker(color, 0.35)
	if e.flash > 0 {
		fill, color = golib.White, golib.White
	}
	drawShape(screen, enemyShapes[e.kind], e.position, e.angle, fill, color)
	if e.kind == heavy {
		screen.DrawCircle(e.position.X, e.position.Y, 9, color)
	}
	if e.hull < k.hull && k.hull > 1 {
		top := e.position.Y - k.radius - 12
		bar := golib.Rectangle{X: e.position.X - enemyBarWidth/2, Y: top, Width: enemyBarWidth, Height: 4}
		screen.DrawRectangle(bar, panelColor)
		bar.Width *= float32(e.hull) / float32(k.hull)
		screen.DrawRectangle(bar, color)
	}
}

// drawWarp draws the ring that shows where an enemy is about to arrive.
func drawWarp(screen *golib.Screen, wp warp, view golib.Rectangle) {
	k := enemyKinds[wp.kind]
	if !inView(view, wp.position, k.radius+80) {
		return
	}
	share := wp.left / warpTime // 1 when it starts, 0 when the enemy arrives
	screen.DrawCircleOutline(wp.position.X, wp.position.Y, k.radius+70*share, 3, withAlpha(warpColor, 1-share*0.7))
	screen.DrawCircle(wp.position.X, wp.position.Y, k.radius*(1-share), withAlpha(warpColor, 0.4))
}

// drawRepair draws a repair kit: a green cross in a ring that pulses, and
// blinks when it is about to go.
func drawRepair(screen *golib.Screen, r repairKit, time float32, view golib.Rectangle) {
	if !inView(view, r.position, repairRadius+6) {
		return
	}
	if r.life < repairLifetime-repairBlinkAfter && int(time*blinkRate/2)%2 == 0 {
		return
	}
	at := r.position
	pulse := 1 + 0.12*float32(math.Sin(float64(time)*8))
	radius := repairRadius * pulse
	screen.DrawCircle(at.X, at.Y, radius, withAlpha(repairColor, 0.2))
	screen.DrawCircleOutline(at.X, at.Y, radius, 2, repairColor)
	arm, thick := radius*0.6, radius*0.28
	screen.DrawRectangle(golib.Rectangle{X: at.X - arm, Y: at.Y - thick/2, Width: 2 * arm, Height: thick}, repairColor)
	screen.DrawRectangle(golib.Rectangle{X: at.X - thick/2, Y: at.Y - arm, Width: thick, Height: 2 * arm}, repairColor)
}

// drawCrosshair draws the mouse aim at a screen point.
func drawCrosshair(screen *golib.Screen, at golib.Vector2) {
	screen.DrawCircleOutline(at.X, at.Y, 11, 2, textColor)
	for _, way := range [...]golib.Vector2{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		from, to := at.Add(way.Scale(6)), at.Add(way.Scale(17))
		screen.DrawLine(from.X, from.Y, to.X, to.Y, 2, textColor)
	}
	screen.DrawCircle(at.X, at.Y, 1.5, textColor)
}

// withAlpha returns color faded by share: 0 is invisible, 1 unchanged.
// golib.WithOpacity sets an opacity outright; this scales the color's own, so
// that colors already see-through, such as dashTrailColor, stay that way.
func withAlpha(color golib.Color, share float32) golib.Color {
	return golib.WithOpacity(color, float32(color.A)/255*clamp(share, 0, 1))
}

// darker returns color with its red, green and blue scaled by share.
func darker(color golib.Color, share float32) golib.Color {
	color.R = uint8(float32(color.R) * share)
	color.G = uint8(float32(color.G) * share)
	color.B = uint8(float32(color.B) * share)
	return color
}
