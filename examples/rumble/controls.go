package main

import "golib"

// The camera behind the player.
const (
	cameraDistance  = 4.6  // meters from the point it looks at
	cameraPitch     = -22  // degrees: negative looks down
	cameraLookUp    = 0.7  // meters above the player's feet the camera looks at
	cameraLag       = 0.08 // seconds of smoothing
	cameraTurnSpeed = 110  // degrees per second, with Q and E or the right stick
	cameraClosest   = 2.4  // meters: as close as the camera comes, with its back to a wall
	cameraSteepest  = -55  // degrees it looks down from there
	cameraWallSwing = 160  // degrees per second it swings round when the player runs at it by a wall
	shiftPush       = 0.45 // how hard the keys push while Shift is held: a walk
)

// readIntent turns the keyboard and gamepad 0 into what the player wants,
// from where camera looks: arrows, WASD, the d-pad or the left stick to run,
// Space or A to jump, J, X, the left mouse button, the gamepad's X or its
// right bumper to swing the stick.
//
// The keys push all the way, which sprints, or gently with Shift held, which
// walks; the stick pushes as far as it is tilted.
func readIntent(input *golib.Input, camera *golib.Camera3D) intent {
	right, down := input.GamepadLeftStick(0)
	forward := -down // the stick's y grows downwards
	keys := false
	if input.KeyDown(golib.KeyLeft) || input.KeyDown(golib.KeyA) || input.GamepadDown(0, golib.GamepadLeft) {
		right, keys = -1, true
	}
	if input.KeyDown(golib.KeyRight) || input.KeyDown(golib.KeyD) || input.GamepadDown(0, golib.GamepadRight) {
		right, keys = 1, true
	}
	if input.KeyDown(golib.KeyUp) || input.KeyDown(golib.KeyW) || input.GamepadDown(0, golib.GamepadUp) {
		forward, keys = 1, true
	}
	if input.KeyDown(golib.KeyDown) || input.KeyDown(golib.KeyS) || input.GamepadDown(0, golib.GamepadDown) {
		forward, keys = -1, true
	}
	move := camera.Direction(forward, right)
	if keys && (input.KeyDown(golib.KeyLeftShift) || input.KeyDown(golib.KeyRightShift)) {
		move = move.Normalize().Scale(shiftPush)
	}
	return intent{
		move: move,
		jump: input.KeyPressed(golib.KeySpace) || input.GamepadPressed(0, golib.GamepadA),
		swing: input.KeyPressed(golib.KeyJ) || input.KeyPressed(golib.KeyX) || input.MousePressed(golib.MouseLeft) ||
			input.GamepadPressed(0, golib.GamepadX) || input.GamepadPressed(0, golib.GamepadRightBumper),
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

// follow keeps camera behind the player, looking at it from above. Near a
// wall the camera comes closer and looks down more steeply, so that it stays
// inside the courtyard instead of looking at the player through the stones;
// when the player runs at it there, it swings round behind the player, and
// when even that isn't enough, round to look out at the wall.
func follow(camera *golib.Camera3D, p *player, dt float32) {
	target := p.position.Add(golib.Vector3{Y: cameraLookUp})
	distance, pitch, fits := cameraFit(target, camera.Heading)
	if distance < cameraDistance-0.4 && p.speed() > 1 {
		way := golib.Vector3{X: p.velocity.X, Z: p.velocity.Z}
		camera.Heading = turnTowards(camera.Heading, golib.RotationTowards(way).Yaw, cameraWallSwing*dt)
	} else if !fits {
		out := golib.Vector3{X: p.position.X, Z: p.position.Z}
		camera.Heading = turnTowards(camera.Heading, golib.RotationTowards(out).Yaw, cameraWallSwing*dt)
	}
	distance, pitch, _ = cameraFit(target, camera.Heading)
	camera.Follow(target, p.velocity, distance, pitch, dt)
}

// cameraFit is how far behind target, and how steeply down, a camera facing
// heading can be while it stays inside the walls: cameraDistance and
// cameraPitch in the open, closer and steeper near a wall. fits is false
// when even the closest is outside.
func cameraFit(target golib.Vector3, heading float32) (distance, pitch float32, fits bool) {
	for distance = cameraDistance; distance > cameraClosest; distance -= 0.1 {
		pitch = golib.Lerp(cameraSteepest, cameraPitch, (distance-cameraClosest)/(cameraDistance-cameraClosest))
		if cameraInside(target, heading, distance, pitch) {
			return distance, pitch, true
		}
	}
	return cameraClosest, cameraSteepest, cameraInside(target, heading, cameraClosest, cameraSteepest)
}

// cameraInside says whether a camera distance meters behind target, facing
// heading and looking pitch degrees down, is inside the walls.
func cameraInside(target golib.Vector3, heading, distance, pitch float32) bool {
	const inside = arenaHalf - 0.4 // meters from the middle the camera may go
	eye := target.Sub(golib.Rotation{Yaw: heading, Pitch: pitch}.Forward().Scale(distance))
	return abs(eye.X) < inside && abs(eye.Z) < inside
}
