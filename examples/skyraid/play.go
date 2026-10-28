package main

import "golib"

// Input tuning.
const (
	stickAimDeadZone = 0.35 // how far the right stick must tilt to aim and fire, from 0 to 1
)

// Camera tuning. The camera is golib.Camera; the game only says where to look
// and how hard to shake.
const (
	cameraLead = 110  // pixels the camera looks ahead of the ship, where it aims
	cameraLag  = 0.14 // seconds the view takes to close two thirds of the way to that point
	shakeMax   = 14   // pixels the view moves at the strongest shake
)

// playScene is the game itself. Update turns the keyboard, the mouse and
// gamepad 0 into controls for the world (world.go) and moves the camera; Draw
// draws the world (draw.go) and the HUD (hud.go).
type playScene struct {
	session *session
	world   world
	camera  *golib.Camera
	aim     aimDevice
	mouse   golib.Vector2 // the pointer in the previous update, to notice when it moves
	mouseOn bool          // mouse holds a real position
}

// aimDevice is what the player last aimed with. The crosshair only shows for
// the mouse.
type aimDevice int

const (
	aimByMoving aimDevice = iota // no aim input: the ship aims where it flies
	aimByMouse
	aimByArrows
	aimByStick
)

func newPlayScene(s *session) *playScene {
	p := &playScene{session: s, world: newWorld()}
	p.camera = newArenaCamera(&p.world)
	return p
}

// newArenaCamera returns a camera on the arena, looking where the ship aims.
func newArenaCamera(w *world) *golib.Camera {
	c := golib.NewCamera(screenWidth, screenHeight)
	c.Bounds = arena
	c.Lag = cameraLag
	c.Target = cameraTarget(w)
	c.Snap() // ready to draw before the first update
	return c
}

// cameraTarget returns the point to keep in the middle of the screen: the
// ship, a little ahead of where it aims.
func cameraTarget(w *world) golib.Vector2 {
	return w.ship.position.Add(golib.Vector2FromAngle(w.ship.angle).Scale(cameraLead))
}

// followCamera points the camera ahead of the ship and shakes it as much as
// the world's trauma asks. Call it from Update, after the world's step: the
// shake uses random numbers.
func (s *playScene) followCamera(dt float32) {
	s.camera.Target = cameraTarget(&s.world)
	// Trauma squared keeps small hits gentle. Camera.Shake fades a shake out
	// evenly and keeps the strongest one, so asking again every update, each
	// time for the seconds trauma still has at its current rate, makes the
	// view follow the trauma-squared curve.
	s.camera.Shake(s.world.trauma*s.world.trauma*shakeMax, s.world.trauma/(2*traumaDecay))
	s.camera.Update(dt)
}

// applyEffects tells the screen effects (effects.go) how the game is going:
// how badly the picture is still breaking up from the last hit, and how close
// the ship is to being destroyed. The pause and game over scenes call it too,
// so the effects follow the same world.
func (s *playScene) applyEffects() {
	s.session.effects.set(s.world.glitch, s.world.danger())
}

// Update reads the input, advances the world and moves the camera. golib.Run
// calls it 60 times per second, always with dt = 1/60.
func (s *playScene) Update(input *golib.Input, dt float32) {
	s.session.handleKeys(input)
	if !s.world.over && pausePressed(input) {
		golib.SwitchScene(&pauseScene{paused: s})
		return
	}
	golib.SetMouseVisible(s.aim != aimByMouse || s.world.over)

	s.world.step(s.readControls(input), dt)
	s.followCamera(dt)
	s.applyEffects()

	if s.world.over && s.world.overTime >= overDelay {
		s.session.record(&s.world)
		golib.SwitchScene(&gameOverScene{finished: s})
	}
}

// readControls turns the keyboard, the mouse and gamepad 0 into controls.
func (s *playScene) readControls(input *golib.Input) controls {
	var c controls

	// Flying: WASD or the d-pad, else the left stick, which is analog.
	if input.KeyDown(golib.KeyA) || input.GamepadDown(0, golib.GamepadLeft) {
		c.move.X--
	}
	if input.KeyDown(golib.KeyD) || input.GamepadDown(0, golib.GamepadRight) {
		c.move.X++
	}
	if input.KeyDown(golib.KeyW) || input.GamepadDown(0, golib.GamepadUp) {
		c.move.Y--
	}
	if input.KeyDown(golib.KeyS) || input.GamepadDown(0, golib.GamepadDown) {
		c.move.Y++
	}
	if c.move == (golib.Vector2{}) {
		stickX, stickY := input.GamepadLeftStick(0)
		c.move = golib.Vector2{X: stickX, Y: stickY}
	}

	// Aiming: whichever device the player used last.
	var arrows golib.Vector2
	if input.KeyDown(golib.KeyLeft) {
		arrows.X--
	}
	if input.KeyDown(golib.KeyRight) {
		arrows.X++
	}
	if input.KeyDown(golib.KeyUp) {
		arrows.Y--
	}
	if input.KeyDown(golib.KeyDown) {
		arrows.Y++
	}
	stickX, stickY := input.GamepadRightStick(0)
	stick := golib.Vector2{X: stickX, Y: stickY}
	stickAiming := stick.Length() >= stickAimDeadZone

	mouseX, mouseY := input.MousePosition()
	mouse := golib.Vector2{X: mouseX, Y: mouseY}
	mouseMoved := s.mouseOn && mouse != s.mouse
	s.mouse, s.mouseOn = mouse, true
	mouseFire := input.MouseDown(golib.MouseLeft)

	switch {
	case arrows != (golib.Vector2{}):
		s.aim = aimByArrows
	case stickAiming:
		s.aim = aimByStick
	case mouseMoved || mouseFire:
		s.aim = aimByMouse
	case s.aim != aimByMouse:
		s.aim = aimByMoving
	}
	switch s.aim {
	case aimByArrows:
		c.aim = arrows
	case aimByStick:
		c.aim = stick
	case aimByMouse:
		// The pointer is in screen pixels; the ship is in the arena's.
		c.aim = s.camera.ToWorld(input.MousePosition()).Sub(s.world.ship.position)
	}

	c.fire = input.KeyDown(golib.KeySpace) || mouseFire ||
		arrows != (golib.Vector2{}) || stickAiming ||
		input.GamepadDown(0, golib.GamepadA) || input.GamepadDown(0, golib.GamepadRightTrigger)
	c.dash = input.KeyPressed(golib.KeyLeftShift) || input.KeyPressed(golib.KeyRightShift) ||
		input.MousePressed(golib.MouseRight) ||
		input.GamepadPressed(0, golib.GamepadB) || input.GamepadPressed(0, golib.GamepadLeftBumper)
	return c
}

// pausePressed reports whether the player asked to pause, or to resume.
func pausePressed(input *golib.Input) bool {
	return input.KeyPressed(golib.KeyEscape) || input.KeyPressed(golib.KeyP) ||
		input.GamepadPressed(0, golib.GamepadStart)
}

// Draw draws the world and the HUD over it. It reads the state and never
// changes it.
func (s *playScene) Draw(screen *golib.Screen) {
	drawWorld(screen, &s.world, s.session.backdrop, s.camera)
	drawHUD(screen, &s.world, s.camera)
	if s.aim == aimByMouse && !s.world.over {
		drawCrosshair(screen, s.mouse)
	}
	drawNotes(screen, s.session.notes()...)
}
