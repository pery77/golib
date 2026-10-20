package main

import (
	"fmt"
	"math"

	"golib"
)

// The meshes, made once: every shape in the game is one of these, scaled,
// turned and colored by a golib.Transform.
var (
	boxMesh      = golib.NewBoxMesh()
	sphereMesh   = golib.NewSphereMesh()
	cylinderMesh = golib.NewCylinderMesh()
)

// How things look; the rules don't need these.
const (
	tileSize      = 2    // meters: the checkered floor's squares
	tileDepth     = 0.2  // meters
	gemSize       = 0.55 // meters: the side of the gem's cube, stood on a corner
	gemSpinSpeed  = 120  // degrees per second
	gemBob        = 0.15 // meters up and down
	markSize      = 0.2  // meters: the dots on the marble
	blinkInterval = 0.25 // seconds: the marble hides for a third of each while it can't be hurt
)

// cornerUp is the turn that stands a cube on one of its corners, a diamond:
// a quarter turn of roll, then the tilt that brings a corner to the top.
var cornerUp = golib.Rotation{Roll: 45, Pitch: 35.26}

// scenery is something far away that never moves: a cloud or a small island.
type scenery struct {
	position, scale golib.Vector3
	island          bool
}

// sky is the scenery, in a ring around the platform.
var sky = []scenery{
	{position: golib.Vector3{X: -38, Y: -6, Z: 44}, scale: golib.Vector3{X: 8, Y: 2, Z: 8}, island: true},
	{position: golib.Vector3{X: 52, Y: -12, Z: 20}, scale: golib.Vector3{X: 6, Y: 2, Z: 6}, island: true},
	{position: golib.Vector3{X: 30, Y: -4, Z: -50}, scale: golib.Vector3{X: 7, Y: 2, Z: 7}, island: true},
	{position: golib.Vector3{X: -55, Y: -14, Z: -24}, scale: golib.Vector3{X: 9, Y: 2, Z: 9}, island: true},
	{position: golib.Vector3{X: 10, Y: 4, Z: 60}, scale: golib.Vector3{X: 14, Y: 3, Z: 8}},
	{position: golib.Vector3{X: -45, Y: 2, Z: 10}, scale: golib.Vector3{X: 10, Y: 2.5, Z: 14}},
	{position: golib.Vector3{X: 48, Y: 6, Z: -8}, scale: golib.Vector3{X: 9, Y: 2, Z: 12}},
	{position: golib.Vector3{X: -12, Y: -18, Z: -58}, scale: golib.Vector3{X: 16, Y: 3, Z: 9}},
	{position: golib.Vector3{X: 60, Y: -2, Z: 40}, scale: golib.Vector3{X: 12, Y: 2.5, Z: 9}},
	{position: golib.Vector3{X: -30, Y: 8, Z: -60}, scale: golib.Vector3{X: 12, Y: 2.5, Z: 8}},
}

// drawSky fills the screen with a sky that fades from deep blue at the top to
// pale near the horizon, before the 3D is drawn over it.
func drawSky(screen *golib.Screen) {
	const bands = 36
	height := screen.Height() / bands
	for i := range bands {
		t := float32(i) / (bands - 1)
		color := mixColors(skyTopColor, skyBottomColor, t)
		screen.DrawRectangle(golib.Rectangle{Y: float32(i) * height, Width: screen.Width(), Height: height + 1}, color)
	}
}

// mixColors returns the color a share t of the way from a to b.
func mixColors(a, b golib.Color, t float32) golib.Color {
	mix := func(x, y uint8) uint8 { return uint8(golib.Lerp(float32(x), float32(y), t) + 0.5) }
	return golib.Color{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: mix(a.A, b.A)}
}

// drawWorld draws the world in 3D through camera: the scenery, the platform,
// the gems, the cubes and the marble.
func drawWorld(screen *golib.Screen, w *world, camera *golib.Camera3D) {
	drawSky(screen)
	screen.Begin3D(camera)
	drawScenery(screen)
	drawPlatform(screen)
	for _, g := range w.gems {
		drawGem(screen, g, w.time)
	}
	for _, c := range w.cubes {
		drawCube(screen, c, w.marble.position)
	}
	blinking := w.grace.Running() && math.Mod(float64(w.grace.Left()), blinkInterval) < blinkInterval/3
	if !blinking && !(w.over && w.marble.position.Y < -fallDepth) {
		drawMarble(screen, w.marble)
	}
	screen.End3D()
}

