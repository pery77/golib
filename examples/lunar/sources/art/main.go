// Generate SELENE's original planet illustrations and icon using the standard
// library. Run from the repository root: golib go -C examples/lunar run ./sources/art
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

func main() {
	if err := os.MkdirAll("assets/planets", 0755); err != nil {
		panic(err)
	}
	tints := [][3]float64{{60, 140, 161}, {136, 103, 164}, {172, 116, 80}, {65, 153, 140}, {94, 119, 178}}
	for i, tint := range tints {
		im := image.NewNRGBA(image.Rect(0, 0, 600, 600))
		for y := 0; y < 600; y++ {
			for x := 0; x < 600; x++ {
				nx, ny := (float64(x)-299.5)/258, (float64(y)-299.5)/258
				rr := nx*nx + ny*ny
				if rr > 1 {
					glow := math.Exp(-(math.Sqrt(rr)-1)*37) * 0.28
					if glow > 0.004 {
						im.SetNRGBA(x, y, color.NRGBA{R: uint8(tint[0]), G: uint8(tint[1]), B: uint8(tint[2]), A: uint8(glow * 255)})
					}
					continue
				}
				nz := math.Sqrt(1 - rr)
				light := math.Max(0.045, math.Min(1, -nx*0.67-ny*0.40+nz*0.54))
				lon, lat := math.Atan2(nx, nz), math.Asin(ny)
				land := math.Sin(lon*8+math.Sin(lat*11)*1.3) + math.Cos(lat*19-lon*6)*0.65
				cloud := math.Pow(math.Max(0, math.Sin(lat*37+math.Sin(lon*5)*2)), 12)
				texture := 0.85 + 0.10*math.Tanh(land*3) + cloud*0.13
				rim := math.Pow(1-nz, 5) * 0.28 * math.Max(0, -nx+0.4)
				noise := math.Sin(float64(x)*12.9898+float64(y)*78.233) * 0.7
				var rgb [3]uint8
				for c := range rgb {
					rgb[c] = uint8(math.Max(0, math.Min(255, 8+tint[c]*(light*texture+rim)+noise)))
				}
				alpha := math.Min(1, (1-math.Sqrt(rr))*258)
				im.SetNRGBA(x, y, color.NRGBA{R: rgb[0], G: rgb[1], B: rgb[2], A: uint8(alpha * 255)})
			}
		}
		write(filepath.Join("assets", "planets", fmt.Sprintf("%d.png", i)), im)
	}
	icon := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			dx, dy := float64(x-128), float64(y-128)
			r := math.Hypot(dx, dy)
			c := color.NRGBA{R: 9, G: 20, B: 32, A: 255}
			if r > 121 {
				c.A = 0
			}
			if r > 108 && r < 111 {
				c = color.NRGBA{R: 107, G: 240, B: 221, A: 255}
			}
			// Symmetric lander mark and a warm engine plume.
			if y >= 56 && y < 149 && math.Abs(dx) < math.Min(34, float64(y-49)*0.78) {
				c = color.NRGBA{R: 217, G: 232, B: 229, A: 255}
			}
			if y > 83 && y < 115 && math.Abs(dx) < 23 {
				c = color.NRGBA{R: 60, G: 204, B: 193, A: 255}
			}
			if y >= 145 && y < 198 && math.Abs(dx) < (198-float64(y))*0.29 {
				c = color.NRGBA{R: 255, G: 172, B: 99, A: 255}
			}
			if y > 130 && y < 173 && math.Abs(math.Abs(dx)-(29+float64(y-130)*0.6)) < 4 {
				c = color.NRGBA{R: 170, G: 194, B: 203, A: 255}
			}
			if y >= 170 && y < 175 && math.Abs(dx) > 43 && math.Abs(dx) < 66 {
				c = color.NRGBA{R: 107, G: 240, B: 221, A: 255}
			}
			icon.SetNRGBA(x, y, c)
		}
	}
	write("icon.png", icon)
}
func write(name string, im image.Image) {
	f, err := os.Create(name)
	if err != nil {
		panic(err)
	}
	if err = png.Encode(f, im); err != nil {
		_ = f.Close()
		panic(err)
	}
	if err = f.Close(); err != nil {
		panic(err)
	}
	fmt.Println(name)
}
