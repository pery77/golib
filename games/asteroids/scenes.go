package main

import (
	"fmt"
	"math"

	"golib"
)

// The game's scenes. Each one is a golib.Game: golib.Run starts with the title,
// and golib.SwitchScene moves between them.
//
//	title --Enter/A--> play --Esc/Start--> pause --Esc/Start--> back to the same play
//	  |                  |                   '------Q/B------> title
//	  |                  '--last ship lost--> game over --Enter/A--> a new play
//	  |                                          '------Esc/B----> title
//	  '--Esc--> quit
//
// In every scene, F11 or Alt+Enter switches fullscreen, F2 or the Y button
// turns the screen effects on and off, and F3 or the X button turns the music
// on and off.

// options are the player's settings, shared by every scene.
type options struct {
	effects   bool
	music     bool
	glow, crt *golib.Shader
}

// newOptions makes the screen effects and turns them, and the music, on.
func newOptions() *options {
	o := &options{music: true, glow: golib.NewShader(glowSource), crt: golib.NewShader(crtSource)}
	o.glow.SetUniform("strength", glowStrength)
	o.crt.SetUniform("curvature", crtCurvature)
	o.setEffects(true)
	theme.SetVolume(musicVolume)
	return o
}

// setEffects turns the glow and CRT post-processing on or off. The glow runs
// first, so the CRT effect bends and scans the glowing picture.
func (o *options) setEffects(on bool) {
	o.effects = on
	if on {
		golib.SetPostProcess(o.glow, o.crt)
	} else {
		golib.SetPostProcess()
	}
}

// handleKeys reads the keys every scene shares: fullscreen, screen effects and
// music.
func (o *options) handleKeys(input *golib.Input) {
	if input.KeyPressed(golib.KeyF11) || (altDown(input) && input.KeyPressed(golib.KeyEnter)) {
		golib.SetFullscreen(!golib.IsFullscreen())
	}
	if input.KeyPressed(golib.KeyF2) || input.GamepadPressed(0, golib.GamepadY) {
		o.setEffects(!o.effects)
	}
	if input.KeyPressed(golib.KeyF3) || input.GamepadPressed(0, golib.GamepadX) {
		o.music = !o.music
	}
	// Both calls are safe in every update: the first Play starts the music,
	// once golib.Run has opened the sound device.
	if o.music {
		theme.Play()
	} else {
		theme.Pause()
	}
}

func altDown(input *golib.Input) bool {
	return input.KeyDown(golib.KeyLeftAlt) || input.KeyDown(golib.KeyRightAlt)
}

// confirmed reports whether the player pressed Enter, A or Start. Alt+Enter
// doesn't count: it switches fullscreen.
func confirmed(input *golib.Input) bool {
	return (input.KeyPressed(golib.KeyEnter) && !altDown(input)) ||
		input.GamepadPressed(0, golib.GamepadA) || input.GamepadPressed(0, golib.GamepadStart)
}

// titleScene shows the game's name and controls over drifting rocks.
type titleScene struct {
	options *options
	world   world // rocks only: the game is over before it starts
}

func newTitleScene(o *options) *titleScene {
	w := newWorld()
	w.ship.alive = false
	w.over = true
	return &titleScene{options: o, world: w}
}

func (s *titleScene) Update(input *golib.Input, dt float32) {
	s.options.handleKeys(input)
	s.world.step(controls{}, dt)
	if confirmed(input) {
		golib.SwitchScene(newPlayScene(s.options))
	}
	// No key quits by itself, not even Esc: the game calls golib.Quit when it
	// wants to end.
	if input.KeyPressed(golib.KeyEscape) {
		golib.Quit()
	}
}

func (s *titleScene) Draw(screen *golib.Screen) {
	drawWorld(screen, &s.world)
	drawCentered(screen, "ASTEROIDS", 140, 96, titleColor)
	drawCentered(screen, "Shoot the rocks. Big ones split in two.", 260, 28, textColor)
	drawCentered(screen, "Turn: arrows, A/D, d-pad or left stick", 340, 24, textColor)
	drawCentered(screen, "Thrust: Up, W or B     Fire: Space or A", 376, 24, textColor)
	drawCentered(screen, "Pause: Esc or Start", 412, 24, textColor)
	drawCentered(screen, "Press Enter or A to start", 510, 36, titleColor)
	settings := "Esc: quit     F11 or Alt+Enter: fullscreen     F2 or Y: effects " + onOff(s.options.effects) +
		"     F3 or X: music " + onOff(s.options.music)
	drawCentered(screen, settings, 640, 20, textColor)
}

