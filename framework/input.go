package golib

import (
	"slices"

	"golib/internal/device"
)

// Input is the keyboard, the mouse, the touch screen and the gamepads as one
// update sees them. Run passes it to Game.Update; use it only there.
type Input struct {
	down    [keyCount]bool
	pressed [keyCount]bool

	mouseX, mouseY float32
	mouseMoved     bool
	mouseDown      [mouseButtonCount]bool
	mousePressed   [mouseButtonCount]bool
	mouseWheel     float32

	touches    [maxTouches]Touch
	touchCount int

	gamepads [maxGamepads]gamepadState
}

// KeyDown reports whether key is held down. Use it for things that last, like
// walking.
func (in *Input) KeyDown(key Key) bool {
	return validKey(key) && in.down[key]
}

// KeyPressed reports whether key went down since the previous update. Each
// press is reported to exactly one update, so use it for one-off actions, like
// jumping or confirming a menu.
func (in *Input) KeyPressed(key Key) bool {
	return validKey(key) && in.pressed[key]
}

// MousePosition returns where the mouse pointer is, in the pixel coordinates
// Screen draws with: from the top-left corner of the screen, with Y growing
// downwards, however Run scales the screen to the window. Over the black bars
// around the screen, the position is outside it. When the pointer leaves the
// window, it keeps its last position inside it or gets values outside the
// screen.
func (in *Input) MousePosition() (x, y float32) {
	return in.mouseX, in.mouseY
}

// MouseMoved reports whether the mouse pointer moved since the previous update,
// in any scene. It is false in the first update. Let the pointer pick a menu
// item only when it moves or clicks, so that a pointer resting over a button
// doesn't fight the arrow keys, or pick a button as a menu opens under it:
//
//	x, y := input.MousePosition()
//	if pointed := buttonAt(x, y); pointed >= 0 && (input.MouseMoved() || input.MousePressed(golib.MouseLeft)) {
//		s.selected = pointed
//	}
func (in *Input) MouseMoved() bool {
	return in.mouseMoved
}

// MouseDown reports whether button is held down. Use it for things that last,
// like dragging.
func (in *Input) MouseDown(button MouseButton) bool {
	return validMouseButton(button) && in.mouseDown[button]
}

// MousePressed reports whether button went down since the previous update.
// Each click is reported to exactly one update, so use it for one-off actions,
// like clicking an on-screen button:
//
//	x, y := input.MousePosition()
//	if input.MousePressed(golib.MouseLeft) && playButton.Contains(x, y) {
//		golib.SwitchScene(newPlayScene())
//	}
func (in *Input) MousePressed(button MouseButton) bool {
	return validMouseButton(button) && in.mousePressed[button]
}

// MouseWheel returns how far the mouse wheel turned since the previous update,
// in notches: positive when turned up, away from the player, and negative when
// turned down. Each turn is reported to exactly one update, so 0 means the
// wheel didn't move:
//
//	if input.MouseWheel() > 0 {
//		zoom *= 1.1
//	}
func (in *Input) MouseWheel() float32 {
	return in.mouseWheel
}

// GamepadConnected reports whether gamepad number pad is connected. Pads are
// numbered from 0 to 3, in the order they were connected; a one-player game
// reads pad 0.
func (in *Input) GamepadConnected(pad int) bool {
	gamepad := in.gamepad(pad)
	return gamepad != nil && gamepad.connected
}

// GamepadName returns the name the system gives gamepad number pad, such as
// "Xbox Controller", or "" when it isn't connected.
func (in *Input) GamepadName(pad int) string {
	gamepad := in.gamepad(pad)
	if gamepad == nil {
		return ""
	}
	return gamepad.name
}

// GamepadDown reports whether button is held down on gamepad number pad. It is
// false when that gamepad isn't connected, so games can read the gamepad
// alongside the keyboard without checking first:
//
//	if input.KeyDown(golib.KeyRight) || input.GamepadDown(0, golib.GamepadRight) {
//		move++
//	}
func (in *Input) GamepadDown(pad int, button GamepadButton) bool {
	gamepad := in.gamepad(pad)
	return gamepad != nil && validGamepadButton(button) && gamepad.down[button]
}

// GamepadPressed reports whether button went down on gamepad number pad since
// the previous update. Each press is reported to exactly one update.
func (in *Input) GamepadPressed(pad int, button GamepadButton) bool {
	gamepad := in.gamepad(pad)
	return gamepad != nil && validGamepadButton(button) && gamepad.pressed[button]
}

