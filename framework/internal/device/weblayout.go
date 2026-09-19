// This file has no build tag, unlike the rest of the web backend: the input
// buffer's layout is read by the web backend, which only builds for a browser,
// and by the checks in webtouch_test.go, which run in a browser started from
// the desktop. web.js holds the same list on the JavaScript side, so change one
// and change the other; the checks fail when the two disagree.

package device

// The keyboard, the mouse, the touch screen and the gamepads come over in one
// buffer of bytes, once a frame, instead of a call for each key: package golib
// asks about every key it knows every frame, and a call each would cost more
// than the game. web.js writes the buffer and readInput copies it here.
//
// The buffer starts with the numbers, as little-endian float32, and then the
// keys and buttons, one byte each: 0 for up, 1 for down.
const (
	inMouseX     = 0 // in floats from the start
	inMouseY     = 1
	inWheel      = 2
	inSticks     = 3 // 4 pads of leftX, leftY, rightX, rightY
	inTouchCount = inSticks + 4*maxPads
	inTouches    = inTouchCount + 1 // MaxTouches fingers of id, x, y
	inFloats     = inTouches + 3*MaxTouches
	inBytes      = inFloats * 4 // where the bytes start

	padButtons = GamepadRightStickButton + 1 // buttons a pad can report
	maxPads    = 4

	inKeysDown      = inBytes
	inKeysPressed   = inKeysDown + KeyCount
	inMouseDown     = inKeysPressed + KeyCount
	inMousePressed  = inMouseDown + mouseButtons
	inPadsConnected = inMousePressed + mouseButtons
	inPadsDown      = inPadsConnected + maxPads
	inPadsPressed   = inPadsDown + maxPads*padButtons
	inTouchesNew    = inPadsPressed + maxPads*padButtons
	inSize          = inTouchesNew + MaxTouches

	mouseButtons = MouseMiddle + 1
)
