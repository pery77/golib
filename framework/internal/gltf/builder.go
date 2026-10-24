package gltf

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
)

// Builder writes small .glb files, for tests: GoLib's tests make the models
// they read rather than carry files whose licenses would have to travel with
// them. It writes what Blender's exporter writes, in the same shapes.
type Builder struct {
	JSON map[string]any
	bin  bytes.Buffer
}

// NewBuilder starts an empty glTF 2.0 document with one scene.
func NewBuilder() *Builder {
	return &Builder{JSON: map[string]any{
		"asset":  map[string]any{"version": "2.0", "generator": "golib test"},
		"scene":  0,
		"scenes": []any{map[string]any{"nodes": []any{}}},
	}}
}

func (b *Builder) list(name string) []any {
	l, _ := b.JSON[name].([]any)
	return l
}

func (b *Builder) add(name string, value any) int {
	l := append(b.list(name), value)
	b.JSON[name] = l
	return len(l) - 1
}

// View appends data to the binary buffer and returns its buffer view.
func (b *Builder) View(data []byte) int {
	for b.bin.Len()%4 != 0 {
		b.bin.WriteByte(0)
	}
	offset := b.bin.Len()
	b.bin.Write(data)
	return b.add("bufferViews", map[string]any{"buffer": 0, "byteOffset": offset, "byteLength": len(data)})
}

// Floats adds an accessor of float32 values, comps to an element (1 to 4).
func (b *Builder) Floats(values []float32, comps int) int {
	var data bytes.Buffer
	for _, v := range values {
		binary.Write(&data, binary.LittleEndian, math.Float32bits(v))
	}
	accessor := map[string]any{
		"bufferView": b.View(data.Bytes()), "componentType": 5126,
		"count": len(values) / comps, "type": [...]string{"", "SCALAR", "VEC2", "VEC3", "VEC4"}[comps],
	}
	if comps == 3 {
		lo := []float32{math.MaxFloat32, math.MaxFloat32, math.MaxFloat32}
		hi := []float32{-math.MaxFloat32, -math.MaxFloat32, -math.MaxFloat32}
		for i, v := range values {
			lo[i%3], hi[i%3] = min(lo[i%3], v), max(hi[i%3], v)
		}
		accessor["min"], accessor["max"] = lo, hi
	}
	return b.add("accessors", accessor)
}

// Bytes adds an accessor of normalized unsigned bytes, comps to an element,
// as Blender writes vertex colors.
func (b *Builder) Bytes(values []uint8, comps int) int {
	return b.add("accessors", map[string]any{
		"bufferView": b.View(values), "componentType": 5121, "normalized": true,
		"count": len(values) / comps, "type": [...]string{"", "SCALAR", "VEC2", "VEC3", "VEC4"}[comps],
	})
}

// Joints adds an accessor of unsigned bytes, four to an element, not
// normalized, as Blender writes the joints that skin each vertex.
func (b *Builder) Joints(values []uint8) int {
	return b.add("accessors", map[string]any{
		"bufferView": b.View(values), "componentType": 5121, "count": len(values) / 4, "type": "VEC4",
	})
}

// Matrices adds an accessor of 4 by 4 matrices, 16 float32 values each in
// column-major order, as a skin's inverse bind matrices are written.
func (b *Builder) Matrices(values []float32) int {
	var data bytes.Buffer
	for _, v := range values {
		binary.Write(&data, binary.LittleEndian, math.Float32bits(v))
	}
	return b.add("accessors", map[string]any{
		"bufferView": b.View(data.Bytes()), "componentType": 5126, "count": len(values) / 16, "type": "MAT4",
	})
}

// Indices adds an accessor of 16-bit indices.
func (b *Builder) Indices(values []uint16) int {
	var data bytes.Buffer
	binary.Write(&data, binary.LittleEndian, values)
	return b.add("accessors", map[string]any{
		"bufferView": b.View(data.Bytes()), "componentType": 5123, "count": len(values), "type": "SCALAR",
	})
}

// Add appends value to the top-level list called name, such as "materials",
// and returns its index.
func (b *Builder) Add(name string, value any) int {
	return b.add(name, value)
}

// Root makes node one of the scene's top nodes.
func (b *Builder) Root(node int) {
	scene := b.list("scenes")[0].(map[string]any)
	scene["nodes"] = append(scene["nodes"].([]any), node)
}

// GLB returns the .glb file.
func (b *Builder) GLB() []byte {
	for b.bin.Len()%4 != 0 {
		b.bin.WriteByte(0)
	}
	if b.bin.Len() > 0 {
		b.JSON["buffers"] = []any{map[string]any{"byteLength": b.bin.Len()}}
	}
	text, err := json.Marshal(b.JSON)
	if err != nil {
		panic(err)
	}
	for len(text)%4 != 0 {
		text = append(text, ' ')
	}
	var out bytes.Buffer
	out.WriteString("glTF")
	binary.Write(&out, binary.LittleEndian, uint32(2))
	total := 12 + 8 + len(text)
	if b.bin.Len() > 0 {
		total += 8 + b.bin.Len()
	}
	binary.Write(&out, binary.LittleEndian, uint32(total))
	binary.Write(&out, binary.LittleEndian, uint32(len(text)))
	out.WriteString("JSON")
	out.Write(text)
	if b.bin.Len() > 0 {
		binary.Write(&out, binary.LittleEndian, uint32(b.bin.Len()))
		out.WriteString("BIN\x00")
		out.Write(b.bin.Bytes())
	}
	return out.Bytes()
}
