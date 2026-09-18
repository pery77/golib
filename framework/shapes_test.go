package golib

import (
	"image"
	"testing"

	"golib/internal/device"
)

func TestShapesInAWindow(t *testing.T) {
	screen, capture := openTestWindow(t, 32, 32)
	takeError()
	painted := func(picture *image.NRGBA, x, y int) bool { return picture.NRGBAAt(x, y).A != 0 }
	check := func(name string, picture *image.NRGBA, filled, empty [][2]int) {
		t.Helper()
		for _, p := range filled {
			if !painted(picture, p[0], p[1]) {
				t.Errorf("%s: pixel %v is empty, want it drawn", name, p)
			}
		}
		for _, p := range empty {
			if painted(picture, p[0], p[1]) {
				t.Errorf("%s: pixel %v is drawn, want it empty", name, p)
			}
		}
	}

	outline := capture(func() { screen.DrawRectangleOutline(Rectangle{X: 2, Y: 4, Width: 10, Height: 8}, 2, Red) })
	check("rectangle outline", outline, [][2]int{{2, 4}, {3, 5}, {11, 11}, {6, 10}}, [][2]int{{4, 6}, {7, 8}, {12, 4}, {1, 4}})

	ring := capture(func() { screen.DrawCircleOutline(16, 16, 10, 2, Red) })
	check("circle outline", ring, [][2]int{{16, 7}, {7, 16}, {24, 16}}, [][2]int{{16, 16}, {16, 11}, {16, 3}})

	square := []Vector2{{X: 2, Y: 2}, {X: 12, Y: 2}, {X: 12, Y: 12}, {X: 2, Y: 12}}
	backwards := []Vector2{square[3], square[2], square[1], square[0]}
	for name, points := range map[string][]Vector2{"clockwise": square, "counterclockwise": backwards} {
		filled := capture(func() { screen.DrawPolygon(points, Red) })
		check("polygon, "+name, filled, [][2]int{{3, 3}, {7, 7}, {11, 11}, {11, 3}}, [][2]int{{13, 7}, {7, 13}})
	}
	// A star: every corner can be seen from its middle, but it isn't convex.
	star := make([]Vector2, 10)
	for i := range star {
		radius := float32(14)
		if i%2 == 1 {
			radius = 5
		}
		star[i] = Vector2FromAngle(float32(i)*36 - 90).Scale(radius).Add(Vector2{X: 16, Y: 16})
	}
	stars := capture(func() { screen.DrawPolygon(star, Red) })
	check("star", stars, [][2]int{{16, 16}, {16, 4}, {16, 19}}, [][2]int{{10, 8}, {22, 8}})
	check("no polygon from two points", capture(func() { screen.DrawPolygon(square[:2], Red) }), nil, [][2]int{{2, 2}, {7, 2}})

	triangle := []Vector2{{X: 4, Y: 28}, {X: 16, Y: 4}, {X: 28, Y: 28}}
	edges := capture(func() { screen.DrawPolygonOutline(triangle, 2, Red) })
	check("polygon outline", edges, [][2]int{{16, 27}, {10, 16}, {22, 16}}, [][2]int{{16, 20}})

	if err := takeError(); err != nil {
		t.Fatal(err)
	}
}

func TestTextAlignInAWindow(t *testing.T) {
	screen, capture := openTestWindow(t, 64, 32)
	takeError()
	const text = "AB\nAB"
	width := screen.TextWidth("AB", 10)
	left := capture(func() { screen.DrawText(text, 0, 4, 10, White) })
	for name, options := range map[string]TextOptions{
		"right":  {Align: AlignRight},
		"center": {Align: AlignCenter},
	} {
		x := width
		if options.Align == AlignCenter {
			x = width / 2
		}
		aligned := capture(func() { screen.DrawText(text, x, 4, 10, White, options) })
		for y := range 32 {
			for x := range 64 {
				if aligned.NRGBAAt(x, y) != left.NRGBAAt(x, y) {
					t.Fatalf("aligned %s, pixel %d, %d is %v, but %v aligned left: the lines don't match", name, x, y, aligned.NRGBAAt(x, y), left.NRGBAAt(x, y))
				}
			}
		}
	}
	// The second line is 12 pixels down: 10 for the letters and 2 between.
	first := firstPaintedRow(left, 0)
	if second := firstPaintedRow(left, first+10); second-first != 12 {
		t.Errorf("the lines start at rows %d and %d, want 12 apart", first, second)
	}
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
	capture(func() { screen.DrawText("x", 0, 0, 10, White, TextOptions{Align: 7}) })
	wantError(t, "golib: Screen.DrawText got TextOptions.Align 7")
}

