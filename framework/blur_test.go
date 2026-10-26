package golib

import "testing"

func TestBlurLevels(t *testing.T) {
	for blur, want := range map[float32]int{0: 0, -4: 0, 1: 1, 3: 1, 6: 2, 8: 2, 12: 3, 32: 4, 64: 5, 10000: maxBlurLevels} {
		if got := blurLevels(blur); got != want {
			t.Errorf("Blur %v softens by %d halvings, want %d", blur, got, want)
		}
	}
	if (Environment{}).uniforms().blur != 0 {
		t.Error("the zero Environment blurs")
	}
}

// A white square over a black 2D background: blurred, its edge spreads into
// the black and its middle stays white, and where the pass drew nothing the
// 2D under it still shows.
func TestBlurInAWindow(t *testing.T) {
	screen, capture := openTestWindow(t, 128, 128)
	takeError()
	camera := NewCamera3D(128, 128)
	camera.Position, camera.Target = Vector3{Z: -4}, Vector3{}
	box := NewBoxMesh()
	white := &Material{Color: White, Unlit: true}
	draw := func(blur float32) func(x, y int) int {
		picture := capture(func() {
			screen.Clear(Color{R: 0, G: 0, B: 120, A: 255})
			screen.Begin3D(camera)
			screen.SetEnvironment(Environment{Blur: blur, NoBloom: true, NoShadows: true})
			screen.DrawMeshWith(box, Transform{Scale: Vector3{X: 1.2, Y: 1.2, Z: 0.1}}, white)
			screen.End3D()
		})
		// The red at x, y; for x below 0, the blue at a corner, where the
		// pass drew nothing and the 2D shows.
		return func(x, y int) int {
			if x < 0 {
				return int(picture.NRGBAAt(2, 2).B)
			}
			return int(picture.NRGBAAt(x, y).R)
		}
	}
	sharp, soft := draw(0), draw(16)
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
	// Walk out from the middle along a row: find where the sharp square
	// ends, and look just past it.
	edge := 64
	for edge < 127 && sharp(edge, 64) > 128 {
		edge++
	}
	if edge == 127 || edge == 64 {
		t.Fatalf("the square's edge wasn't found: red %d at the middle", sharp(64, 64))
	}
	if sharp(edge+4, 64) > 10 {
		t.Errorf("without blur, 4 pixels past the edge is %d red, want black", sharp(edge+4, 64))
	}
	if got := soft(edge+4, 64); got < 30 {
		t.Errorf("with Blur 16, 4 pixels past the edge is %d red, want the square's light spread there", got)
	}
	if got := soft(edge-4, 64); got > 245 {
		t.Errorf("with Blur 16, 4 pixels inside the edge is %d red, want it softened", got)
	}
	if got := soft(64, 64); got < 200 {
		t.Errorf("with Blur 16, the square's middle is %d red, want it nearly white", got)
	}
	if got := soft(-1, 0); got < 110 {
		t.Errorf("with Blur 16, the 2D under an empty corner is %d blue, want its 120 still showing", got)
	}
}
