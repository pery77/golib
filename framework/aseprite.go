package golib

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"slices"
	"unicode/utf8"
)

// Aseprite files are read as the format's specification describes:
// https://github.com/aseprite/aseprite/blob/main/docs/ase-file-specs.md
// Layers are combined the way Aseprite 1.3 combines them, blend modes
// included, so a frame looks as it does in Aseprite.

// Header flags.
const (
	aseLayerOpacityValid = 1 // layer opacity counts
	aseGroupsComposite   = 2 // groups are combined on their own first, then blended with their mode and opacity
)

// Layer flags and types.
const (
	aseLayerVisible    = 1
	aseLayerBackground = 8
	aseLayerReference  = 64

	aseLayerGroup = 1 // a layer type; the others are 0, an image, and 2, a tilemap
)

// Chunk types that GoLib reads. The others, such as slices and user data,
// are skipped.
const (
	aseChunkOldPalette   = 0x0004
	aseChunkOldPalette63 = 0x0011
	aseChunkLayer        = 0x2004
	aseChunkCel          = 0x2005
	aseChunkTags         = 0x2018
	aseChunkPalette      = 0x2019
)

// aseFile is what GoLib keeps of an Aseprite file.
type aseFile struct {
	width, height int
	flags         uint32
	layers        []aseLayer
	frames        []aseFrame
	tags          []aseTag
}

type aseLayer struct {
	flags   uint16
	kind    uint16
	level   int // how deep in groups: 0 at the top
	blend   uint16
	opacity uint8
	name    string
	parent  int // index of the group it is in, or -1
}

type aseFrame struct {
	duration float32 // seconds
	cels     []aseCel
}

type aseCel struct {
	layer   int
	x, y    int
	opacity uint8
	z       int
	pixels  *image.NRGBA
	link    int // for a linked cel, the frame whose cel of the same layer it shows; otherwise -1
	tilemap bool
}

type aseTag struct {
	from, to  int
	direction uint8 // 0 forward, 1 reverse, 2 ping-pong, 3 ping-pong reverse
	repeat    int   // 0 forever
	name      string
}

// aseReader reads the little-endian values of an Aseprite file. The first
// read past the end sets err, and every read after that returns zero.
type aseReader struct {
	data []byte
	at   int
	err  error
}

func (r *aseReader) bytes(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || r.at+n > len(r.data) {
		r.err = errors.New("the file is cut short: save it again from Aseprite")
		return nil
	}
	b := r.data[r.at : r.at+n]
	r.at += n
	return b
}

func (r *aseReader) byte() uint8 {
	if b := r.bytes(1); b != nil {
		return b[0]
	}
	return 0
}

func (r *aseReader) word() uint16 {
	if b := r.bytes(2); b != nil {
		return binary.LittleEndian.Uint16(b)
	}
	return 0
}

func (r *aseReader) short() int {
	return int(int16(r.word()))
}

func (r *aseReader) dword() uint32 {
	if b := r.bytes(4); b != nil {
		return binary.LittleEndian.Uint32(b)
	}
	return 0
}

func (r *aseReader) skip(n int) {
	r.bytes(n)
}

func (r *aseReader) string() string {
	b := r.bytes(int(r.word()))
	if !utf8.Valid(b) {
		return string([]rune(string(b)))
	}
	return string(b)
}

