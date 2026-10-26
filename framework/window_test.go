package golib

import (
	"image"
	"image/color"
	"os"
	"runtime"
	"testing"

	"golib/internal/device"
)

// openTestWindow opens a hidden window of width by height pixels for the rest
// of the test, and returns a screen to draw on and a function that returns
// what was drawn. It skips the test when no window can be opened.
//
// The window is opened, drawn in and closed on the main thread (see
// onMainThread), which macOS requires and OpenGL, drawing from the thread
// that opened the window, needs anyway.
func openTestWindow(t *testing.T, width, height int) (*Screen, func(draw func()) *image.NRGBA) {
	t.Helper()
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Skip("no display to open a window on")
	}
	config := Config{Title: t.Name(), Width: width, Height: height}
	var render *renderer
	var err error
	onMainThread(func() {
		if err = openWindow(config, true); err == nil {
			render = newRenderer(config)
		}
	})
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() {
		onMainThread(func() {
			render.close()
			device.CloseWindow()
		})
	})
	screen := &Screen{width: float32(width), height: float32(height)}
	capture := func(draw func()) *image.NRGBA {
		var picture *image.NRGBA
		onMainThread(func() {
			device.BeginTarget(render.scene)
			screen.Clear(Blank)
			draw()
			device.EndTarget()
			picture = device.ReadTarget(render.scene)
		})
		return picture
	}
	return screen, capture
}

func TestSmoothSpritesInAWindow(t *testing.T) {
	screen, capture := openTestWindow(t, 16, 4)
	sprite := newGeneratedSprite("smooth sampling test", func(s *Sprite) error {
		s.pixels = image.NewNRGBA(image.Rect(0, 0, 2, 1))
		s.pixels.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
		s.pixels.SetNRGBA(1, 0, color.NRGBA{B: 255, A: 255})
		s.width, s.height = 2, 1
		s.frames = []image.Rectangle{s.pixels.Rect}
		return nil
	})
	picture := capture(func() {
		screen.Clear(Black)
		screen.DrawSprite(sprite, 0, 0, 0, DrawOptions{Scale: 4, Smooth: true})
		screen.DrawRectangle(Rectangle{Width: 2, Height: 4}, Green)
		screen.DrawSprite(sprite, 0, 8, 0, DrawOptions{Scale: 4})
	})
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
	if p := picture.NRGBAAt(3, 2); p.R == 0 || p.R == 255 || p.B == 0 || p.B == 255 {
		t.Errorf("smooth sampling has no intermediate colors: %v", p)
	}
	if p := picture.NRGBAAt(0, 2); p.R != Green.R || p.G != Green.G || p.B != Green.B {
		t.Errorf("a later rectangle did not cover the smooth sprite: %v", p)
	}
	for x, want := range map[int]color.NRGBA{11: {R: 255, A: 255}, 12: {B: 255, A: 255}} {
		if got := picture.NRGBAAt(x, 2); got != want {
			t.Errorf("smooth drawing changed sharp sampling at %d: got %v, want %v", x, got, want)
		}
	}
}

func TestSpritesInAWindow(t *testing.T) {
	useAssets(t, map[string][]byte{"dots.png": pngFile(t, 4, 2)})
	screen, capture := openTestWindow(t, 16, 8)
	dots := NewSpriteSheet("dots.png", 2, 2)
	picture := capture(func() {
		screen.DrawSprite(dots, 1, 3.5, 1.4)                                  // rounded to 4, 1
		screen.DrawSprite(dots, 0, 10, 4, DrawOptions{FlipX: true, Scale: 2}) // 4 by 4, mirrored
	})
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
	// pngFile gives pixel x, y red x and green y; frame 1 starts at x 2.
	tests := []struct {
		x, y int
		want color.NRGBA
	}{
		{4, 1, color.NRGBA{R: 2, G: 0, A: 255}},
		{5, 1, color.NRGBA{R: 3, G: 0, A: 255}},
		{5, 2, color.NRGBA{R: 3, G: 1, A: 255}},
		{3, 1, color.NRGBA{}},
		{6, 1, color.NRGBA{}},
		{4, 3, color.NRGBA{}},
		{10, 4, color.NRGBA{R: 1, G: 0, A: 255}},
		{11, 5, color.NRGBA{R: 1, G: 0, A: 255}},
		{12, 4, color.NRGBA{R: 0, G: 0, A: 255}},
		{13, 7, color.NRGBA{R: 0, G: 1, A: 255}},
	}
	for _, test := range tests {
		if got := picture.NRGBAAt(test.x, test.y); got != test.want {
			t.Errorf("pixel %d, %d = %v, want %v", test.x, test.y, got, test.want)
		}
	}
}
