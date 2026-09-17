package golib

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestCameraStart(t *testing.T) {
	c := NewCamera(320, 180)
	if want := (Rectangle{Width: 320, Height: 180}); c.View() != want {
		t.Errorf("a new camera shows %v, want %v", c.View(), want)
	}
	if got := c.ToWorld(10, 20); got != (Vector2{X: 10, Y: 20}) {
		t.Errorf("ToWorld(10, 20) = %v on a new camera", got)
	}
	// Update without a new Target keeps the view.
	c.Update(1.0 / 60)
	if want := (Rectangle{Width: 320, Height: 180}); c.View() != want {
		t.Errorf("after an update, a new camera shows %v, want %v", c.View(), want)
	}
	if err := takeError(); err != nil {
		t.Fatal(err)
	}

	NewCamera(0, 180)
	wantError(t, "golib.NewCamera: the screen size is 0 by 180")
}

func TestCameraFollow(t *testing.T) {
	c := NewCamera(320, 180)
	c.Target = Vector2{X: 500, Y: 300}
	c.Update(1.0 / 60)
	if want := (Vector2{X: 500, Y: 300}); c.Center() != want {
		t.Errorf("without Lag, the view's center is %v, want %v", c.Center(), want)
	}
	if want := (Rectangle{X: 340, Y: 210, Width: 320, Height: 180}); c.View() != want {
		t.Errorf("View() = %v, want %v", c.View(), want)
	}

	// With Lag, the view closes about two thirds of the way in Lag seconds.
	c.Lag = 0.5
	c.Target = Vector2{X: 1500, Y: 300}
	for range 30 {
		c.Update(1.0 / 60)
	}
	if left, want := 1500-c.center.X, 1000/float32(math.E); math.Abs(float64(left-want)) > 1 {
		t.Errorf("after Lag seconds, %v of 1000 pixels are left, want %v", left, want)
	}
	c.Snap()
	if c.Center().X != 1500 {
		t.Errorf("after Snap, the view's center is %v, want x 1500", c.Center())
	}
}

func TestCameraBounds(t *testing.T) {
	c := NewCamera(320, 180)
	c.Bounds = Rectangle{X: 0, Y: 0, Width: 1000, Height: 150}
	tests := []struct {
		target Vector2
		want   Rectangle
	}{
		// The view stops at the left edge, and is centered on the bounds'
		// height, which is less than the screen's.
		{Vector2{X: 20, Y: 20}, Rectangle{X: 0, Y: -15, Width: 320, Height: 180}},
		{Vector2{X: 500, Y: 75}, Rectangle{X: 340, Y: -15, Width: 320, Height: 180}},
		{Vector2{X: 990, Y: 500}, Rectangle{X: 680, Y: -15, Width: 320, Height: 180}},
	}
	for _, test := range tests {
		c.Target = test.target
		c.Update(1.0 / 60)
		if c.View() != test.want {
			t.Errorf("following %v, the view is %v, want %v", test.target, c.View(), test.want)
		}
	}
	c.Target = Vector2{X: -500, Y: 0}
	c.Snap()
	if c.View().X != 0 {
		t.Errorf("after Snap outside the bounds, the view is %v, want it at x 0", c.View())
	}
}

func TestCameraZoom(t *testing.T) {
	c := NewCamera(320, 180)
	c.Zoom = 2
	c.Target = Vector2{X: 100, Y: 50}
	c.Update(1.0 / 60)
	if want := (Rectangle{X: 20, Y: 5, Width: 160, Height: 90}); c.View() != want {
		t.Errorf("at zoom 2, View() = %v, want %v", c.View(), want)
	}
	if got, want := c.ToWorld(0, 0), (Vector2{X: 20, Y: 5}); got != want {
		t.Errorf("ToWorld(0, 0) = %v, want %v", got, want)
	}
	if got, want := c.ToScreen(Vector2{X: 30, Y: 10}), (Vector2{X: 20, Y: 10}); got != want {
		t.Errorf("ToScreen(30, 10) = %v, want %v", got, want)
	}
	for _, point := range []Vector2{{X: 0, Y: 0}, {X: 17, Y: 3}, {X: 319, Y: 179}} {
		if got := c.ToScreen(c.ToWorld(point.X, point.Y)); !near(got, point) {
			t.Errorf("ToScreen(ToWorld(%v)) = %v", point, got)
		}
	}
}

func TestCameraWholePixels(t *testing.T) {
	tests := []struct {
		zoom, target, want float32
	}{
		{1, 100.3, 100},
		{1, 100.6, 101},
		{2, 100.3, 100.5}, // half a world pixel is a whole screen pixel
		{3, 100.2, 100 + 1.0/3},
	}
	for _, test := range tests {
		c := NewCamera(320, 180)
		c.Zoom = test.zoom
		c.Target = Vector2{X: test.target, Y: 90}
		c.Update(1.0 / 60)
		if got := c.Center().X; math.Abs(float64(got-test.want)) > 1e-4 {
			t.Errorf("zoom %v, target x %v: center x %v, want %v", test.zoom, test.target, got, test.want)
		}
		// The view's left edge is on a whole screen pixel.
		if left := c.View().X * test.zoom; math.Abs(float64(left)-math.Round(float64(left))) > 1e-3 {
			t.Errorf("zoom %v, target x %v: the view starts at screen pixel %v", test.zoom, test.target, left)
		}
	}
}

