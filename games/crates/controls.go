package main

import "golib"

// The controls every scene reads: the keyboard and gamepad 0 together.

// stickTilt is how far the left stick has to tilt before it counts as a
// direction, from 0 to 1.
const stickTilt = 0.5

// directionControls are the keys and gamepad buttons for each direction.
var directionControls = []struct {
	dir    direction
	keys   [2]golib.Key
	button golib.GamepadButton
}{
	{up, [2]golib.Key{golib.KeyUp, golib.KeyW}, golib.GamepadUp},
	{down, [2]golib.Key{golib.KeyDown, golib.KeyS}, golib.GamepadDown},
	{left, [2]golib.Key{golib.KeyLeft, golib.KeyA}, golib.GamepadLeft},
	{right, [2]golib.Key{golib.KeyRight, golib.KeyD}, golib.GamepadRight},
}

// stickDirection returns where the left stick of gamepad 0 points, if it
// tilts far enough.
func stickDirection(input *golib.Input) direction {
	x, y := input.GamepadLeftStick(0)
	return stickToDirection(x, y)
}

// stickToDirection turns a stick's tilt into the direction it leans most
// towards, or none if it tilts less than stickTilt.
func stickToDirection(x, y float32) direction {
	ax, ay := max(x, -x), max(y, -y)
	switch {
	case max(ax, ay) < stickTilt:
		return noDirection
	case ax > ay && x < 0:
		return left
	case ax > ay:
		return right
	case y < 0:
		return up
	default:
		return down
	}
}

// directionReader reads directions from the arrows, WASD, the d-pad and the
// left stick. The keys and buttons say themselves when they are pressed; the
// stick counts as pressed when it starts pointing a new way, so the reader
// remembers where it pointed.
type directionReader struct {
	stick direction // where the stick pointed in the previous update
}

// pressed returns the direction pressed in this update, or none. Call it once
// per update, before held.
func (r *directionReader) pressed(input *golib.Input) direction {
	pressed := noDirection
	stick := stickDirection(input)
	if stick != r.stick {
		pressed = stick
	}
	r.stick = stick
	for _, c := range directionControls {
		if input.KeyPressed(c.keys[0]) || input.KeyPressed(c.keys[1]) || input.GamepadPressed(0, c.button) {
			pressed = c.dir
		}
	}
	return pressed
}

// held reports whether direction d is held down: its keys, its d-pad button
// or the stick.
func (r *directionReader) held(input *golib.Input, d direction) bool {
	if d != noDirection && r.stick == d {
		return true
	}
	for _, c := range directionControls {
		if c.dir == d {
			return input.KeyDown(c.keys[0]) || input.KeyDown(c.keys[1]) || input.GamepadDown(0, c.button)
		}
	}
	return false
}

// pointer notices when the mouse pointer moves, so a menu lets a resting
// pointer be while the keys move the selection. A menu that has just opened
// hasn't seen the pointer yet, so a pointer resting over one of its buttons
// doesn't pick it.
type pointer struct {
	x, y  float32
	known bool
}

// read returns where the pointer is, in screen pixels, and whether it moved
// since the previous update.
func (p *pointer) read(input *golib.Input) (x, y float32, moved bool) {
	x, y = input.MousePosition()
	moved = p.known && (x != p.x || y != p.y)
	p.x, p.y, p.known = x, y, true
	return x, y, moved
}

// confirmPressed reports whether Enter, Space or the A button was pressed.
// Alt+Enter doesn't count: it switches fullscreen.
func confirmPressed(input *golib.Input) bool {
	return (input.KeyPressed(golib.KeyEnter) && !altDown(input)) || input.KeyPressed(golib.KeySpace) ||
		input.GamepadPressed(0, golib.GamepadA)
}

// backPressed reports whether Esc or the B button was pressed.
func backPressed(input *golib.Input) bool {
	return input.KeyPressed(golib.KeyEscape) || input.GamepadPressed(0, golib.GamepadB)
}

// pausePressed reports whether Esc or Start was pressed.
func pausePressed(input *golib.Input) bool {
	return input.KeyPressed(golib.KeyEscape) || input.GamepadPressed(0, golib.GamepadStart)
}

// undoPressed reports whether Z, U, Backspace or the B button was pressed.
func undoPressed(input *golib.Input) bool {
	return input.KeyPressed(golib.KeyZ) || input.KeyPressed(golib.KeyU) || input.KeyPressed(golib.KeyBackspace) ||
		input.GamepadPressed(0, golib.GamepadB)
}

// undoHeld reports whether Z, U, Backspace or the B button is held down.
func undoHeld(input *golib.Input) bool {
	return input.KeyDown(golib.KeyZ) || input.KeyDown(golib.KeyU) || input.KeyDown(golib.KeyBackspace) ||
		input.GamepadDown(0, golib.GamepadB)
}

// restartPressed reports whether R or the Y button was pressed.
func restartPressed(input *golib.Input) bool {
	return input.KeyPressed(golib.KeyR) || input.GamepadPressed(0, golib.GamepadY)
}
