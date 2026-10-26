package golib

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"

	"golib/internal/device"
)

// HDRI is a panorama of a sky, or of any place, brighter than white where
// the sun is, read from a Radiance .hdr file in the game's assets folder: the
// equirectangular kind Poly Haven and most HDRI sites give, twice as wide as
// it is high. Given to an Environment, it lights the 3D as that place would,
// with soft light from all around and reflections in shiny things, and shows
// behind everything with SkyBackground:
//
//	var park = golib.NewHDRI("skies/park.hdr")
//
//	screen.SetEnvironment(golib.Environment{HDRI: park, SkyBackground: true})
//
// The middle of the picture faces +Z, as a zero [Rotation] does, and its
// right half is to the -X side; Environment.HDRITurn turns it round.
// The picture's sun becomes the Environment's: as the HDRI loads, its disc,
// thousands of times brighter than the sky, is taken out of the panorama's
// light, and shines instead from the same place as GoLib's sun, which casts
// the shadows: it would otherwise light everything twice. SunDirection,
// SunColor and SunStrength still change it. The file is read the first time
// the HDRI is used.
type HDRI struct {
	name string

	mu         sync.Mutex
	read       bool
	err        error
	width      int
	height     int
	levels     [][]uint16    // RGBA half floats, each mipmap half the size of the last
	irradiance [9][4]float32 // the light all around, as spherical harmonics, ready for the lit shader
	sun        [4]float32    // the sun taken out of the picture, as the lit shader's sunColor
	toSun      Vector3       // the way to it, length 1
	texture    texture3D
	keepSun    bool // a reflection probe's panorama: its lamps and windows stay in it
}

// NewHDRI returns the HDRI in the game's assets folder named name, such as
// "skies/park.hdr".
func NewHDRI(name string) *HDRI {
	return &HDRI{name: name}
}

// Err reads the HDRI, if it has not been read yet, and returns what stopped
// it from loading. Using one that didn't load stops Run with the same
// mistake.
func (h *HDRI) Err() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.prepare()
}

// maxHDRIWidth is how wide a panorama is kept: wider ones are made smaller
// as they load, so a 4K sky fits a browser's memory and loads quickly.
const maxHDRIWidth = 2048

func (h *HDRI) prepare() error {
	if h.read {
		return h.err
	}
	h.read = true
	data, err := ReadAsset(h.name)
	if err == nil {
		err = h.load(data)
	}
	if err != nil {
		h.err = fmt.Errorf("golib.NewHDRI(%q): %w", h.name, err)
	}
	return h.err
}

func (h *HDRI) load(data []byte) error {
	pixels, width, height, err := decodeRadiance(data)
	if err != nil {
		return err
	}
	for width > maxHDRIWidth {
		pixels, width, height = halve(pixels, width, height)
	}
	h.width, h.height = width, height
	if !h.keepSun {
		h.sun, h.toSun = takeSun(pixels, width, height)
	}
	h.irradiance = sphericalHarmonics(pixels, width, height)
	level, w, ht := pixels, width, height
	for {
		h.levels = append(h.levels, halfFloats(level))
		if w == 1 && ht == 1 {
			break
		}
		level, w, ht = halve(level, w, ht)
	}
	return nil
}

// upload puts the panorama on the graphics card, the first time it lights a
// pass.
func (h *HDRI) upload() (device.Texture3D, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.prepare(); err != nil {
		return device.Texture3D{}, err
	}
	if h.texture.gpu.ID == 0 {
		h.texture.gpu = device.NewHDRTexture(h.width, h.height, h.levels, true)
		loadedTextures.add(&h.texture)
	}
	return h.texture.gpu, nil
}

