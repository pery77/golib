package golib

import (
	"image/color"
	"strings"
	"testing"
)

func TestImagePixels(t *testing.T) {
	img := NewImage(4, 3)
	if img.Width() != 4 || img.Height() != 3 || img.Pixel(0, 0) != (Color{}) {
		t.Fatalf("a new 4 by 3 picture: %d by %d, pixel 0, 0 = %v", img.Width(), img.Height(), img.Pixel(0, 0))
	}
	img.sharp.changed = false
	img.SetPixel(3, 2, Red)
	if img.Pixel(3, 2) != Red || !img.sharp.changed || !img.smooth.changed {
		t.Errorf("after SetPixel: %v, changed %v", img.Pixel(3, 2), img.sharp.changed)
	}
	// Setting a pixel to the color it has changes nothing to upload.
	img.sharp.changed = false
	img.SetPixel(3, 2, Red)
	if img.sharp.changed {
		t.Error("the same color again counts as a change")
	}
	// Outside the picture: nothing, and no panic.
	for _, at := range [][2]int{{-1, 0}, {4, 0}, {0, 3}, {0, -1}} {
		img.SetPixel(at[0], at[1], Green)
		if got := img.Pixel(at[0], at[1]); got != (Color{}) {
			t.Errorf("pixel %v outside the picture is %v", at, got)
		}
	}
	img.Clear(Blue)
	if img.Pixel(0, 0) != Blue || img.Pixel(3, 2) != Blue {
		t.Error("Clear left pixels")
	}
}

func TestImageSizes(t *testing.T) {
	for _, size := range [][2]int{{0, 4}, {4, -1}, {5000, 2}, {2, 4097}} {
		img := NewImage(size[0], size[1])
		img.SetPixel(1, 1, Red) // no panic
		img.Clear(Red)
		if _, err := img.upload(false); err == nil || !strings.Contains(err.Error(), "1 to 4096 pixels") {
			t.Errorf("NewImage(%d, %d): %v", size[0], size[1], err)
		}
	}
}

func TestImagesInAWindow(t *testing.T) {
	screen, capture := openTestWindow(t, 16, 8)
	img := NewImage(2, 2)
	img.SetPixel(0, 0, Red)
	img.SetPixel(1, 1, Green)
	picture := capture(func() { screen.DrawImage(img, 4, 2, DrawOptions{Scale: 2}) })
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
	nrgba := func(c Color) color.NRGBA { return color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A} }
	red, green := nrgba(Red), nrgba(Green)
	for _, test := range []struct {
		x, y int
		want color.NRGBA
	}{
		{4, 2, red}, {5, 3, red}, // each pixel 2 by 2, not smoothed
		{6, 4, green}, {7, 5, green},
		{6, 2, color.NRGBA{}}, {3, 2, color.NRGBA{}}, {8, 4, color.NRGBA{}},
	} {
		if got := picture.NRGBAAt(test.x, test.y); got != test.want {
			t.Errorf("pixel %d, %d = %v, want %v", test.x, test.y, got, test.want)
		}
	}

	// A change goes to the graphics card at the next draw.
	img.SetPixel(0, 0, Blue)
	picture = capture(func() { screen.DrawImage(img, 0, 0) })
	if got := picture.NRGBAAt(0, 0); got != nrgba(Blue) {
		t.Errorf("after changing it, pixel 0, 0 = %v, want blue", got)
	}
	screen.DrawImage(nil, 0, 0)
	wantError(t, "Screen.DrawImage got a nil picture")
}
