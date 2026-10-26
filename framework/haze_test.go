package golib

import "testing"

// Haze glows where the sun reaches it and stays dark in a roof's shadow,
// so light shows as shafts; a spotlight shows as a beam; and no haze
// leaves the air clear.
func TestHazeInAWindow(t *testing.T) {
	screen, capture := openTestWindow(t, 64, 64)
	takeError()
	camera := NewCamera3D(64, 64)
	camera.Position, camera.Target = Vector3{Y: 1, Z: -5}, Vector3{Y: 1}
	camera.Snap()
	box := NewBoxMesh()
	bright := func(env Environment, lamp *Light, x, y int) int {
		c := capture(func() {
			screen.Clear(Black)
			screen.Begin3D(camera)
			screen.SetEnvironment(env)
			if lamp != nil {
				screen.AddLight(*lamp)
			}
			// A roof over the right half of the view, seen from below.
			screen.DrawMesh(box, Transform{Position: Vector3{X: -5, Y: 4, Z: 10}, Scale: Vector3{X: 10, Y: 0.2, Z: 40}}, White)
			screen.End3D()
		}).NRGBAAt(x, y)
		return int(c.R) + int(c.G) + int(c.B)
	}
	sunny := Environment{NoBloom: true, SunDirection: Vector3{Y: -1}, Sky: Black, Ground: Black}
	if clear := bright(sunny, nil, 16, 34); clear > 6 {
		t.Errorf("air without haze is %d bright, want black", clear)
	}
	hazy := sunny
	hazy.Haze = 0.2
	lit, shaded := bright(hazy, nil, 16, 34), bright(hazy, nil, 48, 34)
	if lit < 150 || shaded*3 > lit {
		t.Errorf("haze in the sun is %d bright and under the roof %d, want it bright in the sun and dark in the shadow", lit, shaded)
	}

	// At night, a spotlight shining across the view lights the haze in
	// its beam, and none below it.
	night := Environment{NoBloom: true, SunColor: Black, Sky: Black, Ground: Black, Haze: 0.2}
	spot := &Light{Position: Vector3{X: 4, Y: 1, Z: 2}, Direction: Vector3{X: -1}, Cone: 10, Intensity: 30, Range: 12}
	beam, below := bright(night, spot, 32, 32), bright(night, spot, 32, 56)
	if beam < 60 || below*3 > beam {
		t.Errorf("haze in a spotlight's beam is %d bright and below it %d, want the beam to show", beam, below)
	}
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
}
