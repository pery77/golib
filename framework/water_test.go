package golib

import "testing"

// Clear shallow water shows its bottom; deep water its own color; still
// water mirrors what stands over it.
func TestWaterInAWindow(t *testing.T) {
	screen, capture := openTestWindow(t, 64, 64)
	takeError()
	camera := NewCamera3D(64, 64)
	box := NewBoxMesh()
	env := Environment{NoBloom: true, NoShadows: true, Sky: Gray, Ground: Gray}
	water := Water{Color: Color{R: 0, G: 40, B: 120, A: 255}, Clarity: 1, Still: true, NoFoam: true}

	// Looking down at water over a white bottom.
	camera.Position, camera.Target = Vector3{Y: 5, Z: -0.01}, Vector3{}
	camera.Snap()
	down := func(bottom float32) (r, g, b int) {
		c := capture(func() {
			screen.Begin3D(camera)
			screen.SetEnvironment(env)
			screen.DrawMesh(box, Transform{Position: Vector3{Y: bottom - 0.5}, Scale: Vector3{X: 20, Y: 1, Z: 20}}, White)
			screen.DrawWater(water, Transform{Scale: Vector3{X: 20, Z: 20}})
			screen.End3D()
		}).NRGBAAt(32, 32)
		return int(c.R), int(c.G), int(c.B)
	}
	sr, sg, sb := down(-0.05)
	dr, dg, db := down(-20)
	if sr < 150 || sg < 150 {
		t.Errorf("water 5 cm over a white bottom is %d, %d, %d, want the white showing through", sr, sg, sb)
	}
	if db < 2*dr || dr > 60 {
		t.Errorf("water 20 m deep is %d, %d, %d, want its own blue", dr, dg, db)
	}

	// Looking along still water at a red wall beyond it: under the wall,
	// the water shows it, redder than without the wall.
	camera.Position, camera.Target = Vector3{Y: 1, Z: -6}, Vector3{Y: -0.4, Z: 2}
	camera.Snap()
	redInWater := func(wall bool) int {
		picture := capture(func() {
			screen.Begin3D(camera)
			screen.SetEnvironment(env)
			screen.DrawMesh(box, Transform{Position: Vector3{Y: -10.5}, Scale: Vector3{X: 40, Y: 1, Z: 40}}, Black)
			if wall {
				screen.DrawMesh(box, Transform{Position: Vector3{Y: 1.5, Z: 4}, Scale: Vector3{X: 20, Y: 3, Z: 0.5}}, Red)
			}
			screen.DrawWater(water, Transform{Scale: Vector3{X: 40, Z: 40}})
			screen.End3D()
		})
		red := 0
		for y := 30; y < 64; y++ { // the water, under the wall
			red = max(red, int(picture.NRGBAAt(32, y).R))
		}
		return red
	}
	if with, without := redInWater(true), redInWater(false); with < without+40 {
		t.Errorf("still water before a red wall is at most %d red, and %d without the wall: want the wall reflected in it", with, without)
	}
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
}
