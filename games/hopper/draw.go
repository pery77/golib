package main

import (
	"fmt"
	"math"

	"golib"
)

// Shapes made in code, made once: the clouds and the shadow under the
// character.
var (
	sphereMesh   = golib.NewSphereMesh()
	cylinderMesh = golib.NewCylinderMesh()
)

// How things look; the rules don't need these.
const (
	coinSpinSpeed = 150  // degrees per second
	coinBob       = 0.08 // meters up and down
	coinScale     = 1.3  // the kit's coin is 0.4 m across: a little larger reads better
	coinHalf      = 0.2 * coinScale
	shadowSize    = 0.75 // meters across, on the ground right under the character
	strideRoll    = 7    // degrees the character rocks from side to side as it runs
	strideRate    = 2.2  // rocks per meter run
	blinkInterval = 0.2  // seconds: the character blinks while it starts again
)

// cloud is a puff of spheres far from the level.
type cloud struct {
	position golib.Vector3
	size     float32
}

// clouds float around the level, below and beside it.
var clouds = []cloud{
	{golib.Vector3{X: -14, Y: 6, Z: 8}, 3}, {golib.Vector3{X: 15, Y: 3, Z: 2}, 2.5},
	{golib.Vector3{X: 16, Y: 9, Z: 26}, 3.5}, {golib.Vector3{X: -16, Y: 1, Z: 22}, 3},
	{golib.Vector3{X: -12, Y: 10, Z: 40}, 2.5}, {golib.Vector3{X: 14, Y: -1, Z: 48}, 3},
	{golib.Vector3{X: 2, Y: 12, Z: 62}, 4}, {golib.Vector3{X: -8, Y: -6, Z: 30}, 3.5},
	{golib.Vector3{X: 6, Y: -8, Z: 12}, 3}, {golib.Vector3{X: -4, Y: 2, Z: 58}, 3},
}

