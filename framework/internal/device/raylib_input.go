//go:build !js

package device

import rl "github.com/gen2brain/raylib-go/raylib"

// IsKeyDown reports whether a key is held down now.
func IsKeyDown(key int32) bool {
	return rl.IsKeyDown(key)
}

// IsKeyPressed reports whether a key went down since the last frame.
func IsKeyPressed(key int32) bool {
	return rl.IsKeyPressed(key)
}

// IsMouseDown reports whether a mouse button is held down now.
func IsMouseDown(button int32) bool {
	return rl.IsMouseButtonDown(rl.MouseButton(button))
}

// IsMousePressed reports whether a mouse button went down since the last frame.
func IsMousePressed(button int32) bool {
	return rl.IsMouseButtonPressed(rl.MouseButton(button))
}

// MousePosition returns where the mouse pointer is, in window pixels.
func MousePosition() (x, y float32) {
	position := rl.GetMousePosition()
	return position.X, position.Y
}

// MouseWheel returns how far the wheel turned since the last frame.
func MouseWheel() float32 {
	return rl.GetMouseWheelMove()
}

// TouchPoints is always empty on the desktop, and TouchScreen always false.
// raylib reads the desktop through GLFW, which reports no touch, and raylib
// fills its own touch point 0 from the mouse instead: passing that on would
// make every click a finger, and a game that draws its on-screen controls only
// where there is a touch screen would draw them on every computer. A touch
// screen on Windows moves the mouse pointer anyway, so those games are played
// with the mouse there. Games in a browser get the real thing from the web
// backend.
func TouchPoints() []TouchPoint {
	return nil
}

// TouchScreen reports whether a finger is how this machine is pointed at,
// which on the desktop it never is: see TouchPoints above.
func TouchScreen() bool {
	return false
}

// GamepadConnected reports whether gamepad number pad is plugged in.
func GamepadConnected(pad int) bool {
	return rl.IsGamepadAvailable(int32(pad))
}

// GamepadName returns what gamepad number pad calls itself.
func GamepadName(pad int) string {
	return rl.GetGamepadName(int32(pad))
}

// IsGamepadDown reports whether a gamepad button is held down now.
func IsGamepadDown(pad int, button int32) bool {
	return rl.IsGamepadButtonDown(int32(pad), button)
}

// IsGamepadPressed reports whether a gamepad button went down since the
// last frame.
func IsGamepadPressed(pad int, button int32) bool {
	return rl.IsGamepadButtonPressed(int32(pad), button)
}

// GamepadSticks returns how far both sticks of a gamepad are tilted, from -1
// to 1 on each axis, as the machine reports them: the dead zone is still in.
func GamepadSticks(pad int) (leftX, leftY, rightX, rightY float32) {
	id := int32(pad)
	return rl.GetGamepadAxisMovement(id, rl.GamepadAxisLeftX),
		rl.GetGamepadAxisMovement(id, rl.GamepadAxisLeftY),
		rl.GetGamepadAxisMovement(id, rl.GamepadAxisRightX),
		rl.GetGamepadAxisMovement(id, rl.GamepadAxisRightY)
}