func drawScenery(screen *golib.Screen) {
	for _, s := range sky {
		if !s.island {
			screen.DrawMesh(sphereMesh, golib.Transform{Position: s.position, Scale: s.scale}, cloudColor)
			continue
		}
		// An island: grass on a rock that narrows downwards.
		screen.DrawMesh(boxMesh, golib.Transform{Position: s.position.Add(golib.Vector3{Y: s.scale.Y / 2}), Scale: golib.Vector3{X: s.scale.X, Y: 0.5, Z: s.scale.Z}}, grassColor)
		screen.DrawMesh(boxMesh, golib.Transform{Position: s.position, Scale: s.scale}, rockColor)
		screen.DrawMesh(boxMesh, golib.Transform{Position: s.position.Add(golib.Vector3{Y: -s.scale.Y}), Scale: s.scale.Scale(0.55)}, rockDarkColor)
	}
}

// drawPlatform draws the checkered floor, its base and the rock under it.
func drawPlatform(screen *golib.Screen) {
	tiles := platformSize / tileSize
	for row := range tiles {
		for column := range tiles {
			color := tileColor
			if (row+column)%2 == 1 {
				color = tileOtherColor
			}
			center := golib.Vector3{
				X: -half + tileSize/2 + float32(column*tileSize),
				Y: -tileDepth / 2,
				Z: -half + tileSize/2 + float32(row*tileSize),
			}
			screen.DrawMesh(boxMesh, golib.Transform{Position: center, Scale: golib.Vector3{X: tileSize, Y: tileDepth, Z: tileSize}}, color)
		}
	}
	// The base under the tiles, then rock in tiers, narrower as it goes down.
	screen.DrawMesh(boxMesh, golib.Transform{Position: golib.Vector3{Y: -tileDepth - 0.4}, Scale: golib.Vector3{X: platformSize, Y: 0.8, Z: platformSize}}, platformColor)
	tiers := []struct{ size, height float32 }{{17, 1.6}, {12.5, 1.8}, {8, 2}, {3.5, 2.2}}
	top := float32(-tileDepth - 0.8)
	for i, tier := range tiers {
		color := rockColor
		if i%2 == 1 {
			color = rockDarkColor
		}
		screen.DrawMesh(boxMesh, golib.Transform{Position: golib.Vector3{Y: top - tier.height/2}, Scale: golib.Vector3{X: tier.size, Y: tier.height, Z: tier.size}}, color)
		top -= tier.height
	}
}

// drawShadow draws a dark disc on the platform under something, since GoLib
// has no shadows yet.
func drawShadow(screen *golib.Screen, at golib.Vector3, size float32) {
	if offPlatform(at) {
		return
	}
	screen.DrawMesh(cylinderMesh, golib.Transform{Position: golib.Vector3{X: at.X, Y: 0.01, Z: at.Z}, Scale: golib.Vector3{X: size, Y: 0.01, Z: size}}, shadowColor)
}

// drawGem draws a gem as a diamond that bobs and spins, in a faint glow.
func drawGem(screen *golib.Screen, g gem, time float32) {
	t := time + g.phase
	at := g.position.Add(golib.Vector3{Y: gemBob * float32(math.Sin(float64(t)*3))})
	turn := cornerUp
	turn.Yaw = t * gemSpinSpeed
	size := golib.Vector3{X: gemSize, Y: gemSize, Z: gemSize}
	drawShadow(screen, g.position, 0.6)
	screen.DrawMesh(boxMesh, golib.Transform{Position: at, Rotation: turn, Scale: size}, gemColor)
	screen.DrawMesh(sphereMesh, golib.Transform{Position: at, Scale: size.Scale(2)}, gemGlowColor)
}