// look is how the scene wants the character drawn this frame: the effects
// that are only for the eye, kept by the scene, not the rules.
type look struct {
	time    float32 // seconds, for the coins' spin
	stride  float32 // meters run, for the rocking
	squash  float32 // from 0 to 1: how squashed it is after a landing
	blink   bool    // hidden this frame, while it starts again
	hideAll bool    // the character is out of the picture
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

// drawWorld draws the world in 3D through camera: the sky, the level, the
// coins, the checkpoints and the character.
func drawWorld(screen *golib.Screen, w *world, camera *golib.Camera3D, l look) {
	drawSky(screen)
	screen.Begin3D(camera)
	for _, c := range clouds {
		drawCloud(screen, c)
	}
	screen.DrawModel(levelModel, golib.Transform{}, golib.White)
	for i, at := range w.level.checkpoints {
		tint := checkpointOff
		if w.reached[i] {
			tint = golib.White
		}
		screen.DrawModel(flagModel, golib.Transform{Position: at}, tint)
	}
	for i, at := range w.level.coins {
		if !w.taken[i] {
			drawCoin(screen, at, l.time+float32(i)*0.37)
		}
	}
	if !l.hideAll {
		drawShadow(screen, w)
		if !l.blink {
			drawCharacter(screen, w.player, l)
		}
	}
	screen.End3D()
}

func drawCloud(screen *golib.Screen, c cloud) {
	puffs := []golib.Vector3{{}, {X: c.size * 0.6, Y: -0.2 * c.size, Z: 0.1}, {X: -c.size * 0.6, Y: -0.25 * c.size}}
	for i, p := range puffs {
		size := c.size
		if i > 0 {
			size *= 0.7
		}
		screen.DrawMesh(sphereMesh, golib.Transform{Position: c.position.Add(p), Scale: golib.Vector3{X: size * 1.4, Y: size, Z: size}}, cloudColor)
	}
}

// drawCoin draws the kit's coin, spinning and bobbing around its middle.
func drawCoin(screen *golib.Screen, at golib.Vector3, t float32) {
	bob := coinBob * float32(math.Sin(float64(t)*3))
	place := golib.Transform{
		Position: at.Add(golib.Vector3{Y: bob - coinHalf}),
		Rotation: golib.Rotation{Yaw: t * coinSpinSpeed},
		Scale:    golib.Vector3{X: coinScale, Y: coinScale, Z: coinScale},
	}
	screen.DrawModel(coinModel, place, golib.White)
}

// drawShadow draws a dark disc on the block under the character, smaller
// the higher it is, since GoLib has no shadows yet: it shows where a jump
// will land.
func drawShadow(screen *golib.Screen, w *world) {
	at := w.player.position
	ground, ok := w.level.groundBelow(at)
	if !ok {
		return
	}
	size := shadowSize * golib.Clamp(1-(at.Y-ground)/6, 0.35, 1)
	screen.DrawMesh(cylinderMesh, golib.Transform{
		Position: golib.Vector3{X: at.X, Y: ground + 0.012, Z: at.Z},
		Scale:    golib.Vector3{X: size, Y: 0.01, Z: size},
	}, shadowColor)
}

// drawCharacter draws the kit's character, turned where it faces. GoLib
// doesn't play the kit's animations yet, so it stretches as it jumps,
// squashes as it lands and rocks from side to side as it runs.
func drawCharacter(screen *golib.Screen, p player, l look) {
	stretch := float32(0)
	if !p.onGround {
		stretch = golib.Clamp(p.velocity.Y/60, -0.08, 0.14)
	}
	stretch -= 0.22 * l.squash
	height := 1 + stretch
	width := 1 / float32(math.Sqrt(float64(height)))
	roll := float32(0)
	if p.onGround {
		speed := golib.Vector3{X: p.velocity.X, Z: p.velocity.Z}.Length()
		roll = strideRoll * speed / runSpeed * float32(math.Sin(float64(l.stride)*strideRate*math.Pi))
	}
	screen.DrawModel(characterModel, golib.Transform{
		Position: p.position,
		Rotation: golib.Rotation{Yaw: p.yaw, Roll: roll},
		Scale:    golib.Vector3{X: width, Y: height, Z: width},
	}, golib.White)
}

// drawText draws text with a dark shadow under it, so it reads over the sky
// and the level.
func drawText(screen *golib.Screen, text string, x, y, size float32, color golib.Color, options ...golib.TextOptions) {
	offset := size / 10
	screen.DrawText(text, x+offset, y+offset, size, textShadowColor, options...)
	screen.DrawText(text, x, y, size, color, options...)
}

var centered = golib.TextOptions{Align: golib.AlignCenter}

// formatTime writes seconds as minutes, seconds and tenths: 1:07.3.
func formatTime(seconds float32) string {
	tenths := int(seconds * 10)
	return fmt.Sprintf("%d:%02d.%d", tenths/600, tenths/10%60, tenths%10)
}

// drawHUD draws the coins, the time and the lives in the top-left corner.
func drawHUD(screen *golib.Screen, w *world) {
	screen.DrawRectangle(golib.Rectangle{X: 16, Y: 16, Width: 236, Height: 150}, panelColor)
	// Coins: a coin and the count.
	screen.DrawCircle(46, 48, 16, coinDarkColor)
	screen.DrawCircle(46, 48, 12, coinColor)
	screen.DrawRectangle(golib.Rectangle{X: 44, Y: 40, Width: 4, Height: 16}, coinDarkColor)
	drawText(screen, fmt.Sprintf("%d / %d", w.collected, len(w.level.coins)), 76, 30, 40, textColor)
	// Time: a clock and the time.
	screen.DrawCircle(46, 98, 15, textColor)
	screen.DrawCircle(46, 98, 12, panelColor)
	screen.DrawLine(46, 98, 46, 89, 3, textColor)
	screen.DrawLine(46, 98, 53, 98, 3, textColor)
	drawText(screen, formatTime(w.time), 76, 80, 40, textColor)
	// Lives: hearts, empty for the ones lost.
	for i := range startLives {
		drawHeart(screen, float32(46+i*36), 140, i < w.lives)
	}
}

// drawHeart draws a heart centered on x, y, filled or as an outline.
func drawHeart(screen *golib.Screen, x, y float32, full bool) {
	color := heartColor
	if !full {
		color = golib.WithOpacity(golib.White, 0.3)
	}
	screen.DrawCircle(x-6, y-4, 7.5, color)
	screen.DrawCircle(x+6, y-4, 7.5, color)
	screen.DrawTriangle(x-13.3, y-1.5, x+13.3, y-1.5, x, y+13, color)
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
