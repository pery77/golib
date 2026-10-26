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

// Lamps cast shadows into the same depth picture, below the sun's slices,
// which it grows to hold them the first time a lamp does: tiles half a
// slice wide, six across and four down. A spotlight draws the world into
// one, seen from the lamp; a point light into six, one for each face of a
// cube around it, each seeing 90 degrees.
const (
	lampTile       = shadowResolution / 2
	lampTilesAcross = 6
	lampTilesDown   = 4
	maxLampShadows  = 4
	lampShadowNear  = 0.05
)

// cubeFaces are the ways a point light's six tiles look, and which way is up
// in each, as the lit shader has them too.
var cubeFaces = [6]struct{ forward, up Vector3 }{
	{Vector3{X: 1}, Vector3{Y: 1}}, {Vector3{X: -1}, Vector3{Y: 1}},
	{Vector3{Y: 1}, Vector3{Z: 1}}, {Vector3{Y: -1}, Vector3{Z: -1}},
	{Vector3{Z: 1}, Vector3{Y: 1}}, {Vector3{Z: -1}, Vector3{Y: 1}},
}

// lampShadows are the lamps of a pass that cast shadows, and the views each
// of their tiles is drawn with.
type lampShadows struct {
	tiles    []matrix          // the view of each tile, in order
	uniforms []float32         // for each of the pass's lights: first tile or -1, spot index or -1, near, far
	spots    []float32         // the spotlights' views, as the lit shader reads them
	count    int               // how many lamps cast shadows
}

// lookAlong is the view of an eye looking along forward, with up as up.
func lookAlong(eye, forward, up Vector3) matrix {
	f := forward.Normalize()
	s := f.Cross(up).Normalize()
	u := s.Cross(f)
	return matrix{
		s.X, u.X, -f.X, 0,
		s.Y, u.Y, -f.Y, 0,
		s.Z, u.Z, -f.Z, 0,
		-s.Dot(eye), -u.Dot(eye), f.Dot(eye), 1,
	}
}

// lampShadowsFor picks, of the lights the pass uses, the nearest four that
// ask for shadows, as many as the tiles hold, and works out their views.
func lampShadowsFor(lights []Light) lampShadows {
	out := lampShadows{uniforms: make([]float32, 0, 4*len(lights))}
	for _, l := range lights {
		reach := l.Range
		if reach <= 0 {
			reach = 10
		}
		tile, spot := float32(-1), float32(-1)
		point := l.Direction.Normalize() == (Vector3{})
		needs := 1
		if point {
			needs = 6
		}
		if l.Shadows && out.count < maxLampShadows && len(out.tiles)+needs <= lampTilesAcross*lampTilesDown {
			out.count++
			tile = float32(len(out.tiles))
			if point {
				projection := perspectiveMatrix(90, 1, lampShadowNear, reach)
				for _, face := range cubeFaces {
					out.tiles = append(out.tiles, projection.mul(lookAlong(l.Position, face.forward, face.up)))
				}
			} else {
				cone := l.Cone
				if cone <= 0 {
					cone = 45
				}
				d := l.Direction.Normalize()
				up := Vector3{Y: 1}
				if math.Abs(float64(d.Y)) > 0.99 {
					up = Vector3{Z: 1}
				}
				view := perspectiveMatrix(min(cone*1.1+4, 170), 1, lampShadowNear, reach).mul(lookAlong(l.Position, d, up))
				spot = float32(len(out.spots) / 16)
				out.tiles = append(out.tiles, view)
				out.spots = append(out.spots, view[:]...)
			}
		}
		out.uniforms = append(out.uniforms, tile, spot, lampShadowNear, reach)
	}
	return out
}

// drawLampShadows draws the solid groups into the lamps' tiles.
func (r *state3D) drawLampShadows(lamps *lampShadows, groups []shadowGroup, clear bool) {
	programs := r.shadowPrograms(groups)
	culling := true
	device.SetCulling(true)
	for k, view := range lamps.tiles {
		x, y := k%lampTilesAcross*lampTile, shadowResolution+k/lampTilesAcross*lampTile
		device.BeginShadow(r.shadowMap, x, y, lampTile, lampTile, clear && k == 0)
		lightViewProjection := [16]float32(view)
		for bending, s := range programs {
			r.drawShadowGroups(s, &lightViewProjection, groups, bending == 1, &culling)
		}
	}
	device.EndShadow()
	device.SetCulling(true)
}

