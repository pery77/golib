//go:build ignore

// rune paints the glowing rune a gem leaves on the floor,
// assets/textures/rune.png: a ring with a glyph inside, white lines with soft
// edges, as opaque as they are bright, on a black, transparent ground, which
// the game tints and presses onto the floor as a glowing decal. Run it from the project root:
//
//	./golib go run games/lantern/sources/textures/rune.go
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
)

const (
	size   = 256
	output = "games/lantern/assets/textures/rune.png"
)

// segment is a stroke of the glyph, from (x0, y0) to (x1, y1), with the
// picture running from -1 to 1 each way.
type segment struct{ x0, y0, x1, y1 float64 }

var glyph = []segment{
	{0, -0.62, 0, 0.62},      // the stem
	{0, 0.62, -0.38, 0.22},   // the arms of the top
	{0, 0.62, 0.38, 0.22},    //
	{-0.42, -0.1, 0.42, -0.1}, // the bar
	{-0.3, -0.5, 0, -0.25},   // the feet
	{0.3, -0.5, 0, -0.25},    //
}

// distanceToSegment is how far p is from the segment.
func distanceToSegment(px, py float64, s segment) float64 {
	dx, dy := s.x1-s.x0, s.y1-s.y0
	t := ((px-s.x0)*dx + (py-s.y0)*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(px-(s.x0+t*dx), py-(s.y0+t*dy))
}

// stroke is how much of a line of half-width w covers a point d away from
// its middle, softened over a few pixels.
func stroke(d, w float64) float64 {
	soft := 3.0 / size
	return math.Max(0, math.Min(1, (w-d)/soft+0.5))
}

func main() {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			px := (float64(x)+0.5)/size*2 - 1
			py := 1 - (float64(y)+0.5)/size*2
			r := math.Hypot(px, py)
			a := stroke(math.Abs(r-0.86), 0.045) // the outer ring
			a = math.Max(a, stroke(math.Abs(r-0.74), 0.015))
			// Small ticks between the rings, every 30 degrees.
			angle := math.Atan2(py, px)
			tick := math.Abs(math.Remainder(angle, math.Pi/6)) * r
			if r > 0.74 && r < 0.86 {
				a = math.Max(a, stroke(tick, 0.012))
			}
			for _, s := range glyph {
				a = math.Max(a, stroke(distanceToSegment(px, py, s), 0.05))
			}
			// A faint glow around everything.
			glow := 0.25 * math.Max(0, 1-r) * math.Max(0, 1-r)
			a = math.Max(a, glow)
			v := uint8(math.Round(a * 255))
			img.SetNRGBA(x, y, color.NRGBA{R: v, G: v, B: v, A: v}) // white where it shows, both in color and in opacity
		}
	}
	f, err := os.Create(output)
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
	log.Println("wrote", output)
}
