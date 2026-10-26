package golib

import (
	"fmt"
	"strings"

	"golib/internal/device"
)

// Haze is air that catches light, Environment.Haze: every pixel's way from
// the camera to what it shows is walked in steps, and at each the haze
// takes some of the light that comes from behind and gives back the light
// of the sun, the lamps and the sky that reaches it there. Where the sun's
// or a lamp's shadow falls on the haze, it gives back none of theirs, so
// light through a window or between trees shows as shafts, and a spotlight
// as a beam. The shadows reach as far as the sun's do,
// Environment.ShadowDistance; past it the haze is lit without them.
//
// The haze is drawn over everything solid, the sky and water, before what
// is see-through and the particles, which it doesn't cover. It is walked at
// half the pass's width and height, each pixel's steps starting at a
// different place along them, and smoothed back over the pass by its
// depth, so a narrow beam the steps of one pixel miss shows from its
// neighbours', and no haze spills across an edge.

// hazeShader is the haze's part of the lit fragment shader, compiled with
// HAZE in place of the surface's main: it draws the light the haze gives
// back along each pixel's way, and how much of what is behind still shows,
// into a picture of their own, from a copy of the pass's depth.
const hazeShader = `#ifdef HAZE
uniform sampler2D depthCopy;
uniform mat4 inverseViewProjection;
uniform vec2 viewSize;
uniform vec4 haze;        // its color, linear, and how much light it takes in a meter
uniform float hazeHeight; // the height it lies below, or 0 for everywhere
uniform float hazeReach;  // how far it is walked in steps with shadows

// How thick the haze is at p.
float hazeAt(vec3 p)
{
    if (hazeHeight <= 0.0) return haze.w;
    float thinning = max(1.0, hazeHeight * 0.25);
    return haze.w * clamp(1.0 - (p.y - hazeHeight) / thinning, 0.0, 1.0);
}

// How much of the light that travels at cosine c to the way back to the
// eye the haze sends there: Henyey and Greenstein's, more ahead, as dust
// and droplets do, and 1 on average over every way.
float hazePhase(float c)
{
    const float g = 0.4;
    return (1.0 - g * g) / pow(1.0 + g * g - 2.0 * g * c, 1.5);
}

void main()
{
    vec2 uv = gl_FragCoord.xy / viewSize;
    float depth = texture(depthCopy, uv).r;
    vec4 w = inverseViewProjection * vec4(uv * 2.0 - 1.0, depth * 2.0 - 1.0, 1.0);
    vec3 ray = w.xyz / w.w - eye;
    float far = length(ray);
    vec3 dir = ray / max(far, 1e-4);
    float near = min(far, hazeReach);
    float sunPhase = hazePhase(dot(dir, -sunDirection));
    vec3 sky = hasEnvironment == 1
        ? 0.5 * (surrounding(vec3(0.0, 1.0, 0.0)) + surrounding(vec3(0.0, -1.0, 0.0)))
        : 0.5 * (skyColor + groundColor);
    vec3 around = sky;
    // Each step sampled at the same place along it, different from pixel
    // to pixel, so the steps show as fine noise instead of bands.
    float jitter = fract(52.9829189 * fract(dot(gl_FragCoord.xy, vec2(0.06711056, 0.00583715))));
    vec3 gathered = vec3(0.0);
    float through = 1.0;
    // Near, where shadows fall, in 32 steps, shorter close to the camera,
    // where the eye sees more of them; past it, in 8 without shadows.
    for (int i = 0; i < 40; i++) {
        bool shadowed = i < 32;
        float a = float(i) / 32.0, b = float(i + 1) / 32.0;
        float from = shadowed ? near * a * a : near + (far - near) * float(i - 32) / 8.0;
        float to = shadowed ? near * b * b : near + (far - near) * float(i - 31) / 8.0;
        float span = to - from;
        if (span <= 0.0) continue;
        vec3 p = eye + dir * (from + span * jitter);
        float density = hazeAt(p);
        if (density <= 0.0) continue;
        worldPosition = p;
        if (hasProbes == 1 && (i & 3) == 0) {
            // Inside a level, the light its probes hold, not the sky's.
            vec4 probe = probeLight(p, vec3(0.0, 1.0, 0.0));
            around = mix(sky, probe.rgb, probe.a);
        }
        float sun = 1.0;
        if (shadowed && hasShadows == 1) {
            float d = dot(p - eye, viewForward);
            if (d < shadowSplits.z) {
                int slice = d < shadowSplits.x ? 0 : (d < shadowSplits.y ? 1 : 2);
                sun = mix(sunShadowAt(p, slice, 0), 1.0, smoothstep(shadowSplits.z * 0.85, shadowSplits.z, d));
            }
        }
        vec3 light = around + sunColor * sunPhase * sun;
        for (int k = 0; k < 8; k++) {
            if (k >= lightCount) break;
            vec3 toLight = lightPositions[k].xyz - p;
            float distance = length(toLight);
            vec3 l = toLight / max(distance, 1e-4);
            float reach = distance / lightPositions[k].w;
            if (reach >= 1.0) continue;
            float fade = 1.0 - reach * reach * reach * reach;
            float strength = fade * fade / max(distance * distance, 0.01);
            float inner = lightDirections[k].w, outer = lightColors[k].w;
            if (outer > -1.5) strength *= smoothstep(outer, inner, dot(-l, lightDirections[k].xyz));
            if (shadowed && hasLampShadows == 1 && strength > 0.0) strength *= lampShadowAt(k, p, 0);
            light += lightColors[k].rgb * strength * hazePhase(dot(dir, l));
        }
        float lose = exp(-density * span);
        gathered += through * (1.0 - lose) * light * haze.rgb;
        through *= lose;
    }
    fragColor = vec4(gathered, through);
}
#endif
`