// shadowPrograms returns the programs the groups' shadows are drawn with:
// the plain one, and the one that bends when any group is animated.
func (r *state3D) shadowPrograms(groups []shadowGroup) []*shadowProgram {
	programs := []*shadowProgram{&r.shadow}
	for _, g := range groups {
		if g.animation.stride > 0 {
			if bent := r.bentShadow(); bent != nil {
				programs = append(programs, bent)
			}
			break
		}
	}
	return programs
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

// ANIMATION

void main()
{
    texcoord = vertexTexcoord;
    opacity = vertexColor.a * instanceTint.a;
    vec3 position = vertexPosition;
#ifdef ANIMATED
    vec3 n = vec3(0.0);
    animate(position, n);
#endif
    gl_Position = lightViewProjection * instanceModel * vec4(position, 1.0);
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
	bending                          bendingUniforms
}

// locateShadow finds where the shadow shader's uniforms sit in program.
func locateShadow(program device.Program) shadowProgram {
	at := func(name string) int32 { return device.UniformLocation(program, name) }
	return shadowProgram{
		program: program, lightViewProjection: at("lightViewProjection"),
		baseTexture: at("baseTexture"), hasBaseTexture: at("hasBaseTexture"),
		baseOpacity: at("baseOpacity"), cutout: at("cutout"), alphaCutoff: at("alphaCutoff"),
		bending: locateBending(program),
	}
}

// bentShadow returns the shadow program that bends what it draws, made the
// first time a pass casts the shadow of something animated, or nil when it
// doesn't compile.
func (r *state3D) bentShadow() *shadowProgram {
	if r.shadowBent == nil {
		program, err := compile3D("animated shadow", animated3D(shadowVertexShader), shadowFragmentShader)
		if err != nil {
			reportError(err)
			r.shadowBent = &shadowProgram{}
		} else {
			p := locateShadow(program)
			r.shadowBent = &p
		}
	}
	if r.shadowBent.program.ID == 0 {
		return nil
	}
	return r.shadowBent
}

// shadowMapFor makes sure the shadow map is there, tall enough for the
// lamps' tiles when lamps is true, and says how many slices tall it is.
func (r *state3D) shadowMapFor(lamps bool) int {
	rows := 1
	if lamps || r.shadowMap.Height > shadowResolution {
		rows = 3
	}
	if r.shadowMap.ID == 0 || r.shadowMap.Height != rows*shadowResolution {
		device.UnloadShadowMap(r.shadowMap)
		r.shadowMap = device.NewShadowMap(shadowCascades*shadowResolution, rows*shadowResolution)
	}
	return rows
}

// drawShadows draws the solid groups into the sun's shadow map, one slice at
// a time, and leaves the pass's own target bound: those that aren't
// animated, then those that are, with the program that bends them.
func (r *state3D) drawShadows(c *cascades, groups []shadowGroup) {
	programs := r.shadowPrograms(groups)
	culling := true
	device.SetCulling(true)
	for k := range shadowCascades {
		device.BeginShadow(r.shadowMap, k*shadowResolution, 0, shadowResolution, shadowResolution, k == 0)
		lightViewProjection := [16]float32(c.viewProjections[k])
		for bending, s := range programs {
			r.drawShadowGroups(s, &lightViewProjection, groups, bending == 1, &culling)
		}
	}
	device.EndShadow()
	device.SetCulling(true)
}

// drawShadowGroups draws, with s, the groups that are animated, when bent
// is true, or those that aren't.
func (r *state3D) drawShadowGroups(s *shadowProgram, lightViewProjection *[16]float32, groups []shadowGroup, bent bool, culling *bool) {
	device.UseProgram(s.program)
	device.SetUniformInt(s.baseTexture, 0)
	device.SetUniformMatrix(s.lightViewProjection, lightViewProjection)
	var current *material3D
	for _, g := range groups {
		if (g.animation.stride > 0) != bent {
			continue
		}
		if m := g.material; m != current {
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
			if m.twoSided == *culling {
				*culling = !m.twoSided
				device.SetCulling(*culling)
			}
		}
		if bent {
			s.bending.set(g.animation)
		}
		device.DrawMeshInstanced(g.gpu, g.instances)
	}
}

// shadowGroup is a group of solid instances, as the shadow pass draws them.
type shadowGroup struct {
	gpu       device.Mesh
	material  *material3D
	instances []float32
	animation groupAnimation
	lightmap  *texture3D // for the picture: the light baked for it, or nil
}
