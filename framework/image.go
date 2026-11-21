package golib

import (
	"fmt"
	"image"
	"sync"

	"golib/internal/device"
)

// Image is a picture the game makes pixel by pixel as it runs, and draws with
// [Screen.DrawImage]: a screen of an emulator or a fantasy console, a
// plasma effect, a minimap, a picture a player paints. Pictures that come
// from files are sprites: see [NewSprite].
//
//	var pixels = golib.NewImage(64, 64)
//
//	pixels.SetPixel(10, 20, golib.Green)                    // in Update
//	screen.DrawImage(pixels, 0, 0, golib.DrawOptions{Scale: 4}) // in Draw
//
// Changing pixels is plain Go, and cheap: the picture goes to the graphics
// card once, in the next DrawImage after a change, however many pixels
// changed. It is drawn without smoothing, so its pixels stay square and
// sharp at any scale, unless DrawOptions.Smooth asks for it: then it is
// blended between its pixels, for a picture that stands for something
// smooth at a coarser grid, such as fog of war or a heat map.
type Image struct {
	mu            sync.Mutex
	width, height int
	pixels        []byte  // RGBA, one byte a channel, the top row first
	sharp, smooth texture // the picture on the graphics card, unsmoothed and smoothed, each made when first drawn so
}

// texture is a picture's pixels on the graphics card, filtered one way.
type texture struct {
	texture device.Texture
	loaded  bool
	changed bool // pixels changed since they went to the texture
}

// maxImageSide is the widest and tallest an Image can be: every graphics
// card GoLib runs on takes a texture this large.
const maxImageSide = 4096

// NewImage returns a picture width by height pixels, every pixel of it
// transparent, Color{}, until the game sets it. A size below 1 or above 4096
// stops Run with an error the first time the picture is drawn.
func NewImage(width, height int) *Image {
	img := &Image{width: width, height: height}
	if img.sizeError() == nil {
		img.pixels = make([]byte, 4*width*height)
	}
	return img
}

// sizeError says what is wrong with the picture's size, or returns nil.
func (img *Image) sizeError() error {
	if img.width < 1 || img.height < 1 || img.width > maxImageSide || img.height > maxImageSide {
		return fmt.Errorf("golib.NewImage(%d, %d): a picture is 1 to %d pixels wide and high", img.width, img.height, maxImageSide)
	}
	return nil
}

// Width returns how many pixels wide the picture is.
func (img *Image) Width() int { return img.width }

// Height returns how many pixels high the picture is.
func (img *Image) Height() int { return img.height }

// SetPixel sets the pixel at x, y, counted from 0, 0 at the top-left corner.
// A pixel outside the picture is left alone, so drawing can run off its
// edges.
func (img *Image) SetPixel(x, y int, color Color) {
	if !img.holds(x, y) {
		return
	}
	img.mu.Lock()
	defer img.mu.Unlock()
	at := img.pixels[4*(y*img.width+x):]
	if at[0] != color.R || at[1] != color.G || at[2] != color.B || at[3] != color.A {
		at[0], at[1], at[2], at[3] = color.R, color.G, color.B, color.A
		img.sharp.changed, img.smooth.changed = true, true
	}
}

// Pixel returns the color of the pixel at x, y, or Color{} outside the
// picture.
func (img *Image) Pixel(x, y int) Color {
	if !img.holds(x, y) {
		return Color{}
	}
	img.mu.Lock()
	defer img.mu.Unlock()
	at := img.pixels[4*(y*img.width+x):]
	return Color{R: at[0], G: at[1], B: at[2], A: at[3]}
}

// holds reports whether x, y is a pixel of the picture. A picture of a size
// NewImage doesn't take has none.
func (img *Image) holds(x, y int) bool {
	return img.pixels != nil && x >= 0 && y >= 0 && x < img.width && y < img.height
}

// Clear sets every pixel of the picture to color.
func (img *Image) Clear(color Color) {
	img.mu.Lock()
	defer img.mu.Unlock()
	for at := 0; at < len(img.pixels); at += 4 {
		img.pixels[at], img.pixels[at+1], img.pixels[at+2], img.pixels[at+3] = color.R, color.G, color.B, color.A
	}
	img.sharp.changed, img.smooth.changed = true, true
}

// DrawImage draws a picture made with NewImage with its top-left corner at x,
// y, one screen pixel for each of its pixels. Pass one DrawOptions to scale,
// flip, rotate or tint it, as with DrawSprite. x and y are rounded to whole
// pixels, and the picture isn't smoothed, so its pixels stay sharp, unless
// DrawOptions.Smooth is true: then each pixel's color is at its middle and
// blended into its neighbours' between them, with no mipmaps, and x and y
// are kept as they are. A see-through pixel blends its color too, so give
// one beside opaque ones their color, or edges darken.
func (s *Screen) DrawImage(img *Image, x, y float32, options ...DrawOptions) {
	if img == nil {
		reportError(fmt.Errorf("golib: Screen.DrawImage got a nil picture: make pictures with golib.NewImage"))
		return
	}
	if len(options) > 1 {
		reportError(fmt.Errorf("golib: Screen.DrawImage got %d DrawOptions: pass at most one", len(options)))
		return
	}
	var option DrawOptions
	if len(options) == 1 {
		option = options[0]
	}
	texture, err := img.upload(option.Smooth)
	if err != nil {
		reportError(err)
		return
	}
	option.draw(texture, image.Rect(0, 0, img.width, img.height), x, y)
}

// upload puts the picture on the graphics card, smoothed or not, the first
// time it is drawn so, and its pixels again after they change.
func (img *Image) upload(smooth bool) (device.Texture, error) {
	if err := img.sizeError(); err != nil {
		return device.Texture{}, err
	}
	img.mu.Lock()
	defer img.mu.Unlock()
	t, newTexture := &img.sharp, device.NewTexture
	if smooth {
		t, newTexture = &img.smooth, device.NewLinearTexture
	}
	switch {
	case !t.loaded:
		t.texture = newTexture(img.pixels, img.width, img.height)
		if t.texture.ID == 0 {
			return device.Texture{}, fmt.Errorf("golib.NewImage(%d, %d): the graphics card could not take the picture: see the raylib warnings above", img.width, img.height)
		}
		t.loaded, t.changed = true, false
		if !img.sharp.loaded || !img.smooth.loaded { // the first of the two
			loadedImages.add(img)
		}
	case t.changed:
		device.UpdateTexture(t.texture, img.pixels)
		t.changed = false
	}
	return t.texture, nil
}

// unload frees the picture's texture; its pixels stay, and go to the
// graphics card again if a game runs again.
func (img *Image) unload() {
	img.mu.Lock()
	defer img.mu.Unlock()
	for _, t := range []*texture{&img.sharp, &img.smooth} {
		if t.loaded {
			device.UnloadTexture(t.texture)
		}
		*t = texture{}
	}
}

// loadedImages are the pictures on the graphics card, for Run to free when
// it ends.
var loadedImages imageList

type imageList struct {
	sync.Mutex
	images []*Image
}

func (l *imageList) add(img *Image) {
	l.Lock()
	defer l.Unlock()
	l.images = append(l.images, img)
}

// unloadAll frees every picture's texture. The window must still be open.
func (l *imageList) unloadAll() {
	l.Lock()
	images := l.images
	l.images = nil
	l.Unlock()
	for _, img := range images {
		img.unload()
	}
}
