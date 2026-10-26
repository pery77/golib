package golib

import (
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
