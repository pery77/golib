package golib

import (
	"math"

	"golib/internal/device"
)

// renderer draws a game into a texture the size of its screen, runs the
// post-processing shaders over it, and puts the result on the window, or into
// another texture for screenshots, with what goes over it at the window's
// resolution: the frames drawn with DrawOptions.FullResolution and the mouse
// sprite.
//
// With Config.Antialias the game draws into a texture samples times as wide
// and high, in screen pixels still, and present first shrinks it to the
// screen's size, so every later step sees the screen as it would without it.
type renderer struct {
	width, height float32
	pixelArt      bool
	samples       float32          // texture pixels a screen pixel, each way: 1, or 2 with Antialias
	scene         device.Target    // what the game's Draw draws
	resolved      device.Target    // the scene shrunk to the screen's size, loaded when samples > 1
	passes        []device.Target  // results between shaders, loaded when a chain needs them
	shaders       map[*Shader]bool // shaders compiled while this renderer ran
	over          []overDraw       // what the last Draw drew at the window's resolution
	pointer       pointerDraw      // the mouse sprite, which the loop sets before present
}

// overDraw is a frame drawn with DrawOptions.FullResolution, waiting to go
// over the screen once the screen is in the window. Its rectangles and origin
// are in screen pixels.
type overDraw struct {
	texture      device.Texture
	source, dest device.Rectangle
	origin       device.Vector2
	rotation     float32
	tint         Color
}

// textScale is how many texture pixels a screen pixel gets while a Draw runs:
// fonts draw their letters that many times larger, so text stays as sharp as
// the shapes around it with Config.Antialias. It is 1 outside Draw.
var textScale float32 = 1

// antialiasSamples is how many texture pixels a screen pixel gets, each way,
// with Config.Antialias: four in all, blended into one.
const antialiasSamples = 2

func newRenderer(config Config) *renderer {
	r := &renderer{
		width:    float32(config.Width),
		height:   float32(config.Height),
		pixelArt: config.PixelArt,
		samples:  1,
		shaders:  map[*Shader]bool{},
	}
	if config.Antialias && !config.PixelArt {
		r.samples = antialiasSamples
	}
	r.loadScene()
	return r
}

// loadTarget loads a texture the size of the screen to draw into, smoothed
// when scaled unless the game is pixel art.
func (r *renderer) loadTarget() device.Target {
	return device.NewTarget(int(r.width), int(r.height), !r.pixelArt)
}

// loadScene loads the texture the game draws into: the screen's size, or
// samples times larger with its shrunk copy beside it.
func (r *renderer) loadScene() {
	if r.samples == 1 {
		r.scene = r.loadTarget()
		return
	}
	r.scene = device.NewTarget(int(r.width*r.samples), int(r.height*r.samples), true)
	r.resolved = r.loadTarget()
}

// unloadScene frees what loadScene loaded.
func (r *renderer) unloadScene() {
	device.UnloadTarget(r.scene)
	if r.samples > 1 {
		device.UnloadTarget(r.resolved)
	}
}

// resize makes the screen's textures width by height pixels, for a game with
// Config.FillWindow or Config.WindowScale whose window changed size. Nothing
// happens at the size they have.
func (r *renderer) resize(width, height float32) {
	if width == r.width && height == r.height {
		return
	}
	r.unloadScene()
	for _, pass := range r.passes {
		device.UnloadTarget(pass)
	}
	r.width, r.height, r.passes = width, height, nil
	r.loadScene()
}

// drawScene draws scene into the scene texture, and returns the first mistake
// found while it drew, or while it updated before.
func (r *renderer) drawScene(scene Game, screen *Screen) error {
	device.BeginTarget(r.scene)
	if r.samples > 1 {
		device.SetDrawSize(r.width, r.height)
	}
	textScale = r.samples
	scene.Draw(screen)
	screen.endDraw()
	textScale = 1
	device.EndTarget()
	// What goes at the window's resolution waits for present.
	r.over, screen.over = screen.over, r.over[:0]
	return takeError()
}

// present draws the scene texture, through the post-processing shaders, into
// fit: a rectangle of the window, or of picture when picture isn't nil. time
// is the game time in seconds, for the shaders.
func (r *renderer) present(picture *device.Target, fit device.Rectangle, time float32) error {
	shaders, err := currentPostProcess()
	if err != nil {
		return err
	}
	for _, shader := range shaders {
		if err := shader.load(); err != nil {
			return err
		}
		r.shaders[shader] = true
	}

	// Every shader but the last draws into a texture the size of the screen,
	// and the next shader reads that texture.
	source := device.TargetTexture(r.scene)
	whole := device.Rectangle{Width: r.width, Height: r.height}
	if r.samples > 1 {
		// Shrink the scene to the screen's size first. Drawn at half its size
		// with smoothing, every screen pixel lands between four of its pixels
		// and gets their average.
		device.BeginTarget(r.resolved)
		device.Clear(device.Blank)
		r.drawThrough(nil, source, whole, time)
		device.EndTarget()
		source = device.TargetTexture(r.resolved)
	}
	for i := 0; i < len(shaders)-1; i++ {
		target := r.pass(i % 2)
		device.BeginTarget(target)
		device.Clear(device.Blank)
		r.drawThrough(shaders[i], source, whole, time)
		device.EndTarget()
		source = device.TargetTexture(target)
	}

	var last *Shader
	if len(shaders) > 0 {
		last = shaders[len(shaders)-1]
	}
	if picture != nil {
		device.BeginTarget(*picture)
	} else {
		device.BeginFrame()
	}
	device.Clear(device.Black) // the bars around a screen that doesn't fill the window
	r.drawThrough(last, source, fit, time)
	r.drawOver(fit)
	if picture != nil {
		device.EndTarget()
	} else {
		device.EndFrame()
	}
	return nil
}

