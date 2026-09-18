package golib

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"path"
	"slices"
	"strings"
	"sync"

	"golib/internal/device"
)

// Sprite is a picture from the game's assets folder, made of one or more
// frames of the same size: a PNG image, a PNG sprite sheet cut into a grid,
// or an Aseprite file with its animations.
//
//	var (
//		tree  = golib.NewSprite("sprites/tree.png")               // one frame: the whole image
//		hero  = golib.NewSpriteSheet("sprites/hero.png", 32, 32) // frames of 32 by 32 pixels
//		slime = golib.NewSprite("sprites/slime.aseprite")        // Aseprite's frames and tags
//	)
//
// Draw a frame with [Screen.DrawSprite]. The file is read the first time the
// sprite is used, so games make their sprites once, as package variables or
// in main, before Run opens the window. A missing file, or one GoLib can't
// read, stops Run with an error that says what to fix.
type Sprite struct {
	name                  string
	gridWidth, gridHeight int // the frame size NewSpriteSheet was given; 0 for NewSprite

	// A sprite that GoLib makes itself, such as a tileset of a map, has a
	// label for messages and a function that fills in its frames and pixels.
	label    string
	generate func(*Sprite) error

	mu         sync.Mutex
	read       bool // the file has been read, successfully or not
	err        error
	width      int // of one frame, in pixels
	height     int
	frames     []image.Rectangle // where each frame is in pixels
	animations map[string]Animation
	pixels     *image.NRGBA // the picture, until it becomes a texture
	texture    device.Texture
	loaded     bool
}

// NewSprite returns the sprite in the game's assets folder named name, which
// is relative to that folder and uses forward slashes, as in [ReadAsset]:
//
//   - A .png file is one frame, the whole image.
//   - An .aseprite or .ase file has the frames Aseprite shows, with its layers
//     combined as Aseprite combines them, and an [Animation] for each tag.
//
// For a PNG sprite sheet, use [NewSpriteSheet].
func NewSprite(name string) *Sprite {
	return &Sprite{name: name}
}

// NewSpriteSheet returns the PNG sprite sheet in the game's assets folder
// named name, cut into frames of frameWidth by frameHeight pixels. Frames are
// numbered from 0, left to right, then top to bottom: in a sheet 4 frames
// wide, frame 5 is the second one on the second row. The image must be a whole
// number of frames wide and high.
//
//	var hero = golib.NewSpriteSheet("sprites/hero.png", 32, 32)
//	var run = golib.Animation{Frames: []int{8, 9, 10, 11}, FrameTime: 0.1}
func NewSpriteSheet(name string, frameWidth, frameHeight int) *Sprite {
	return &Sprite{name: name, gridWidth: frameWidth, gridHeight: frameHeight}
}

// Width returns the width of one frame, in pixels.
func (s *Sprite) Width() float32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.prepare() != nil {
		return 0
	}
	return float32(s.width)
}

// Height returns the height of one frame, in pixels.
func (s *Sprite) Height() float32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.prepare() != nil {
		return 0
	}
	return float32(s.height)
}

// Frames returns how many frames the sprite has. They are numbered from 0.
func (s *Sprite) Frames() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.prepare() != nil {
		return 0
	}
	return len(s.frames)
}

// Animation returns the animation named name: in an Aseprite file, the one
// its tag of that name describes. Every sprite has an animation named "",
// which shows all its frames in order, as long as the Aseprite file says or
// for the default [Animation.FrameTime]. A name the sprite doesn't have stops
// Run with an error that lists the names it has, and returns an animation
// that shows frame 0.
//
//	var slime = golib.NewSprite("sprites/slime.aseprite")
//	var hop = slime.Animation("hop")
func (s *Sprite) Animation(name string) Animation {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.prepare(); err != nil {
		return Animation{}
	}
	animation, found := s.animations[name]
	if !found {
		reportError(fmt.Errorf("%s has no animation named %q: %s", s.call(), name, s.animationNames()))
		return Animation{}
	}
	animation.Frames = slices.Clone(animation.Frames)
	animation.Durations = slices.Clone(animation.Durations)
	return animation
}

