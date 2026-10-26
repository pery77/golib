package golib

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"golib/internal/gltf"
)

// bakedGLB makes a level as tools/blender/bake_lightmaps.py exports it: a
// floor with a second UV map and its lightmap named in a custom property, a
// crate without one, and a sun, a point lamp and a spot lamp, in Blender's
// own units (Raw).
func bakedGLB(lightmap string) []byte {
	b := gltf.NewBuilder()
	quad := func(withLightmap bool) int {
		attributes := map[string]any{
			"POSITION":   b.Floats([]float32{-1, 0, 1, 1, 0, 1, 1, 0, -1, -1, 0, -1}, 3),
			"NORMAL":     b.Floats([]float32{0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0}, 3),
			"TEXCOORD_0": b.Floats([]float32{0, 1, 1, 1, 1, 0, 0, 0}, 2),
		}
		if withLightmap {
			attributes["TEXCOORD_1"] = b.Floats([]float32{0, 0.5, 0.5, 0.5, 0.5, 0, 0, 0}, 2)
		}
		return b.Add("meshes", map[string]any{"primitives": []any{map[string]any{
			"attributes": attributes, "indices": b.Indices([]uint16{0, 1, 2, 0, 2, 3}),
		}}})
	}
	down := []float32{-float32(math.Sqrt2 / 2), 0, 0, float32(math.Sqrt2 / 2)} // -Z turned to -Y
	b.JSON["extensionsUsed"] = []any{"KHR_lights_punctual"}
	b.JSON["extensions"] = map[string]any{"KHR_lights_punctual": map[string]any{"lights": []any{
		map[string]any{"type": "directional", "color": []float32{1, 0.5, 0.25}, "intensity": 3},
		map[string]any{"type": "point", "color": []float32{1, 1, 1}, "intensity": 100},
		map[string]any{"type": "spot", "color": []float32{1, 1, 1}, "intensity": 50, "spot": map[string]any{"innerConeAngle": 0.4, "outerConeAngle": math.Pi / 6}},
	}}}
	light := func(name string, at int, position []float32) int {
		return b.Add("nodes", map[string]any{"name": name, "translation": position, "rotation": down,
			"extensions": map[string]any{"KHR_lights_punctual": map[string]any{"light": at}}})
	}
	floor := b.Add("nodes", map[string]any{"name": "Floor", "mesh": quad(true), "extras": map[string]any{"lightmap": lightmap}})
	crate := b.Add("nodes", map[string]any{"name": "Crate", "mesh": quad(false), "translation": []float32{3, 0, 0}})
	for _, n := range []int{floor, crate, light("Sun", 0, []float32{0, 10, 0}), light("Lamp", 1, []float32{0, 2, 0}), light("Spot", 2, []float32{1, 3, -1})} {
		b.Root(n)
	}
	return b.GLB()
}

func TestModelReadsItsLightmapAndLamps(t *testing.T) {
	useAssets(t, map[string][]byte{
		"models/level.glb":          bakedGLB("level.lightmap.hdr"),
		"models/level.lightmap.hdr": radianceFile(4, 2, [4]byte{128, 64, 32, 129}, false),
	})
	level := NewModel("models/level.glb")
	if err := level.Err(); err != nil {
		t.Fatal(err)
	}
	if len(level.parts) != 2 {
		t.Fatalf("%d parts, want the floor and the crate", len(level.parts))
	}
	floor, crate := level.parts[0], level.parts[1]
	if floor.lightmap == nil || floor.lightmap.hdrWidth != 4 || floor.lightmap.hdrHeight != 2 || len(floor.lightmap.hdr) != 4*2*4 {
		t.Errorf("the floor's lightmap is %+v, want the 4 by 2 file beside the model", floor.lightmap)
	}
	if len(floor.mesh.lightcoords) != 8 || floor.mesh.lightcoords[2] != 0.5 {
		t.Errorf("the floor's lightmap coordinates are %v, want TEXCOORD_1", floor.mesh.lightcoords)
	}
	if crate.lightmap != nil || crate.mesh.lightcoords != nil {
		t.Error("the crate, which has no second UV map, got a lightmap")
	}
	if markers := level.Markers(); len(markers) != 0 {
		t.Errorf("Markers = %+v, want none: lamps aren't markers", markers)
	}

	sun, ok := level.Sun()
	if !ok || !near3(sun.Direction, Vector3{Y: -1}) {
		t.Fatalf("Sun = %+v, %v, want one shining down", sun, ok)
	}
	// Blender's sun of strength 3 lights a white surface facing it 3/π:
	// GoLib's linear sun, its color times its strength, must be the same.
	linear := linearColor(sun.Color)
	for i, want := range []float32{3 / math.Pi, 1.5 / math.Pi, 0.75 / math.Pi} {
		if got := linear[i] * sun.Strength; !within(got, want, 0.01) {
			t.Errorf("the sun's linear channel %d is %v, want %v", i, got, want)
		}
	}

	lights := level.Lights()
	if len(lights) != 2 {
		t.Fatalf("Lights = %+v, want the point lamp and the spot", lights)
	}
	lamp, spot := lights[0], lights[1]
	// A 100 W lamp lights a white surface a meter away 100/4π² in Cycles.
	if !near3(lamp.Position, Vector3{Y: 2}) || !within(lamp.Intensity, float32(100/(4*math.Pi*math.Pi)), 0.001) || lamp.Direction != (Vector3{}) {
		t.Errorf("the lamp is %+v, want at 0, 2, 0, intensity %v, all around", lamp, 100/(4*math.Pi*math.Pi))
	}
	if lamp.Range < 10 {
		t.Errorf("the lamp reaches %v m, want far, as Blender's does", lamp.Range)
	}
	if !near3(spot.Direction, Vector3{Y: -1}) || !within(spot.Cone, 60, 0.01) || !near3(spot.Position, Vector3{X: 1, Y: 3, Z: -1}) {
		t.Errorf("the spot is %+v, want pointing down, 60 degrees across, at 1, 3, -1", spot)
	}
}