// parseAseprite reads an Aseprite file.
func parseAseprite(data []byte) (*aseFile, error) {
	r := &aseReader{data: data}
	r.dword() // file size
	if magic := r.word(); r.err == nil && magic != 0xA5E0 {
		return nil, errors.New("not an Aseprite file: save it again from Aseprite")
	}
	frameCount := int(r.word())
	f := &aseFile{width: int(r.word()), height: int(r.word())}
	depth := r.word()
	f.flags = r.dword()
	r.skip(2 + 4 + 4) // speed, which frames override, and two zeros
	transparent := r.byte()
	r.skip(3 + 2 + 1 + 1 + 2 + 2 + 2 + 2 + 84)
	if r.err != nil {
		return nil, r.err
	}
	if depth != 32 && depth != 16 && depth != 8 {
		return nil, fmt.Errorf("the file has %d bits per pixel, which Aseprite doesn't write: save it again from Aseprite", depth)
	}
	if f.width == 0 || f.height == 0 || frameCount == 0 {
		return nil, fmt.Errorf("the sprite is %d by %d pixels with %d frames: it has nothing to draw", f.width, f.height, frameCount)
	}

	palette := make([]color.NRGBA, 256)
	pixel := pixelDecoder(depth, transparent, palette)
	for frameIndex := range frameCount {
		start := r.at
		size := int(r.dword())
		if magic := r.word(); r.err == nil && magic != 0xF1FA {
			return nil, fmt.Errorf("frame %d is damaged: save the file again from Aseprite", frameIndex)
		}
		chunks := int(r.word())
		frame := aseFrame{duration: float32(r.word()) / 1000}
		r.skip(2)
		if newCount := int(r.dword()); newCount != 0 {
			chunks = newCount
		}
		for range chunks {
			chunkStart := r.at
			chunkSize := int(r.dword())
			kind := r.word()
			if r.err != nil {
				return nil, r.err
			}
			if chunkSize < 6 || chunkStart+chunkSize > len(data) {
				return nil, errors.New("the file is cut short: save it again from Aseprite")
			}
			chunk := &aseReader{data: data[:chunkStart+chunkSize], at: r.at}
			switch kind {
			case aseChunkLayer:
				f.layers = append(f.layers, readAseLayer(chunk, f.flags))
			case aseChunkCel:
				cel, err := readAseCel(chunk, f.layers, pixel, depth)
				if err != nil {
					return nil, fmt.Errorf("frame %d: %w", frameIndex, err)
				}
				frame.cels = append(frame.cels, cel)
			case aseChunkTags:
				f.tags = readAseTags(chunk)
			case aseChunkPalette:
				readAsePalette(chunk, palette)
			case aseChunkOldPalette, aseChunkOldPalette63:
				readAseOldPalette(chunk, palette, kind == aseChunkOldPalette63)
			}
			if chunk.err != nil {
				return nil, chunk.err
			}
			r.at = chunkStart + chunkSize
		}
		f.frames = append(f.frames, frame)
		r.at = start + size
	}
	if err := f.check(); err != nil {
		return nil, err
	}
	return f, nil
}

// check finds the groups of layers, and mistakes that would draw nonsense.
func (f *aseFile) check() error {
	var groups []int // the enclosing group at each level
	for i := range f.layers {
		layer := &f.layers[i]
		if layer.level > len(groups) {
			return fmt.Errorf("layer %q is in a group that doesn't exist: save the file again from Aseprite", layer.name)
		}
		groups = groups[:layer.level]
		layer.parent = -1
		if layer.level > 0 {
			layer.parent = groups[layer.level-1]
		}
		groups = append(groups, i)
	}
	for frameIndex, frame := range f.frames {
		for _, cel := range frame.cels {
			if cel.layer >= len(f.layers) {
				return fmt.Errorf("frame %d has a cel on layer %d, which doesn't exist: save the file again from Aseprite", frameIndex, cel.layer)
			}
			if cel.tilemap && f.shows(cel.layer) {
				return fmt.Errorf("layer %q is a tilemap layer, which GoLib can't draw yet: hide it, or convert it to a normal layer in Aseprite", f.layers[cel.layer].name)
			}
			if cel.link >= 0 && (cel.link >= len(f.frames) || f.celOf(cel.link, cel.layer) == nil) {
				return fmt.Errorf("frame %d links to a cel in frame %d that doesn't exist: save the file again from Aseprite", frameIndex, cel.link)
			}
		}
	}
	for _, tag := range f.tags {
		if tag.from > tag.to || tag.to >= len(f.frames) {
			return fmt.Errorf("tag %q goes from frame %d to frame %d, but the file has %d frames: fix the tag in Aseprite", tag.name, tag.from, tag.to, len(f.frames))
		}
	}
	return nil
}