// decodeRadiance reads a Radiance .hdr file: its header, then its pixels as
// RGBE, flat or run-length encoded, scanline by scanline. It returns RGB
// floats, linear, the top row first.
func decodeRadiance(data []byte) ([]float32, int, int, error) {
	reader := bufio.NewReader(bytes.NewReader(data))
	first, err := reader.ReadString('\n')
	if err != nil || !strings.HasPrefix(first, "#?") {
		return nil, 0, 0, errors.New("not a Radiance .hdr file: it doesn't start with #?")
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, 0, 0, errors.New("a Radiance header that never ends")
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if format, ok := strings.CutPrefix(line, "FORMAT="); ok && format != "32-bit_rle_rgbe" {
			return nil, 0, 0, fmt.Errorf("a Radiance file in %s: GoLib reads 32-bit_rle_rgbe, the usual kind", format)
		}
	}
	size, err := reader.ReadString('\n')
	if err != nil {
		return nil, 0, 0, errors.New("a Radiance file without its size")
	}
	fields := strings.Fields(size)
	if len(fields) != 4 || fields[0] != "-Y" || fields[2] != "+X" {
		return nil, 0, 0, fmt.Errorf("a Radiance file laid out as %q: GoLib reads -Y height +X width, the usual layout", strings.TrimSpace(size))
	}
	height, err1 := strconv.Atoi(fields[1])
	width, err2 := strconv.Atoi(fields[3])
	if err1 != nil || err2 != nil || width <= 0 || height <= 0 || width > 1<<15 || height > 1<<15 {
		return nil, 0, 0, fmt.Errorf("a Radiance file %q in size", strings.TrimSpace(size))
	}
	pixels := make([]float32, 0, width*height*3)
	scanline := make([]byte, width*4)
	for y := 0; y < height; y++ {
		if err := readScanline(reader, scanline, width); err != nil {
			return nil, 0, 0, fmt.Errorf("row %d: %w", y, err)
		}
		for x := 0; x < width; x++ {
			r, g, b, e := scanline[x*4], scanline[x*4+1], scanline[x*4+2], scanline[x*4+3]
			if e == 0 {
				pixels = append(pixels, 0, 0, 0)
				continue
			}
			scale := float32(math.Ldexp(1, int(e)-136))
			pixels = append(pixels, (float32(r)+0.5)*scale, (float32(g)+0.5)*scale, (float32(b)+0.5)*scale)
		}
	}
	return pixels, width, height, nil
}

// readScanline reads one row of RGBE pixels into out.
func readScanline(reader *bufio.Reader, out []byte, width int) error {
	head, err := reader.Peek(4)
	if err != nil {
		return errors.New("the file ends early")
	}
	if width < 8 || width > 0x7fff || head[0] != 2 || head[1] != 2 || head[2]&0x80 != 0 {
		// Flat: four bytes a pixel.
		_, err := readFull(reader, out)
		return err
	}
	if int(head[2])<<8|int(head[3]) != width {
		return errors.New("a run-length row of the wrong width")
	}
	reader.Discard(4)
	// Run-length: each of the four channels in turn, for the whole row.
	for channel := 0; channel < 4; channel++ {
		for x := 0; x < width; {
			count, err := reader.ReadByte()
			if err != nil {
				return errors.New("the file ends early")
			}
			if count > 128 {
				run := int(count) - 128
				value, err := reader.ReadByte()
				if err != nil || x+run > width {
					return errors.New("a run past the end of the row")
				}
				for k := 0; k < run; k++ {
					out[(x+k)*4+channel] = value
				}
				x += run
				continue
			}
			if count == 0 || x+int(count) > width {
				return errors.New("a run past the end of the row")
			}
			for k := 0; k < int(count); k++ {
				value, err := reader.ReadByte()
				if err != nil {
					return errors.New("the file ends early")
				}
				out[(x+k)*4+channel] = value
			}
			x += int(count)
		}
	}
	return nil
}

func readFull(reader *bufio.Reader, out []byte) (int, error) {
	n := 0
	for n < len(out) {
		k, err := reader.Read(out[n:])
		n += k
		if err != nil {
			return n, errors.New("the file ends early")
		}
	}
	return n, nil
}

// takeSun takes the sun out of a panorama: every pixel far brighter than its
// sky is brought down to the sky's brightest, and the light taken away is
// returned as the sun's, as the lit shader's sunColor wants it, the light a
// surface facing it gets divided by pi, with the way to it. A panorama
// without a sun returns none.
func takeSun(pixels []float32, width, height int) (sun [4]float32, toSun Vector3) {
	peak := func(at int) float32 { return max(pixels[at], pixels[at+1], pixels[at+2]) }
	step := max(1, width*height/65536)
	var sample []float32
	for i := 0; i < width*height; i += step {
		sample = append(sample, peak(i*3))
	}
	slices.Sort(sample)
	// The sky's brightest: well above most of the sky, and twice what all
	// but a hundredth of it is. The sun's disc is a hundred thousandth of a
	// panorama, and its glow around it not much more.
	limit := max(16*sample[len(sample)/2], 2*sample[len(sample)*99/100])
	if limit <= 0 {
		return sun, toSun
	}
	var light [3]float64
	var way Vector3
	for y := 0; y < height; y++ {
		v := (float64(y) + 0.5) / float64(height)
		solid := 2 * math.Pi / float64(width) * math.Pi / float64(height) * math.Sin(v*math.Pi)
		for x := 0; x < width; x++ {
			at := (y*width + x) * 3
			brightest := peak(at)
			if brightest <= limit {
				continue
			}
			keep := limit / brightest
			for c := range 3 {
				light[c] += float64(pixels[at+c]*(1-keep)) * solid
				pixels[at+c] *= keep
			}
			d := panoramaDirection((float64(x)+0.5)/float64(width), v)
			way = way.Add(d.Scale(float32(float64(brightest-limit) * solid)))
		}
	}
	if way == (Vector3{}) {
		return sun, toSun
	}
	for c := range 3 {
		sun[c] = float32(light[c] / math.Pi)
	}
	sun[3] = 1
	return sun, way.Normalize()
}