// drawThrough draws texture into dest, through shader unless it is nil.
func (r *renderer) drawThrough(shader *Shader, texture device.Texture, dest device.Rectangle, time float32) {
	// Copy the pixels as they are instead of blending them. A game that draws
	// see-through shapes leaves alpha below 1 in its texture, and blending that
	// over the black bars would darken those shapes.
	device.BeginBlendCopy()
	if shader != nil {
		device.BeginShader(shader.shader)
		shader.apply(time, r.width, r.height, dest.Width, dest.Height)
	}
	// Render textures are stored upside down; a negative source height flips
	// them back.
	source := device.Rectangle{Width: float32(texture.Width), Height: -float32(texture.Height)}
	device.DrawTexture(texture, source, dest, device.Vector2{}, 0, device.White)
	if shader != nil {
		device.EndShader()
	}
	device.EndBlend()
}

// drawOver draws over the screen, now in fit, what goes there at the
// window's resolution: the frames the last Draw drew with
// DrawOptions.FullResolution, in their order, and then the mouse sprite.
func (r *renderer) drawOver(fit device.Rectangle) {
	scale := fit.Width / r.width // window pixels a screen pixel
	if len(r.over) > 0 {
		device.BeginBlendPremultiplied()
		for _, d := range r.over {
			dest := device.Rectangle{X: fit.X + d.dest.X*scale, Y: fit.Y + d.dest.Y*scale, Width: d.dest.Width * scale, Height: d.dest.Height * scale}
			device.DrawTexture(d.texture, d.source, dest, device.Vector2{X: d.origin.X * scale, Y: d.origin.Y * scale}, d.rotation, d.tint)
		}
		device.EndBlend()
	}
	r.pointer.draw(fit, scale)
}

// pointerDraw is the mouse sprite as a frame draws it, with the pointer at x,
// y in screen pixels, unrounded. A nil sprite draws nothing.
type pointerDraw struct {
	sprite     *Sprite
	frame      int
	tipX, tipY float32 // the frame's pixel the pointer points with
	x, y       float32
}

// pointerAt returns the mouse sprite to draw with the pointer at x, y in
// screen pixels: none without one, while the pointer is outside the window,
// while SetMouseVisible hides it, or while the player plays with a gamepad
// or with fingers, and so doesn't point with it.
func pointerAt(x, y float32, inWindow bool) pointerDraw {
	if !inWindow || mouseHiddenWanted.Load() || playingWithGamepad.Load() || playingWithTouch.Load() {
		return pointerDraw{}
	}
	mouseSprite.Lock()
	defer mouseSprite.Unlock()
	return pointerDraw{sprite: mouseSprite.sprite, frame: mouseSprite.frame, tipX: mouseSprite.tipX, tipY: mouseSprite.tipY, x: x, y: y}
}

// draw draws the mouse sprite over the screen in fit, scale window pixels a
// screen pixel: as many whole times larger as is nearest, at least once, so
// its pixels stay square, at whole pixels of the window, with its tip on the
// pointer, between screen pixels too, so it moves as smoothly as the
// system's pointer.
func (p pointerDraw) draw(fit device.Rectangle, scale float32) {
	if p.sprite == nil {
		return
	}
	texture, place, err := p.sprite.frameTexture(p.frame)
	if err != nil {
		return // SetMouseSprite checked the frame, and a file that can't be read is reported
	}
	source := device.Rectangle{X: float32(place.Min.X), Y: float32(place.Min.Y), Width: float32(place.Dx()), Height: float32(place.Dy())}
	device.DrawTexture(texture, source, p.dest(fit, scale, source.Width, source.Height), device.Vector2{}, 0, White)
}

// dest returns where draw puts a frame width by height pixels in the window.
func (p pointerDraw) dest(fit device.Rectangle, scale, width, height float32) device.Rectangle {
	times := max(1, float32(math.Round(float64(scale))))
	return device.Rectangle{
		X:      float32(math.Round(float64(fit.X + p.x*scale - p.tipX*times))),
		Y:      float32(math.Round(float64(fit.Y + p.y*scale - p.tipY*times))),
		Width:  width * times,
		Height: height * times,
	}
}

// pass returns intermediate texture number i, loading it the first time.
func (r *renderer) pass(i int) device.Target {
	for len(r.passes) <= i {
		r.passes = append(r.passes, r.loadTarget())
	}
	return r.passes[i]
}

// close frees the textures and the shaders the renderer loaded, and the
// sprites and pictures the game drew.
func (r *renderer) close() {
	loadedSprites.unloadAll()
	loadedImages.unloadAll()
	loadedFonts.unloadAll()
	r.unloadScene()
	for _, pass := range r.passes {
		device.UnloadTarget(pass)
	}
	for shader := range r.shaders {
		shader.unload()
	}
}
