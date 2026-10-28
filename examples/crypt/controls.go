package main

import "golib"

// The camera behind the player.
const (
	cameraDistance  = 3.8  // meters from the point it looks at
	cameraPitch     = -26  // degrees: negative looks down
	cameraLookUp    = 0.7  // meters above the astronaut's feet the camera looks at
	cameraLag       = 0.1  // seconds of smoothing
	cameraView      = 72   // degrees seen from the bottom of the screen to the top: wide, for tight halls
	cameraTurnSpeed = 110  // degrees per second, with Q and E or the right stick
	stickWalk       = 0.15 // a stick tilted less than this is at rest
)

// controls is what the keyboard, the mouse and gamepad 0 say in one update,
// before the camera turns it into an intent. The play scene reads it from
// golib.Input; the tests make it up, to play the scene without a window.
type controls struct {
	forward, right float32 // the push, from -1 to 1 each, as the player sees it
	turn           float32 // the camera's turn, from -1 to 1: positive turns the view left
	click          bool    // the left mouse button went down
	mouseX, mouseY float32 // where the mouse pointer is, in screen pixels
	smash          bool    // F, J, Space, the gamepad's X or A was pressed: break the crate in front
}

// readControls reads the keyboard, the mouse and gamepad 0: arrows, WASD,
// the d-pad or the left stick to run; Q and E or the right stick to turn the
// camera; a click on a crate, or F, J, Space, the gamepad's X or A, to break
// one.
func readControls(input *golib.Input) controls {
	var c controls
	right, down := input.GamepadLeftStick(0)
	if (golib.Vector2{X: right, Y: down}).Length() >= stickWalk {
		c.right, c.forward = right, -down // the stick's y grows downwards
	}
	if input.KeyDown(golib.KeyLeft) || input.KeyDown(golib.KeyA) || input.GamepadDown(0, golib.GamepadLeft) {
		c.right = -1
	}
	if input.KeyDown(golib.KeyRight) || input.KeyDown(golib.KeyD) || input.GamepadDown(0, golib.GamepadRight) {
		c.right = 1
	}
	if input.KeyDown(golib.KeyUp) || input.KeyDown(golib.KeyW) || input.GamepadDown(0, golib.GamepadUp) {
		c.forward = 1
	}
	if input.KeyDown(golib.KeyDown) || input.KeyDown(golib.KeyS) || input.GamepadDown(0, golib.GamepadDown) {
		c.forward = -1
	}
	stick, _ := input.GamepadRightStick(0)
	c.turn = -stick
	if input.KeyDown(golib.KeyQ) {
		c.turn = 1
	}
	if input.KeyDown(golib.KeyE) {
		c.turn = -1
	}
	c.click = input.MousePressed(golib.MouseLeft)
	c.mouseX, c.mouseY = input.MousePosition()
	c.smash = input.KeyPressed(golib.KeyF) || input.KeyPressed(golib.KeyJ) || input.KeyPressed(golib.KeySpace) ||
		input.GamepadPressed(0, golib.GamepadX) || input.GamepadPressed(0, golib.GamepadA)
	return c
}

// intent turns the controls into what the player wants in the world, from
// where camera looks: forward is away from the camera, and a click breaks
// the crate under the pointer.
func (c controls) intent(camera *golib.Camera3D, w *world) intent {
	want := intent{move: camera.Direction(c.forward, c.right), smash: -1, smashNear: c.smash}
	if c.click {
		want.smash = w.pickCrate(camera.Ray(c.mouseX, c.mouseY))
	}
	return want
}

// follow keeps camera behind the astronaut, looking down at it, and in front
// of any wall that would hide it. Backed into a corner, where the walls would
// bring it right up to the astronaut's head, it rises and looks down more
// steeply instead, over the walls.
func follow(camera *golib.Camera3D, w *world, dt float32) {
	p := &w.player
	target := p.body.Position.Add(golib.Vector3{Y: cameraLookUp})
	camera.Follow(target, p.velocity, cameraDistance, cameraPitch, dt)
	camera.AvoidWalls(w.physics)
	best, bestPitch, tried := camera.Position.Distance(target), float32(cameraPitch), float32(cameraPitch)
	for _, pitch := range cameraSteeper {
		if best >= cameraDistance*cameraRoomy {
			break
		}
		camera.Follow(target, p.velocity, cameraDistance, pitch, 0) // dt 0: the heading stays
		camera.AvoidWalls(w.physics)
		tried = pitch
		if d := camera.Position.Distance(target); d > best+0.05 {
			best, bestPitch = d, pitch
		}
	}
	if tried != bestPitch { // the last one tried wasn't the best: put the best one back
		camera.Follow(target, p.velocity, cameraDistance, bestPitch, 0)
		camera.AvoidWalls(w.physics)
	}
	camera.Update(dt)
}

// The pitches the camera tries, steeper and steeper, when walls bring it
// closer than cameraRoomy of its distance.
var cameraSteeper = []float32{-45, -62, -78, -88}

const cameraRoomy = 0.75

// newFollowCamera is a camera behind the astronaut, already in place.
func newFollowCamera(w *world) *golib.Camera3D {
	camera := golib.NewCamera3D(screenWidth, screenHeight)
	camera.Heading = w.player.yaw
	camera.Lag = cameraLag
	camera.Near = 0.05
	camera.FieldOfView = cameraView
	follow(camera, w, 1.0/60)
	camera.Snap()
	return camera
}