// animationNames describes the animations the sprite has, for messages.
func (s *Sprite) animationNames() string {
	if len(s.animations) <= 1 {
		if strings.EqualFold(path.Ext(s.name), ".png") {
			return `PNG files have only the animation named "", of all their frames: make others with golib.Animation{Frames: ...}`
		}
		return `it has only the animation named "", of all its frames: add tags in Aseprite, or make one with golib.Animation{Frames: ...}`
	}
	names := make([]string, 0, len(s.animations))
	for name := range s.animations {
		names = append(names, name)
	}
	slices.Sort(names)
	for i, name := range names {
		names[i] = fmt.Sprintf("%q", name)
	}
	return "it has " + strings.Join(names, ", ")
}

// newGeneratedSprite returns a sprite whose frames and pixels generate fills
// in, called label in messages.
func newGeneratedSprite(label string, generate func(*Sprite) error) *Sprite {
	return &Sprite{label: label, generate: generate}
}

// call returns how the game made the sprite, for messages.
func (s *Sprite) call() string {
	if s.label != "" {
		return s.label
	}
	if s.gridWidth > 0 || s.gridHeight > 0 {
		return fmt.Sprintf("golib.NewSpriteSheet(%q, %d, %d)", s.name, s.gridWidth, s.gridHeight)
	}
	return fmt.Sprintf("golib.NewSprite(%q)", s.name)
}

// prepare reads the file the first time the sprite is used, and reports a
// failure to Run every time it is used. Call it with s.mu held.
func (s *Sprite) prepare() error {
	if !s.read {
		s.read = true
		if err := s.readFile(); err != nil {
			s.err = fmt.Errorf("%s: %w", s.call(), err)
		}
	}
	if s.err != nil {
		reportError(s.err)
	}
	return s.err
}

// readFile reads the sprite's file into its frames and pixels.
func (s *Sprite) readFile() error {
	if s.generate != nil {
		return s.generate(s)
	}
	if s.gridWidth < 0 || s.gridHeight < 0 || (s.gridWidth == 0) != (s.gridHeight == 0) {
		return errors.New("frames must be at least 1 by 1 pixels")
	}
	format := strings.ToLower(path.Ext(s.name))
	if s.gridWidth > 0 && format != ".png" {
		return errors.New("golib.NewSpriteSheet reads PNG files only: Aseprite files have frames of their own, so read them with golib.NewSprite")
	}
	var decode func([]byte) error
	switch format {
	case ".png":
		decode = s.decodePNG
	case ".aseprite", ".ase":
		decode = s.decodeAseprite
	default:
		return fmt.Errorf("GoLib reads sprites from .png, .aseprite and .ase files, not %q", format)
	}
	data, err := ReadAsset(s.name)
	if err != nil {
		return err
	}
	return decode(data)
}

// decodePNG reads a PNG image, whole or cut into a grid.
func (s *Sprite) decodePNG(data []byte) error {
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("not a PNG image GoLib can read (%v): save it again as PNG", err)
	}
	pixels := toNRGBA(decoded)
	width, height := pixels.Rect.Dx(), pixels.Rect.Dy()
	s.pixels = pixels
	if s.gridWidth == 0 {
		s.width, s.height = width, height
		s.frames = []image.Rectangle{pixels.Rect}
		s.animations = allFrames(1)
		return nil
	}
	if width%s.gridWidth != 0 || height%s.gridHeight != 0 {
		return fmt.Errorf("the image is %d by %d pixels, which isn't a whole number of %d by %d frames: check the frame size", width, height, s.gridWidth, s.gridHeight)
	}
	s.width, s.height = s.gridWidth, s.gridHeight
	s.frames = gridFrames(width/s.gridWidth, height/s.gridHeight, s.gridWidth, s.gridHeight, 0)
	s.animations = allFrames(len(s.frames))
	return nil
}

// allFrames returns the animations of a PNG sprite: only the one named "",
// of its count frames in order, at the default frame time.
func allFrames(count int) map[string]Animation {
	var all Animation
	for i := range count {
		all.Frames = append(all.Frames, i)
	}
	return map[string]Animation{"": all}
}

// gridFrames returns the places of columns by rows frames of width by height
// pixels, left to right, then top to bottom, with gap pixels between them.
func gridFrames(columns, rows, width, height, gap int) []image.Rectangle {
	frames := make([]image.Rectangle, 0, columns*rows)
	for row := range rows {
		for column := range columns {
			x, y := column*(width+gap), row*(height+gap)
			frames = append(frames, image.Rect(x, y, x+width, y+height))
		}
	}
	return frames
}