// halve returns pixels at half the width and height, each the average of the
// two by two it covers; a side of 1 stays 1.
func halve(pixels []float32, width, height int) ([]float32, int, int) {
	w, h := max(1, width/2), max(1, height/2)
	out := make([]float32, w*h*3)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sum [3]float32
			n := float32(0)
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					sx, sy := min(x*2+dx, width-1), min(y*2+dy, height-1)
					at := (sy*width + sx) * 3
					sum[0] += pixels[at]
					sum[1] += pixels[at+1]
					sum[2] += pixels[at+2]
					n++
				}
			}
			at := (y*w + x) * 3
			out[at], out[at+1], out[at+2] = sum[0]/n, sum[1]/n, sum[2]/n
		}
	}
	return out, w, h
}

// halfFloats returns RGB pixels as RGBA half floats, opaque.
func halfFloats(pixels []float32) []uint16 {
	out := make([]uint16, len(pixels)/3*4)
	for i := 0; i < len(pixels)/3; i++ {
		out[i*4] = toHalf(pixels[i*3])
		out[i*4+1] = toHalf(pixels[i*3+1])
		out[i*4+2] = toHalf(pixels[i*3+2])
		out[i*4+3] = toHalf(1)
	}
	return out
}

// toHalf returns f as a 16-bit float, the largest half for anything larger,
// rounded to nearest.
func toHalf(f float32) uint16 {
	bits := math.Float32bits(f)
	sign := uint16(bits>>16) & 0x8000
	exponent := int(bits>>23&0xff) - 127 + 15
	mantissa := bits & 0x7fffff
	switch {
	case exponent >= 31:
		return sign | 0x7bff // the largest half, not infinity
	case exponent <= 0:
		if exponent < -10 {
			return sign
		}
		mantissa |= 0x800000
		shift := uint(14 - exponent)
		half := uint16(mantissa >> shift)
		if mantissa>>(shift-1)&1 != 0 {
			half++
		}
		return sign | half
	}
	half := sign | uint16(exponent)<<10 | uint16(mantissa>>13)
	if mantissa&0x1000 != 0 {
		half++ // rounds up into the exponent where it must
	}
	return half
}

// panoramaDirection returns the direction a point of an equirectangular
// panorama shows, u across and v down, each from 0 to 1: the middle is +Z,
// the top straight up, and u grows to the right of someone looking along +Z.
// The shaders turn a direction back with panorama().
func panoramaDirection(u, v float64) Vector3 {
	polar := v * math.Pi
	around := (0.5 - u) * 2 * math.Pi
	return Vector3{
		X: float32(math.Sin(polar) * math.Sin(around)),
		Y: float32(math.Cos(polar)),
		Z: float32(math.Sin(polar) * math.Cos(around)),
	}
}

// sphericalHarmonics returns the light all around a panorama as nine
// spherical harmonics, convolved with a cosine, so that the lit shader's
// sum of them for a normal is the soft light a surface facing that way
// gets: each band already carries its factor and the division by pi.
func sphericalHarmonics(pixels []float32, width, height int) [9][4]float32 {
	for width > 128 {
		pixels, width, height = halve(pixels, width, height)
	}
	var sums [9][3]float64
	for y := 0; y < height; y++ {
		v := (float64(y) + 0.5) / float64(height)
		solid := 2 * math.Pi / float64(width) * math.Pi / float64(height) * math.Sin(v*math.Pi)
		for x := 0; x < width; x++ {
			d := panoramaDirection((float64(x)+0.5)/float64(width), v)
			dx, dy, dz := float64(d.X), float64(d.Y), float64(d.Z)
			basis := [9]float64{
				0.282095,
				0.488603 * dy, 0.488603 * dz, 0.488603 * dx,
				1.092548 * dx * dy, 1.092548 * dy * dz, 0.315392 * (3*dz*dz - 1), 1.092548 * dx * dz, 0.546274 * (dx*dx - dy*dy),
			}
			at := (y*width + x) * 3
			for k, b := range basis {
				for c := 0; c < 3; c++ {
					sums[k][c] += float64(pixels[at+c]) * b * solid
				}
			}
		}
	}
	// Convolving with a cosine weighs each band by pi, 2pi/3 and pi/4; the
	// shader then divides by pi, as Lambert's diffuse does.
	bands := [9]float64{1, 2.0 / 3, 2.0 / 3, 2.0 / 3, 0.25, 0.25, 0.25, 0.25, 0.25}
	var out [9][4]float32
	for k := range sums {
		for c := 0; c < 3; c++ {
			out[k][c] = float32(sums[k][c] * bands[k])
		}
	}
	return out
}