// shows reports whether layer is drawn: visible, in visible groups, and not
// a reference layer.
func (f *aseFile) shows(layer int) bool {
	for ; layer >= 0; layer = f.layers[layer].parent {
		flags := f.layers[layer].flags
		if flags&aseLayerVisible == 0 || flags&aseLayerReference != 0 {
			return false
		}
	}
	return true
}

// celOf returns the cel of layer in frame, following links, or nil.
func (f *aseFile) celOf(frame, layer int) *aseCel {
	for range len(f.frames) { // a link never needs more steps than that
		var found *aseCel
		for i := range f.frames[frame].cels {
			if f.frames[frame].cels[i].layer == layer {
				found = &f.frames[frame].cels[i]
				break
			}
		}
		if found == nil || found.link < 0 {
			return found
		}
		if found.link >= len(f.frames) {
			return nil
		}
		frame = found.link
	}
	return nil
}

func readAseLayer(r *aseReader, headerFlags uint32) aseLayer {
	layer := aseLayer{flags: r.word(), kind: r.word(), level: int(r.word())}
	r.skip(4) // default width and height
	layer.blend = r.word()
	layer.opacity = r.byte()
	r.skip(3)
	layer.name = r.string()
	if headerFlags&aseLayerOpacityValid == 0 {
		layer.opacity = 255
	}
	return layer
}

// pixelDecoder returns a function that turns one pixel of a cel, in the
// file's color depth, into a color. Indexed pixels look up palette, which
// the file fills in as it is read.
func pixelDecoder(depth uint16, transparent uint8, palette []color.NRGBA) func(p []byte, background bool) color.NRGBA {
	switch depth {
	case 32:
		return func(p []byte, _ bool) color.NRGBA { return color.NRGBA{p[0], p[1], p[2], p[3]} }
	case 16:
		return func(p []byte, _ bool) color.NRGBA { return color.NRGBA{p[0], p[0], p[0], p[1]} }
	default:
		return func(p []byte, background bool) color.NRGBA {
			if p[0] == transparent && !background {
				return color.NRGBA{}
			}
			return palette[p[0]]
		}
	}
}

func readAseCel(r *aseReader, layers []aseLayer, pixel func([]byte, bool) color.NRGBA, depth uint16) (aseCel, error) {
	cel := aseCel{layer: int(r.word()), x: r.short(), y: r.short(), opacity: r.byte(), link: -1}
	kind := r.word()
	cel.z = r.short()
	r.skip(5)
	if r.err != nil {
		return cel, r.err
	}
	switch kind {
	case 1: // linked
		cel.link = int(r.word())
		return cel, r.err
	case 3: // tilemap
		cel.tilemap = true
		return cel, nil
	case 0, 2: // raw or compressed pixels
	default:
		return cel, fmt.Errorf("a cel has type %d, which GoLib doesn't know: save the file again from Aseprite", kind)
	}
	width, height := int(r.word()), int(r.word())
	bytesPerPixel := int(depth / 8)
	var raw []byte
	if kind == 0 {
		raw = r.bytes(width * height * bytesPerPixel)
	} else {
		rest := r.bytes(len(r.data) - r.at)
		unzipped, err := zlib.NewReader(bytes.NewReader(rest))
		if err != nil {
			return cel, fmt.Errorf("a cel's pixels are damaged (%v): save the file again from Aseprite", err)
		}
		raw = make([]byte, width*height*bytesPerPixel)
		if _, err := io.ReadFull(unzipped, raw); err != nil {
			return cel, fmt.Errorf("a cel's pixels are damaged (%v): save the file again from Aseprite", err)
		}
	}
	if r.err != nil {
		return cel, r.err
	}
	background := cel.layer < len(layers) && layers[cel.layer].flags&aseLayerBackground != 0
	cel.pixels = image.NewNRGBA(image.Rect(0, 0, width, height))
	for i := range width * height {
		c := pixel(raw[i*bytesPerPixel:], background)
		p := cel.pixels.Pix[i*4 : i*4+4]
		p[0], p[1], p[2], p[3] = c.R, c.G, c.B, c.A
	}
	return cel, nil
}

