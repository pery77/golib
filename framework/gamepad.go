package golib

import (
	"math"

	"golib/internal/device"
)

// GamepadButton is a button on a gamepad. The face buttons are named after an
// Xbox controller: A is the bottom one (Cross on PlayStation), B the right one
// (Circle), X the left one (Square) and Y the top one (Triangle).
type GamepadButton int32

// Gamepad buttons.
const (
	GamepadUp    GamepadButton = device.GamepadUp // the d-pad
	GamepadRight GamepadButton = device.GamepadRight
	GamepadDown  GamepadButton = device.GamepadDown
	GamepadLeft  GamepadButton = device.GamepadLeft

	GamepadY GamepadButton = device.GamepadY
	GamepadB GamepadButton = device.GamepadB
	GamepadA GamepadButton = device.GamepadA
	GamepadX GamepadButton = device.GamepadX

	GamepadLeftBumper   GamepadButton = device.GamepadLeftBumper
	GamepadLeftTrigger  GamepadButton = device.GamepadLeftTrigger // read as a button: pressed or not
	GamepadRightBumper  GamepadButton = device.GamepadRightBumper
	GamepadRightTrigger GamepadButton = device.GamepadRightTrigger

	GamepadBack  GamepadButton = device.GamepadBack  // View, Select or Share
	GamepadStart GamepadButton = device.GamepadStart // Menu, Start or Options

	GamepadLeftStickButton  GamepadButton = device.GamepadLeftStickButton // pressing the left stick in
	GamepadRightStickButton GamepadButton = device.GamepadRightStickButton
)

// maxGamepads is how many gamepads Input follows, numbered from 0.
const maxGamepads = 4

// gamepadButtonCount is one more than the highest GamepadButton, so buttons
// can index arrays.
const gamepadButtonCount = GamepadRightStickButton + 1

// stickDeadZone is how far a stick must move from its center before it counts,
// as a fraction of full tilt. Sticks rarely rest exactly at the center.
const stickDeadZone = 0.2

// gamepadButtonNames names every GamepadButton constant the way golib shot
// --input spells it.
var gamepadButtonNames = map[GamepadButton]string{
	GamepadUp: "GamepadUp", GamepadRight: "GamepadRight", GamepadDown: "GamepadDown", GamepadLeft: "GamepadLeft",
	GamepadY: "GamepadY", GamepadB: "GamepadB", GamepadA: "GamepadA", GamepadX: "GamepadX",
	GamepadLeftBumper: "GamepadLeftBumper", GamepadLeftTrigger: "GamepadLeftTrigger",
	GamepadRightBumper: "GamepadRightBumper", GamepadRightTrigger: "GamepadRightTrigger",
	GamepadBack: "GamepadBack", GamepadStart: "GamepadStart",
	GamepadLeftStickButton: "GamepadLeftStickButton", GamepadRightStickButton: "GamepadRightStickButton",
}

func validGamepadButton(button GamepadButton) bool {
	return button > 0 && button < gamepadButtonCount
}

// gamepadState is one gamepad as an update sees it, or, in inputQueue, as the
// latest frame left it, with presses not yet delivered.
type gamepadState struct {
	connected      bool
	name           string
	down           [gamepadButtonCount]bool
	pressed        [gamepadButtonCount]bool
	leftX, leftY   float32 // dead zone already removed
	rightX, rightY float32
}

// gamepadFrame is what the machine reports about one gamepad in one frame.
type gamepadFrame struct {
	connected      bool
	name           string
	down           [gamepadButtonCount]bool
	pressed        [gamepadButtonCount]bool
	leftX, leftY   float32 // raw, dead zone included
	rightX, rightY float32
}

// deviceGamepadFrame reads gamepad number pad from the machine.
func deviceGamepadFrame(pad int) gamepadFrame {
	var frame gamepadFrame
	if !device.GamepadConnected(pad) {
		return frame
	}
	frame.connected = true
	frame.name = device.GamepadName(pad)
	for button := range gamepadButtonNames {
		frame.down[button] = device.IsGamepadDown(pad, int32(button))
		frame.pressed[button] = device.IsGamepadPressed(pad, int32(button))
	}
	frame.leftX, frame.leftY, frame.rightX, frame.rightY = device.GamepadSticks(pad)
	return frame
}

// applyDeadZone removes the dead zone from a stick position: positions within
// stickDeadZone of the center become 0, 0, and the rest is rescaled so that
// the edge of the dead zone is 0 and full tilt is 1.
func applyDeadZone(x, y float32) (float32, float32) {
	length := float32(math.Hypot(float64(x), float64(y)))
	if length <= stickDeadZone {
		return 0, 0
	}
	scale := min((length-stickDeadZone)/(1-stickDeadZone), 1) / length
	return x * scale, y * scale
}
