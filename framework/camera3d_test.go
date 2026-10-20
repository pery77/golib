package golib

import (
	"math"
	"testing"
)

func TestCamera3DProjects(t *testing.T) {
	camera := NewCamera3D(1280, 720)
	if got, ok := camera.ToScreen(camera.Target); !ok || math.Abs(float64(got.X-640)) > 0.01 || math.Abs(float64(got.Y-360)) > 0.01 {
		t.Errorf("ToScreen(Target) = %v, %v, want the middle of the screen", got, ok)
	}
	// From 0, 4, -8 looking at the origin, towards +Z, -X is to the right
	// of the screen and +Y up.
	if right, _ := camera.ToScreen(Vector3{X: -1}); right.X <= 640 {
		t.Errorf("-X is at %v, want right of the middle", right)
	}
	if up, _ := camera.ToScreen(Vector3{Y: 1}); up.Y >= 360 {
		t.Errorf("+Y is at %v, want above the middle", up)
	}
	if _, ok := camera.ToScreen(Vector3{Y: 4, Z: -20}); ok {
		t.Error("a point behind the camera is on the screen, want false")
	}
	if right := camera.Right(); !near3(right, Vector3{X: -1}) {
		t.Errorf("Right = %v, want -X", right)
	}

	for _, at := range []Vector2{{X: 640, Y: 360}, {X: 100, Y: 600}, {X: 1200, Y: 50}} {
		origin, direction := camera.Ray(at.X, at.Y)
		back, ok := camera.ToScreen(origin.Add(direction.Scale(10)))
		if !ok || math.Abs(float64(back.X-at.X)) > 0.05 || math.Abs(float64(back.Y-at.Y)) > 0.05 {
			t.Errorf("a point on Ray(%v) is at %v on the screen, want where the ray went through", at, back)
		}
	}
	if _, direction := camera.Ray(640, 360); !near3(direction, camera.Forward()) {
		t.Errorf("the ray through the middle points %v, want Forward, %v", direction, camera.Forward())
	}

	camera.Orthographic = true
	camera.FieldOfView = 10
	if got, ok := camera.ToScreen(camera.Target); !ok || math.Abs(float64(got.X-640)) > 0.01 || math.Abs(float64(got.Y-360)) > 0.01 {
		t.Errorf("orthographic ToScreen(Target) = %v, %v, want the middle", got, ok)
	}
	// 10 meters from bottom to top: a point 5 meters up from the target, at
	// right angles to the view, is at the top edge.
	upward := camera.Forward().Cross(camera.Right()).Scale(-5)
	if top, _ := camera.ToScreen(camera.Target.Add(upward)); math.Abs(float64(top.Y)) > 0.05 {
		t.Errorf("5 meters up in a 10 meter view is at %v, want the top edge", top)
	}
}

func TestCamera3DPlaces(t *testing.T) {
	camera := NewCamera3D(800, 600)
	camera.Orbit(Vector3{X: 1}, 0, -30, 10)
	if want := (Vector3{X: 1, Y: 5, Z: float32(-10 * math.Sqrt(3) / 2)}); !near3(camera.Position, want) {
		t.Errorf("Orbit yaw 0, pitch -30 put the camera at %v, want %v: behind, above", camera.Position, want)
	}
	if camera.Target != (Vector3{X: 1}) {
		t.Errorf("Orbit's Target = %v, want the center", camera.Target)
	}
	camera.Look(Vector3{Y: 2}, 90, 0)
	if !near3(camera.Forward(), Vector3{X: 1}) {
		t.Errorf("Look yaw 90 looks %v, want +X", camera.Forward())
	}

	camera.Lag = 1
	camera.Update(1.0 / 60)
	camera.Look(Vector3{Y: 2, Z: 10}, 90, 0)
	camera.Update(1.0 / 60)
	eye, _ := camera.view()
	if eye.Z <= 0 || eye.Z >= 1 {
		t.Errorf("after one update with Lag 1, the eye is at %v, want a little way towards Z 10", eye)
	}
	camera.Snap()
	if eye, _ := camera.view(); eye != camera.Position {
		t.Errorf("after Snap the eye is at %v, want Position %v", eye, camera.Position)
	}

	camera.Shake(1, 0.5)
	camera.Update(1.0 / 60)
	if camera.shakeOffset == (Vector3{}) || camera.shakeOffset.Length() > float32(math.Sqrt(3)) {
		t.Errorf("shake moved the view by %v, want a little, at most 1 a side", camera.shakeOffset)
	}
	for range 60 {
		camera.Update(1.0 / 60)
	}
	if camera.shakeOffset != (Vector3{}) {
		t.Errorf("a shake of half a second still moves the view after a second: %v", camera.shakeOffset)
	}
}

func TestCamera3DFollows(t *testing.T) {
	camera := NewCamera3D(800, 600)
	target := Vector3{}
	// Running towards +X, the camera swings round behind, towards yaw 90.
	for range 120 {
		target = target.Add(Vector3{X: 0.1})
		camera.Follow(target, Vector3{X: 6}, 5, -20, 1.0/60)
	}
	if camera.Heading < 85 || camera.Heading > 91 {
		t.Errorf("after two seconds running towards +X, Heading is %v, want about 90", camera.Heading)
	}
	if !near3(camera.Target, target) || camera.Position.X >= target.X {
		t.Errorf("the camera is at %v looking at %v, want behind the runner at %v", camera.Position, camera.Target, target)
	}
	// Running back towards the camera leaves it where it is.
	heading := camera.Heading
	for range 60 {
		camera.Follow(target, Vector3{X: -6}, 5, -20, 1.0/60)
	}
	if camera.Heading != heading {
		t.Errorf("running back towards the camera turned it from %v to %v", heading, camera.Heading)
	}
	// Forward on the stick is away from the camera, flat on the ground.
	if got := camera.Direction(1, 0); !near3(got, Vector3{X: 1}) && got.X < 0.99 {
		t.Errorf("Direction(1, 0) = %v, want away from the camera, +X", got)
	}
	if got := camera.Direction(1, 1); got.Length() > 1.0001 {
		t.Errorf("pushing diagonally gives %v, longer than 1", got)
	}
	if got := shortestTurn(170, -170); got != 20 {
		t.Errorf("the short way from 170 to -170 is %v, want 20", got)
	}
}
