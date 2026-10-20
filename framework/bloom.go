package golib

import "golib/internal/device"

// Bloom: what is brighter than white - a glowing material, the sun shining
// off something shiny, a lamp close to a wall - spills a soft glow around
// it, as it does in a camera's lens. The pass's picture is shrunk to half,
// keeping only what is brighter than white, then to half again, five more
// times; each smaller picture, smoothed, is added back onto the larger one;
// and the compose shader adds the result to the picture.

// bloomLevels is how many times the glow is shrunk: enough for a glow
// across a fortieth of the screen.
const bloomLevels = 6

const (
	bloomDownShader = `#version 330
in vec2 texcoord;
uniform sampler2D source;
uniform vec2 texel;
uniform int first;
out vec4 fragColor;

// A sample, and, from the pass's own picture, no brighter than 16: a sun
// thousands of times brighter than white would flood the screen with glow.
vec3 sampled(vec2 at)
{
    vec3 c = texture(source, at).rgb;
    float bright = max(c.r, max(c.g, c.b));
    return first == 1 && bright > 16.0 ? c * (16.0 / bright) : c;
}

void main()
{
    // Four smoothed samples: sixteen texels, averaged.
    vec3 c = (sampled(texcoord + texel * vec2(-1.0, -1.0))
        + sampled(texcoord + texel * vec2(1.0, -1.0))
        + sampled(texcoord + texel * vec2(-1.0, 1.0))
        + sampled(texcoord + texel * vec2(1.0, 1.0))) * 0.25;
    if (first == 1) {
        // Only what is brighter than white glows, starting softly from 0.8.
        float bright = max(c.r, max(c.g, c.b));
        float knee = clamp(bright - 0.8, 0.0, 0.4);
        float glow = max(knee * knee / 0.8, bright - 1.0) / max(bright, 1e-4);
        c *= glow;
    }
    fragColor = vec4(c, 1.0);
}
`
	bloomUpShader = `#version 330
in vec2 texcoord;
uniform sampler2D source;
uniform vec2 texel;
out vec4 fragColor;

void main()
{
    // A tent of three by three, added onto the larger picture.
    vec3 c = texture(source, texcoord).rgb * 4.0
        + (texture(source, texcoord + texel * vec2(-1.0, 0.0)).rgb
         + texture(source, texcoord + texel * vec2(1.0, 0.0)).rgb
         + texture(source, texcoord + texel * vec2(0.0, -1.0)).rgb
         + texture(source, texcoord + texel * vec2(0.0, 1.0)).rgb) * 2.0
        + texture(source, texcoord + texel * vec2(-1.0, -1.0)).rgb
        + texture(source, texcoord + texel * vec2(1.0, -1.0)).rgb
        + texture(source, texcoord + texel * vec2(-1.0, 1.0)).rgb
        + texture(source, texcoord + texel * vec2(1.0, 1.0)).rgb;
    fragColor = vec4(c / 16.0, 1.0);
}
`
)

// bloomProgram is one of bloom's two programs.
type bloomProgram struct {
	program              device.Program
	source, texel, first int32
}

// bloomChain is the shrinking pictures, for one size of pass.
type bloomChain struct {
	width, height int
	levels        []device.WorkTarget
}

// glow draws the pass's glow into r.bloom.levels[0], for a pass width by
// height texels, and leaves the pass's own target bound.
func (r *state3D) glow(width, height int) {
	if r.bloom.width != width || r.bloom.height != height {
		for _, l := range r.bloom.levels {
			device.UnloadWorkTarget(l)
		}
		r.bloom = bloomChain{width: width, height: height}
		w, h := width, height
		for range bloomLevels {
			w, h = max(1, w/2), max(1, h/2)
			r.bloom.levels = append(r.bloom.levels, device.NewWorkTarget(w, h))
		}
	}
	levels := r.bloom.levels
	down, up := r.bloomDown, r.bloomUp
	device.SetCulling(false)
	device.UseProgram(down.program)
	device.SetUniformInt(down.source, 0)
	for i, level := range levels {
		device.BeginWork(level, true)
		if i == 0 {
			device.BindPassPicture(0)
			device.SetUniform(down.texel, 2, [4]float32{1 / float32(width), 1 / float32(height)})
		} else {
			device.BindWorkTarget(0, levels[i-1])
			device.SetUniform(down.texel, 2, [4]float32{1 / float32(levels[i-1].Width), 1 / float32(levels[i-1].Height)})
		}
		device.SetUniformInt(down.first, boolInt(i == 0))
		device.DrawFullscreen()
	}
	device.UseProgram(up.program)
	device.SetUniformInt(up.source, 0)
	device.SetAdditive3D(true)
	for i := len(levels) - 2; i >= 0; i-- {
		device.BeginWork(levels[i], false)
		device.BindWorkTarget(0, levels[i+1])
		device.SetUniform(up.texel, 2, [4]float32{1 / float32(levels[i+1].Width), 1 / float32(levels[i+1].Height)})
		device.DrawFullscreen()
	}
	device.SetAdditive3D(false)
	device.EndWork()
	device.SetCulling(true)
}

// unloadBloom frees the shrinking pictures.
func (r *state3D) unloadBloom() {
	for _, l := range r.bloom.levels {
		device.UnloadWorkTarget(l)
	}
	r.bloom = bloomChain{}
}