// hazeBlendShader lays the haze's picture over the pass: what was there,
// dimmed by how much of it shows through, and the haze's light, each
// smoothed over 3 by 3 of the haze's texels that are about as far away.
const hazeBlendShader = `#version 330
in vec2 texcoord;
uniform sampler2D colorCopy;
uniform sampler2D depthCopy;
uniform sampler2D hazed;
uniform mat4 inverseViewProjection;
uniform vec3 eye;
uniform vec2 hazeTexel;
out vec4 fragColor;

float distanceAt(vec2 uv)
{
    float d = texture(depthCopy, uv).r;
    vec4 w = inverseViewProjection * vec4(uv * 2.0 - 1.0, d * 2.0 - 1.0, 1.0);
    return length(w.xyz / w.w - eye);
}

void main()
{
    float here = distanceAt(texcoord);
    vec4 sum = vec4(0.0);
    float weights = 0.0;
    for (int x = -1; x <= 1; x++) {
        for (int y = -1; y <= 1; y++) {
            vec2 uv = texcoord + vec2(float(x), float(y)) * hazeTexel;
            float tent = (x == 0 ? 2.0 : 1.0) * (y == 0 ? 2.0 : 1.0);
            float w = tent * exp(-abs(distanceAt(uv) - here) / (0.05 * here + 0.05));
            sum += texture(hazed, uv) * w;
            weights += w;
        }
    }
    vec4 h = sum / max(weights, 1e-4);
    fragColor = vec4(texture(colorCopy, texcoord).rgb * h.a + h.rgb, 1.0);
}
`

// hazeBlendProgram lays the haze over the pass.
type hazeBlendProgram struct {
	program                               device.Program
	colorCopy, depthCopy, hazed           int32
	inverseViewProjection, eye, hazeTexel int32
}

// hazeBlend returns the program that lays the haze over the pass, made the
// first time a pass has haze, or nil when it doesn't compile.
func (r *state3D) hazeBlend() *hazeBlendProgram {
	if r.hazeBlender == nil {
		program, err := compile3D("haze", fullscreenVertexShader, hazeBlendShader)
		if err != nil {
			reportError(err)
			r.hazeBlender = &hazeBlendProgram{}
		} else {
			at := func(name string) int32 { return device.UniformLocation(program, name) }
			r.hazeBlender = &hazeBlendProgram{
				program: program, colorCopy: at("colorCopy"), depthCopy: at("depthCopy"), hazed: at("hazed"),
				inverseViewProjection: at("inverseViewProjection"), eye: at("eye"), hazeTexel: at("hazeTexel"),
			}
		}
	}
	if r.hazeBlender.program.ID == 0 {
		return nil
	}
	return r.hazeBlender
}

// hazeTargetFor returns the picture the haze is walked into, half the
// pass's width and height, made again when the pass's size changes.
func (r *state3D) hazeTargetFor(width, height int) device.WorkTarget {
	w, h := max(1, width/2), max(1, height/2)
	if r.hazeTarget.ID == 0 || r.hazeTarget.Width != w || r.hazeTarget.Height != h {
		device.UnloadWorkTarget(r.hazeTarget)
		r.hazeTarget = device.NewWorkTarget(w, h)
	}
	return r.hazeTarget
}

// hazeProgram returns the lit program compiled to draw haze, made the first
// time a pass has haze, or nil when it doesn't compile.
func (r *state3D) hazeProgram() *litProgram {
	kind := programKind{haze: true}
	if p, ok := r.surfaces[kind]; ok {
		return p
	}
	fragment := strings.Replace(litFragment(litFragmentShader), "// HAZE", hazeShader, 1)
	fragment = strings.Replace(fragment, "#version 330\n", "#version 330\n#define HAZE\n", 1)
	program, err := device.NewProgram(fullscreenVertexShader, fragment)
	if r.surfaces == nil {
		r.surfaces = map[programKind]*litProgram{}
	}
	if err != nil {
		reportError(fmt.Errorf("golib: the 3D's haze shader doesn't compile on this graphics card: %w", err))
		r.surfaces[kind] = nil
		return nil
	}
	p := locateLit(program)
	r.surfaces[kind] = &p
	return &p
}
