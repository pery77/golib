//go:build js

package device

import (
	"errors"
	"syscall/js"
	"unsafe"
)

// The rigid bodies in a browser: JoltPhysics.js, through web.js, which
// loads it the first time a game makes a world. Its calls are direct, not
// commands: the physics runs in Update, apart from the drawing.

// NewPhysics makes a world of rigid bodies for up to maxBodies of them, or
// says why it can't, waiting for JoltPhysics.js the first time.
func NewPhysics(maxBodies int) (Physics, error) {
	usesPhysics = PhysicsMarker
	done := make(chan string, 1)
	var answer js.Func
	answer = js.FuncOf(func(_ js.Value, args []js.Value) any {
		answer.Release()
		done <- args[0].String()
		return nil
	})
	js_().Call("loadJolt", answer)
	if message := <-done; message != "" {
		return Physics{}, errors.New(message)
	}
	return Physics{ID: uintptr(js_().Call("newPhysics", maxBodies).Int())}, nil
}

// FreePhysics frees a world and its bodies.
func FreePhysics(p Physics) {
	if p.ID != 0 {
		js_().Call("freePhysics", int(p.ID))
	}
}

// SetPhysicsGravity sets the pull on every body, in meters per second per
// second.
func SetPhysicsGravity(p Physics, gravity [3]float32) {
	js_().Call("physicsGravity", int(p.ID), gravity[0], gravity[1], gravity[2])
}

// AddPhysicsMesh adds a body of triangles, three corners each, three floats
// a corner, in the body's own space, placed at position and rotation, a
// quaternion: still, or moved by the game with SetMoveTo when moved is
// true. It returns its id, or NoBody.
func AddPhysicsMesh(p Physics, corners []float32, friction, restitution float32, position [3]float32, rotation [4]float32, moved bool) uint32 {
	more := []float32{friction, restitution, position[0], position[1], position[2], rotation[0], rotation[1], rotation[2], rotation[3], 0}
	if moved {
		more[9] = 1
	}
	return uint32(js_().Call("addPhysicsMesh", int(p.ID), jsBytes(float32Bytes(corners)), jsBytes(float32Bytes(more))).Int())
}

// AddPhysicsBody adds a body as description says (see BodyShape), and
// returns its id, or NoBody.
func AddPhysicsBody(p Physics, description []float32) uint32 {
	return uint32(js_().Call("addPhysicsBody", int(p.ID), jsBytes(float32Bytes(description))).Int())
}

// RemovePhysicsBody takes a body out of its world.
func RemovePhysicsBody(p Physics, id uint32) {
	js_().Call("removePhysicsBody", int(p.ID), id)
}

// StepPhysics moves the world on by dt seconds, checking collisions steps
// times.
func StepPhysics(p Physics, dt float32, steps int) {
	js_().Call("stepPhysics", int(p.ID), dt, steps)
}

// ReadPhysicsBodies writes BodyStateFloats floats for each body of ids into
// out, which holds that many.
func ReadPhysicsBodies(p Physics, ids []uint32, out []float32) {
	if len(ids) == 0 {
		return
	}
	buffer := js.Global().Get("Uint8Array").New(len(ids) * BodyStateFloats * 4)
	js_().Call("readPhysicsBodies", int(p.ID), jsBytes(uint32Bytes(ids)), buffer)
	js.CopyBytesToGo(unsafe.Slice((*byte)(unsafe.Pointer(&out[0])), len(ids)*BodyStateFloats*4), buffer)
}

// SetPhysicsBody changes a body as what says (see SetPlace), from values.
func SetPhysicsBody(p Physics, id uint32, what int, values []float32) {
	js_().Call("setPhysicsBody", int(p.ID), id, what, jsBytes(float32Bytes(values)))
}