// onOff turns a setting into the word the title screen shows.
func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

// playScene is the game itself. Update turns the keyboard and gamepad into
// controls for the world, which holds the rules (world.go).
type playScene struct {
	options *options
	world   world
}

func newPlayScene(o *options) *playScene {
	return &playScene{options: o, world: newWorld()}
}

// Update reads the input and advances the world. golib.Run calls it 60 times
// per second, always with dt = 1/60.
func (s *playScene) Update(input *golib.Input, dt float32) {
	s.options.handleKeys(input)
	if input.KeyPressed(golib.KeyEscape) || input.GamepadPressed(0, golib.GamepadStart) {
		thrustSound.Stop() // the world stops, and so does its rumble
		golib.SwitchScene(&pauseScene{paused: s})
		return
	}
	s.world.step(readControls(input), dt)
	if s.world.over {
		thrustSound.Stop()
		golib.SwitchScene(&gameOverScene{finished: s})
	}
}

// readControls turns the keyboard and gamepad 0 into controls.
func readControls(input *golib.Input) controls {
	var c controls
	if input.KeyDown(golib.KeyLeft) || input.KeyDown(golib.KeyA) || input.GamepadDown(0, golib.GamepadLeft) {
		c.turn--
	}
	if input.KeyDown(golib.KeyRight) || input.KeyDown(golib.KeyD) || input.GamepadDown(0, golib.GamepadRight) {
		c.turn++
	}
	if c.turn == 0 {
		// The stick is analog: tilted halfway, the ship turns at half speed.
		c.turn, _ = input.GamepadLeftStick(0)
	}
	c.thrust = input.KeyDown(golib.KeyUp) || input.KeyDown(golib.KeyW) ||
		input.GamepadDown(0, golib.GamepadB) || input.GamepadDown(0, golib.GamepadUp) || input.GamepadDown(0, golib.GamepadRightTrigger)
	// Holding fire keeps shooting, as fast as fireCooldown allows.
	c.fire = input.KeyDown(golib.KeySpace) || input.GamepadDown(0, golib.GamepadA)
	return c
}

func (s *playScene) Draw(screen *golib.Screen) {
	drawWorld(screen, &s.world)
	drawHUD(screen, &s.world)
}

// pauseScene freezes a play scene under a message. Resuming switches back to
// that same play scene.
type pauseScene struct {
	paused *playScene
}

func (s *pauseScene) Update(input *golib.Input, dt float32) {
	s.paused.options.handleKeys(input)
	if input.KeyPressed(golib.KeyEscape) || input.GamepadPressed(0, golib.GamepadStart) {
		golib.SwitchScene(s.paused)
	}
	if input.KeyPressed(golib.KeyQ) || input.GamepadPressed(0, golib.GamepadB) {
		golib.SwitchScene(newTitleScene(s.paused.options))
	}
}

func (s *pauseScene) Draw(screen *golib.Screen) {
	s.paused.Draw(screen)
	drawMessage(screen, "PAUSED", "Esc or Start to resume, Q or B to quit to the title")
}

// gameOverScene shows the final score while the rocks keep drifting.
type gameOverScene struct {
	finished *playScene
}

func (s *gameOverScene) Update(input *golib.Input, dt float32) {
	s.finished.options.handleKeys(input)
	s.finished.world.step(controls{}, dt)
	if confirmed(input) {
		golib.SwitchScene(newPlayScene(s.finished.options))
	}
	if input.KeyPressed(golib.KeyEscape) || input.GamepadPressed(0, golib.GamepadB) {
		golib.SwitchScene(newTitleScene(s.finished.options))
	}
}

func (s *gameOverScene) Draw(screen *golib.Screen) {
	s.finished.Draw(screen)
	drawMessage(screen, "GAME OVER", fmt.Sprintf("Score %d. Enter or A to play again, Esc or B for the title", s.finished.world.score))
}

