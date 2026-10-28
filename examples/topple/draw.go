package main

import (
	"fmt"
	"math"

	"golib"
)

// The game's colors for text and the HUD.
var (
	textColor   = golib.Color{R: 250, G: 246, B: 236, A: 255}
	shadowColor = golib.Color{R: 20, G: 16, B: 12, A: 200}
	accentColor = golib.Color{R: 255, G: 196, B: 80, A: 255}
	panelColor  = golib.Color{R: 24, G: 20, B: 16, A: 170}
	skyColor    = golib.Color{R: 120, G: 176, B: 230, A: 255}
	arcColor    = golib.Color{R: 255, G: 255, B: 255, A: 150}
)

// The decoration outside the fence: trees, rocks and flowers, placed once.
type decoration struct {
	model *golib.Model
	place golib.Transform
}

var decorations = func() []decoration {
	var d []decoration
	for i := range 28 {
		a := float64(i) / 28 * 2 * math.Pi
		r := float32(13.5 + 2.5*math.Sin(float64(i)*2.3))
		model := tree
		if i%3 == 0 {
			model = pineTree
		}
		size := float32(1.6 + 0.5*math.Sin(float64(i)*1.7))
		d = append(d, decoration{model, golib.Transform{
			Position: golib.Vector3{X: r * float32(math.Sin(a)), Z: r * float32(math.Cos(a))},
			Rotation: golib.Rotation{Yaw: float32(i) * 47},
			Scale:    golib.Vector3{X: size, Y: size, Z: size},
		}})
	}
	for i := range 12 {
		a := float64(i)/12*2*math.Pi + 0.3
		d = append(d, decoration{rocks, golib.Transform{
			Position: golib.Vector3{X: 11.2 * float32(math.Sin(a)), Z: 11.2 * float32(math.Cos(a))},
			Rotation: golib.Rotation{Yaw: float32(i) * 71},
			Scale:    golib.Vector3{X: 1.4, Y: 1.4, Z: 1.4},
		}})
	}
	for i := range 16 {
		x := float32(i%4)*4.6 - 7
		z := float32(i/4)*4.8 - 8.2
		if i%2 == 0 {
			x += 1.3
		}
		d = append(d, decoration{flowers, golib.Transform{Position: golib.Vector3{X: x, Y: 0.01, Z: z}, Rotation: golib.Rotation{Yaw: float32(i) * 37}}})
	}
	return d
}()

// look is how a scene shows the world: the camera, the throw's arc, and
// the player's animator.
type look struct {
	camera   *golib.Camera3D
	animator *golib.Animator
	arc      []golib.Vector3
}

