package golib

import (
	"math"

	"golib/internal/device"
)

// The sun's shadows are cascaded shadow maps: the view from the camera to
// ShadowDistance is cut into three slices, nearest the smallest, and each is
// drawn from the sun into its own third of one depth picture, so shadows
// near the camera get the most texels. Each slice is fitted with a sphere,
// whose size doesn't change as the camera turns, and moved by whole texels
// only, so shadows hold still while the camera moves.

const (
	shadowCascades   = 3
	shadowResolution = 1024 // texels each way, for each slice

	// shadowReach is how far towards the sun, past a slice, things still
	// cast into it: a tower behind the camera shades what is in front.
	shadowReach = 60

	// shadowSplitLog is how much the slices follow a logarithmic split,
	// which gives the near ones more, against an even one.
	shadowSplitLog = 0.7
)

// cascades are the sun's views for one pass.
type cascades struct {
	viewProjections [shadowCascades]matrix
	splits          [4]float32 // the view depth where each slice ends
	texels          [4]float32 // the size of a texel in each, in meters
}

// cascadesFor fits the sun's views around what camera sees up to distance
// meters, the sun shining along sun.
func cascadesFor(camera *Camera3D, sun Vector3, distance float32) cascades {
	view, projection := camera.matrices()
	inverse := projection.mul(view).inverse()
	near, far := camera.depthRange()
	reach := min(distance, far)
	var nearCorners, farCorners [4]Vector3
	for i, c := range [4][2]float32{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
		nearCorners[i] = inverse.transformPoint(Vector3{X: c[0], Y: c[1], Z: -1})
		farCorners[i] = inverse.transformPoint(Vector3{X: c[0], Y: c[1], Z: 1})
	}
	at := func(depth float32) [4]Vector3 {
		t := (depth - near) / (far - near)
		var out [4]Vector3
		for i := range out {
			out[i] = nearCorners[i].Lerp(farCorners[i], t)
		}
		return out
	}

	var c cascades
	start := near
	for k := range shadowCascades {
		share := float64(k+1) / shadowCascades
		logSplit := float64(near) * math.Pow(float64(reach/near), share)
		evenSplit := float64(near) + float64(reach-near)*share
		end := float32(shadowSplitLog*logSplit + (1-shadowSplitLog)*evenSplit)
		c.splits[k] = end

		first, last := at(start), at(end)
		corners := append(first[:], last[:]...)
		var center Vector3
		for _, p := range corners {
			center = center.Add(p)
		}
		center = center.Scale(1 / float32(len(corners)))
		var radius float32
		for _, p := range corners {
			radius = max(radius, p.Distance(center))
		}
		radius = float32(math.Ceil(float64(radius)*16) / 16)
		texel := 2 * radius / shadowResolution
		c.texels[k] = texel

		lightView := lookAtMatrix(center.Sub(sun.Scale(radius+shadowReach)), center)
		// Move by whole texels only, so shadows don't crawl as the camera
		// moves.
		middle := lightView.transformPoint(center)
		lightView[12] += float32(math.Round(float64(middle.X/texel)))*texel - middle.X
		lightView[13] += float32(math.Round(float64(middle.Y/texel)))*texel - middle.Y
		lightProjection := orthographicMatrix(2*radius, 1, 0.1, 2*radius+2*shadowReach)
		c.viewProjections[k] = lightProjection.mul(lightView)
		start = end
	}
	return c
}

// The program that draws depth into a shadow map: everything solid, and of
// what is cut out, only what isn't cut away.
const (
	shadowVertexShader = `#version 330
layout(location = 0) in vec3 vertexPosition;
layout(location = 2) in vec2 vertexTexcoord;
layout(location = 3) in vec4 vertexColor;
layout(location = 4) in mat4 instanceModel;
layout(location = 8) in vec4 instanceTint;
uniform mat4 lightViewProjection;
out vec2 texcoord;
out float opacity;

void main()
{
    texcoord = vertexTexcoord;
    opacity = vertexColor.a * instanceTint.a;
    gl_Position = lightViewProjection * instanceModel * vec4(vertexPosition, 1.0);
}
`
	shadowFragmentShader = `#version 330
in vec2 texcoord;
in float opacity;
uniform sampler2D baseTexture;
uniform int hasBaseTexture;
uniform float baseOpacity;
uniform int cutout;
uniform float alphaCutoff;

void main()
{
    if (cutout == 1) {
        float a = baseOpacity * opacity;
        if (hasBaseTexture == 1) a *= texture(baseTexture, texcoord).a;
        if (a < alphaCutoff) discard;
    }
}
`
)

// shadowProgram draws depth into the shadow map.
type shadowProgram struct {
	program                          device.Program
	lightViewProjection              int32
	baseTexture, hasBaseTexture      int32
	baseOpacity, cutout, alphaCutoff int32
}

// drawShadows draws the solid groups into the sun's shadow map, one slice at
// a time, and leaves the pass's own target bound.
func (r *state3D) drawShadows(c *cascades, groups []shadowGroup) {
	if r.shadowMap.ID == 0 {
		r.shadowMap = device.NewShadowMap(shadowCascades*shadowResolution, shadowResolution)
	}
	s := r.shadow
	device.UseProgram(s.program)
	device.SetUniformInt(s.baseTexture, 0)
	culling := true
	device.SetCulling(true)
	for k := range shadowCascades {
		device.BeginShadow(r.shadowMap, k*shadowResolution, 0, shadowResolution, shadowResolution, k == 0)
		lightViewProjection := [16]float32(c.viewProjections[k])
		device.SetUniformMatrix(s.lightViewProjection, &lightViewProjection)
		var current *material3D
		for _, g := range groups {
			m := g.material
			if m != current {
				current = m
				cut := m.alpha == alphaMask
				device.SetUniformInt(s.cutout, boolInt(cut))
				if cut {
					device.SetUniform(s.baseOpacity, 1, [4]float32{m.baseColor[3]})
					device.SetUniform(s.alphaCutoff, 1, [4]float32{m.alphaCutoff})
					if gpu := m.baseTexture.upload(); gpu.ID != 0 {
						device.BindTexture3D(0, gpu)
						device.SetUniformInt(s.hasBaseTexture, 1)
					} else {
						device.SetUniformInt(s.hasBaseTexture, 0)
					}
				}
				if m.twoSided == culling {
					culling = !m.twoSided
					device.SetCulling(culling)
				}
			}
			device.DrawMeshInstanced(g.gpu, g.instances)
		}
	}
	device.EndShadow()
	device.SetCulling(true)
}

// shadowGroup is a group of solid instances, as the shadow pass draws them.
type shadowGroup struct {
	gpu       device.Mesh
	material  *material3D
	instances []float32
}
