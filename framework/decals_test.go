package golib

import "testing"

// A decal darkens the ground in its box and nowhere else, a glowing one
// brightens it, and a wall standing across the way it is pressed is left
// alone.
func TestDecalsInAWindow(t *testing.T) {
	screen, capture := openTestWindow(t, 64, 64)
	takeError()
	camera := NewCamera3D(64, 64)
	camera.Position, camera.Target = Vector3{Y: 6, Z: -0.01}, Vector3{} // looking down
	ground, wall := NewPlaneMesh(), NewBoxMesh()
	look := func(decals ...Decal) (middle, side int) {
		picture := capture(func() {
			screen.Begin3D(camera)
			screen.SetEnvironment(Environment{NoBloom: true, NoShadows: true})
			screen.DrawMesh(ground, Transform{Scale: Vector3{X: 20, Z: 20}}, Gray)
			for _, d := range decals {
				screen.DrawDecal(d, Transform{Scale: Vector3{X: 2, Y: 1, Z: 2}})
			}
			screen.End3D()
		})
		brightness := func(x, y int) int {
			c := picture.NRGBAAt(x, y)
			return int(c.R) + int(c.G) + int(c.B)
		}
		return brightness(32, 32), brightness(4, 4)
	}
	plainMiddle, plainSide := look()
	darkMiddle, darkSide := look(Decal{Color: Black})
	glowMiddle, _ := look(Decal{Color: Orange, Glow: true, Brightness: 2})
	if darkMiddle > plainMiddle/4 {
		t.Errorf("under a black decal the ground is %d bright, and %d without: want darkened", darkMiddle, plainMiddle)
	}
	if darkSide != plainSide {
		t.Errorf("outside the decal's box the ground is %d bright, and %d without: want it as it is", darkSide, plainSide)
	}
	if glowMiddle < plainMiddle+60 {
		t.Errorf("under a glowing decal the ground is %d bright, and %d without: want brighter", glowMiddle, plainMiddle)
	}

	// A wall's face standing across the way the decal is pressed, seen
	// from the side, isn't smeared with it.
	camera.Position, camera.Target = Vector3{Z: -6, Y: 0.5}, Vector3{Y: 0.5}
	camera.Snap()
	picture := capture(func() {
		screen.Begin3D(camera)
		screen.SetEnvironment(Environment{NoBloom: true, NoShadows: true})
		screen.DrawMesh(wall, Transform{Position: Vector3{Y: 0.5}, Scale: Vector3{X: 2, Y: 1, Z: 0.2}}, Gray)
		screen.DrawDecal(Decal{Color: Black}, Transform{Position: Vector3{Y: 0.5}, Scale: Vector3{X: 2, Y: 2, Z: 2}})
		screen.End3D()
	})
	if c := picture.NRGBAAt(32, 32); int(c.R)+int(c.G)+int(c.B) < 150 {
		t.Errorf("a wall facing across the way a decal is pressed is %v, want it left as it was", c)
	}
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
}
