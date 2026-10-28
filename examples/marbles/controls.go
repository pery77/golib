package main

import "golib"

// The camera behind the marble.
const (
	cameraDistance  = 10   // meters from the point it looks at
	cameraPitch     = -30  // degrees: negative looks down at it
	cameraAhead     = 2.5  // meters ahead of the marble the camera looks at
	cameraLag       = 0.12 // seconds of smoothing
	cameraFollow    = 1.6  // how fast the camera swings round behind the marble, per second
	cameraFollowMin = 1.5  // meters per second the marble must roll before the camera swings
	cameraFollowArc = 75   // degrees: the camera swings only when the marble rolls at most this far from where it looks
	cameraTurnSpeed = 120  // degrees per second, with the right stick
)

// rollDirection turns the player's intentions into a direction on the ground,
// from where the camera looks: forward is up, away from the camera, and right
// is the screen's right. forward and right go from -1 to 1. cameraYaw is the
// camera's yaw, as golib.Camera3D.Orbit takes it.
func rollDirection(cameraYaw, forward, right float32) golib.Vector3 {
	facing := golib.Rotation{Yaw: cameraYaw}
	return facing.Forward().Scale(forward).Add(facing.Right().Scale(right)).ClampLength(1)
}

// followYaw swings the camera's yaw towards the way the marble rolls, so the
// camera ends up behind it. It leaves the camera alone while the marble is
// slow, and while it rolls sideways or back towards the camera, so that
// rolling back doesn't spin the view round.
func followYaw(cameraYaw float32, velocity golib.Vector3, dt float32) float32 {
	ground := flat(velocity)
	if ground.Length() < cameraFollowMin {
		return cameraYaw
	}
	turn := angleBetween(cameraYaw, golib.RotationTowards(ground).Yaw)
	if turn > cameraFollowArc || turn < -cameraFollowArc {
		return cameraYaw
	}
	return cameraYaw + turn*min(1, cameraFollow*dt)
}

// angleBetween returns the shortest turn from one yaw to another, from -180 to
// 180 degrees.
func angleBetween(from, to float32) float32 {
	turn := to - from
	for turn > 180 {
		turn -= 360
	}
	for turn < -180 {
		turn += 360
	}
	return turn
}
