package main

import "golib"

// controls reads the keyboard and gamepad 0 into the hovercraft's throttle
// and steer, each from -1 to 1: throttle 1 is full ahead, steer 1 is right.
// Keys and the d-pad are full on; the left stick is analog.
func controls(input *golib.Input) (throttle, steer float32) {
	stickX, stickY := input.GamepadLeftStick(0)
	throttle, steer = -stickY, stickX // a stick's Y grows downwards
	if input.KeyDown(golib.KeyUp) || input.KeyDown(golib.KeyW) || input.GamepadDown(0, golib.GamepadUp) ||
		input.GamepadDown(0, golib.GamepadA) || input.GamepadDown(0, golib.GamepadRightTrigger) {
		throttle = 1
	}
	if input.KeyDown(golib.KeyDown) || input.KeyDown(golib.KeyS) || input.GamepadDown(0, golib.GamepadDown) ||
		input.GamepadDown(0, golib.GamepadB) || input.GamepadDown(0, golib.GamepadLeftTrigger) {
		throttle = -1
	}
	if input.KeyDown(golib.KeyLeft) || input.KeyDown(golib.KeyA) || input.GamepadDown(0, golib.GamepadLeft) {
		steer = -1
	}
	if input.KeyDown(golib.KeyRight) || input.KeyDown(golib.KeyD) || input.GamepadDown(0, golib.GamepadRight) {
		steer = 1
	}
	return throttle, steer
}

// toggleFullscreen switches fullscreen on F11 or Alt+Enter, in every scene.
func toggleFullscreen(input *golib.Input) {
	altEnter := altDown(input) && input.KeyPressed(golib.KeyEnter)
	if input.KeyPressed(golib.KeyF11) || altEnter {
		golib.SetFullscreen(!golib.IsFullscreen())
	}
}

func altDown(input *golib.Input) bool {
	return input.KeyDown(golib.KeyLeftAlt) || input.KeyDown(golib.KeyRightAlt)
}

// confirmed says whether the player pressed a key or button that starts or
// accepts.
func confirmed(input *golib.Input) bool {
	return (input.KeyPressed(golib.KeyEnter) && !altDown(input)) || input.KeyPressed(golib.KeySpace) ||
		input.GamepadPressed(0, golib.GamepadA) || input.GamepadPressed(0, golib.GamepadStart)
}

// pausePressed says whether the player asked to pause, or to carry on.
func pausePressed(input *golib.Input) bool {
	return input.KeyPressed(golib.KeyEscape) || input.KeyPressed(golib.KeyP) || input.GamepadPressed(0, golib.GamepadStart)
}