func readAseTags(r *aseReader) []aseTag {
	count := int(r.word())
	r.skip(8)
	var tags []aseTag
	for range count {
		tag := aseTag{from: int(r.word()), to: int(r.word()), direction: r.byte(), repeat: int(r.word())}
		r.skip(6 + 3 + 1)
		tag.name = r.string()
		if r.err != nil {
			return tags
		}
		tags = append(tags, tag)
	}
	return tags
}

func readAsePalette(r *aseReader, palette []color.NRGBA) {
	r.dword() // new size
	first, last := int(r.dword()), int(r.dword())
	r.skip(8)
	for i := first; i <= last && r.err == nil; i++ {
		flags := r.word()
		c := color.NRGBA{r.byte(), r.byte(), r.byte(), r.byte()}
		if flags&1 != 0 {
			r.string()
		}
		if i < len(palette) {
			palette[i] = c
		}
	}
}

func readAseOldPalette(r *aseReader, palette []color.NRGBA, sixBit bool) {
	packets := int(r.word())
	index := 0
	for range packets {
		index += int(r.byte())
		count := int(r.byte())
		if count == 0 {
			count = 256
		}
		for range count {
			c := color.NRGBA{r.byte(), r.byte(), r.byte(), 255}
			if sixBit {
				c.R, c.G, c.B = c.R<<2|c.R>>4, c.G<<2|c.G>>4, c.B<<2|c.B>>4
			}
			if r.err != nil {
				return
			}
			if index < len(palette) {
				palette[index] = c
			}
			index++
		}
	}
}

// render draws frame as Aseprite shows it.
func (f *aseFile) render(frame int) *image.NRGBA {
	canvas := image.NewNRGBA(image.Rect(0, 0, f.width, f.height))
	f.renderGroup(canvas, frame, -1)
	return canvas
}

// renderGroup draws the layers inside group, or at the top when group is
// -1, onto canvas.
func (f *aseFile) renderGroup(canvas *image.NRGBA, frame, group int) {
	// The layers to draw, in order from the bottom. When groups aren't
	// combined on their own, a group's layers are drawn with the rest. A layer's
	// place in the order is its index in the file, which a cel's z-index
	// shifts.
	type item struct {
		layer, order, z int
	}
	var items []item
	var collect func(parent int)
	collect = func(parent int) {
		for i, layer := range f.layers {
			if layer.parent != parent {
				continue
			}
			if layer.flags&aseLayerVisible == 0 || layer.flags&aseLayerReference != 0 {
				continue
			}
			switch {
			case layer.kind == aseLayerGroup && f.flags&aseGroupsComposite == 0:
				collect(i)
			case layer.kind == aseLayerGroup:
				items = append(items, item{layer: i, order: i})
			default:
				if cel := f.celOf(frame, i); cel != nil {
					items = append(items, item{layer: i, order: i, z: f.celZ(frame, i)})
				}
			}
		}
	}
	collect(group)
	// A cel's z-index moves it up or down among the layers.
	slices.SortStableFunc(items, func(a, b item) int {
		if a.order+a.z != b.order+b.z {
			return (a.order + a.z) - (b.order + b.z)
		}
		return a.z - b.z
	})
	for _, it := range items {
		layer := f.layers[it.layer]
		if layer.kind == aseLayerGroup {
			combined := image.NewNRGBA(canvas.Rect)
			f.renderGroup(combined, frame, it.layer)
			blendImage(canvas, combined, 0, 0, layer.opacity, layer.blend)
			continue
		}
		cel := f.celOf(frame, it.layer)
		opacity := mulUn8(int(cel.opacity), int(layer.opacity))
		blendImage(canvas, cel.pixels, f.celX(frame, it.layer), f.celY(frame, it.layer), uint8(opacity), layer.blend)
	}
}

// celZ, celX and celY return the z-index and the position of the cel of
// layer in frame. A linked cel has its own.
func (f *aseFile) celZ(frame, layer int) int {
	if cel := f.ownCel(frame, layer); cel != nil {
		return cel.z
	}
	return 0
}

func (f *aseFile) celX(frame, layer int) int {
	if cel := f.ownCel(frame, layer); cel != nil {
		return cel.x
	}
	return 0
}

func (f *aseFile) celY(frame, layer int) int {
	if cel := f.ownCel(frame, layer); cel != nil {
		return cel.y
	}
	return 0
}