// firstPaintedRow returns the first row at or below from with a drawn pixel.
func firstPaintedRow(picture *image.NRGBA, from int) int {
	for y := from; y < picture.Rect.Dy(); y++ {
		for x := range picture.Rect.Dx() {
			if picture.NRGBAAt(x, y).A != 0 {
				return y
			}
		}
	}
	return -1
}

func TestWithOpacity(t *testing.T) {
	tests := []struct {
		opacity float32
		want    uint8
	}{{0, 0}, {0.5, 128}, {1, 255}, {-1, 0}, {2, 255}}
	for _, test := range tests {
		got := WithOpacity(Maroon, test.opacity)
		if got.A != test.want || got.R != Maroon.R || got.G != Maroon.G || got.B != Maroon.B {
			t.Errorf("WithOpacity(Maroon, %v) = %v, want Maroon with A %d", test.opacity, got, test.want)
		}
	}
}

func TestMouseVisibleInAWindow(t *testing.T) {
	if !IsMouseVisible() {
		t.Fatal("the mouse pointer starts hidden")
	}
	openTestWindow(t, 16, 8)
	t.Cleanup(func() { SetMouseVisible(true) })
	var w window
	SetMouseVisible(false)
	if IsMouseVisible() {
		t.Error("IsMouseVisible is true after SetMouseVisible(false)")
	}
	w.apply()
	if device.CursorVisible() {
		t.Error("the pointer isn't hidden after the frame applies it")
	}
	SetMouseVisible(true)
	w.apply()
	if !device.CursorVisible() {
		t.Error("the pointer is still hidden after SetMouseVisible(true)")
	}
}

func TestBlendModeInAWindow(t *testing.T) {
	screen, capture := openTestWindow(t, 16, 8)
	takeError()
	glow := Color{R: 60, A: 255}
	left := Rectangle{X: 0, Y: 0, Width: 10, Height: 8}
	right := Rectangle{X: 6, Y: 0, Width: 10, Height: 8}

	added := capture(func() {
		screen.SetBlendMode(BlendAdd)
		screen.DrawRectangle(left, glow)
		screen.DrawRectangle(right, glow)
		screen.SetBlendMode(BlendNormal)
		screen.DrawRectangle(Rectangle{X: 0, Y: 6, Width: 16, Height: 2}, Color{G: 60, A: 255})
	})
	if got := added.NRGBAAt(8, 2).R; got != 120 {
		t.Errorf("where the two glows overlap, red is %d, want 120: they didn't add up", got)
	}
	if got := added.NRGBAAt(2, 2).R; got != 60 {
		t.Errorf("where one glow is, red is %d, want 60", got)
	}
	if got := added.NRGBAAt(8, 7); got.R != 0 || got.G != 60 {
		t.Errorf("after BlendNormal, the pixel is %v, want green over the glows", got)
	}

	// Every Draw starts over: Run resets the mode when a Draw leaves it set.
	screen.SetBlendMode(BlendAdd)
	screen.endDraw()
	covered := capture(func() {
		screen.DrawRectangle(left, glow)
		screen.DrawRectangle(right, glow)
	})
	if got := covered.NRGBAAt(8, 2).R; got != 60 {
		t.Errorf("after the draw ended, red where they overlap is %d, want 60: the blend mode stayed", got)
	}
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
	screen.SetBlendMode(9)
	wantError(t, "golib: Screen.SetBlendMode got 9")
}
