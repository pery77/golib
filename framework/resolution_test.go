package golib

import "testing"

func TestResolutionShares(t *testing.T) {
	for resolution, want := range map[float32]float32{0: 1, -1: 1, 0.75: 0.75, 0.1: 0.25, 2: 1} {
		if got := (Environment{Resolution: resolution}).uniforms().resolution; got != want {
			t.Errorf("Resolution %v draws at %v of the screen's pixels across, want %v", resolution, got, want)
		}
	}
	// Blur is measured in the screen's pixels: at half the resolution, half
	// as many of the pass's.
	if full, half := (Environment{Blur: 16}).uniforms().blur, (Environment{Blur: 16, Resolution: 0.5}).uniforms().blur; half != full-1 {
		t.Errorf("Blur 16 softens by %d halvings at full resolution and %d at half, want one fewer", full, half)
	}
}

// A white square over a blue 2D background, drawn at a quarter of the
// screen's resolution, is enlarged to the same place on the screen: its
// middle white, its edge where it was, the 2D still showing round it.
func TestResolutionInAWindow(t *testing.T) {
	screen, capture := openTestWindow(t, 128, 128)
	takeError()
	camera := NewCamera3D(128, 128)
	camera.Position, camera.Target = Vector3{Z: -4}, Vector3{}
	box := NewBoxMesh()
	white := &Material{Color: White, Unlit: true}
	edgeAt := func(resolution float32) (edge, middle, corner int) {
		picture := capture(func() {
			screen.Clear(Color{R: 0, G: 0, B: 120, A: 255})
			screen.Begin3D(camera)
			screen.SetEnvironment(Environment{Resolution: resolution, NoBloom: true, NoShadows: true})
			screen.DrawMeshWith(box, Transform{Scale: Vector3{X: 1.2, Y: 1.2, Z: 0.1}}, white)
			screen.End3D()
		})
		edge = 64
		for edge < 127 && picture.NRGBAAt(edge, 64).R > 128 {
			edge++
		}
		return edge, int(picture.NRGBAAt(64, 64).R), int(picture.NRGBAAt(2, 2).B)
	}
	fullEdge, _, _ := edgeAt(1)
	edge, middle, corner := edgeAt(0.25)
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
	if middle < 225 {
		t.Errorf("at a quarter of the resolution the square's middle is %d red, want white", middle)
	}
	if edge < fullEdge-3 || edge > fullEdge+3 {
		t.Errorf("at a quarter of the resolution the square ends at x %d, at full resolution at %d: want the same place", edge, fullEdge)
	}
	if corner < 110 {
		t.Errorf("at a quarter of the resolution the corner is %d blue, want the 2D under the pass", corner)
	}
}
