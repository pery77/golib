package golib

import "golib/internal/device"

// MouseButton is a button on the mouse.
type MouseButton int32

// Mouse buttons.
const (
	MouseLeft   = MouseButton(device.MouseLeft)
	MouseRight  = MouseButton(device.MouseRight)
	MouseMiddle = MouseButton(device.MouseMiddle)
)

// mouseButtonCount is one more than the highest MouseButton, so buttons can
// index arrays.
const mouseButtonCount = MouseMiddle + 1

// mouseButtonNames names every MouseButton constant the way golib shot --input
// spells it.
var mouseButtonNames = map[MouseButton]string{
	MouseLeft:   "MouseLeft",
	MouseRight:  "MouseRight",
	MouseMiddle: "MouseMiddle",
}

func validMouseButton(button MouseButton) bool {
	return button >= 0 && button < mouseButtonCount
}
