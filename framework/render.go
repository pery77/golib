package golib

import rl "github.com/gen2brain/raylib-go/raylib"

// renderer draws a game into a texture the size of its screen, runs the
// post-processing shaders over it, and puts the result on the window, or into
// another texture for screenshots.
type renderer struct {
	width, height float32
	pixelArt      bool
	scene         rl.RenderTexture2D   // what the game's Draw draws
	passes        []rl.RenderTexture2D // results between shaders, loaded when a chain needs them
	shaders       map[*Shader]bool     // shaders compiled while this renderer ran
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
func (r *renderer) loadTarget() rl.RenderTexture2D {
	target := rl.LoadRenderTexture(int32(r.width), int32(r.height))
	filter := rl.FilterBilinear
	if r.pixelArt {
		filter = rl.FilterPoint
	}
	rl.SetTextureFilter(target.Texture, filter)
	return target
}

// drawScene draws scene into the scene texture, and returns the first mistake
// found while it drew, or while it updated before.
func (r *renderer) drawScene(scene Game, screen *Screen) error {
	rl.BeginTextureMode(r.scene)
	scene.Draw(screen)
	screen.endCamera()
	rl.EndTextureMode()
	return takeError()
}

// present draws the scene texture, through the post-processing shaders, into
// fit: a rectangle of the window, or of picture when picture isn't nil. time
// is the game time in seconds, for the shaders.
func (r *renderer) present(picture *rl.RenderTexture2D, fit rl.Rectangle, time float32) error {
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
	source := r.scene.Texture
	whole := rl.Rectangle{Width: r.width, Height: r.height}
	for i := 0; i < len(shaders)-1; i++ {
		target := r.pass(i % 2)
		rl.BeginTextureMode(target)
		rl.ClearBackground(rl.Blank)
		r.drawThrough(shaders[i], source, whole, time)
		rl.EndTextureMode()
		source = target.Texture
	}

	var last *Shader
	if len(shaders) > 0 {
		last = shaders[len(shaders)-1]
	}
	if picture != nil {
		rl.BeginTextureMode(*picture)
	} else {
		rl.BeginDrawing()
	}
	rl.ClearBackground(rl.Black) // the bars around a screen that doesn't fill the window
	r.drawThrough(last, source, fit, time)
	if picture != nil {
		rl.EndTextureMode()
	} else {
		rl.EndDrawing()
	}
	return nil
}

// OpenGL blend values, for rl.SetBlendFactors.
const (
	glZero    = 0
	glOne     = 1
	glFuncAdd = 0x8006
)

// drawThrough draws texture into dest, through shader unless it is nil.
func (r *renderer) drawThrough(shader *Shader, texture rl.Texture2D, dest rl.Rectangle, time float32) {
	// Copy the pixels as they are instead of blending them. A game that draws
	// see-through shapes leaves alpha below 1 in its texture, and blending that
	// over the black bars would darken those shapes.
	rl.SetBlendFactors(glOne, glZero, glFuncAdd)
	rl.BeginBlendMode(rl.BlendCustom)
	if shader != nil {
		rl.BeginShaderMode(shader.shader)
		shader.apply(time, r.width, r.height, dest.Width, dest.Height)
	}
	// Render textures are stored upside down; a negative source height flips
	// them back.
	source := rl.Rectangle{Width: float32(texture.Width), Height: -float32(texture.Height)}
	rl.DrawTexturePro(texture, source, dest, rl.Vector2{}, 0, rl.White)
	if shader != nil {
		rl.EndShaderMode()
	}
	rl.EndBlendMode()
}

// pass returns intermediate texture number i, loading it the first time.
func (r *renderer) pass(i int) rl.RenderTexture2D {
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
	rl.UnloadRenderTexture(r.scene)
	for _, pass := range r.passes {
		rl.UnloadRenderTexture(pass)
	}
	for shader := range r.shaders {
		shader.unload()
	}
}