func TestModelWithAMissingLightmapSaysSo(t *testing.T) {
	useAssets(t, map[string][]byte{"models/level.glb": bakedGLB("gone.hdr")})
	err := NewModel("models/level.glb").Err()
	if err == nil || !strings.Contains(err.Error(), `object "Floor": its lightmap gone.hdr`) {
		t.Errorf("Err = %v, want the object and the lightmap named", err)
	}
}

// probedGLB makes a level with light probes and nothing else to draw but a
// speck far below.
func probedGLB() []byte {
	b := gltf.NewBuilder()
	speck := b.Add("meshes", map[string]any{"primitives": []any{map[string]any{
		"attributes": map[string]any{
			"POSITION": b.Floats([]float32{0, -100, 0, 0.01, -100, 0, 0, -100, 0.01}, 3),
			"NORMAL":   b.Floats([]float32{0, 1, 0, 0, 1, 0, 0, 1, 0}, 3),
		},
		"indices": b.Indices([]uint16{0, 1, 2}),
	}}})
	b.Root(b.Add("nodes", map[string]any{"name": "Speck", "mesh": speck}))
	b.Root(b.Add("nodes", map[string]any{"name": "GoLib light probes", "extras": map[string]any{"lightprobes": "level.probes.json"}}))
	return b.GLB()
}

// Two probes 2 m apart, the left one in the dark and the right one in white
// light from every way, light a box between them as it moves.
func TestLightProbesLightWhatMoves(t *testing.T) {
	white := float32(1 / 0.282095)
	useAssets(t, map[string][]byte{
		"models/level.glb": probedGLB(),
		"models/level.probes.json": []byte(fmt.Sprintf(`{"origin":[-1,0,0],"spacing":2,"size":[2,1,1],"probes":[[0,0,0,0,0,0,0,0,0,0,0,0],[%v,%v,%v,0,0,0,0,0,0,0,0,0]]}`, white, white, white)),
	})
	level := NewModel("models/level.glb")
	if err := level.Err(); err != nil {
		t.Fatal(err)
	}
	if level.probes == nil || level.probes.size != [3]int{2, 1, 1} || level.probes.texels[3] != 1 || level.probes.texels[16] != white {
		t.Fatalf("the probes read are %+v", level.probes)
	}
	if markers := level.Markers(); len(markers) != 0 {
		t.Errorf("Markers = %+v, want none: the probes aren't a marker", markers)
	}

	screen, capture := openTestWindow(t, 64, 64)
	takeError()
	camera := NewCamera3D(64, 64)
	box := NewBoxMesh()
	night := Environment{SunColor: Black, Sky: Color{R: 10, G: 10, B: 10, A: 255}, Ground: Color{R: 10, G: 10, B: 10, A: 255}, NoBloom: true}
	brightness := func(x float32, withLevel bool) int {
		camera.Position, camera.Target = Vector3{X: x, Z: -4}, Vector3{X: x}
		camera.Snap()
		picture := capture(func() {
			screen.Begin3D(camera)
			screen.SetEnvironment(night)
			if withLevel {
				screen.DrawModel(level, Transform{}, White)
			}
			screen.DrawMesh(box, Transform{Position: Vector3{X: x}, Scale: Vector3{X: 0.5, Y: 0.5, Z: 0.5}}, Gray)
			screen.End3D()
		})
		c := picture.NRGBAAt(32, 32)
		return int(c.R) + int(c.G) + int(c.B)
	}
	alone := brightness(1, false)
	dark, middle, lit, far := brightness(-1, true), brightness(0, true), brightness(1, true), brightness(10, true)
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
	if !(dark < middle && middle < lit) {
		t.Errorf("a box moved from the dark probe to the white one is %d, %d, %d bright, want brighter and brighter", dark, middle, lit)
	}
	if lit < alone+150 {
		t.Errorf("by the white probe the box is %d bright, and %d without the level's probes: want much brighter", lit, alone)
	}
	if far != alone {
		t.Errorf("far from the probes the box is %d bright, want %d, as the Environment alone lights it", far, alone)
	}
}