// ownCel returns the cel chunk of layer in frame, without following links.
func (f *aseFile) ownCel(frame, layer int) *aseCel {
	for i := range f.frames[frame].cels {
		if f.frames[frame].cels[i].layer == layer {
			return &f.frames[frame].cels[i]
		}
	}
	return nil
}

// animations returns an animation for each tag, and, named "", one of every
// frame in order.
func (f *aseFile) animations() map[string]Animation {
	durations := make([]float32, len(f.frames))
	for i, frame := range f.frames {
		durations[i] = frame.duration
	}
	all := Animation{Durations: durations}
	for i := range f.frames {
		all.Frames = append(all.Frames, i)
	}
	animations := map[string]Animation{"": all}
	for _, tag := range f.tags {
		if _, taken := animations[tag.name]; taken {
			continue // Aseprite plays the first tag of a name too
		}
		frames := tag.frames()
		animation := Animation{Frames: frames, Once: tag.repeat > 0}
		for _, frame := range frames {
			animation.Durations = append(animation.Durations, durations[frame])
		}
		animations[tag.name] = animation
	}
	return animations
}

// frames returns the frames a tag plays, in order: once for each repeat, or
// one loop of a tag that repeats forever. Ping-pong goes back and forth
// without showing the frames at the ends twice in a row.
func (t aseTag) frames() []int {
	forward := func(from, to int) []int {
		var frames []int
		for i := from; i <= to; i++ {
			frames = append(frames, i)
		}
		return frames
	}
	backward := func(high, low int) []int {
		frames := forward(low, high)
		slices.Reverse(frames)
		return frames
	}
	if t.from == t.to {
		return repeatFrames([]int{t.from}, max(t.repeat, 1))
	}
	switch t.direction {
	case 1: // reverse
		pass := backward(t.to, t.from)
		return repeatFrames(pass, max(t.repeat, 1))
	case 2, 3: // ping-pong, and ping-pong starting at the end
		up := t.direction == 2
		var frames []int
		if up {
			frames = forward(t.from, t.to)
		} else {
			frames = backward(t.to, t.from)
		}
		passes := t.repeat
		if passes == 0 {
			passes = 2 // there and back, then loop
		}
		for range passes - 1 {
			up = !up
			if up {
				frames = append(frames, forward(t.from+1, t.to)...)
			} else {
				frames = append(frames, backward(t.to-1, t.from)...)
			}
		}
		if t.repeat == 0 {
			frames = frames[:len(frames)-1] // the loop starts again at the first frame
		}
		return frames
	default: // forward
		return repeatFrames(forward(t.from, t.to), max(t.repeat, 1))
	}
}

// repeatFrames returns pass, times times over, or once for an endless loop.
func repeatFrames(pass []int, times int) []int {
	var frames []int
	for range times {
		frames = append(frames, pass...)
	}
	return frames
}

// decodeAseprite reads an Aseprite file into the sprite's frames, pixels and
// animations. The frames go into one picture, in rows, with a gap between
// them so that a scaled frame never shows the edge of its neighbor.
func (s *Sprite) decodeAseprite(data []byte) error {
	file, err := parseAseprite(data)
	if err != nil {
		return err
	}
	count := len(file.frames)
	const maxWidth = 4096 // every graphics card GoLib runs on takes textures this wide
	columns := max(1, min(count, (maxWidth+1)/(file.width+1)))
	rows := (count + columns - 1) / columns
	s.width, s.height = file.width, file.height
	s.frames = gridFrames(columns, rows, file.width, file.height, 1)[:count]
	s.pixels = image.NewNRGBA(image.Rect(0, 0, columns*(file.width+1)-1, rows*(file.height+1)-1))
	for i := range count {
		frame := file.render(i)
		place := s.frames[i]
		for y := range file.height {
			copy(s.pixels.Pix[s.pixels.PixOffset(place.Min.X, place.Min.Y+y):], frame.Pix[frame.PixOffset(0, y):frame.PixOffset(file.width, y)])
		}
	}
	s.animations = file.animations()
	return nil
}
