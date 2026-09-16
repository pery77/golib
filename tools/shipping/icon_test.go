package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writePNG saves img as a PNG file in a new folder and returns its path.
func writePNG(t *testing.T, img image.Image) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), iconFile)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// checkerboard returns a size by size image of black and white pixels.
func checkerboard(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			if (x+y)%2 == 0 {
				img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
			} else {
				img.SetNRGBA(x, y, color.NRGBA{A: 255})
			}
		}
	}
	return img
}

func TestReadIcon(t *testing.T) {
	if _, found, err := readIcon(filepath.Join(t.TempDir(), iconFile)); found || err != nil {
		t.Errorf("without icon.png: found = %v, error = %v; want false and no error", found, err)
	}

	// Images that don't start at 0, 0 and aren't NRGBA come out as both.
	gray := image.NewGray(image.Rect(10, 10, 42, 42))
	gray.SetGray(10, 10, color.Gray{Y: 200})
	icon, found, err := readIcon(writePNG(t, gray))
	if err != nil || !found {
		t.Fatalf("a 32 by 32 gray PNG: found = %v, error = %v", found, err)
	}
	if icon.Bounds() != image.Rect(0, 0, 32, 32) {
		t.Errorf("bounds = %v, want 0, 0 to 32, 32", icon.Bounds())
	}
	if got := icon.NRGBAAt(0, 0); got != (color.NRGBA{R: 200, G: 200, B: 200, A: 255}) {
		t.Errorf("top-left pixel = %v, want the gray one", got)
	}

	for _, tt := range []struct {
		name string
		img  image.Image
		want string
	}{
		{"not square", image.NewNRGBA(image.Rect(0, 0, 64, 32)), "64 by 32 pixels, but an icon must be square"},
		{"too small", image.NewNRGBA(image.Rect(0, 0, 8, 8)), "needs at least 16 by 16"},
	} {
		if _, _, err := readIcon(writePNG(t, tt.img)); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: error = %v, want one containing %q", tt.name, err, tt.want)
		}
	}

	path := filepath.Join(t.TempDir(), iconFile)
	if err := os.WriteFile(path, []byte("GIF89a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readIcon(path); err == nil || !strings.Contains(err.Error(), "not a PNG image") {
		t.Errorf("a GIF named icon.png: error = %v, want one saying it isn't a PNG", err)
	}
}

func TestResizeGrowsBySharpPixels(t *testing.T) {
	src := checkerboard(16)
	dst := resize(src, 64)
	for y := range 64 {
		for x := range 64 {
			if got, want := dst.NRGBAAt(x, y), src.NRGBAAt(x/4, y/4); got != want {
				t.Fatalf("pixel %d, %d = %v, want %v, the source pixel it repeats", x, y, got, want)
			}
		}
	}
}

func TestResizeShrinksByAveraging(t *testing.T) {
	dst := resize(checkerboard(64), 16)
	for y := range 16 {
		for x := range 16 {
			if got, want := dst.NRGBAAt(x, y), (color.NRGBA{R: 128, G: 128, B: 128, A: 255}); got != want {
				t.Fatalf("pixel %d, %d = %v, want %v, half black and half white", x, y, got, want)
			}
		}
	}

	// A red pixel next to a transparent one stays red, half transparent.
	src := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := 0; x < 32; x += 2 {
			src.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	if got, want := resize(src, 16).NRGBAAt(3, 3), (color.NRGBA{R: 255, A: 128}); got != want {
		t.Errorf("red beside transparent = %v, want %v", got, want)
	}

	// Sizes that don't divide evenly cover every source pixel exactly once.
	for _, from := range []int{256, 100, 37} {
		for _, size := range iconSizes {
			if size >= from {
				continue
			}
			total := 0.0
			for _, shares := range coverage(from, size) {
				for _, s := range shares {
					total += s.share
				}
			}
			if total < float64(size)-1e-9 || total > float64(size)+1e-9 {
				t.Errorf("coverage(%d, %d) shares add up to %v, want %d", from, size, total, size)
			}
		}
	}
}

func TestIconBitmap(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 20, 20))
	img.SetNRGBA(0, 0, color.NRGBA{R: 1, G: 2, B: 3, A: 255}) // top left: opaque
	data := iconBitmap(img)

	maskRow := 4 // 20 pixels fit in 32 bits
	if want := 40 + 20*20*4 + maskRow*20; len(data) != want {
		t.Fatalf("len = %d, want %d", len(data), want)
	}
	if le.Uint32(data[4:]) != 20 || le.Uint32(data[8:]) != 40 || le.Uint16(data[14:]) != 32 {
		t.Errorf("header = % x, want width 20, height 40 (colors and mask) and 32 bits", data[:40])
	}
	// Rows go bottom up, so the top row is the last one, in BGRA order.
	top := 40 + 19*20*4
	if got := data[top : top+4]; !bytes.Equal(got, []byte{3, 2, 1, 255}) {
		t.Errorf("top-left pixel = % x, want 03 02 01 ff", got)
	}
	// In the mask, transparent pixels are 1 bits; the top row comes last.
	mask := 40 + 20*20*4
	if got := data[mask : mask+maskRow]; !bytes.Equal(got, []byte{0xff, 0xff, 0xf0, 0}) {
		t.Errorf("bottom mask row = % x, want 20 transparent pixels: ff ff f0 00", got)
	}
	topMask := mask + 19*maskRow
	if got := data[topMask : topMask+maskRow]; !bytes.Equal(got, []byte{0x7f, 0xff, 0xf0, 0}) {
		t.Errorf("top mask row = % x, want the first pixel opaque: 7f ff f0 00", got)
	}
}

func TestIconResources(t *testing.T) {
	resources := iconResources(checkerboard(256))
	if len(resources) != len(iconSizes)+1 {
		t.Fatalf("got %d resources, want one per size and a group", len(resources))
	}
	group := resources[len(resources)-1]
	if group.kind != rtGroupIcon || group.name != iconGroupName {
		t.Fatalf("last resource = type %d named %q, want the icon group named %s", group.kind, group.name, iconGroupName)
	}
	if le.Uint16(group.data[2:]) != 1 || int(le.Uint16(group.data[4:])) != len(iconSizes) {
		t.Errorf("group header = % x, want type 1 and %d images", group.data[:6], len(iconSizes))
	}
	for i, size := range iconSizes {
		entry := group.data[6+14*i : 6+14*(i+1)]
		icon := resources[i]
		if icon.kind != rtIcon || le.Uint16(entry[12:]) != icon.id {
			t.Errorf("entry %d points at icon %d, but resource %d is type %d, number %d", i, le.Uint16(entry[12:]), i, icon.kind, icon.id)
		}
		if width := int(entry[0]); width != size%256 || entry[1] != entry[0] {
			t.Errorf("entry %d is %d by %d, want %d (0 for 256)", i, entry[0], entry[1], size%256)
		}
		if int(le.Uint32(entry[8:])) != len(icon.data) {
			t.Errorf("entry %d says %d bytes, but the image has %d", i, le.Uint32(entry[8:]), len(icon.data))
		}
		isPNG := bytes.HasPrefix(icon.data, []byte("\x89PNG"))
		if isPNG != (size == 256) {
			t.Errorf("the %d-pixel image is PNG: %v, want PNG only for 256", size, isPNG)
		}
	}
	decoded, err := png.Decode(bytes.NewReader(resources[len(iconSizes)-1].data))
	if err != nil || decoded.Bounds().Dx() != 256 {
		t.Errorf("the 256-pixel PNG decodes to %v, error %v", decoded.Bounds(), err)
	}
}