// toNRGBA returns img as 8-bit RGBA pixels that aren't premultiplied, which
// is what raylib loads, with its top-left corner at 0, 0.
func toNRGBA(img image.Image) *image.NRGBA {
	bounds := img.Bounds()
	if nrgba, ok := img.(*image.NRGBA); ok && bounds.Min == (image.Point{}) && nrgba.Stride == 4*bounds.Dx() {
		return nrgba
	}
	pixels := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(pixels, pixels.Rect, img, bounds.Min, draw.Src)
	return pixels
}

// frameTexture returns the texture and the place of frame, loading the
// texture the first time. The window must be open.
func (s *Sprite) frameTexture(frame int) (device.Texture, image.Rectangle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.prepare(); err != nil {
		return device.Texture{}, image.Rectangle{}, err
	}
	if frame < 0 || frame >= len(s.frames) {
		return device.Texture{}, image.Rectangle{}, fmt.Errorf("golib: Screen.DrawSprite got frame %d of %s, which has %d frame(s), numbered from 0 to %d", frame, s.call(), len(s.frames), len(s.frames)-1)
	}
	if !s.loaded {
		pixels := s.pixels
		texture := device.NewTexture(pixels.Pix, pixels.Rect.Dx(), pixels.Rect.Dy())
		if texture.ID == 0 {
			s.err = fmt.Errorf("%s: raylib could not load the %d by %d pixel image as a texture: see the raylib warnings above; it may be larger than the graphics card allows", s.call(), pixels.Rect.Dx(), pixels.Rect.Dy())
			return device.Texture{}, image.Rectangle{}, s.err
		}
		s.texture, s.loaded, s.pixels = texture, true, nil
		loadedSprites.add(s)
	}
	return s.texture, s.frames[frame], nil
}

// frameCount returns how many frames s has, reading its file if needed.
func (s *Sprite) frameCount() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.prepare(); err != nil {
		return 0, err
	}
	return len(s.frames), nil
}

// unload frees the texture, so that the file is read again if a game runs
// again.
func (s *Sprite) unload() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loaded {
		device.UnloadTexture(s.texture)
	}
	s.read, s.err, s.loaded = false, nil, false
	s.width, s.height, s.frames, s.animations = 0, 0, nil, nil
	s.pixels, s.texture = nil, device.Texture{}
}

// loadedSprites are the sprites whose textures are loaded, for Run to free
// when it ends.
var loadedSprites spriteList

type spriteList struct {
	sync.Mutex
	sprites []*Sprite
}

func (l *spriteList) add(s *Sprite) {
	l.Lock()
	defer l.Unlock()
	l.sprites = append(l.sprites, s)
}

// unloadAll frees every loaded sprite. The window must still be open.
func (l *spriteList) unloadAll() {
	l.Lock()
	sprites := l.sprites
	l.sprites = nil
	l.Unlock()
	for _, s := range sprites {
		s.unload()
	}
}

// DrawOptions changes how [Screen.DrawSprite] draws a frame. Fields left at
// their zero value change nothing.
//
//	screen.DrawSprite(hero, frame, x, y, golib.DrawOptions{FlipX: facingLeft, Scale: 2})
type DrawOptions struct {
	Scale float32 // Size, as a multiple of the frame's own. Default: 1.
	FlipX bool    // Mirror the frame left to right, such as a character facing the other way.
	FlipY bool    // Mirror the frame top to bottom.

	// Rotation turns the frame clockwise by this many degrees, around its
	// origin.
	Rotation float32

	// OriginX and OriginY are the point of the frame, in its own pixels from
	// its top-left corner, that DrawSprite puts at x, y, and that Rotation
	// turns around. Default: 0, 0, the top-left corner. Flipping mirrors the
	// frame around its middle, not around the origin.
	OriginX, OriginY float32

	// Tint multiplies the frame's colors; its A makes the frame see-through,
	// such as Color{R: 255, G: 255, B: 255, A: 128} for half. Default: White,
	// which changes nothing.
	Tint Color
}