func TestCameraShake(t *testing.T) {
	positions := func() string {
		SetRandomSeed(7)
		c := NewCamera(320, 180)
		c.Target = Vector2{X: 160, Y: 90}
		c.Shake(4, 0.5)
		var trail strings.Builder
		for update := range 40 {
			c.Update(1.0 / 60)
			offset := c.Center().Sub(Vector2{X: 160, Y: 90})
			limit := max(0, 4*(0.5-float32(update+1)/60)/0.5)
			if math.Abs(float64(offset.X)) > float64(limit)+0.5 || math.Abs(float64(offset.Y)) > float64(limit)+0.5 {
				t.Errorf("update %d: the shake moved the view by %v, more than %v", update, offset, limit)
			}
			if update == 0 {
				// A weaker shake doesn't cut a strong one short.
				c.Shake(1, 5)
			}
			fmt.Fprint(&trail, offset, " ")
			if update >= 30 && offset != (Vector2{}) {
				t.Errorf("update %d: the shake still moves the view by %v after it ended", update, offset)
			}
		}
		return trail.String()
	}
	first := positions()
	if first != positions() {
		t.Error("the same random seed shook the view differently")
	}
	if !strings.Contains(first, "{1 ") && !strings.Contains(first, "{-1 ") && !strings.Contains(first, "{2 ") && !strings.Contains(first, "{-2 ") {
		t.Errorf("the view hardly moved: %s", first)
	}
}

func TestCameraInAWindow(t *testing.T) {
	var data strings.Builder
	data.WriteString(`<data encoding="csv">`)
	const columns, rows = 20, 10
	for i := range columns * rows {
		if i > 0 {
			data.WriteString(",")
		}
		fmt.Fprint(&data, 1+(i*7)%4)
	}
	data.WriteString("</data>")
	useAssets(t, map[string][]byte{
		"big.tmx": mapFile(columns, rows, data.String()),
		"t.png":   pngFile(t, 4, 4),
	})
	level := readMap(t, "big.tmx")
	screen, capture := openTestWindow(t, 16, 8)

	camera := NewCamera(16, 8)
	camera.Target = Vector2{X: 30.4, Y: 12}
	camera.Update(1.0 / 60) // the center is rounded to 30, 12
	hud := Rectangle{X: 1, Y: 1, Width: 3, Height: 2}
	thing := Rectangle{X: 31, Y: 13, Width: 2, Height: 2}
	through := capture(func() {
		screen.SetCamera(camera)
		screen.DrawMap(level, 0, 0)
		screen.DrawRectangle(thing, Red)
		screen.SetCamera(nil)
		screen.DrawRectangle(hud, Blue)
	})
	// Without a camera, the same picture takes offsets: the view starts at
	// 22, 8 in the world.
	by := capture(func() {
		screen.DrawMap(level, -22, -8)
		screen.DrawRectangle(Rectangle{X: 31 - 22, Y: 13 - 8, Width: 2, Height: 2}, Red)
		screen.DrawRectangle(hud, Blue)
	})
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
	for y := range 8 {
		for x := range 16 {
			if through.NRGBAAt(x, y) != by.NRGBAAt(x, y) {
				t.Fatalf("pixel %d, %d is %v through the camera, but %v with offsets", x, y, through.NRGBAAt(x, y), by.NRGBAAt(x, y))
			}
		}
	}
	if through.NRGBAAt(15, 7).A == 0 {
		t.Error("the map doesn't reach the screen's bottom-right corner: tiles in the camera's view were skipped")
	}

	// At zoom 2, a world pixel is 2 by 2 screen pixels.
	camera.Zoom = 2
	camera.Target = Vector2{X: 8, Y: 4}
	camera.Update(1.0 / 60)
	zoomed := capture(func() {
		screen.SetCamera(camera)
		screen.DrawRectangle(Rectangle{X: 10, Y: 5, Width: 2, Height: 1}, Red)
	})
	for _, p := range [][2]int{{12, 6}, {15, 7}} {
		if zoomed.NRGBAAt(p[0], p[1]).R != 230 {
			t.Errorf("at zoom 2, pixel %v = %v, want red", p, zoomed.NRGBAAt(p[0], p[1]))
		}
	}
	for _, p := range [][2]int{{11, 6}, {12, 5}} {
		if zoomed.NRGBAAt(p[0], p[1]).A != 0 {
			t.Errorf("at zoom 2, pixel %v = %v, want nothing", p, zoomed.NRGBAAt(p[0], p[1]))
		}
	}
	screen.endCamera()

	// A camera made for another screen size is a mistake.
	capture(func() { screen.SetCamera(NewCamera(320, 180)) })
	screen.endCamera()
	wantError(t, "golib: Screen.SetCamera got a camera for a 320 by 180 screen, but the screen is 16 by 8")
}
