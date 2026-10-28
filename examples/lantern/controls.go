package main

import "golib"

// The camera behind the explorer.
const (
	cameraDistance  = 4.4 // meters from the point it looks at
	cameraPitch     = -16 // degrees: negative looks down
	cameraLookUp    = 1.4 // meters above the explorer's feet the camera looks at
	cameraLag       = 0.08
	cameraView      = 66  // degrees seen from the bottom of the screen to the top
	cameraTurnSpeed = 110 // degrees per second, with the left and right keys or the right stick
	stickDeadZone   = 0.15
)

// controls is what the keyboard and gamepad 0 say in one update.
type controls struct {
	forward, right float32 // the push, from -1 to 1, as the player sees it
	turn           float32 // the camera's turn, from -1 to 1: positive turns the view left
}

// readControls reads the keyboard and gamepad 0: Up and Down, W and S or the
// left stick walk; Left and Right, A and D or the right stick turn the view,
// and the explorer walks where it looks; Q and E or the d-pad's sides step
// sideways.
func readControls(input *golib.Input) controls {
	var c controls
	sx, sy := input.GamepadLeftStick(0)
	if (golib.Vector2{X: sx, Y: sy}).Length() >= stickDeadZone {
		c.right, c.forward = sx, -sy // the stick's y grows downwards
	}
	if input.KeyDown(golib.KeyUp) || input.KeyDown(golib.KeyW) || input.GamepadDown(0, golib.GamepadUp) {
		c.forward = 1
	}
	if input.KeyDown(golib.KeyDown) || input.KeyDown(golib.KeyS) || input.GamepadDown(0, golib.GamepadDown) {
		c.forward = -1
	}
	if input.KeyDown(golib.KeyQ) || input.GamepadDown(0, golib.GamepadLeft) {
		c.right = -1
	}
	if input.KeyDown(golib.KeyE) || input.GamepadDown(0, golib.GamepadRight) {
		c.right = 1
	}
	rx, _ := input.GamepadRightStick(0)
	if rx > stickDeadZone || rx < -stickDeadZone {
		c.turn = -rx
	}
	if input.KeyDown(golib.KeyLeft) || input.KeyDown(golib.KeyA) {
		c.turn = 1
	}
	if input.KeyDown(golib.KeyRight) || input.KeyDown(golib.KeyD) {
		c.turn = -1
	}
	return c
}

// newCamera is the camera behind the explorer, placed at once.
func newCamera(w *world) *golib.Camera3D {
	camera := golib.NewCamera3D(screenWidth, screenHeight)
	camera.FieldOfView = cameraView
	camera.Lag = cameraLag
	camera.Heading = w.player.yaw
	camera.Orbit(w.player.body.Position.Add(golib.Vector3{Y: cameraLookUp}), camera.Heading, cameraPitch, cameraDistance)
	camera.AvoidWalls(w.physics)
	camera.Snap()
	return camera
}

// steer turns the view by the controls and returns what the player wants
// from where the camera looks: forward is away from it.
func steer(c controls, camera *golib.Camera3D, dt float32) intent {
	camera.Heading += c.turn * cameraTurnSpeed * dt
	return intent{move: camera.Direction(c.forward, c.right)}
}

// follow keeps the camera behind the explorer and in front of any wall that
// would hide them.
func follow(camera *golib.Camera3D, w *world, dt float32) {
	p := &w.player
	camera.Follow(p.body.Position.Add(golib.Vector3{Y: cameraLookUp}), p.body.Moved.Scale(1/dt), cameraDistance, cameraPitch, dt)
	camera.AvoidWalls(w.physics)
	camera.Update(dt)
}
