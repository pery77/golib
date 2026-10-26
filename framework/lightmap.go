package golib

import (
	"encoding/json"
	"fmt"
	"math"

	"golib/internal/device"
	"golib/internal/gltf"
)

// Light baked in Blender: a level made there can carry a lightmap, light
// worked out by Cycles, a ray tracer, for every surface of its still
// objects. The bake keeps what GoLib can't work out as it draws: the sky's
// light as much of it as reaches each place, and the light of the sun, the
// lamps and the sky bounced off everything, colored by what it bounced off.
// The sun and the lamps themselves stay GoLib's, drawn each frame with their
// shadows, so things that move are lit as the level is and shade it. A
// level's object names its lightmap in its custom property lightmap, the
// .hdr file beside the model, which tools/blender/bake_lightmaps.py writes,
// with a second UV map, Lightmap, that it unwraps for every object.
//
// The sun and lamps the level was baked with are in the file too, and GoLib
// must light it with the same ones: Model.Sun and Model.Lights give them.
//
// What moves through the level gets the same light from light probes: a grid
// of points baked with it, each holding the light that comes to it from
// every way, as four spherical harmonics. A model drawn in a pass with a
// level that has them takes its soft light from the probes around it,
// instead of the Environment's sky and ground.

// ModelSun is the sun lamp of a model made in Blender, as an Environment
// takes it:
//
//	if sun, ok := level.Sun(); ok {
//		env.SunDirection, env.SunColor, env.SunStrength = sun.Direction, sun.Color, sun.Strength
//	}
type ModelSun struct {
	Direction Vector3 // the way its light travels, in the model's space
	Color     Color
	Strength  float32
}

// modelLight is a lamp in a model's file, where it is at rest.
type modelLight struct {
	light gltf.Light
	place matrix
}

// Sun returns the sun lamp the model was exported with from Blender, the
// first if there are several, and false when it has none. Blender exports
// lamps with Include > Data > Punctual Lights, and the strength GoLib reads
// with Lighting Mode set to Raw (export_import_convert_lighting_mode='RAW'),
// as tools/blender/bake_lightmaps.py does: then a level lit by it in GoLib
// matches the light Blender baked.
func (m *Model) Sun() (ModelSun, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.prepare() != nil {
		return ModelSun{}, false
	}
	for _, l := range m.lights {
		if l.light.Type != "directional" {
			continue
		}
		color, scale := lampColor(l.light.Color)
		// A sun of strength S in Blender lights a surface facing it S/π as
		// brightly as GoLib's sun of strength 1 does.
		return ModelSun{
			Direction: l.place.transformDirection(Vector3{Z: -1}).Normalize(),
			Color:     color,
			Strength:  l.light.Intensity / math.Pi * scale,
		}, true
	}
	return ModelSun{}, false
}

// Lights returns the model's point and spot lamps, as Blender exported them,
// as golib.Lights in the model's own space: add them to each pass with
// Screen.AddLight, placed with Transform.Child when the model is drawn
// moved. Their intensity matches what Blender baked, as for Sun.
func (m *Model) Lights() []Light {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.prepare() != nil {
		return nil
	}
	var lights []Light
	for _, l := range m.lights {
		if l.light.Type != "point" && l.light.Type != "spot" {
			continue
		}
		color, scale := lampColor(l.light.Color)
		// A lamp of P watts in Blender lights a surface a meter away facing
		// it P/4π² as brightly as GoLib's sun of strength 1 does.
		intensity := l.light.Intensity / (4 * math.Pi * math.Pi) * scale
		light := Light{
			Position:  l.place.transformPoint(Vector3{}),
			Color:     color,
			Intensity: intensity,
			Range:     l.light.Range,
		}
		if light.Range <= 0 {
			// Blender's lamps reach forever; GoLib's fade out, here where
			// they are a five-hundredth of the sun.
			light.Range = float32(math.Sqrt(float64(intensity) * 500))
		}
		if l.light.Type == "spot" {
			light.Direction = l.place.transformDirection(Vector3{Z: -1}).Normalize()
			light.Cone = 2 * l.light.OuterAngle * 180 / math.Pi
		}
		lights = append(lights, light)
	}
	return lights
}

