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
func openTestWindow(t *testing.T, width, height int) (*Screen, func(draw func()) *image.NRGBA) {
	t.Helper()
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Skip("no display to open a window on")
	}
	// OpenGL draws from the thread that opened the window, and tests run on
	// any thread: keep this one until the test ends.
	runtime.LockOSThread()
	config := Config{Title: t.Name(), Width: width, Height: height}
	if err := openWindow(config, true); err != nil {
		runtime.UnlockOSThread()
		t.Skip(err)
	}
	render := newRenderer(config)
	t.Cleanup(func() {
		render.close()
		device.CloseWindow()
		runtime.UnlockOSThread()
	})
	screen := &Screen{width: float32(width), height: float32(height)}
	capture := func(draw func()) *image.NRGBA {
		device.BeginTarget(render.scene)
		screen.Clear(Blank)
		draw()
		device.EndTarget()
		return device.ReadTarget(render.scene)
	}
	return screen, capture
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
