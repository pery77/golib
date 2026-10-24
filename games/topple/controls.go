package main

import (
	"math"

	"golib"
)

// The camera behind the player, turned towards the stacks.
const (
	cameraDistance = 8   // meters behind the point it looks at
	cameraPitch    = -22 // degrees: negative looks down
	cameraAhead    = 2.5 // meters in front of the player the camera looks at
	cameraLookUp   = 1.1 // meters above the ground it looks at
	cameraSide     = 1.1 // meters to the player's right: the camera looks over its shoulder
	cameraTurn     = 4   // how quickly the camera swings round to face the stacks, per second
	cameraNear     = 2   // meters from the focus within which the camera stops turning
	cameraLag      = 0.08
	cursorSpeed    = 700  // screen pixels per second, for the right stick
	stickRest      = 0.15 // a stick tilted less than this is at rest
)

// controls is what the keyboard, the mouse and gamepad 0 say in one update.
// The play scene reads it from golib.Input; tests make it up.
type controls struct {
	forward, right float32 // the push, from -1 to 1 each, as the player sees it
	throw          bool    // throw at the cursor
	mouseMoved     bool
	mouseX, mouseY float32 // the mouse pointer, in screen pixels
	cursorX        float32 // the right stick, moving the cursor
	cursorY        float32
}

// readControls reads the keyboard, the mouse and gamepad 0: arrows, WASD,
// the d-pad or the left stick to walk; the mouse or the right stick to aim;
// a left click, Space, or the gamepad's A or right shoulder to throw.
func readControls(input *golib.Input) controls {
	var c controls
	right, down := input.GamepadLeftStick(0)
	if (golib.Vector2{X: right, Y: down}).Length() >= stickRest {
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
	sx, sy := input.GamepadRightStick(0)
	if (golib.Vector2{X: sx, Y: sy}).Length() >= stickRest {
		c.cursorX, c.cursorY = sx, sy
	}
	c.mouseMoved = input.MouseMoved()
	c.mouseX, c.mouseY = input.MousePosition()
	c.throw = input.MousePressed(golib.MouseLeft) || input.KeyPressed(golib.KeySpace) ||
		input.GamepadPressed(0, golib.GamepadA) || input.GamepadPressed(0, golib.GamepadRightTrigger)
	if input.MousePressed(golib.MouseLeft) {
		c.mouseMoved = true // a click throws where the pointer is
	}
	return c
}

// aimer keeps the cursor the player throws at, in screen pixels.
type aimer struct {
	x, y float32
}

func newAimer() aimer {
	return aimer{x: screenWidth / 2, y: screenHeight * 0.42}
}

// update follows the mouse when it moves, and the right stick.
func (a *aimer) update(c controls, dt float32) {
	if c.mouseMoved {
		a.x, a.y = c.mouseX, c.mouseY
	}
	a.x = golib.Clamp(a.x+c.cursorX*cursorSpeed*dt, 0, screenWidth)
	a.y = golib.Clamp(a.y+c.cursorY*cursorSpeed*dt, 0, screenHeight)
}

// intent turns the controls into what the player wants in the world, from
// where the camera looks: forward is away from it.
func (c controls) intent(camera *golib.Camera3D, aim aimer) intent {
	want := intent{move: camera.Direction(c.forward, c.right), throw: c.throw}
	want.aimAt, want.aimBy = camera.Ray(aim.x, aim.y)
	return want
}

// view is the camera's heading, kept between updates.
type view struct {
	camera  *golib.Camera3D
	heading float32
}

// newView is a camera behind the player, facing the stacks, already in place.
func newView(w *world) *view {
	v := &view{camera: golib.NewCamera3D(screenWidth, screenHeight)}
	v.camera.Lag = cameraLag
	v.heading = golib.RotationTowards(w.focus().Sub(w.player.body.Position)).Yaw
	v.place(w)
	v.camera.Snap()
	return v
}

// follow swings the camera round to look past the player at the stacks.
func (v *view) follow(w *world, dt float32) {
	to := w.focus().Sub(w.player.body.Position)
	to.Y = 0
	if to.Length() > cameraNear {
		want := golib.RotationTowards(to).Yaw
		v.heading += angleBetween(v.heading, want) * min(1, cameraTurn*dt)
	}
	v.place(w)
	v.camera.Update(dt)
}

// place puts the camera behind the player along the heading.
func (v *view) place(w *world) {
	turn := golib.Rotation{Yaw: v.heading}
	head := w.player.body.Position.Add(golib.Vector3{Y: cameraLookUp})
	// The point looked at stays out of what is solid, or AvoidWalls can't
	// keep it in sight.
	way := turn.Forward().Scale(cameraAhead).Add(turn.Right().Scale(cameraSide))
	reach := way.Length()
	if hit, ok := w.physics.SphereCast(head, way.Normalize(), 0.3, reach); ok {
		reach = max(0, hit.Distance-0.1)
	}
	target := head.Add(way.Normalize().Scale(reach))
	v.camera.Orbit(target, v.heading, cameraPitch, cameraDistance)
	v.camera.AvoidWalls(w.physics)
}

// angleBetween is the shortest turn from one yaw to another, in degrees,
// from -180 to 180.
func angleBetween(from, to float32) float32 {
	d := float32(math.Mod(float64(to-from), 360))
	if d > 180 {
		d -= 360
	}
	if d < -180 {
		d += 360
	}
	return d
}
