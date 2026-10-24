//go:build !js

package device

import (
	"math"
	"testing"
)

// A box dropped onto a floor comes to rest on it, as Jolt has it.
func TestPhysicsDropsABox(t *testing.T) {
	p, err := NewPhysics(1024)
	if err != nil {
		t.Skip(err) // no native library on this machine yet
	}
	defer FreePhysics(p)
	floor := []float32{-10, 0, -10, -10, 0, 10, 10, 0, 10, -10, 0, -10, 10, 0, 10, 10, 0, -10}
	if id := AddPhysicsMesh(p, floor, 0.5, 0, [3]float32{}, [4]float32{0, 0, 0, 1}, false); id == NoBody {
		t.Fatal("the floor made no body")
	}
	d := make([]float32, BodyFloats)
	d[BodyShape], d[BodySizeX], d[BodySizeY], d[BodySizeZ] = ShapeBox, 0.5, 0.5, 0.5
	d[BodyPositionY], d[BodyRotationW] = 5, 1
	d[BodyMotion], d[BodyFriction], d[BodyGravity] = MotionFree, 0.5, 1
	box := AddPhysicsBody(p, d)
	if box == NoBody {
		t.Fatal("the box made no body")
	}
	for range 180 {
		StepPhysics(p, 1.0/60, 1)
	}
	state := make([]float32, BodyStateFloats)
	ReadPhysicsBodies(p, []uint32{box}, state)
	if math.Abs(float64(state[1]-0.5)) > 0.02 {
		t.Errorf("three seconds after falling from 5 m, the box's middle is at %v, want resting on the floor at 0.5", state[:3])
	}
	SetPhysicsBody(p, box, SetImpulse, []float32{0, 50, 0, 0, 0.5, 0})
	StepPhysics(p, 1.0/60, 1)
	ReadPhysicsBodies(p, []uint32{box}, state)
	if state[8] <= 0 {
		t.Errorf("after an impulse up, the box's velocity is %v, want up", state[7:10])
	}
}
