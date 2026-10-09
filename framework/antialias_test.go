package golib

import (
	"image"
	"os"
	"runtime"
	"testing"

	"golib/internal/device"
)

// drawFunc is a Game that only draws.
type drawFunc func(*Screen)

func (d drawFunc) Update(*Input, float32) {}
func (d drawFunc) Draw(s *Screen)         { d(s) }

// drawFrame draws one frame with config in a hidden window, the way golib
// shot does, through the renderer, and returns the picture at the screen's
// size. It skips the test when no window can be opened.
func drawFrame(t *testing.T, config Config, draw func(*Screen)) *image.NRGBA {
	t.Helper()
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Skip("no display to open a window on")
	}
	config.Title = t.Name()
	config, err := config.resolve()
	if err != nil {
		t.Fatal(err)
	}
	var (
		picture *image.NRGBA
		opened  error
	)
	onMainThread(func() {
		if opened = openWindow(config, true); opened != nil {
			return
		}
		defer device.CloseWindow()
		render := newRenderer(config)
		defer render.close()
		target := render.loadTarget()
		defer device.UnloadTarget(target)
		screen := &Screen{width: float32(config.Width), height: float32(config.Height)}
		if err = render.drawScene(drawFunc(draw), screen); err != nil {
			return
		}
		whole := device.Rectangle{Width: float32(config.Width), Height: float32(config.Height)}
		if err = render.present(&target, whole, 0); err != nil {
			return
		}
		picture = device.ReadTarget(target)
	})
	if opened != nil {
		t.Skip(opened)
	}
	if err != nil {
		t.Fatal(err)
	}
	return picture
}

// edgePixels counts the pixels of picture that are neither black nor white:
// blends at the edge of a white shape on black.
func edgePixels(picture *image.NRGBA) int {
	n := 0
	for y := range picture.Bounds().Dy() {
		for x := range picture.Bounds().Dx() {
			if r := picture.NRGBAAt(x, y).R; r != 0 && r != 255 {
				n++
			}
		}
	}
	return n
}

// With Antialias, a circle's edge blends into what is around it; without,
// every pixel is in or out.
func TestAntialiasSmoothsEdges(t *testing.T) {
	circle := func(s *Screen) {
		s.Clear(Black)
		s.DrawCircle(32, 32, 20.3, White)
		s.DrawLine(4, 60, 60, 50, 2, White)
	}
	stepped := drawFrame(t, Config{Width: 64, Height: 64}, circle)
	smooth := drawFrame(t, Config{Width: 64, Height: 64, Antialias: true}, circle)
	if n := edgePixels(stepped); n != 0 {
		t.Errorf("without Antialias %d pixels are blends, want none", n)
	}
	if n := edgePixels(smooth); n < 40 {
		t.Errorf("with Antialias %d pixels are blends, want the circle's and the line's edges", n)
	}
	for _, picture := range []*image.NRGBA{stepped, smooth} {
		if picture.NRGBAAt(32, 32).R != 255 || picture.NRGBAAt(1, 1).R != 0 {
			t.Error("the circle's middle isn't white, or the corner isn't black")
		}
	}
}

// Antialias changes edges, not places: a rectangle on whole pixels keeps
// sharp edges, and a camera shows the world where it does without it.
func TestAntialiasKeepsPlaces(t *testing.T) {
	camera := NewCamera(64, 64)
	camera.Target = Vector2{X: 100, Y: 100}
	camera.Zoom = 2
	camera.Snap()
	draw := func(s *Screen) {
		s.Clear(Black)
		s.DrawRectangle(Rectangle{X: 10, Y: 10, Width: 12, Height: 8}, White)
		s.SetCamera(camera)
		s.DrawRectangle(Rectangle{X: 100, Y: 100, Width: 5, Height: 5}, Red) // 32 to 42 on the screen
	}
	for _, antialias := range []bool{false, true} {
		picture := drawFrame(t, Config{Width: 64, Height: 64, Antialias: antialias}, draw)
		for _, p := range []struct {
			x, y int
			want Color
		}{
			{10, 10, White}, {21, 17, White}, {9, 10, Black}, {22, 10, Black}, {10, 18, Black},
			{32, 32, Red}, {41, 41, Red}, {31, 32, Black}, {42, 41, Black},
		} {
			got := picture.NRGBAAt(p.x, p.y)
			if got.R != p.want.R || got.G != p.want.G || got.B != p.want.B {
				t.Errorf("Antialias %v: pixel %d, %d is %v, want %v", antialias, p.x, p.y, got, p.want)
			}
		}
	}
}

// With Antialias, text in a font from a file is drawn from letters made twice
// as large, and measures as it does without it.
func TestAntialiasDrawsTextTwiceAsLarge(t *testing.T) {
	useAssets(t, map[string][]byte{"test.ttf": testFont()})
	font := NewFont("test.ttf")
	options := TextOptions{Font: font}
	var (
		widths []float32
		at20   []bool // whether the frame made the letters at 20 pixels
	)
	for _, antialias := range []bool{false, true} {
		drawFrame(t, Config{Width: 64, Height: 32, Antialias: antialias}, func(s *Screen) {
			s.DrawText("AB", 2, 2, 10, White, options)
			widths = append(widths, s.TextWidth("AB", 10, options))
			at20 = append(at20, font.sizes[20] != nil)
		})
	}
	if widths[0] != 13 || widths[1] != 13 {
		t.Errorf("TextWidth(AB, 10) is %v without Antialias and %v with it, want 13 both times", widths[0], widths[1])
	}
	if at20[0] || !at20[1] {
		t.Errorf("letters made at 20 for text at 10: %v without Antialias, %v with it; want only with it", at20[0], at20[1])
	}
}
