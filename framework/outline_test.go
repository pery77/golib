package golib

import "testing"

// A toon material is lit in flat bands; an outline draws a line round a
// mesh, just outside it.
func TestToonAndOutlineInAWindow(t *testing.T) {
	screen, capture := openTestWindow(t, 64, 64)
	takeError()
	camera := NewCamera3D(64, 64)
	camera.Position, camera.Target = Vector3{Z: -4}, Vector3{}
	ball := NewSphereMesh()
	look := func(m *Material) (third, half, outside int) {
		picture := capture(func() {
			screen.Clear(White)
			screen.Begin3D(camera)
			screen.SetEnvironment(Environment{NoBloom: true, NoShadows: true, SunDirection: Vector3{Z: 1}, Sky: Black, Ground: Black})
			screen.DrawMeshWith(ball, Transform{Scale: Vector3{X: 2, Y: 2, Z: 2}}, m)
			screen.End3D()
		})
		b := func(x, y int) int {
			c := picture.NRGBAAt(x, y)
			return int(c.R) + int(c.G) + int(c.B)
		}
		// The ball, 2 meters across 4 meters away, seen 60 degrees from the
		// bottom of 64 pixels to the top, is 28 pixels across: a point a
		// third of the way out, past the highlight in its middle, one half
		// way out, and one 2 pixels past its edge.
		return b(32+5, 32), b(32+8, 32), b(32+16, 32)
	}
	smoothThird, smoothHalf, _ := look(&Material{Color: Gray})
	toonThird, toonHalf, _ := look(&Material{Color: Gray, Toon: 2})
	if smoothThird-smoothHalf < 6 {
		t.Fatalf("a smooth ball is %d bright a third of the way out and %d half way: the test needs them to differ", smoothThird, smoothHalf)
	}
	if d := toonThird - toonHalf; d < -12 || d > 12 || d*3 > smoothThird-smoothHalf {
		t.Errorf("a toon ball is %d bright a third of the way out and %d half way, and a smooth one %d and %d: want one flat band", toonThird, toonHalf, smoothThird, smoothHalf)
	}
	_, _, plainOutside := look(&Material{Color: Gray})
	_, _, lined := look(&Material{Color: Gray, Outline: Black, OutlineWidth: 4})
	if plainOutside < 700 || lined > 300 {
		t.Errorf("just past the ball's edge is %d bright without an outline and %d with a black one, want white, then black", plainOutside, lined)
	}
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
}
