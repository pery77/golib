package golib

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"testing"
)

// radianceFile writes a Radiance file width by height, every pixel the RGBE
// bytes given, with run-length rows when rle is true.
func radianceFile(width, height int, rgbe [4]byte, rle bool) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "#?RADIANCE\nFORMAT=32-bit_rle_rgbe\n\n-Y %d +X %d\n", height, width)
	for y := 0; y < height; y++ {
		if !rle {
			for x := 0; x < width; x++ {
				b.Write(rgbe[:])
			}
			continue
		}
		b.Write([]byte{2, 2, byte(width >> 8), byte(width)})
		for channel := 0; channel < 4; channel++ {
			for left := width; left > 0; {
				run := min(left, 127)
				b.Write([]byte{byte(128 + run), rgbe[channel]})
				left -= run
			}
		}
	}
	return b.Bytes()
}

func TestDecodeRadiance(t *testing.T) {
	// 128, 64, 0 with exponent 129 is 1, 0.5 and 0, give or take half a step.
	for _, rle := range []bool{false, true} {
		pixels, width, height, err := decodeRadiance(radianceFile(16, 4, [4]byte{128, 64, 0, 129}, rle))
		if err != nil {
			t.Fatalf("rle %v: %v", rle, err)
		}
		if width != 16 || height != 4 || len(pixels) != 16*4*3 {
			t.Fatalf("rle %v: %d by %d, %d numbers", rle, width, height, len(pixels))
		}
		if r, g := pixels[0], pixels[1]; math.Abs(float64(r-1)) > 0.01 || math.Abs(float64(g-0.5)) > 0.01 {
			t.Errorf("rle %v: a pixel read as %v, %v, want 1, 0.5", rle, r, g)
		}
	}
	for _, c := range []struct{ file, want string }{
		{"P6 2 2 255", "not a Radiance"},
		{"#?RADIANCE\nFORMAT=32-bit_rle_xyze\n\n-Y 1 +X 1\n", "32-bit_rle_rgbe"},
		{"#?RADIANCE\n\n+Y 1 +X 1\n", "-Y height +X width"},
		{"#?RADIANCE\n\n-Y 2 +X 2\n\x01\x02", "ends early"},
	} {
		if _, _, _, err := decodeRadiance([]byte(c.file)); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("decoding %q: %v, want it to say %q", c.file, err, c.want)
		}
	}
}

func TestToHalf(t *testing.T) {
	for _, c := range []struct {
		f    float32
		want uint16
	}{
		{0, 0}, {1, 0x3c00}, {-2, 0xc000}, {0.5, 0x3800}, {65504, 0x7bff}, {1e9, 0x7bff}, {6e-8, 1},
	} {
		if got := toHalf(c.f); got != c.want {
			t.Errorf("toHalf(%v) = %#04x, want %#04x", c.f, got, c.want)
		}
	}
}

func TestHDRILightsEvenly(t *testing.T) {
	useAssets(t, map[string][]byte{"skies/white.hdr": radianceFile(64, 32, [4]byte{128, 128, 128, 129}, true)})
	sky := NewHDRI("skies/white.hdr")
	if err := sky.Err(); err != nil {
		t.Fatal(err)
	}
	// A sky as bright as 1 all around lights a surface facing any way with
	// 1: the spherical harmonics' sum, the lit shader's surrounding().
	for _, n := range []Vector3{{Y: 1}, {Y: -1}, {X: 1}, {Z: -1}, Vector3{X: 1, Y: 1, Z: 1}.Normalize()} {
		x, y, z := float64(n.X), float64(n.Y), float64(n.Z)
		c := sky.irradiance
		sum := float64(c[0][0])*0.282095 + float64(c[1][0])*0.488603*y + float64(c[2][0])*0.488603*z + float64(c[3][0])*0.488603*x +
			float64(c[4][0])*1.092548*x*y + float64(c[5][0])*1.092548*y*z + float64(c[6][0])*0.315392*(3*z*z-1) +
			float64(c[7][0])*1.092548*x*z + float64(c[8][0])*0.546274*(x*x-y*y)
		if math.Abs(sum-1) > 0.02 {
			t.Errorf("a white sky lights a surface facing %v with %v, want 1", n, sum)
		}
	}
	if len(sky.levels) != 7 || len(sky.levels[6]) != 4 {
		t.Errorf("%d mipmaps, the last %d halves, want 64x32 down to 1x1: 7, the last one texel", len(sky.levels), len(sky.levels[len(sky.levels)-1]))
	}
	if d := panoramaDirection(0.5, 0.5); !near3(d, Vector3{Z: 1}) {
		t.Errorf("the middle of a panorama faces %v, want +Z", d)
	}
	if d := panoramaDirection(0.75, 0.5); !near3(d, Vector3{X: -1}) {
		t.Errorf("right of the middle faces %v, want -X, the right of someone looking along +Z", d)
	}
	if err := NewHDRI("skies/missing.hdr").Err(); err == nil || !strings.Contains(err.Error(), `golib.NewHDRI("skies/missing.hdr")`) {
		t.Errorf("a missing HDRI reported %v", err)
	}
}
