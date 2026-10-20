package golib

import (
	"math"
	"testing"
)

func near3(a, b Vector3) bool {
	const e = 1e-4
	return math.Abs(float64(a.X-b.X)) < e && math.Abs(float64(a.Y-b.Y)) < e && math.Abs(float64(a.Z-b.Z)) < e
}

func TestVector3(t *testing.T) {
	a, b := Vector3{X: 1, Y: 2, Z: 3}, Vector3{X: 4, Y: 5, Z: 6}
	if got := a.Add(b); got != (Vector3{X: 5, Y: 7, Z: 9}) {
		t.Errorf("Add = %v", got)
	}
	if got := b.Sub(a); got != (Vector3{X: 3, Y: 3, Z: 3}) {
		t.Errorf("Sub = %v", got)
	}
	if got := a.Dot(b); got != 32 {
		t.Errorf("Dot = %v, want 32", got)
	}
	if got := (Vector3{X: 1}).Cross(Vector3{Y: 1}); got != (Vector3{Z: 1}) {
		t.Errorf("X cross Y = %v, want Z: the world is right-handed", got)
	}
	if got := (Vector3{X: 3, Z: 4}).Length(); got != 5 {
		t.Errorf("Length = %v, want 5", got)
	}
	if got := (Vector3{}).Normalize(); got != (Vector3{}) {
		t.Errorf("zero Normalize = %v, want zero", got)
	}
	if got := (Vector3{X: 10}).ClampLength(2); got != (Vector3{X: 2}) {
		t.Errorf("ClampLength = %v", got)
	}
	if got := (Vector3{}).MoveTowards(Vector3{Z: 10}, 3); got != (Vector3{Z: 3}) {
		t.Errorf("MoveTowards = %v", got)
	}
	if got := (Vector3{}).MoveTowards(Vector3{Z: 1}, 3); got != (Vector3{Z: 1}) {
		t.Errorf("MoveTowards past the target = %v, want the target", got)
	}
	if got := a.Lerp(b, 0.5); got != (Vector3{X: 2.5, Y: 3.5, Z: 4.5}) {
		t.Errorf("Lerp = %v", got)
	}
}

func TestRotation(t *testing.T) {
	cases := []struct {
		rotation       Rotation
		forward, right Vector3
	}{
		{Rotation{}, Vector3{Z: 1}, Vector3{X: -1}},
		{Rotation{Yaw: 90}, Vector3{X: 1}, Vector3{Z: 1}},
		{Rotation{Yaw: 180}, Vector3{Z: -1}, Vector3{X: 1}},
		{Rotation{Yaw: -90}, Vector3{X: -1}, Vector3{Z: -1}},
		{Rotation{Pitch: 90}, Vector3{Y: 1}, Vector3{X: -1}},
		{Rotation{Pitch: -90}, Vector3{Y: -1}, Vector3{X: -1}},
		{Rotation{Roll: 90}, Vector3{Z: 1}, Vector3{Y: -1}}, // rolling right lowers the right side
		{Rotation{Yaw: 90, Pitch: 45}, Vector3{X: float32(math.Sqrt2 / 2), Y: float32(math.Sqrt2 / 2)}, Vector3{Z: 1}},
	}
	for _, c := range cases {
		if got := c.rotation.Forward(); !near3(got, c.forward) {
			t.Errorf("%+v Forward = %v, want %v", c.rotation, got, c.forward)
		}
		if got := c.rotation.Right(); !near3(got, c.right) {
			t.Errorf("%+v Right = %v, want %v", c.rotation, got, c.right)
		}
	}
	if got := (Rotation{Roll: 90}).Up(); !near3(got, Vector3{X: -1}) {
		t.Errorf("rolled right, Up = %v, want -X: the top leans right", got)
	}

	for _, direction := range []Vector3{{X: 1}, {Z: -2}, {X: 1, Y: 1, Z: 1}, {X: -3, Y: -1, Z: 0.5}, {Y: 5}} {
		got := RotationTowards(direction).Forward()
		if want := direction.Normalize(); !near3(got, want) {
			t.Errorf("RotationTowards(%v).Forward() = %v, want %v", direction, got, want)
		}
	}
	if got := RotationTowards(Vector3{}); got != (Rotation{}) {
		t.Errorf("RotationTowards(zero) = %+v, want none", got)
	}
}

func TestTransformMatrix(t *testing.T) {
	plain := Transform{Position: Vector3{X: 1, Y: 2, Z: 3}}
	if got := plain.matrix().transformPoint(Vector3{X: 1}); !near3(got, Vector3{X: 2, Y: 2, Z: 3}) {
		t.Errorf("zero Scale moved X to %v, want it kept at size 1", got)
	}
	flat := Transform{Scale: Vector3{X: 4, Z: 2}}
	if got := flat.matrix().transformPoint(Vector3{X: 1, Y: 1, Z: 1}); !near3(got, Vector3{X: 4, Y: 1, Z: 2}) {
		t.Errorf("Scale with Y left at 0 put 1, 1, 1 at %v, want Y kept at size 1", got)
	}
	scaled := Transform{Scale: Vector3{X: 2, Y: 3, Z: 4}, Rotation: Rotation{Yaw: 90}, Position: Vector3{Y: 10}}
	// Scaled to 0, 0, 4, turned to 4, 0, 0, moved up.
	if got := scaled.matrix().transformPoint(Vector3{Z: 1}); !near3(got, Vector3{X: 4, Y: 10}) {
		t.Errorf("scale, turn and move put +Z at %v, want 4, 10, 0", got)
	}

	m := Transform{Position: Vector3{X: 3, Y: -1, Z: 2}, Rotation: Rotation{Yaw: 30, Pitch: 10, Roll: 5}, Scale: Vector3{X: 2, Y: 1, Z: 0.5}}.matrix()
	product := m.mul(m.inverse())
	for i, v := range identityMatrix() {
		if math.Abs(float64(product[i]-v)) > 1e-4 {
			t.Fatalf("m times its inverse = %v, want the identity", product)
		}
	}
	// Normals stay at right angles to the surfaces they belong to, even under
	// a scale larger one way than another.
	normal := m.normalMatrix().transformDirection(Vector3{Y: 1})
	along := m.transformDirection(Vector3{X: 1})
	if dot := normal.Dot(along); math.Abs(float64(dot)) > 1e-4 {
		t.Errorf("a turned normal and its surface meet at dot %v, want 0", dot)
	}
}
