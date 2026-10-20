//go:build ignore

// lakesky makes the game's sky, assets/skies/lake_sunset.hdr, from Poly
// Haven's Venice Sunset (CC0), sources/skies/venice_sunset_1k.hdr. The
// panorama was taken on a waterfront: away from the sea it shows houses,
// trees and paving close by, which GoLib would show behind the lagoon and
// reflect in its water, since GoLib shows an HDRI whole and can't hide its
// ground. This program keeps the sky and the open sea
// towards the sunset, paints the houses and trees out with the sky beside
// them, and replaces everything under the horizon with the sky mirrored and
// darkened, as calm water reflects it, and dims the sun's disc. Run it from the project root:
//
//	./golib go run games/sunset/sources/skies/lakesky.go
package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	source = "games/sunset/sources/skies/venice_sunset_1k.hdr"
	target = "games/sunset/assets/skies/lake_sunset.hdr"

	// The open sea, from the left edge of the panorama's width to the
	// right, kept as it is: the sunset, its sky and the far skyline.
	seaFrom, seaTo = 0.315, 0.735
	edge           = 0.04 // how far the kept part fades into the painted one

	// Rows, as a share of the height from the top: houses and trees reach
	// up to paintTop; above fadeTop the sky is the panorama's own.
	paintTop, fadeTop = 0.30, 0.20

	// reflect is how much of the sky calm water sends back, under the horizon.
	reflect = 0.22

	// sunCap is the brightest the sun's disc is kept. In the photograph it
	// is thousands of times brighter than the sky; the game's own sun,
	// pointed the same way, lights the world and casts the shadows, so the
	// disc is only seen, as a soft glow.
	sunCap = 24
)

func main() {
	data, err := os.ReadFile(source)
	if err != nil {
		log.Fatal(err)
	}
	pixels, width, height, err := decode(data)
	if err != nil {
		log.Fatal(err)
	}
	at := func(x, y int) []float32 { return pixels[(y*width+x)*3 : (y*width+x)*3+3] }

	// The sky beside the houses: for each row, the average of a few columns
	// just inside each edge of the sea. The painted part blends from one
	// edge's sky to the other's, round the back of the panorama.
	reference := func(from, to float32) [][3]float32 {
		rows := make([][3]float32, height)
		for y := range height {
			count := 0
			for x := range width {
				if u := (float32(x) + 0.5) / float32(width); u > from && u < to {
					for c := range 3 {
						rows[y][c] += at(x, y)[c]
					}
					count++
				}
			}
			for c := range 3 {
				rows[y][c] /= float32(count)
			}
		}
		return rows
	}
	left := reference(seaFrom+edge, seaFrom+edge+0.02)
	right := reference(seaTo-edge-0.02, seaTo-edge)
	horizon := height / 2
	for y := range horizon {
		v := (float32(y) + 0.5) / float32(height)
		down := smooth(fadeTop, paintTop, v) // 0 high in the sky, 1 where the houses are
		for x := range width {
			u := (float32(x) + 0.5) / float32(width)
			inSea := smooth(seaFrom, seaFrom+edge, u) * (1 - smooth(seaTo-edge, seaTo, u))
			paint := down * (1 - inSea)
			// How far round the back from the right edge of the sea to the left.
			around := max(0, u-seaTo)
			if u < 0.5 {
				around = u + 1 - seaTo
			}
			t := smooth(0, 1, around/(1-seaTo+seaFrom))
			p := at(x, y)
			for c := range 3 {
				sky := right[y][c]*(1-t) + left[y][c]*t
				p[c] = p[c]*(1-paint) + sky*paint
			}
		}
	}
	for i := 0; i < len(pixels); i += 3 {
		if top := max(pixels[i], pixels[i+1], pixels[i+2]); top > sunCap {
			for c := range 3 {
				pixels[i+c] *= sunCap / top
			}
		}
	}
	// Under the horizon: the sky above, mirrored and darkened.
	for y := horizon; y < height; y++ {
		mirror := height - 1 - y
		for x := range width {
			p, sky := at(x, y), at(x, mirror)
			for c := range 3 {
				p[c] = sky[c] * reflect
			}
		}
	}
	if err := os.WriteFile(target, encode(pixels, width, height), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Println("wrote", filepath.ToSlash(target))
}

// smooth is 0 below a, 1 above b, and eases between.
func smooth(a, b, x float32) float32 {
	t := max(0, min(1, (x-a)/(b-a)))
	return t * t * (3 - 2*t)
}

// decode reads a Radiance .hdr file, run-length rows of RGBE, into linear
// RGB floats.
func decode(data []byte) ([]float32, int, int, error) {
	r := bufio.NewReader(bytes.NewReader(data))
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, 0, 0, errors.New("no size line")
		}
		if strings.HasPrefix(line, "-Y ") {
			f := strings.Fields(line)
			h, _ := strconv.Atoi(f[1])
			w, _ := strconv.Atoi(f[3])
			pixels := make([]float32, w*h*3)
			row := make([]byte, w*4)
			for y := range h {
				head := make([]byte, 4)
				if _, err := r.Read(head); err != nil || head[0] != 2 || head[1] != 2 {
					return nil, 0, 0, fmt.Errorf("row %d is not run-length encoded", y)
				}
				for c := range 4 {
					for x := 0; x < w; {
						n, _ := r.ReadByte()
						if n > 128 {
							v, _ := r.ReadByte()
							for k := range int(n) - 128 {
								row[(x+k)*4+c] = v
							}
							x += int(n) - 128
						} else {
							for k := range int(n) {
								row[(x+k)*4+c], _ = r.ReadByte()
							}
							x += int(n)
						}
					}
				}
				for x := range w {
					e := row[x*4+3]
					if e == 0 {
						continue
					}
					s := float32(math.Ldexp(1, int(e)-136))
					for c := range 3 {
						pixels[(y*w+x)*3+c] = (float32(row[x*4+c]) + 0.5) * s
					}
				}
			}
			return pixels, w, h, nil
		}
	}
}

// encode writes pixels as a Radiance .hdr file, each row run-length encoded
// with literal runs only, as every reader takes.
func encode(pixels []float32, w, h int) []byte {
	var out bytes.Buffer
	fmt.Fprintf(&out, "#?RADIANCE\nFORMAT=32-bit_rle_rgbe\n\n-Y %d +X %d\n", h, w)
	row := make([]byte, w*4)
	for y := range h {
		for x := range w {
			p := pixels[(y*w+x)*3 : (y*w+x)*3+3]
			top := max(p[0], p[1], p[2])
			if top < 1e-32 {
				copy(row[x*4:], []byte{0, 0, 0, 0})
				continue
			}
			frac, exp := math.Frexp(float64(top))
			scale := float32(frac*256) / top
			row[x*4] = byte(p[0] * scale)
			row[x*4+1] = byte(p[1] * scale)
			row[x*4+2] = byte(p[2] * scale)
			row[x*4+3] = byte(exp + 128)
		}
		out.Write([]byte{2, 2, byte(w >> 8), byte(w & 255)})
		for c := range 4 {
			for x := 0; x < w; {
				n := min(128, w-x)
				out.WriteByte(byte(n))
				for k := range n {
					out.WriteByte(row[(x+k)*4+c])
				}
				x += n
			}
		}
	}
	return out.Bytes()
}