// DrawSprite draws frame number frame of sprite with its top-left corner at x,
// y, at its own size. Pass one DrawOptions to flip, scale, rotate or tint it.
// x and y are rounded to whole pixels, so that pixel art stays sharp.
// Frames are numbered from 0; a frame the sprite doesn't have stops Run with
// an error.
//
//	s.animationTime += dt // in Update
//	frame := run.Frame(s.animationTime)
//	screen.DrawSprite(hero, frame, s.x, s.y, golib.DrawOptions{FlipX: s.facingLeft})
func (s *Screen) DrawSprite(sprite *Sprite, frame int, x, y float32, options ...DrawOptions) {
	if sprite == nil {
		reportError(errors.New("golib: Screen.DrawSprite got a nil sprite: make sprites with golib.NewSprite or golib.NewSpriteSheet"))
		return
	}
	if len(options) > 1 {
		reportError(fmt.Errorf("golib: Screen.DrawSprite got %d DrawOptions: pass at most one", len(options)))
		return
	}
	texture, place, err := sprite.frameTexture(frame)
	if err != nil {
		reportError(err)
		return
	}
	var option DrawOptions
	if len(options) == 1 {
		option = options[0]
	}
	option.draw(texture, place, x, y)
}

// draw draws the part of texture at place with its origin at x, y.
func (o DrawOptions) draw(texture device.Texture, place image.Rectangle, x, y float32) {
	scale := o.Scale
	if scale == 0 {
		scale = 1
	}
	tint := o.Tint
	if tint == (Color{}) {
		tint = White
	}
	width, height := float32(place.Dx()), float32(place.Dy())
	// raylib flips a part whose width or height is negative.
	source := device.Rectangle{X: float32(place.Min.X), Y: float32(place.Min.Y), Width: width, Height: height}
	if o.FlipX {
		source.Width = -width
	}
	if o.FlipY {
		source.Height = -height
	}
	dest := device.Rectangle{X: wholePixel(x), Y: wholePixel(y), Width: width * scale, Height: height * scale}
	origin := device.Vector2{X: o.OriginX * scale, Y: o.OriginY * scale}
	device.DrawTexture(texture, source, dest, origin, o.Rotation, tint)
}

// Animation is a sequence of a sprite's frames, each shown for a while. Keep
// the time the animation has played in the game's state, and ask which frame
// to draw:
//
//	var run = golib.Animation{Frames: []int{8, 9, 10, 11}, FrameTime: 0.1}
//
//	p.runTime += dt                  // in Update
//	frame := run.Frame(p.runTime)    // in Draw
//
// Aseprite files have their animations already: see [Sprite.Animation].
type Animation struct {
	Frames    []int     // Frame numbers, in the order they show.
	FrameTime float32   // Seconds each frame shows. Default: 0.1.
	Durations []float32 // Seconds for each frame in Frames, one value per frame, when they differ; FrameTime is then ignored. Aseprite animations set them.
	Once      bool      // Play once and stay on the last frame, instead of starting again.
}

// Frame returns the frame to show after the animation has played for time
// seconds. An animation without frames shows frame 0.
func (a Animation) Frame(time float32) int {
	if len(a.Frames) == 0 {
		return 0
	}
	total := a.Duration()
	if total <= 0 || time <= 0 {
		return a.Frames[0]
	}
	if time >= total {
		if a.Once {
			return a.Frames[len(a.Frames)-1]
		}
		time -= total * float32(int(time/total))
	}
	for i, frame := range a.Frames {
		time -= a.frameDuration(i)
		if time < 0 {
			return frame
		}
	}
	return a.Frames[len(a.Frames)-1]
}

// Duration returns the seconds the animation takes to play once.
func (a Animation) Duration() float32 {
	var total float32
	for i := range a.Frames {
		total += a.frameDuration(i)
	}
	return total
}

// Finished reports whether an animation played for time seconds has shown
// all its frames once, such as an attack that should end.
func (a Animation) Finished(time float32) bool {
	return time >= a.Duration()
}

// frameDuration returns how long frame number i of the animation shows.
func (a Animation) frameDuration(i int) float32 {
	if len(a.Durations) == len(a.Frames) {
		return max(a.Durations[i], 0)
	}
	if len(a.Durations) > 0 {
		reportError(fmt.Errorf("golib.Animation has %d Durations for %d Frames: give one duration for each frame, or use FrameTime", len(a.Durations), len(a.Frames)))
	}
	if a.FrameTime > 0 {
		return a.FrameTime
	}
	return 0.1
}