// drawWorld draws the world from back to front: sparks, rocks, bullets and the
// ship. It reads the state and never changes it.
func drawWorld(screen *golib.Screen, w *world) {
	screen.Clear(spaceColor)
	// Sparks and bullets add their light to what is behind them, so that
	// overlapping ones glow brighter instead of covering each other.
	screen.SetBlendMode(golib.BlendAdd)
	for _, p := range w.particles {
		fade := p.life / particleLifetime
		screen.DrawCircle(p.x, p.y, 1+2*fade, golib.WithOpacity(sparkColor, fade))
	}
	screen.SetBlendMode(golib.BlendNormal)
	for _, r := range w.rocks {
		drawWrapped(r.x, r.y, r.radius()*1.15, func(x, y float32) { drawRock(screen, r, x, y) })
	}
	screen.SetBlendMode(golib.BlendAdd)
	for _, b := range w.bullets {
		screen.DrawCircle(b.x, b.y, 2.5, bulletColor)
	}
	screen.SetBlendMode(golib.BlendNormal)
	// A new ship blinks while it can't crash.
	blinking := w.ship.invulnerable > 0 && int(w.time*12)%2 == 0
	if w.ship.alive && !blinking {
		drawWrapped(w.ship.x, w.ship.y, shipNose+12, func(x, y float32) { drawShip(screen, w.ship, x, y, w.time) })
	}
}

// drawHUD draws the score, the wave and the ships left.
func drawHUD(screen *golib.Screen, w *world) {
	screen.DrawText(fmt.Sprintf("SCORE %d", w.score), 24, 20, 32, textColor)
	screen.DrawText(fmt.Sprintf("WAVE %d", w.wave), screen.Width()/2, 24, 24, textColor, golib.TextOptions{Align: golib.AlignCenter})
	for i := range w.lives {
		drawShip(screen, ship{}, screen.Width()-40-float32(i)*34, 42, 0)
	}
}

// drawWrapped calls draw at x, y, and again across each edge that a shape
// reaching radius pixels from x, y crosses, so things leaving one side of the
// screen already show on the other.
func drawWrapped(x, y, radius float32, draw func(x, y float32)) {
	for _, dx := range []float32{-worldWidth, 0, worldWidth} {
		for _, dy := range []float32{-worldHeight, 0, worldHeight} {
			cx, cy := x+dx, y+dy
			if cx+radius >= 0 && cx-radius <= worldWidth && cy+radius >= 0 && cy-radius <= worldHeight {
				draw(cx, cy)
			}
		}
	}
}

// drawRock draws a rock's outline centered at x, y.
func drawRock(screen *golib.Screen, r rock, x, y float32) {
	var firstX, firstY, lastX, lastY float32
	for i, share := range r.shape {
		angle := float64(r.angle) + 2*math.Pi*float64(i)/rockCorners
		cornerX := x + r.radius()*share*float32(math.Cos(angle))
		cornerY := y + r.radius()*share*float32(math.Sin(angle))
		if i == 0 {
			firstX, firstY = cornerX, cornerY
		} else {
			screen.DrawLine(lastX, lastY, cornerX, cornerY, lineWidth, rockColor)
		}
		lastX, lastY = cornerX, cornerY
	}
	screen.DrawLine(lastX, lastY, firstX, firstY, lineWidth, rockColor)
}

// shipOutline is the ship pointing up, in pixels from its center.
var shipOutline = [...][2]float32{{0, -shipNose}, {12, 13}, {0, 7}, {-12, 13}}

// drawShip draws a ship centered at x, y, with a flickering flame while it
// thrusts. time animates the flame.
func drawShip(screen *golib.Screen, s ship, x, y, time float32) {
	sin, cos := float32(math.Sin(float64(s.angle))), float32(math.Cos(float64(s.angle)))
	turned := func(px, py float32) (float32, float32) {
		return x + px*cos - py*sin, y + px*sin + py*cos
	}
	if s.thrusting {
		flame := 20 + 6*float32(math.Sin(float64(time)*45))
		ax, ay := turned(-6, 10)
		bx, by := turned(6, 10)
		cx, cy := turned(0, flame)
		screen.DrawTriangle(ax, ay, bx, by, cx, cy, flameColor)
	}
	for i, corner := range shipOutline {
		next := shipOutline[(i+1)%len(shipOutline)]
		x1, y1 := turned(corner[0], corner[1])
		x2, y2 := turned(next[0], next[1])
		screen.DrawLine(x1, y1, x2, y2, lineWidth, shipColor)
	}
}

// drawMessage darkens the whole screen and shows a heading and a hint in the
// middle.
func drawMessage(screen *golib.Screen, heading, hint string) {
	screen.DrawRectangle(golib.Rectangle{Width: screen.Width(), Height: screen.Height()}, overlayColor)
	drawCentered(screen, heading, 270, 72, titleColor)
	drawCentered(screen, hint, 380, 28, textColor)
}

// drawCentered draws text centered across the screen, with its top at y.
func drawCentered(screen *golib.Screen, text string, y, size float32, color golib.Color) {
	screen.DrawText(text, (screen.Width()-screen.TextWidth(text, size))/2, y, size, color)
}