// drawCube draws a cube turned to look at the marble, with two eyes.
func drawCube(screen *golib.Screen, c cube, marbleAt golib.Vector3) {
	facing := golib.RotationTowards(flat(marbleAt.Sub(c.position)))
	screen.DrawMesh(boxMesh, golib.Transform{Position: c.position, Rotation: facing, Scale: golib.Vector3{X: cubeSize, Y: cubeSize, Z: cubeSize}}, cubeColor)
	for _, side := range []float32{-1, 1} {
		eye := golib.Vector3{X: side * 0.22, Y: 0.15, Z: cubeSize / 2}
		screen.DrawMesh(boxMesh, golib.Transform{Position: c.position.Add(eye.Rotate(facing)), Rotation: facing, Scale: golib.Vector3{X: 0.24, Y: 0.24, Z: 0.04}}, eyeColor)
		pupil := eye.Add(golib.Vector3{Y: -0.03, Z: 0.03})
		screen.DrawMesh(boxMesh, golib.Transform{Position: c.position.Add(pupil.Rotate(facing)), Rotation: facing, Scale: golib.Vector3{X: 0.11, Y: 0.11, Z: 0.04}}, pupilColor)
	}
}

// drawMarble draws the marble with six dots on it that roll with it: the
// sphere is one color, so without them it wouldn't look like it rolls.
func drawMarble(screen *golib.Screen, m marble) {
	if !m.falling {
		drawShadow(screen, m.position, 0.9)
	}
	screen.DrawMesh(sphereMesh, golib.Transform{Position: m.position, Scale: golib.Vector3{X: 2 * marbleRadius, Y: 2 * marbleRadius, Z: 2 * marbleRadius}}, marbleColor)
	a, b := m.axes[0], m.axes[1]
	c := a.Cross(b)
	for _, axis := range []golib.Vector3{a, b, c, a.Scale(-1), b.Scale(-1), c.Scale(-1)} {
		at := m.position.Add(axis.Scale(marbleRadius - markSize/4))
		screen.DrawMesh(sphereMesh, golib.Transform{Position: at, Scale: golib.Vector3{X: markSize, Y: markSize, Z: markSize}}, marbleMarkColor)
	}
}

// drawText draws text with a dark shadow under it, so it reads over the sky
// and the platform.
func drawText(screen *golib.Screen, text string, x, y, size float32, color golib.Color, options ...golib.TextOptions) {
	offset := size / 10
	screen.DrawText(text, x+offset, y+offset, size, textShadowColor, options...)
	screen.DrawText(text, x, y, size, color, options...)
}

var centered = golib.TextOptions{Align: golib.AlignCenter}

// drawHUD draws the score, the lives and the level in the top-left corner.
func drawHUD(screen *golib.Screen, w *world, best int) {
	panel := golib.Rectangle{X: 16, Y: 16, Width: 250, Height: 128}
	screen.DrawRectangle(panel, golib.Color{R: 10, G: 20, B: 50, A: 110})
	drawText(screen, fmt.Sprintf("SCORE %d", w.score), 32, 28, 40, textColor)
	for i := range startLives {
		x, y := float32(48+i*40), float32(94)
		if i < w.lives {
			screen.DrawCircle(x, y, 14, marbleColor)
			screen.DrawCircle(x-4, y-5, 4, golib.WithOpacity(golib.White, 0.7))
		} else {
			screen.DrawCircleOutline(x, y, 14, 3, golib.WithOpacity(golib.White, 0.5))
		}
	}
	drawText(screen, fmt.Sprintf("LEVEL %d", w.level()+1), 172, 86, 20, accentColor)
	drawText(screen, fmt.Sprintf("BEST %d", max(best, w.score)), 172, 112, 20, textColor)
}

// drawMessage darkens the screen and writes a title and lines under it, in the middle.
func drawMessage(screen *golib.Screen, title string, lines ...string) {
	screen.DrawRectangle(golib.Rectangle{Width: screen.Width(), Height: screen.Height()}, overlayColor)
	y := screen.Height()/2 - float32(40+30*len(lines))/2 - 20
	drawText(screen, title, screen.Width()/2, y, 60, accentColor, centered)
	for i, line := range lines {
		drawText(screen, line, screen.Width()/2, y+90+float32(i)*36, 20, textColor, centered)
	}
}
