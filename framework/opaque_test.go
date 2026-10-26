package golib

import "testing"

// What reaches the window is opaque, however see-through what the game
// drew: a shade over the screen, the edge of a glow, a pass over the 2D.
// On macOS a window shows the desktop where it isn't.
func TestThePresentedScreenIsOpaque(t *testing.T) {
	picture := drawFrame(t, Config{Width: 64, Height: 48}, func(screen *Screen) {
		screen.Clear(Color{R: 20, G: 120, B: 60, A: 255})
		screen.DrawRectangle(Rectangle{Width: 64, Height: 48}, Color{R: 10, G: 10, B: 40, A: 128})
		screen.DrawCircle(32, 24, 10, Color{R: 255, G: 200, B: 80, A: 60})
	})
	for y := range 48 {
		for x := range 64 {
			if c := picture.NRGBAAt(x, y); c.A != 255 {
				t.Fatalf("pixel %d, %d is %v: opacity %d, want 255", x, y, c, c.A)
			}
		}
	}
	// And the colors are as blended, not changed by making it opaque.
	if c := picture.NRGBAAt(2, 2); c.G < 50 || c.G > 80 {
		t.Errorf("the shaded corner is %v, want the green half under the shade", c)
	}
}
