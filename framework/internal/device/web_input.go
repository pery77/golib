//go:build js

package device

import (
	"encoding/binary"
	"math"
	"syscall/js"
)

// The keyboard, the mouse and the gamepads come over in one buffer of bytes,
// once a frame, instead of a call for each key: package golib asks about
// every key it knows every frame, and a call each would cost more than the
// game. web.js writes the buffer and readInput copies it here.
//
// The buffer starts with the numbers, as little-endian float32, and then the
// keys and buttons, one byte each: 0 for up, 1 for down.
const (
	inMouseX = 0 // in floats from the start
	inMouseY = 1
	inWheel  = 2
	inSticks = 3 // 4 pads of leftX, leftY, rightX, rightY
	inFloats = inSticks + 4*maxPads
	inBytes  = inFloats * 4 // where the bytes start

	padButtons = GamepadRightStickButton + 1 // buttons a pad can report
	maxPads    = 4

	inKeysDown      = inBytes
	inKeysPressed   = inKeysDown + KeyCount
	inMouseDown     = inKeysPressed + KeyCount
	inMousePressed  = inMouseDown + mouseButtons
	inPadsConnected = inMousePressed + mouseButtons
	inPadsDown      = inPadsConnected + maxPads
	inPadsPressed   = inPadsDown + maxPads*padButtons
	inSize          = inPadsPressed + maxPads*padButtons

	mouseButtons = MouseMiddle + 1
)

// input is this frame's keyboard, mouse and gamepads, copied from the page.
var input struct {
	bytes []byte
	array js.Value // the same buffer on the JavaScript side
}

// readInput takes the picture of the input for this frame and clears the
// presses the page had waiting, so each press reaches exactly one frame.
func readInput() {
	if input.bytes == nil {
		input.bytes = make([]byte, inSize)
		input.array = js_().Call("inputBuffer", inSize)
	}
	js_().Call("snapshotInput")
	js.CopyBytesToGo(input.bytes, input.array)
}

// number returns float number i of the input buffer.
func number(i int) float32 {
	if input.bytes == nil {
		return 0
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(input.bytes[i*4:]))
}

// flag returns whether the byte at offset is set, with a guard so that a key
// or button the page doesn't report is simply never down.
func flag(offset int) bool {
	return input.bytes != nil && offset >= 0 && offset < len(input.bytes) && input.bytes[offset] != 0
}

// IsKeyDown reports whether a key is held down now.
func IsKeyDown(key int32) bool {
	return key >= 0 && key < KeyCount && flag(inKeysDown+int(key))
}

// IsKeyPressed reports whether a key went down since the last frame.
func IsKeyPressed(key int32) bool {
	return key >= 0 && key < KeyCount && flag(inKeysPressed+int(key))
}

// IsMouseDown reports whether a mouse button is held down now.
func IsMouseDown(button int32) bool {
	return button >= 0 && button < mouseButtons && flag(inMouseDown+int(button))
}

// IsMousePressed reports whether a mouse button went down since the last
// frame.
func IsMousePressed(button int32) bool {
	return button >= 0 && button < mouseButtons && flag(inMousePressed+int(button))
}

// MousePosition returns where the mouse pointer is, in canvas pixels.
func MousePosition() (x, y float32) {
	return number(inMouseX), number(inMouseY)
}

// MouseWheel returns how far the wheel turned since the last frame, in
// notches.
func MouseWheel() float32 {
	return number(inWheel)
}

// GamepadConnected reports whether gamepad number pad is plugged in. A
// browser only shows a gamepad after the player has pressed a button on it.
func GamepadConnected(pad int) bool {
	return pad >= 0 && pad < maxPads && flag(inPadsConnected+pad)
}

// GamepadName returns what gamepad number pad calls itself.
func GamepadName(pad int) string {
	if !GamepadConnected(pad) {
		return ""
	}
	return js_().Call("gamepadName", pad).String()
}

// IsGamepadDown reports whether a gamepad button is held down now.
func IsGamepadDown(pad int, button int32) bool {
	if !GamepadConnected(pad) || button < 0 || button >= padButtons {
		return false
	}
	return flag(inPadsDown + pad*padButtons + int(button))
}

// IsGamepadPressed reports whether a gamepad button went down since the last
// frame.
func IsGamepadPressed(pad int, button int32) bool {
	if !GamepadConnected(pad) || button < 0 || button >= padButtons {
		return false
	}
	return flag(inPadsPressed + pad*padButtons + int(button))
}

// GamepadSticks returns how far both sticks of a gamepad are tilted, from -1
// to 1 on each axis, as the browser reports them: the dead zone is still in.
func GamepadSticks(pad int) (leftX, leftY, rightX, rightY float32) {
	if !GamepadConnected(pad) {
		return 0, 0, 0, 0
	}
	at := inSticks + pad*4
	return number(at), number(at + 1), number(at + 2), number(at + 3)
}
