//go:build !js

package device

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"
)

// The rigid bodies on the desktop: golibnative, GoLib's native library, found
// next to the game, as raylib is, or on the library search path, as golib
// test sets it. It is loaded the first time a game makes a physics world, so
// games without rigid bodies never need it.

// nativeVersion is the version of golibnative's interface this file calls.
const nativeVersion = 1

var native struct {
	loaded bool
	err    error

	worldNew, worldFree, worldGravity uintptr
	addMesh, addBody, removeBody      uintptr
	step, read, set, version          uintptr
}

// nativeName is the native library's file name on this system.
func nativeName() string {
	switch runtime.GOOS {
	case "windows":
		return "golibnative.dll"
	case "darwin":
		return "golibnative.dylib"
	}
	return "golibnative.so"
}

// loadNative loads golibnative and finds its functions, the first time.
func loadNative() error {
	if native.loaded {
		return native.err
	}
	native.loaded = true
	var places []string
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		places = append(places, filepath.Join(dir, nativeName()), filepath.Join(dir, "..", "Frameworks", nativeName()))
	}
	places = append(places, nativeName()) // the library search path
	var handle uintptr
	var tried error
	for _, p := range places {
		if h, err := openLibrary(p); err == nil {
			handle = h
			break
		} else if tried == nil {
			tried = err
		}
	}
	if handle == 0 {
		native.err = fmt.Errorf("rigid bodies need GoLib's native library, %s, which isn't next to the game: run golib setup, then build the game again with golib (%v)", nativeName(), tried)
		return native.err
	}
	for _, f := range []struct {
		to   *uintptr
		name string
	}{
		{&native.worldNew, "golib_world_new"}, {&native.worldFree, "golib_world_free"},
		{&native.worldGravity, "golib_world_gravity"}, {&native.addMesh, "golib_add_mesh"},
		{&native.addBody, "golib_add_body"}, {&native.removeBody, "golib_remove_body"},
		{&native.step, "golib_step"}, {&native.read, "golib_read"}, {&native.set, "golib_set"},
		{&native.version, "golib_version"},
	} {
		p, err := librarySymbol(handle, f.name)
		if err != nil {
			native.err = fmt.Errorf("%s has no %s: run golib setup for the native library this GoLib needs", nativeName(), f.name)
			return native.err
		}
		*f.to = p
	}
	if v, _, _ := purego.SyscallN(native.version); int32(v) != nativeVersion {
		native.err = fmt.Errorf("%s is version %d, and this GoLib needs version %d: run golib setup", nativeName(), int32(v), nativeVersion)
	}
	return native.err
}

func floatsPointer(values []float32) uintptr {
	if len(values) == 0 {
		return 0
	}
	return uintptr(unsafe.Pointer(&values[0]))
}

// NewPhysics makes a world of rigid bodies for up to maxBodies of them, or
// says why it can't.
func NewPhysics(maxBodies int) (Physics, error) {
	usesPhysics = PhysicsMarker
	if err := loadNative(); err != nil {
		return Physics{}, err
	}
	w, _, _ := purego.SyscallN(native.worldNew, uintptr(maxBodies))
	if w == 0 {
		return Physics{}, errors.New("the native library could not make a physics world")
	}
	return Physics{ID: w}, nil
}

// FreePhysics frees a world and its bodies.
func FreePhysics(p Physics) {
	if p.ID != 0 {
		purego.SyscallN(native.worldFree, p.ID)
	}
}

// SetPhysicsGravity sets the pull on every body, in meters per second per
// second.
func SetPhysicsGravity(p Physics, gravity [3]float32) {
	purego.SyscallN(native.worldGravity, p.ID, uintptr(unsafe.Pointer(&gravity[0])))
	runtime.KeepAlive(&gravity)
}

// AddPhysicsMesh adds a body of triangles, three corners each, three floats
// a corner, in the body's own space, placed at position and rotation, a
// quaternion: still, or moved by the game with SetMoveTo when moved is
// true. It returns its id, or NoBody.
func AddPhysicsMesh(p Physics, corners []float32, friction, restitution float32, position [3]float32, rotation [4]float32, moved bool) uint32 {
	more := [10]float32{friction, restitution, position[0], position[1], position[2], rotation[0], rotation[1], rotation[2], rotation[3]}
	if moved {
		more[9] = 1
	}
	id, _, _ := purego.SyscallN(native.addMesh, p.ID, floatsPointer(corners), uintptr(len(corners)/3), uintptr(unsafe.Pointer(&more[0])))
	runtime.KeepAlive(corners)
	runtime.KeepAlive(&more)
	return uint32(id)
}

// AddPhysicsBody adds a body as description says (see BodyShape), and
// returns its id, or NoBody.
func AddPhysicsBody(p Physics, description []float32) uint32 {
	id, _, _ := purego.SyscallN(native.addBody, p.ID, floatsPointer(description))
	runtime.KeepAlive(description)
	return uint32(id)
}

// RemovePhysicsBody takes a body out of its world.
func RemovePhysicsBody(p Physics, id uint32) {
	purego.SyscallN(native.removeBody, p.ID, uintptr(id))
}

// StepPhysics moves the world on by dt seconds, checking collisions steps
// times.
func StepPhysics(p Physics, dt float32, steps int) {
	time := [2]float32{dt, float32(steps)}
	purego.SyscallN(native.step, p.ID, uintptr(unsafe.Pointer(&time[0])))
	runtime.KeepAlive(&time)
}

// ReadPhysicsBodies writes BodyStateFloats floats for each body of ids into
// out, which holds that many.
func ReadPhysicsBodies(p Physics, ids []uint32, out []float32) {
	if len(ids) == 0 {
		return
	}
	purego.SyscallN(native.read, p.ID, uintptr(unsafe.Pointer(&ids[0])), uintptr(len(ids)), floatsPointer(out))
	runtime.KeepAlive(ids)
	runtime.KeepAlive(out)
}

// SetPhysicsBody changes a body as what says (see SetPlace), from values.
func SetPhysicsBody(p Physics, id uint32, what int, values []float32) {
	purego.SyscallN(native.set, p.ID, uintptr(id), uintptr(what), floatsPointer(values))
	runtime.KeepAlive(values)
}
