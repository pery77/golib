package golib

import "golib/internal/device"

// renderer draws a game into a texture the size of its screen, runs the
// post-processing shaders over it, and puts the result on the window, or into
// another texture for screenshots.
type renderer struct {
	width, height float32
	pixelArt      bool
	scene         device.Target    // what the game's Draw draws
	passes        []device.Target  // results between shaders, loaded when a chain needs them
	shaders       map[*Shader]bool // shaders compiled while this renderer ran
}

func newRenderer(config Config) *renderer {
	r := &renderer{
		width:    float32(config.Width),
		height:   float32(config.Height),
		pixelArt: config.PixelArt,
		shaders:  map[*Shader]bool{},
	}
	r.scene = r.loadTarget()
	return r
}

// loadTarget loads a texture the size of the screen to draw into, smoothed
// when scaled unless the game is pixel art.
func (r *renderer) loadTarget() device.Target {
	return device.NewTarget(int(r.width), int(r.height), !r.pixelArt)
}

// drawScene draws scene into the scene texture, and returns the first mistake
// found while it drew, or while it updated before.
func (r *renderer) drawScene(scene Game, screen *Screen) error {
	device.BeginTarget(r.scene)
	scene.Draw(screen)
	screen.endDraw()
	device.EndTarget()
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

// pass returns intermediate texture number i, loading it the first time.
func (r *renderer) pass(i int) device.Target {
	for len(r.passes) <= i {
		r.passes = append(r.passes, r.loadTarget())
	}
	return r.passes[i]
}

// close frees the textures and the shaders the renderer loaded, and the
// sprites the game drew.
func (r *renderer) close() {
	loadedSprites.unloadAll()
	loadedFonts.unloadAll()
	device.UnloadTarget(r.scene)
	for _, pass := range r.passes {
		device.UnloadTarget(pass)
	}
	for shader := range r.shaders {
		shader.unload()
	}
}
