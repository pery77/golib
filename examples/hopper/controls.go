package main

import "golib"

// The camera behind the character.
const (
	cameraDistance  = 6.5  // meters from the point it looks at
	cameraPitch     = -20  // degrees: negative looks down at it
	cameraLookUp    = 1.0  // meters above the character's feet the camera looks at
	cameraAhead     = 1.5  // meters ahead of the character the camera looks at
	cameraLag       = 0.15 // seconds of smoothing
	cameraFollow    = 1.4  // how fast the camera swings round behind the character, per second
	cameraFollowMin = 1.5  // meters per second the character must run before the camera swings
	cameraFollowArc = 70   // degrees: the camera swings only when the character runs at most this far from where it looks
	cameraTurnSpeed = 110  // degrees per second, with Q and E or the right stick
)

// runDirection turns the player's intentions into a direction on the ground,
// from where the camera looks: forward is up, away from the camera, and right
// is the screen's right. forward and right go from -1 to 1. cameraYaw is the
// camera's yaw, as golib.Camera3D.Orbit takes it.
func runDirection(cameraYaw, forward, right float32) golib.Vector3 {
	facing := golib.Rotation{Yaw: cameraYaw}
	return facing.Forward().Scale(forward).Add(facing.Right().Scale(right)).ClampLength(1)
}

// followYaw swings the camera's yaw towards the way the character runs, so
// the camera ends up behind it. It leaves the camera alone while the
// character is slow, and while it runs sideways or back towards the camera,
// so that running back doesn't spin the view round.
func followYaw(cameraYaw float32, velocity golib.Vector3, dt float32) float32 {
	ground := golib.Vector3{X: velocity.X, Z: velocity.Z}
	if ground.Length() < cameraFollowMin {
		return cameraYaw
	}
	turn := angleBetween(cameraYaw, golib.RotationTowards(ground).Yaw)
	if turn > cameraFollowArc || turn < -cameraFollowArc {
		return cameraYaw
	}
	return cameraYaw + turn*min(1, cameraFollow*dt)
}

// readIntent turns the keyboard and gamepad 0 into what the player wants:
// arrows, WASD, the d-pad or the left stick to run, Space or A to jump.
func readIntent(input *golib.Input, cameraYaw float32) intent {
	// Keys and the d-pad run at full speed; the stick is analog, so tilted
	// halfway it runs half as fast. The stick's y grows downwards.
	right, down := input.GamepadLeftStick(0)
	forward := -down
	if input.KeyDown(golib.KeyLeft) || input.KeyDown(golib.KeyA) || input.GamepadDown(0, golib.GamepadLeft) {
		right = -1
	}
	if input.KeyDown(golib.KeyRight) || input.KeyDown(golib.KeyD) || input.GamepadDown(0, golib.GamepadRight) {
		right = 1
	}
	if input.KeyDown(golib.KeyUp) || input.KeyDown(golib.KeyW) || input.GamepadDown(0, golib.GamepadUp) {
		forward = 1
	}
	if input.KeyDown(golib.KeyDown) || input.KeyDown(golib.KeyS) || input.GamepadDown(0, golib.GamepadDown) {
		forward = -1
	}
	return intent{
		move:     runDirection(cameraYaw, forward, right),
		jump:     input.KeyPressed(golib.KeySpace) || input.GamepadPressed(0, golib.GamepadA),
		jumpHeld: input.KeyDown(golib.KeySpace) || input.GamepadDown(0, golib.GamepadA),
	}
}

// cameraTurn reads Q and E, or the right stick, as a turn of the camera, in
// degrees per second: positive turns the view left.
func cameraTurn(input *golib.Input) float32 {
	stick, _ := input.GamepadRightStick(0)
	turn := -stick
	if input.KeyDown(golib.KeyQ) {
		turn = 1
	}
	if input.KeyDown(golib.KeyE) {
		turn = -1
	}
	return turn * cameraTurnSpeed
}
