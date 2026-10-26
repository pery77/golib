package golib

import "testing"

// A polished floor reflects a red wall standing on it, from the pass, and
// only the sky with NoScreenReflections.
func TestScreenReflectionsInAWindow(t *testing.T) {
	screen, capture := openTestWindow(t, 64, 64)
	takeError()
	camera := NewCamera3D(64, 64)
	camera.Position, camera.Target = Vector3{Y: 1.5, Z: -6}, Vector3{Y: 0, Z: 1}
	camera.Snap()
	box := NewBoxMesh()
	floor := &Material{Color: White, Metallic: 1, Roughness: 0.05}
	red := func(env Environment) int {
		c := capture(func() {
			screen.Begin3D(camera)
			screen.SetEnvironment(env)
			screen.DrawMeshWith(box, Transform{Position: Vector3{Y: -0.5}, Scale: Vector3{X: 20, Y: 1, Z: 20}}, floor)
			screen.DrawMesh(box, Transform{Position: Vector3{Y: 1, Z: 3}, Scale: Vector3{X: 6, Y: 2, Z: 0.2}}, Red)
			screen.End3D()
		}).NRGBAAt(32, 35)
		return int(c.R) - int(c.G)
	}
	env := Environment{NoBloom: true, NoShadows: true, Sky: Gray, Ground: Gray}
	with := red(env)
	env.NoScreenReflections = true
	without := red(env)
	if with < 60 || without > 20 {
		t.Errorf("a polished floor in front of a red wall is %d redder than green, and %d with NoScreenReflections: want the wall reflected", with, without)
	}
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
}
