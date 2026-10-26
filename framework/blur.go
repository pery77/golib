package golib

import (
	"math"

	"golib/internal/device"
)

// Blur: the whole pass's picture softened, as a camera out of focus sees it,
// for a menu over a world that goes on behind it. As bloom does, the picture
// is shrunk to half a few times, each step averaging four smoothed samples,
// then grown back a step at a time through a tent of three by three; unlike
// bloom, each step replaces the larger picture instead of adding to it, and
// opacity is kept, so the 2D under the pass shows through where the pass
// drew nothing. The compose shader then reads the half-size result instead
// of the pass's own picture. It costs little: every step after the first is
// a quarter of the one before.

// maxBlurLevels is how many times the picture can be halved: 2⁷ is wider
// than any blur a game asks for.
const maxBlurLevels = 7

// blurLevels returns how many halvings soften a picture by about blur
// pixels: each doubles how far the softening reaches, from about one and a
// half pixels for one.
func blurLevels(blur float32) int {
	if blur <= 0 {
		return 0
	}
	levels := int(math.Round(math.Log2(float64(blur) / 1.5)))
	return max(1, min(levels, maxBlurLevels))
}

const (
	blurDownShader = `#version 330
in vec2 texcoord;
uniform sampler2D source;
uniform vec2 texel;
out vec4 fragColor;

void main()
{
    // Four smoothed samples: sixteen texels, averaged, opacity and all.
    fragColor = (texture(source, texcoord + texel * vec2(-1.0, -1.0))
        + texture(source, texcoord + texel * vec2(1.0, -1.0))
        + texture(source, texcoord + texel * vec2(-1.0, 1.0))
        + texture(source, texcoord + texel * vec2(1.0, 1.0))) * 0.25;
}
`
	blurUpShader = `#version 330
in vec2 texcoord;
uniform sampler2D source;
uniform vec2 texel;
out vec4 fragColor;

void main()
{
    // A tent of three by three, opacity and all.
    fragColor = (texture(source, texcoord) * 4.0
        + (texture(source, texcoord + texel * vec2(-1.0, 0.0))
         + texture(source, texcoord + texel * vec2(1.0, 0.0))
         + texture(source, texcoord + texel * vec2(0.0, -1.0))
         + texture(source, texcoord + texel * vec2(0.0, 1.0))) * 2.0
        + texture(source, texcoord + texel * vec2(-1.0, -1.0))
        + texture(source, texcoord + texel * vec2(1.0, -1.0))
        + texture(source, texcoord + texel * vec2(-1.0, 1.0))
        + texture(source, texcoord + texel * vec2(1.0, 1.0))) / 16.0;
}
`
)

// blurProgram is one of blur's two programs.
type blurProgram struct {
	program       device.Program
	source, texel int32
}

// blurChain is the halved pictures, for one size of pass.
type blurChain struct {
	width, height int
	levels        []device.WorkTarget
}

// soften draws the pass's picture, softened by levels halvings, into
// r.blur.levels[0], half the pass's size, for a pass width by height texels,
// and leaves the pass's own target bound.
func (r *state3D) soften(width, height, levels int) {
	if r.blur.width != width || r.blur.height != height {
		r.unloadBlur()
		r.blur = blurChain{width: width, height: height}
		w, h := width, height
		for range maxBlurLevels {
			w, h = max(1, w/2), max(1, h/2)
			r.blur.levels = append(r.blur.levels, device.NewWorkTarget(w, h))
		}
	}
	chain := r.blur.levels[:levels]
	down, up := r.blurDown, r.blurUp
	device.SetCulling(false)
	device.UseProgram(down.program)
	device.SetUniformInt(down.source, 0)
	for i, level := range chain {
		device.BeginWork(level, true)
		if i == 0 {
			device.BindPassPicture(0)
			device.SetUniform(down.texel, 2, [4]float32{1 / float32(width), 1 / float32(height)})
		} else {
			device.BindWorkTarget(0, chain[i-1])
			device.SetUniform(down.texel, 2, [4]float32{1 / float32(chain[i-1].Width), 1 / float32(chain[i-1].Height)})
		}
		device.DrawFullscreen()
	}
	device.UseProgram(up.program)
	device.SetUniformInt(up.source, 0)
	for i := len(chain) - 2; i >= 0; i-- {
		device.BeginWork(chain[i], true)
		device.BindWorkTarget(0, chain[i+1])
		device.SetUniform(up.texel, 2, [4]float32{1 / float32(chain[i+1].Width), 1 / float32(chain[i+1].Height)})
		device.DrawFullscreen()
	}
	device.EndWork()
	device.SetCulling(true)
}

// unloadBlur frees the halved pictures.
func (r *state3D) unloadBlur() {
	for _, l := range r.blur.levels {
		device.UnloadWorkTarget(l)
	}
	r.blur = blurChain{}
}
