package gltf

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
	"testing"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// quad adds a mesh of one square, two triangles, facing +Z, with what Blender
// writes for it.
func quad(b *Builder, material int) int {
	attributes := map[string]any{
		"POSITION":   b.Floats([]float32{-1, -1, 0, 1, -1, 0, 1, 1, 0, -1, 1, 0}, 3),
		"NORMAL":     b.Floats([]float32{0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1}, 3),
		"TEXCOORD_0": b.Floats([]float32{0, 1, 1, 1, 1, 0, 0, 0}, 2),
		"COLOR_0":    b.Bytes([]uint8{255, 0, 0, 255, 0, 255, 0, 255, 0, 0, 255, 255, 255, 255, 255, 128}, 4),
	}
	return b.Add("meshes", map[string]any{"name": "Quad", "primitives": []any{map[string]any{
		"attributes": attributes, "indices": b.Indices([]uint16{0, 1, 2, 0, 2, 3}), "material": material,
	}}})
}

func TestRead(t *testing.T) {
	b := NewBuilder()
	image := b.Add("images", map[string]any{"name": "checker", "mimeType": "image/png", "bufferView": b.View(pngBytes(t))})
	sampler := b.Add("samplers", map[string]any{"magFilter": 9728, "minFilter": 9984, "wrapS": 33071})
	texture := b.Add("textures", map[string]any{"source": image, "sampler": sampler})
	material := b.Add("materials", map[string]any{
		"name": "Painted",
		"pbrMetallicRoughness": map[string]any{
			"baseColorFactor": []float32{0.5, 0.25, 1, 1}, "baseColorTexture": map[string]any{"index": texture},
			"metallicFactor": 0, "roughnessFactor": 0.5,
		},
		"emissiveFactor": []float32{1, 0.5, 0}, "alphaMode": "MASK", "alphaCutoff": 0.3, "doubleSided": true,
		"extensions": map[string]any{"KHR_materials_emissive_strength": map[string]any{"emissiveStrength": 4}},
	})
	mesh := quad(b, material)
	child := b.Add("nodes", map[string]any{"name": "Sign", "mesh": mesh, "translation": []float32{0, 2, 0}})
	spawn := b.Add("nodes", map[string]any{"name": "Spawn", "translation": []float32{3, 0, -1},
		"extras": map[string]any{"kind": "enemy", "speed": 2.5, "boss": true, "loot": []any{"gem", 3}}})
	parent := b.Add("nodes", map[string]any{"name": "Level", "children": []any{child, spawn},
		"rotation": []float32{0, float32(math.Sin(math.Pi / 4)), 0, float32(math.Cos(math.Pi / 4))}, "scale": []float32{2, 2, 2}})
	b.Root(parent)

	doc, err := Read(b.GLB(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Meshes) != 1 || len(doc.Meshes[0].Primitives) != 1 {
		t.Fatalf("read %d meshes, want 1 with 1 primitive", len(doc.Meshes))
	}
	p := doc.Meshes[0].Primitives[0]
	if len(p.Positions) != 12 || len(p.Normals) != 12 || len(p.Texcoords) != 8 || len(p.Colors) != 16 {
		t.Errorf("the quad has %d, %d, %d, %d numbers of positions, normals, texcoords, colors; want 12, 12, 8, 16",
			len(p.Positions), len(p.Normals), len(p.Texcoords), len(p.Colors))
	}
	if p.Colors[15] < 0.5 || p.Colors[15] > 0.51 || p.Colors[0] != 1 {
		t.Errorf("normalized byte colors read as %v, want 0 to 1", p.Colors)
	}
	if len(p.Indices) != 6 || p.Indices[5] != 3 || p.Material != material {
		t.Errorf("indices %v, material %d", p.Indices, p.Material)
	}

	m := doc.Materials[0]
	if m.BaseColor != [4]float32{0.5, 0.25, 1, 1} || m.BaseColorTexture.Index != texture || m.Roughness != 0.5 || m.Metallic != 0 {
		t.Errorf("material read as %+v", m)
	}
	if m.Emissive != [3]float32{4, 2, 0} {
		t.Errorf("emissive %v, want the factor times its strength, 4, 2, 0", m.Emissive)
	}
	if m.AlphaMode != "MASK" || m.AlphaCutoff != 0.3 || !m.DoubleSided {
		t.Errorf("alpha %s %v, double-sided %v", m.AlphaMode, m.AlphaCutoff, m.DoubleSided)
	}
	tex := doc.Textures[0]
	if !tex.Nearest || !tex.NearestMipmaps || tex.RepeatS || !tex.RepeatT {
		t.Errorf("sampler read as %+v, want nearest, clamped across, repeated down", tex)
	}
	if doc.Images[0].MIME != "image/png" || !bytes.HasPrefix(doc.Images[0].Data, []byte("\x89PNG")) {
		t.Errorf("image read as %s, %d bytes", doc.Images[0].MIME, len(doc.Images[0].Data))
	}

	if len(doc.Roots) != 1 || doc.Roots[0] != parent {
		t.Fatalf("roots %v, want the level node", doc.Roots)
	}
	level := doc.Nodes[parent]
	// A quarter turn around Y, scaled by 2: +X goes to -Z, twice as long.
	if x := level.Local; math.Abs(float64(x[0])) > 1e-5 || math.Abs(float64(x[2]+2)) > 1e-5 {
		t.Errorf("the level's matrix starts %v, want +X turned to -2 Z", x[:4])
	}
	extras := doc.Nodes[spawn].Extras
	if extras["kind"] != "enemy" || extras["speed"] != "2.5" || extras["boss"] != "true" || extras["loot"] != `["gem",3]` {
		t.Errorf("extras read as %v", extras)
	}
	if doc.Nodes[spawn].Mesh != -1 || doc.Nodes[child].Mesh != mesh {
		t.Error("an empty has a mesh, or the sign has none")
	}
}

func TestReadMakesFlatNormalsAndTriangleLists(t *testing.T) {
	b := NewBuilder()
	positions := b.Floats([]float32{0, 0, 0, 1, 0, 0, 0, 1, 0, 1, 1, 0}, 3)
	b.Add("meshes", map[string]any{"primitives": []any{
		map[string]any{"attributes": map[string]any{"POSITION": positions}, "mode": 5}, // a strip of two triangles
		map[string]any{"attributes": map[string]any{"POSITION": positions}, "mode": 1}, // lines: skipped
	}})
	b.Root(b.Add("nodes", map[string]any{"mesh": 0}))
	doc, err := Read(b.GLB(), nil)
	if err != nil {
		t.Fatal(err)
	}
	prims := doc.Meshes[0].Primitives
	if len(prims) != 1 {
		t.Fatalf("read %d primitives, want the strip only", len(prims))
	}
	p := prims[0]
	if len(p.Indices) != 6 || len(p.Positions) != 18 {
		t.Fatalf("the strip became %d indices over %d positions, want 2 triangles of their own vertices", len(p.Indices), len(p.Positions)/3)
	}
	for i := 0; i < len(p.Normals); i += 3 {
		if p.Normals[i+2] != 1 {
			t.Fatalf("flat normal %v, want +Z for both triangles, counterclockwise from the front", p.Normals[i:i+3])
		}
	}
}

func TestReadGLTFWithFiles(t *testing.T) {
	var positions []byte
	for _, v := range []float32{0, 0, 0, 1, 0, 0, 0, 1, 0} {
		positions = binary.LittleEndian.AppendUint32(positions, math.Float32bits(v))
	}
	file := `{"asset":{"version":"2.0"},"scenes":[{"nodes":[0]}],"nodes":[{"mesh":0}],
		"meshes":[{"primitives":[{"attributes":{"POSITION":0}}]}],
		"accessors":[{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"}],
		"bufferViews":[{"buffer":0,"byteLength":36}],
		"buffers":[{"uri":"my%20mesh.bin","byteLength":36}],
		"images":[{"uri":"data:image/png;base64,` + base64.StdEncoding.EncodeToString(pngBytes(t)) + `"}]}`
	var asked []string
	doc, err := Read([]byte(file), func(uri string) ([]byte, error) {
		asked = append(asked, uri)
		return positions, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(asked) != 1 || asked[0] != "my mesh.bin" {
		t.Errorf("asked for %v, want the buffer's file, unescaped", asked)
	}
	if len(doc.Images) != 1 || doc.Images[0].MIME != "image/png" {
		t.Errorf("the data URI image read as %+v", doc.Images)
	}
	if got := doc.Meshes[0].Primitives[0].Positions; got[3] != 1 || got[7] != 1 {
		t.Errorf("positions %v", got)
	}
}

func TestReadRefuses(t *testing.T) {
	for _, c := range []struct{ file, want string }{
		{`{"asset":{"version":"2.0"},"extensionsRequired":["KHR_draco_mesh_compression"]}`, "turn off Compression"},
		{`{"asset":{"version":"1.0"}}`, "glTF 2.0"},
		{`not json`, "not a glTF file"},
		{`{"asset":{"version":"2.0"},"nodes":[{"children":[0]}]}`, "its own ancestor"},
		{`{"asset":{"version":"2.0"},"images":[{"uri":"data:image/gif;base64,R0lGODlh"}]}`, "PNG or JPEG"},
	} {
		_, err := Read([]byte(c.file), nil)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("Read(%.40s) = %v, want a mistake saying %q", c.file, err, c.want)
		}
	}
	// A .glb cut short.
	b := NewBuilder()
	quad(b, -1)
	glb := b.GLB()
	if _, err := Read(glb[:len(glb)-10], nil); err == nil || !strings.Contains(err.Error(), "cut short") {
		t.Errorf("a .glb cut short read with %v, want it said", err)
	}
}

func TestReadSparse(t *testing.T) {
	b := NewBuilder()
	base := b.Floats([]float32{0, 0, 0, 1, 0, 0, 0, 1, 0}, 3)
	accessors := b.list("accessors")
	sparse := accessors[base].(map[string]any)
	var indices, values bytes.Buffer
	indices.Write([]byte{1, 0}) // vertex 1, as a 16-bit index
	for _, v := range []float32{5, 5, 5} {
		bits := math.Float32bits(v)
		values.Write([]byte{byte(bits), byte(bits >> 8), byte(bits >> 16), byte(bits >> 24)})
	}
	sparse["sparse"] = map[string]any{
		"count":   1,
		"indices": map[string]any{"bufferView": b.View(indices.Bytes()), "componentType": 5123},
		"values":  map[string]any{"bufferView": b.View(values.Bytes())},
	}
	b.Add("meshes", map[string]any{"primitives": []any{map[string]any{"attributes": map[string]any{"POSITION": base}}}})
	b.Root(b.Add("nodes", map[string]any{"mesh": 0}))
	doc, err := Read(b.GLB(), nil)
	if err != nil {
		t.Fatal(err)
	}
	// Flattened, the triangle keeps its order: vertex 1 is the second.
	if got := doc.Meshes[0].Primitives[0].Positions[3:6]; got[0] != 5 || got[1] != 5 || got[2] != 5 {
		t.Errorf("vertex 1 is %v, want the sparse value 5, 5, 5", got)
	}
}
