package main

import "golib"

// Input tuning.
const (
	stickAimDeadZone = 0.35 // how far the right stick must tilt to aim and fire, from 0 to 1
)

// playScene is the game itself. Update turns the keyboard, the mouse and
// gamepad 0 into controls for the world (world.go) and moves the camera; Draw
// draws the world (draw.go) and the HUD (hud.go).
type playScene struct {
	session *session
	world   world
	camera  camera
	aim     aimDevice
	mouseX  float32 // the pointer in the previous update, to notice when it moves
	mouseY  float32
	mouseOn bool // mouseX and mouseY hold a real position
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
	p.camera = newCamera(&p.world)
	return p
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
	s.camera.follow(&s.world, dt)

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
		c.moveX--
	}
	if input.KeyDown(golib.KeyD) || input.GamepadDown(0, golib.GamepadRight) {
		c.moveX++
	}
	if input.KeyDown(golib.KeyW) || input.GamepadDown(0, golib.GamepadUp) {
		c.moveY--
	}
	if input.KeyDown(golib.KeyS) || input.GamepadDown(0, golib.GamepadDown) {
		c.moveY++
	}
	if c.moveX == 0 && c.moveY == 0 {
		c.moveX, c.moveY = input.GamepadLeftStick(0)
	}

	// Aiming: whichever device the player used last.
	var arrowX, arrowY float32
	if input.KeyDown(golib.KeyLeft) {
		arrowX--
	}
	if input.KeyDown(golib.KeyRight) {
		arrowX++
	}
	if input.KeyDown(golib.KeyUp) {
		arrowY--
	}
	if input.KeyDown(golib.KeyDown) {
		arrowY++
	}
	stickX, stickY := input.GamepadRightStick(0)
	stickAiming := length(stickX, stickY) >= stickAimDeadZone

	mouseX, mouseY := input.MousePosition()
	mouseMoved := s.mouseOn && (mouseX != s.mouseX || mouseY != s.mouseY)
	s.mouseX, s.mouseY, s.mouseOn = mouseX, mouseY, true
	mouseFire := input.MouseDown(golib.MouseLeft)

	switch {
	case arrowX != 0 || arrowY != 0:
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
		c.aimX, c.aimY = arrowX, arrowY
	case aimByStick:
		c.aimX, c.aimY = stickX, stickY
	case aimByMouse:
		// The pointer is in screen pixels; the ship is in the arena's.
		x, y := s.camera.toWorld(mouseX, mouseY)
		c.aimX, c.aimY = x-s.world.ship.x, y-s.world.ship.y
	}

	c.fire = input.KeyDown(golib.KeySpace) || mouseFire ||
		arrowX != 0 || arrowY != 0 || stickAiming ||
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
		drawCrosshair(screen, s.mouseX, s.mouseY)
	}
}
