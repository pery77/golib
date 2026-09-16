package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/fs"
	"math"
	"os"
)

// iconFile is the image in a game's folder that becomes its icon.
const iconFile = "icon.png"

// iconSizes are the sizes, in pixels, that Windows picks from for lists, the
// taskbar and Explorer's views, at every display scale. Sizes below 256 are
// stored as bitmaps and 256 as a PNG, as Windows' own icons are.
var iconSizes = []int{16, 20, 24, 32, 40, 48, 64, 256}

// minIconSize is the smallest icon.png GoLib accepts, in pixels.
const minIconSize = 16

// iconGroupName names the icon resource. GLFW, which raylib opens its windows
// with, gives a resource icon with this name to the game's window, so the
// title bar and the taskbar show the same icon as Explorer.
const iconGroupName = "GLFW_ICON"

// readIcon reads a square PNG image from path. found is false when there is
// no such file.
func readIcon(path string) (icon *image.NRGBA, found bool, err error) {
	file, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, true, err
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		return nil, true, fmt.Errorf("not a PNG image GoLib can read (%v): save it as PNG, for example from Aseprite", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != bounds.Dy() {
		return nil, true, fmt.Errorf("the image is %d by %d pixels, but an icon must be square: ideally 256 by 256", bounds.Dx(), bounds.Dy())
	}
	if bounds.Dx() < minIconSize {
		return nil, true, fmt.Errorf("the image is %d by %d pixels, but an icon needs at least %d by %d: ideally 256 by 256", bounds.Dx(), bounds.Dy(), minIconSize, minIconSize)
	}
	icon = image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(icon, icon.Bounds(), img, bounds.Min, draw.Src)
	return icon, true, nil
}

// iconResources turns icon into the resources of a Windows icon: one image
// for each of iconSizes, and the group that lists them.
func iconResources(icon *image.NRGBA) []resource {
	resources := make([]resource, 0, len(iconSizes)+1)
	group := le.AppendUint16(nil, 0) // reserved
	group = le.AppendUint16(group, 1)
	group = le.AppendUint16(group, uint16(len(iconSizes)))
	for i, size := range iconSizes {
		img := resize(icon, size)
		var data []byte
		if size >= 256 {
			data = encodePNG(img)
		} else {
			data = iconBitmap(img)
		}
		id := uint16(i + 1)
		resources = append(resources, resource{kind: rtIcon, id: id, data: data})

		// One entry per image. A width and height of 0 mean 256.
		group = append(group, byte(size), byte(size), 0, 0)
		group = le.AppendUint16(group, 1)  // color planes
		group = le.AppendUint16(group, 32) // bits per pixel
		group = le.AppendUint32(group, uint32(len(data)))
		group = le.AppendUint16(group, id)
	}
	return append(resources, resource{kind: rtGroupIcon, name: iconGroupName, data: group})
}

// resize returns src scaled to size by size pixels. Growing repeats pixels, so
// pixel art stays sharp; shrinking averages the pixels that each new pixel
// covers.
func resize(src *image.NRGBA, size int) *image.NRGBA {
	from := src.Bounds().Dx()
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	if size >= from {
		for y := range size {
			for x := range size {
				dst.SetNRGBA(x, y, src.NRGBAAt(x*from/size, y*from/size))
			}
		}
		return dst
	}
	shares := coverage(from, size)
	for y := range size {
		for x := range size {
			// alpha is the average opacity. Colors are averaged weighted by
			// opacity, so transparent pixels don't darken the edges.
			var red, green, blue, alpha float64
			for _, row := range shares[y] {
				for _, column := range shares[x] {
					c := src.NRGBAAt(column.index, row.index)
					weight := row.share * column.share * float64(c.A)
					red += weight * float64(c.R)
					green += weight * float64(c.G)
					blue += weight * float64(c.B)
					alpha += weight
				}
			}
			if alpha == 0 {
				continue // fully transparent
			}
			dst.SetNRGBA(x, y, color.NRGBA{
				R: channel(red / alpha),
				G: channel(green / alpha),
				B: channel(blue / alpha),
				A: channel(alpha),
			})
		}
	}
	return dst
}

// share is how much of an old pixel, by index, a new pixel covers, as a part
// of the new pixel.
type share struct {
	index int
	share float64
}

// coverage returns, for each of size new pixels along a side of from old
// ones, the old pixels it covers.
func coverage(from, size int) [][]share {
	scale := float64(from) / float64(size)
	shares := make([][]share, size)
	for i := range shares {
		start, end := float64(i)*scale, float64(i+1)*scale
		for j := int(start); j < from && float64(j) < end; j++ {
			if overlap := min(end, float64(j+1)) - max(start, float64(j)); overlap > 0 {
				shares[i] = append(shares[i], share{index: j, share: overlap / scale})
			}
		}
	}
	return shares
}

// channel rounds a color channel to a byte.
func channel(value float64) uint8 {
	return uint8(math.Round(min(max(value, 0), 255)))
}

// iconBitmap encodes img the way icons store their smaller images: a 32-bit
// bitmap, bottom row first, followed by a mask that marks transparent pixels
// for programs that ignore the alpha channel.
func iconBitmap(img *image.NRGBA) []byte {
	size := img.Bounds().Dx()
	maskRow := (size + 31) / 32 * 4 // mask rows are padded to 32 bits
	pixels := size * size * 4
	data := make([]byte, 40, 40+pixels+maskRow*size)
	le.PutUint32(data[0:], 40) // BITMAPINFOHEADER size
	le.PutUint32(data[4:], uint32(size))
	le.PutUint32(data[8:], uint32(2*size)) // the colors and the mask, stacked
	le.PutUint16(data[12:], 1)             // color planes
	le.PutUint16(data[14:], 32)            // bits per pixel
	le.PutUint32(data[20:], uint32(pixels+maskRow*size))
	for y := size - 1; y >= 0; y-- {
		for x := range size {
			c := img.NRGBAAt(x, y)
			data = append(data, c.B, c.G, c.R, c.A)
		}
	}
	for y := size - 1; y >= 0; y-- {
		row := make([]byte, maskRow)
		for x := range size {
			if img.NRGBAAt(x, y).A == 0 {
				row[x/8] |= 0x80 >> (x % 8)
			}
		}
		data = append(data, row...)
	}
	return data
}

// encodePNG encodes img as a PNG file.
func encodePNG(img *image.NRGBA) []byte {
	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoder.Encode(&buf, img); err != nil {
		panic(err) // encoding to memory only fails for images too big to exist here
	}
	return buf.Bytes()
}