// drawWorld draws the yard, the platforms, the stacks, the balls and the
// player.
func drawWorld(screen *golib.Screen, w *world, l look) {
	screen.Clear(skyColor)
	screen.Begin3D(l.camera)
	screen.SetEnvironment(golib.Environment{
		SunDirection:  golib.Vector3{X: -0.5, Y: -1, Z: -0.35},
		SunStrength:   1.1,
		Sky:           skyColor,
		Horizon:       golib.Color{R: 200, G: 222, B: 240, A: 255},
		Ground:        golib.Color{R: 90, G: 110, B: 70, A: 255},
		SkyBackground: true,
		FogDistance:   80,
	})
	screen.DrawMeshWith(boxMesh, groundPlace, grassLook)
	screen.DrawMeshWith(boxMesh, golib.Transform{Position: golib.Vector3{Y: -0.49}, Scale: golib.Vector3{X: 2 * yardHalf, Y: 1, Z: 2 * yardHalf}}, yardLook)
	for _, f := range fencePlaces() {
		screen.DrawMeshWith(boxMesh, f, fenceLook)
	}
	for _, d := range decorations {
		screen.DrawModel(d.model, d.place, golib.White)
	}
	for _, p := range w.level.Platforms {
		place := platformPlace(p)
		screen.DrawMeshWith(boxMesh, place, plinthLook)
		top := golib.Transform{Position: golib.Vector3{X: p.X, Y: p.Height - 0.04, Z: p.Z}, Scale: golib.Vector3{X: p.Width + 0.08, Y: 0.1, Z: p.Depth + 0.08}}
		screen.DrawMeshWith(boxMesh, top, plinthTop)
	}
	for _, s := range w.stones {
		screen.DrawMeshWith(boxMesh, s.Transform(), stoneLook)
	}
	for _, p := range w.props {
		drawProp(screen, p)
	}
	left := w.left()
	for _, p := range w.props {
		if p.target && !p.down && left <= markAtMost {
			bob := float32(math.Sin(float64(w.time*4))) * 0.08
			at := p.body.Position().Add(golib.Vector3{Y: 0.75 + bob})
			screen.DrawMeshWith(cylinderMesh, golib.Transform{Position: at, Rotation: golib.Rotation{Pitch: 180}, Scale: golib.Vector3{X: 0.18, Y: 0.25, Z: 0.18}}, markLook)
		}
	}
	for _, b := range w.balls {
		t := b.body.Transform()
		// A ball about to be taken away shrinks out of sight.
		left := min(ballIdle-b.idle, ballLife-b.age)
		if left < ballShrink {
			t.Scale = t.Scale.Scale(max(0.05, left/ballShrink))
		}
		screen.DrawMeshWith(sphereMesh, t, ballLook)
	}
	for i := 1; i < len(l.arc) && !w.cleared; i += 2 {
		screen.DrawLine3D(l.arc[i-1], l.arc[i], arcColor)
	}
	if l.animator != nil {
		body := golib.Transform{Position: w.player.body.Position, Rotation: golib.Rotation{Yaw: w.player.yaw}, Scale: golib.Vector3{X: heroScale, Y: heroScale, Z: heroScale}}
		screen.DrawAnimated(l.animator, body, golib.White)
	}
	screen.End3D()
}

// ballShrink is how many seconds a ball takes to shrink away.
const ballShrink = 0.3

// markAtMost is how few targets must be left before each is marked.
const markAtMost = 3

var markLook = &golib.Material{Color: accentColor, Emission: accentColor, EmissionStrength: 0.6, Unlit: true}

// drawProp draws a crate with its bands, or a barrel with its hoops.
func drawProp(screen *golib.Screen, p *prop) {
	t := p.body.Transform()
	if p.kind == barrelKind {
		screen.DrawMeshWith(cylinderMesh, t, barrelLook)
		for _, y := range []float32{-0.32, 0.32} {
			screen.DrawMeshWith(cylinderMesh, t.Child(golib.Transform{Position: golib.Vector3{Y: y}, Scale: golib.Vector3{X: 1.04, Y: 0.07, Z: 1.04}}), hoopLook)
		}
		return
	}
	screen.DrawMeshWith(boxMesh, t, crateLook)
	for _, y := range []float32{-0.42, 0.42} {
		screen.DrawMeshWith(boxMesh, t.Child(golib.Transform{Position: golib.Vector3{Y: y}, Scale: golib.Vector3{X: 1.03, Y: 0.14, Z: 1.03}}), crateBand)
	}
	for _, x := range []float32{-0.42, 0.42} {
		screen.DrawMeshWith(boxMesh, t.Child(golib.Transform{Position: golib.Vector3{X: x}, Scale: golib.Vector3{X: 0.14, Y: 1.02, Z: 1.03}}), crateBand)
	}
}

// arcOf is the path a ball thrown now would take, as pairs of points for
// dashes, for the first part of the way.
func arcOf(w *world, want intent) []golib.Vector3 {
	mark := w.aimPoint(want.aimAt, want.aimBy)
	from := w.hand(mark.Sub(w.player.body.Position))
	v := throwVelocity(from, mark)
	var points []golib.Vector3
	const steps = 24
	total := golib.Clamp(mark.Distance(from)/throwSpeed, 0.08, throwMaxTime)
	for i := range steps {
		t := total * float32(i) / steps
		points = append(points, golib.Vector3{
			X: from.X + v.X*t,
			Y: from.Y + v.Y*t - 0.5*9.81*t*t,
			Z: from.Z + v.Z*t,
		})
	}
	return points
}

// popup is a word that rises over something that happened, such as a crate
// that came down.
type popup struct {
	at   golib.Vector3
	time float32 // seconds since it appeared
}

const popupLife = 1.0 // seconds