// GamepadLeftStick returns the position of the left stick of gamepad number
// pad: x from -1 (left) to 1 (right) and y from -1 (up) to 1 (down), like
// screen coordinates. It is 0, 0 at rest, inside a small dead zone around the
// center, and when the gamepad isn't connected. Use the values directly for
// analog movement:
//
//	x, _ := input.GamepadLeftStick(0)
//	velocityX := x * walkSpeed // a half-tilted stick walks at half speed
func (in *Input) GamepadLeftStick(pad int) (x, y float32) {
	gamepad := in.gamepad(pad)
	if gamepad == nil {
		return 0, 0
	}
	return gamepad.leftX, gamepad.leftY
}

// GamepadRightStick returns the position of the right stick of gamepad number
// pad, like GamepadLeftStick.
func (in *Input) GamepadRightStick(pad int) (x, y float32) {
	gamepad := in.gamepad(pad)
	if gamepad == nil {
		return 0, 0
	}
	return gamepad.rightX, gamepad.rightY
}

// gamepad returns gamepad number pad, or nil when pad is out of range.
func (in *Input) gamepad(pad int) *gamepadState {
	if pad < 0 || pad >= maxGamepads {
		return nil
	}
	return &in.gamepads[pad]
}

func validKey(key Key) bool {
	return key > 0 && key < keyCount
}

// inputQueue turns the machine's keyboard, mouse and gamepad state, which changes
// once per frame, into one Input per update. A frame can run zero, one or
// several updates (see clock), so a press, click or wheel turn waits for the
// next update and is delivered to exactly one.
type inputQueue struct {
	down    [keyCount]bool
	pending [keyCount]bool // pressed, not yet delivered to an update

	mouseX, mouseY float32
	mouseDown      [mouseButtonCount]bool
	mousePending   [mouseButtonCount]bool // clicked, not yet delivered to an update
	deliveredX     float32                // the pointer, as the previous update saw it
	deliveredY     float32
	deliveredAny   bool    // an update has seen the pointer
	mouseWheel     float32 // turned, not yet delivered to an update

	touches     [maxTouches]Touch // the fingers on the screen now
	touchCount  int
	landed      [maxTouches]Touch // fingers that landed, not yet delivered to an update
	landedCount int

	// What the player used in the frame just read, for PlayingWithTouch.
	keyboardUsed bool
	gamepadUsed  bool

	gamepads [maxGamepads]gamepadState // pressed holds presses not yet delivered to an update
}

// readKeyboard records the keyboard for the current frame. isDown and
// wasPressed read the machine's keys; tests pass their own.
func (q *inputQueue) readKeyboard(isDown, wasPressed func(Key) bool) {
	q.keyboardUsed = false
	for _, key := range polledKeys {
		q.down[key] = isDown(key)
		if q.down[key] || wasPressed(key) {
			q.keyboardUsed = true
		}
		if wasPressed(key) {
			q.pending[key] = true
		}
	}
}

// readMouse records the mouse for the current frame: the pointer at x, y, the
// wheel turned by wheel notches, and its buttons. isDown and wasPressed read
// the machine's buttons; tests pass their own.
func (q *inputQueue) readMouse(x, y, wheel float32, isDown, wasPressed func(MouseButton) bool) {
	q.mouseX, q.mouseY = x, y
	q.mouseWheel += wheel
	for button := MouseButton(0); button < mouseButtonCount; button++ {
		q.mouseDown[button] = isDown(button)
		if wasPressed(button) {
			q.mousePending[button] = true
		}
	}
}

// readTouches records the fingers on the screen for the current frame, in the
// coordinates the game draws with. A finger that landed waits here until an
// update has seen it, even if it has lifted by then, so that a tap between two
// updates reaches the game as a mouse click does.
func (q *inputQueue) readTouches(touches []Touch) {
	q.touchCount = 0
	for _, touch := range touches {
		if q.touchCount >= maxTouches {
			break
		}
		q.touches[q.touchCount] = touch
		q.touchCount++
		if touch.Pressed {
			q.rememberLanded(touch)
		}
	}
}

// rememberLanded keeps a finger that landed until an update has seen it.
func (q *inputQueue) rememberLanded(touch Touch) {
	for i := range q.landedCount {
		if q.landed[i].ID == touch.ID {
			q.landed[i] = touch
			return
		}
	}
	if q.landedCount < maxTouches {
		q.landed[q.landedCount] = touch
		q.landedCount++
	}
}