// lampColor returns a lamp's linear color as a Color, its brightest channel
// full, and how much brighter the lamp is than that Color.
func lampColor(c [3]float32) (Color, float32) {
	peak := max(c[0], c[1], c[2])
	if peak <= 0 {
		return Black, 0
	}
	toSRGB := func(v float32) uint8 {
		return uint8(math.Round(math.Pow(float64(v/peak), 1/2.2) * 255))
	}
	return Color{R: toSRGB(c[0]), G: toSRGB(c[1]), B: toSRGB(c[2]), A: 255}, peak
}

// lightProbes is the grid of light probes a level was baked with, in its
// own space, read from the file tools/blender/bake_lightmaps.py writes.
type lightProbes struct {
	origin  Vector3 // the first probe
	spacing float32 // meters between probes
	size    [3]int  // how many along X, Y and Z
	// texels holds four texels a probe, X first, then Y, then Z: its four
	// spherical harmonics' red, green and blue, and, in the first texel's
	// fourth number, 1 for a probe and 0 for a point inside a wall.
	texels []float32
	gpu    device.DataTexture
}

// probeTexels is how many texels of the probes' data texture a probe takes.
const probeTexels = 4

// readLightProbes reads a level's light probes, a JSON file in the game's
// assets folder.
func readLightProbes(name string) (*lightProbes, error) {
	data, err := ReadAsset(name)
	if err != nil {
		return nil, err
	}
	var file struct {
		Origin  [3]float32  `json:"origin"`
		Spacing float32     `json:"spacing"`
		Size    [3]int      `json:"size"`
		Probes  [][]float32 `json:"probes"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	count := file.Size[0] * file.Size[1] * file.Size[2]
	if file.Spacing <= 0 || count <= 0 || len(file.Probes) != count {
		return nil, fmt.Errorf("a grid of %v probes %v m apart, but %d given: bake it again with tools/blender/bake_lightmaps.py", file.Size, file.Spacing, len(file.Probes))
	}
	p := &lightProbes{
		origin:  Vector3{X: file.Origin[0], Y: file.Origin[1], Z: file.Origin[2]},
		spacing: file.Spacing, size: file.Size,
		texels: make([]float32, count*probeTexels*4),
	}
	for i, h := range file.Probes {
		if h == nil {
			continue
		}
		if len(h) != 12 {
			return nil, fmt.Errorf("probe %d has %d numbers, not 12", i, len(h))
		}
		at := p.texels[i*probeTexels*4:]
		for k := 0; k < 4; k++ {
			copy(at[k*4:k*4+3], h[k*3:k*3+3])
		}
		at[3] = 1
	}
	return p, nil
}

// upload puts the probes on the graphics card, the first time they light a
// pass.
func (p *lightProbes) upload() device.DataTexture {
	if p.gpu.ID == 0 {
		p.gpu = device.NewDataTexture(p.texels)
		loadedProbes = append(loadedProbes, p)
	}
	return p.gpu
}

// fromWorld returns the matrix that takes a point of the world to the grid
// of a level drawn at place: whole numbers at the probes.
func (p *lightProbes) fromWorld(place matrix) matrix {
	toGrid := identityMatrix()
	s := 1 / p.spacing
	toGrid[0], toGrid[5], toGrid[10] = s, s, s
	toGrid[12], toGrid[13], toGrid[14] = -p.origin.X*s, -p.origin.Y*s, -p.origin.Z*s
	return toGrid.mul(place.inverse())
}

// loadedProbes are the probes on the graphics card, for Run to free when it
// ends.
var loadedProbes []*lightProbes

func unloadProbes() {
	for _, p := range loadedProbes {
		device.UnloadDataTexture(p.gpu)
		p.gpu = device.DataTexture{}
	}
	loadedProbes = nil
}