// agePopups moves the popups on, and drops the old ones.
func agePopups(popups []popup, dt float32) []popup {
	kept := popups[:0]
	for _, p := range popups {
		p.time += dt
		if p.time < popupLife {
			kept = append(kept, p)
		}
	}
	return kept
}

// drawPopups writes "Down!" over each target that just came down.
func drawPopups(screen *golib.Screen, popups []popup, camera *golib.Camera3D) {
	for _, p := range popups {
		at, ok := camera.ToScreen(p.at.Add(golib.Vector3{Y: 0.6 + p.time*0.8}))
		if !ok {
			continue
		}
		fade := 1 - golib.EaseIn(p.time/popupLife)
		color := golib.WithOpacity(accentColor, fade)
		center := golib.TextOptions{Align: golib.AlignCenter}
		screen.DrawText("Down!", at.X+2, at.Y+2, 24, golib.WithOpacity(shadowColor, fade), center)
		screen.DrawText("Down!", at.X, at.Y, 24, color, center)
	}
}

// drawText writes text with a shadow under it, to read over the world.
func drawText(screen *golib.Screen, text string, x, y, size float32, color golib.Color) {
	screen.DrawText(text, x+2, y+2, size, shadowColor)
	screen.DrawText(text, x, y, size, color)
}

// drawCentered writes text centered on x.
func drawCentered(screen *golib.Screen, text string, x, y, size float32, color golib.Color) {
	center := golib.TextOptions{Align: golib.AlignCenter}
	screen.DrawText(text, x+2, y+2, size, shadowColor, center)
	screen.DrawText(text, x, y, size, color, center)
}

// drawCrosshair draws the cursor the ball is thrown at.
func drawCrosshair(screen *golib.Screen, x, y float32) {
	screen.DrawCircleOutline(x, y, 16, 4, shadowColor)
	screen.DrawCircleOutline(x, y, 15, 2, textColor)
	for _, d := range [][4]float32{{-24, 0, -9, 0}, {9, 0, 24, 0}, {0, -24, 0, -9}, {0, 9, 0, 24}} {
		screen.DrawLine(x+d[0], y+d[1], x+d[2], y+d[3], 4, shadowColor)
		screen.DrawLine(x+d[0], y+d[1], x+d[2], y+d[3], 2, textColor)
	}
}

// drawHUD shows the level, the balls thrown and the targets left.
func drawHUD(screen *golib.Screen, w *world, r *run) {
	screen.DrawRectangle(golib.Rectangle{X: 16, Y: 16, Width: 360, Height: 104}, panelColor)
	drawText(screen, fmt.Sprintf("Level %d of %d: %s", w.number+1, len(r.levels), w.level.Name), 30, 26, 20, accentColor)
	drawText(screen, fmt.Sprintf("Balls: %d   (par %d)", w.thrown, w.level.Par), 30, 56, 20, textColor)
	drawText(screen, fmt.Sprintf("Still up: %d of %d", w.left(), w.targets()), 30, 84, 20, textColor)
	drawText(screen, "Esc: pause", screenWidth-150, screenHeight-36, 20, textColor)
}

// stars is how well a level went: 3 at or under par, 2 a little over, else 1.
func stars(balls, par int) int {
	switch {
	case balls <= par:
		return 3
	case balls <= par+2:
		return 2
	}
	return 1
}

// drawStars draws three stars centered on x, n of them filled, each size
// pixels across.
func drawStars(screen *golib.Screen, x, y, size float32, n int) {
	for i := range 3 {
		cx := x + (float32(i)-1)*size*1.2
		var points []golib.Vector2
		for k := range 10 {
			r := size / 2
			if k%2 == 1 {
				r *= 0.45
			}
			a := float64(k) / 10 * 2 * math.Pi
			points = append(points, golib.Vector2{X: cx + r*float32(math.Sin(a)), Y: y + size/2 - r*float32(math.Cos(a))})
		}
		if i < n {
			screen.DrawPolygon(points, accentColor)
		} else {
			screen.DrawPolygon(points, golib.WithOpacity(shadowColor, 0.6))
		}
		screen.DrawPolygonOutline(points, 2, shadowColor)
	}
}
