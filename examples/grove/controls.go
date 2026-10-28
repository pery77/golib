package main

import "golib"

// The camera behind the astronaut.
const (
	cameraDistance  = 5.2  // meters from the point it looks at
	cameraPitch     = -24  // degrees: negative looks down
	cameraLookUp    = 0.8  // meters above the astronaut's feet the camera looks at
	cameraSeeDown   = 0.3  // meters above its feet the camera keeps in sight, over the blocks between
	cameraLag       = 0.12 // seconds of smoothing
	cameraTurnSpeed = 120  // degrees per second, with Q and E or the right stick
	cameraClosest   = 2.8  // meters: as close as it comes, over a high block behind the astronaut
	cameraSteepest  = -55  // degrees it looks down from there
	cameraClearance = 0.3  // meters it keeps between its line of sight and the ground
)

// readIntent turns the keyboard and gamepad 0 into what the player wants,
// from where camera looks: arrows, WASD, the d-pad or the left stick to walk,
// Shift, the gamepad's B or its right trigger held to sprint, Space or A to
// jump, J, K, the left mouse button or the gamepad's X to punch.
func readIntent(input *golib.Input, camera *golib.Camera3D) intent {
	right, down := input.GamepadLeftStick(0)
	forward := -down // the stick's y grows downwards
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
		move: camera.Direction(forward, right),
		sprint: input.KeyDown(golib.KeyLeftShift) || input.KeyDown(golib.KeyRightShift) ||
			input.GamepadDown(0, golib.GamepadB) || input.GamepadDown(0, golib.GamepadRightTrigger),
		jump: input.KeyPressed(golib.KeySpace) || input.GamepadPressed(0, golib.GamepadA),
		punch: input.KeyPressed(golib.KeyJ) || input.KeyPressed(golib.KeyK) || input.MousePressed(golib.MouseLeft) ||
			input.GamepadPressed(0, golib.GamepadX),
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

// follow keeps camera behind the astronaut, looking at it from above, and
// closer and steeper where the ground between them would hide it: a block
// higher than the one the astronaut stands on, behind it.
func follow(camera *golib.Camera3D, p *player, l *island, dt float32) {
	target := p.position.Add(golib.Vector3{Y: cameraLookUp})
	if p.splash.Running() {
		target = p.safe.Add(golib.Vector3{Y: cameraLookUp}) // look where it comes back
	}
	distance, pitch := cameraFit(target, target.Add(golib.Vector3{Y: cameraSeeDown - cameraLookUp}), camera.Heading, l)
	camera.Follow(target, p.velocity, distance, pitch, dt)
}

// cameraFit is how far back, and how steeply down, a camera facing heading
// can look at target for the line from it to body to pass over the ground:
// cameraDistance and cameraPitch in the open, closer and steeper behind a
// high block.
func cameraFit(target, body golib.Vector3, heading float32, l *island) (distance, pitch float32) {
	const steps = 12
	for step := range steps + 1 {
		t := float32(step) / steps
		distance = golib.Lerp(cameraDistance, cameraClosest, t)
		pitch = golib.Lerp(cameraPitch, cameraSteepest, t)
		eye := target.Sub(golib.Rotation{Yaw: heading, Pitch: pitch}.Forward().Scale(distance))
		clear := true
		for i := 1; i <= 10 && clear; i++ {
			at := body.Lerp(eye, float32(i)/10)
			clear = at.Y > l.highestAround(at.X, at.Z, 0.2)+cameraClearance
		}
		if clear {
			return distance, pitch
		}
	}
	return distance, pitch
}