// onScreen reports whether the finger with this id is on the screen now.
func (q *inputQueue) onScreen(id int) bool {
	for i := range q.touchCount {
		if q.touches[i].ID == id {
			return true
		}
	}
	return false
}

// landedNow reports whether the finger with this id landed since the previous
// update.
func (q *inputQueue) landedNow(id int) bool {
	for i := range q.landedCount {
		if q.landed[i].ID == id {
			return true
		}
	}
	return false
}

// readGamepads records every gamepad for the current frame. frame is
// deviceGamepadFrame; tests pass their own.
func (q *inputQueue) readGamepads(frame func(pad int) gamepadFrame) {
	q.gamepadUsed = false
	for pad := range q.gamepads {
		read := frame(pad)
		gamepad := &q.gamepads[pad]
		gamepad.connected = read.connected
		gamepad.name = read.name
		gamepad.down = read.down
		for button, pressed := range read.pressed {
			if pressed {
				gamepad.pressed[button] = true
			}
		}
		gamepad.leftX, gamepad.leftY = applyDeadZone(read.leftX, read.leftY)
		gamepad.rightX, gamepad.rightY = applyDeadZone(read.rightX, read.rightY)
		if !gamepad.connected {
			continue
		}
		if slices.Contains(gamepad.down[:], true) || gamepad.leftX != 0 || gamepad.leftY != 0 || gamepad.rightX != 0 || gamepad.rightY != 0 {
			q.gamepadUsed = true
		}
	}
}

// next fills input for the next update and marks the pending presses, clicks
// and wheel turns as delivered.
func (q *inputQueue) next(input *Input) {
	input.down = q.down
	input.pressed = q.pending
	input.mouseX, input.mouseY = q.mouseX, q.mouseY
	input.mouseMoved = q.deliveredAny && (q.mouseX != q.deliveredX || q.mouseY != q.deliveredY)
	q.deliveredX, q.deliveredY, q.deliveredAny = q.mouseX, q.mouseY, true
	input.mouseDown = q.mouseDown
	input.mousePressed = q.mousePending
	input.mouseWheel = q.mouseWheel
	input.touchCount = 0
	for i := range q.touchCount {
		touch := q.touches[i]
		touch.Pressed = q.landedNow(touch.ID)
		input.touches[input.touchCount] = touch
		input.touchCount++
	}
	// A finger that landed and lifted before this update still reaches it,
	// where it last was, as a mouse click does.
	for i := range q.landedCount {
		if q.onScreen(q.landed[i].ID) || input.touchCount >= maxTouches {
			continue
		}
		touch := q.landed[i]
		touch.Pressed = true
		input.touches[input.touchCount] = touch
		input.touchCount++
	}
	input.gamepads = q.gamepads
	q.pending = [keyCount]bool{}
	q.mousePending = [mouseButtonCount]bool{}
	q.mouseWheel = 0
	q.landedCount = 0
	for pad := range q.gamepads {
		q.gamepads[pad].pressed = [gamepadButtonCount]bool{}
	}
}

func deviceKeyDown(key Key) bool {
	return device.IsKeyDown(int32(key))
}

func deviceKeyPressed(key Key) bool {
	return device.IsKeyPressed(int32(key))
}

func deviceMouseDown(button MouseButton) bool {
	return device.IsMouseDown(int32(button))
}

func deviceMousePressed(button MouseButton) bool {
	return device.IsMousePressed(int32(button))
}

// mouseWithTouch returns the mouse buttons with the fingers counted in: the
// oldest finger on the screen holds MouseLeft down, and a finger landing is a
// click of it, so a menu written for a mouse is worked by tapping it. The
// pointer follows that finger in runWindow, which knows where it is.
func mouseWithTouch(touches []Touch, isDown, wasPressed func(MouseButton) bool) (down, pressed func(MouseButton) bool) {
	down = func(button MouseButton) bool {
		return isDown(button) || (button == MouseLeft && len(touches) > 0)
	}
	pressed = func(button MouseButton) bool {
		if wasPressed(button) {
			return true
		}
		if button != MouseLeft {
			return false
		}
		for _, touch := range touches {
			if touch.Pressed {
				return true
			}
		}
		return false
	}
	return down, pressed
}
