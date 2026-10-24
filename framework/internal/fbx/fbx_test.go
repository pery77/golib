package fbx

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"math"
	"os"
	"strings"
	"testing"

	"golib/internal/gltf"
)

// rec is a record for writeFBX: a name, its values (int32, int64, float64,
// bool, string, []byte, []float64, []int32, []int64) and the records in it.
type rec struct {
	name     string
	values   []any
	children []rec
}

// writeFBX writes records as a binary FBX file of version, compressing the
// arrays, so the tests make the files they read.
func writeFBX(version int, records []rec) []byte {
	wide := version >= 7500
	var out bytes.Buffer
	out.WriteString(binaryMagic)
	out.Write([]byte{0x1a, 0})
	binary.Write(&out, binary.LittleEndian, uint32(version))
	var write func(r rec)
	null := func() {
		if wide {
			out.Write(make([]byte, 25))
		} else {
			out.Write(make([]byte, 13))
		}
	}
	write = func(r rec) {
		var values bytes.Buffer
		for _, v := range r.values {
			array := func(kind byte, data []byte, count int) {
				var z bytes.Buffer
				w := zlib.NewWriter(&z)
				w.Write(data)
				w.Close()
				values.WriteByte(kind)
				binary.Write(&values, binary.LittleEndian, uint32(count))
				binary.Write(&values, binary.LittleEndian, uint32(1))
				binary.Write(&values, binary.LittleEndian, uint32(z.Len()))
				values.Write(z.Bytes())
			}
			switch x := v.(type) {
			case int32:
				values.WriteByte('I')
				binary.Write(&values, binary.LittleEndian, x)
			case int64:
				values.WriteByte('L')
				binary.Write(&values, binary.LittleEndian, x)
			case float64:
				values.WriteByte('D')
				binary.Write(&values, binary.LittleEndian, x)
			case bool:
				values.WriteByte('C')
				if x {
					values.WriteByte(1)
				} else {
					values.WriteByte(0)
				}
			case string:
				values.WriteByte('S')
				binary.Write(&values, binary.LittleEndian, uint32(len(x)))
				values.WriteString(x)
			case []byte:
				values.WriteByte('R')
				binary.Write(&values, binary.LittleEndian, uint32(len(x)))
				values.Write(x)
			case []float64:
				var d bytes.Buffer
				binary.Write(&d, binary.LittleEndian, x)
				array('d', d.Bytes(), len(x))
			case []int32:
				var d bytes.Buffer
				binary.Write(&d, binary.LittleEndian, x)
				array('i', d.Bytes(), len(x))
			case []int64:
				var d bytes.Buffer
				binary.Write(&d, binary.LittleEndian, x)
				array('l', d.Bytes(), len(x))
			}
		}
		start := out.Len()
		header := 13
		if wide {
			header = 25
		}
		out.Write(make([]byte, header))
		out.Bytes()[start+header-1] = byte(len(r.name))
		out.WriteString(r.name)
		out.Write(values.Bytes())
		for _, c := range r.children {
			write(c)
		}
		if len(r.children) > 0 {
			null()
		}
		head := out.Bytes()[start:]
		if wide {
			binary.LittleEndian.PutUint64(head, uint64(out.Len()))
			binary.LittleEndian.PutUint64(head[8:], uint64(len(r.values)))
			binary.LittleEndian.PutUint64(head[16:], uint64(values.Len()))
		} else {
			binary.LittleEndian.PutUint32(head, uint32(out.Len()))
			binary.LittleEndian.PutUint32(head[4:], uint32(len(r.values)))
			binary.LittleEndian.PutUint32(head[8:], uint32(values.Len()))
		}
	}
	for _, r := range records {
		write(r)
	}
	null()
	out.Write(make([]byte, 160)) // the footer, which the reader doesn't need
	return out.Bytes()
}

// p is a Properties70 entry.
func p(name, kind, flags string, values ...any) rec {
	return rec{name: "P", values: append([]any{name, kind, "", flags}, values...)}
}

func props(entries ...rec) rec { return rec{name: "Properties70", children: entries} }

// object is an entry of Objects, named as binary FBX names them.
func obj(kind string, id int64, name, class, sub string, children ...rec) rec {
	return rec{name: kind, values: []any{id, name + "\x00\x01" + class, sub}, children: children}
}

func oo(child, parent int64) rec { return rec{name: "C", values: []any{"OO", child, parent}} }

func op(child, parent int64, property string) rec {
	return rec{name: "C", values: []any{"OP", child, parent, property}}
}

// zUpQuad is a file as 3ds Max writes one: Z up, -Y forward, in meters, with
// a square floor one meter a side, raised by its node with a pre-rotation
// and a pivot, a red material and a custom property.
func zUpQuad(version int) []byte {
	return writeFBX(version, []rec{
		{name: "GlobalSettings", children: []rec{props(
			p("UpAxis", "int", "", int32(2)), p("UpAxisSign", "int", "", int32(1)),
			p("FrontAxis", "int", "", int32(1)), p("FrontAxisSign", "int", "", int32(-1)),
			p("CoordAxis", "int", "", int32(0)), p("CoordAxisSign", "int", "", int32(1)),
			p("UnitScaleFactor", "double", "", 100.0),
		)}},
		{name: "Objects", children: []rec{
			obj("Model", 10, "Floor", "Model", "Mesh", props(
				p("Lcl Translation", "Lcl Translation", "A", 0.0, 0.0, 2.0),
				p("PreRotation", "Vector3D", "", 0.0, 0.0, 90.0),
				p("RotationPivot", "Vector3D", "", 1.0, 0.0, 0.0),
				p("kind", "KString", "U", "coin"),
			)),
			obj("Geometry", 20, "Floor", "Geometry", "Mesh",
				rec{name: "Vertices", values: []any{[]float64{0, 0, 0, 1, 0, 0, 1, 1, 0, 0, 1, 0}}},
				rec{name: "PolygonVertexIndex", values: []any{[]int32{0, 1, 2, -4}}},
				rec{name: "LayerElementNormal", values: []any{int32(0)}, children: []rec{
					{name: "MappingInformationType", values: []any{"ByVertice"}},
					{name: "ReferenceInformationType", values: []any{"Direct"}},
					{name: "Normals", values: []any{[]float64{0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1}}},
				}},
			),
			obj("Material", 30, "Red", "Material", "", props(p("DiffuseColor", "Color", "A", 1.0, 0.0, 0.0))),
		}},
		{name: "Connections", children: []rec{oo(10, 0), oo(20, 10), oo(30, 10)}},
	})
}

func TestReadAxesUnitsAndTransforms(t *testing.T) {
	for _, version := range []int{7400, 7500} {
		doc, err := Read(zUpQuad(version), nil)
		if err != nil {
			t.Fatalf("version %d: %v", version, err)
		}
		if len(doc.Nodes) != 1 || doc.Nodes[0].Name != "Floor" || doc.Nodes[0].Mesh != 0 {
			t.Fatalf("version %d: nodes %+v", version, doc.Nodes)
		}
		if doc.Nodes[0].Extras["kind"] != "coin" {
			t.Errorf("the custom property read as %v", doc.Nodes[0].Extras)
		}
		// In the file: the square turned a quarter around Z about the
		// pivot at x = 1, then raised 2 along Z, which is up. In GoLib's
		// space, in meters: Y up, the file's -Y forward along +Z.
		local := mat{}
		for i, v := range doc.Nodes[0].Local {
			local[i] = float64(v)
		}
		mesh := doc.Meshes[0]
		if len(mesh.Primitives) != 1 || len(mesh.Primitives[0].Indices) != 6 {
			t.Fatalf("the square became %+v, want two triangles", mesh.Primitives)
		}
		prim := mesh.Primitives[0]
		var lowest, highest [3]float64
		for i := 0; i < len(prim.Positions); i += 3 {
			q := local.point([3]float64{float64(prim.Positions[i]), float64(prim.Positions[i+1]), float64(prim.Positions[i+2])})
			for k := range 3 {
				if i == 0 || q[k] < lowest[k] {
					lowest[k] = q[k]
				}
				if i == 0 || q[k] > highest[k] {
					highest[k] = q[k]
				}
			}
		}
		// Turned about the pivot at x = 1, the square spans x from 0 to 1
		// and y from -1 to 0, at z = 2. Z is GoLib's Y, and -Y its Z.
		want := [2][3]float64{{0, 2, 0}, {1, 2, 1}}
		for k := range 3 {
			if math.Abs(lowest[k]-want[0][k]) > 1e-5 || math.Abs(highest[k]-want[1][k]) > 1e-5 {
				t.Errorf("version %d: the square spans %v to %v in GoLib's space, want %v to %v", version, lowest, highest, want[0], want[1])
				break
			}
		}
		n := local.normalMatrix().direction([3]float64{float64(prim.Normals[0]), float64(prim.Normals[1]), float64(prim.Normals[2])})
		if math.Abs(n[1]-1) > 1e-5 {
			t.Errorf("the floor's normal, placed, is %v, want up, along Y", n)
		}
		if m := doc.Materials[0]; m.BaseColor != [4]float32{1, 0, 0, 1} || prim.Material != 0 {
			t.Errorf("material %+v, primitive's %d", m, prim.Material)
		}
	}
}

func TestEulerOrders(t *testing.T) {
	r := [3]float64{30, 40, 50}
	x, y, z := euler([3]float64{30, 0, 0}, 0), euler([3]float64{0, 40, 0}, 0), euler([3]float64{0, 0, 50}, 0)
	for order, want := range []mat{z.mul(y).mul(x), y.mul(z).mul(x), x.mul(z).mul(y), z.mul(x).mul(y), y.mul(x).mul(z), x.mul(y).mul(z)} {
		got := euler(r, order)
		for i := range got {
			if math.Abs(got[i]-want[i]) > 1e-9 {
				t.Errorf("order %d: %v, want %v", order, got, want)
				break
			}
		}
	}
	// The quaternion of a turn, back to the turn.
	m := euler(r, 4).mul(scaling([3]float64{2, 3, 4}))
	_, q, s := m.decompose()
	back := quaternionMatrix(q).mul(scaling([3]float64{float64(s[0]), float64(s[1]), float64(s[2])}))
	for i := range m {
		if math.Abs(m[i]-back[i]) > 1e-5 {
			t.Fatalf("decompose then compose gave %v, want %v", back, m)
		}
	}
}

// quaternionMatrix is the turn of q, x, y, z, w.
func quaternionMatrix(q [4]float32) mat {
	x, y, z, w := float64(q[0]), float64(q[1]), float64(q[2]), float64(q[3])
	return mat{
		1 - 2*(y*y+z*z), 2 * (x*y + w*z), 2 * (x*z - w*y), 0,
		2 * (x*y - w*z), 1 - 2*(x*x+z*z), 2 * (y*z + w*x), 0,
		2 * (x*z + w*y), 2 * (y*z - w*x), 1 - 2*(x*x+y*y), 0,
		0, 0, 0, 1,
	}
}

func TestCurves(t *testing.T) {
	c := readCurve(&element{children: []*element{
		{name: "KeyTime", values: []any{[]int64{0, ticks, 2 * ticks}}},
		{name: "KeyValueFloat", values: []any{[]float64{0, 10, 20}}},
		{name: "KeyAttrFlags", values: []any{[]int64{0x02, 0x04}}},
		{name: "KeyAttrRefCount", values: []any{[]int64{1, 2}}},
	}})
	for _, tc := range []struct{ t, want float64 }{{0.5, 0}, {1.5, 15}, {3, 20}, {-1, 0}} {
		if got := c.at(tc.t); got != tc.want {
			t.Errorf("at %v: %v, want %v (constant until 1, then linear)", tc.t, got, tc.want)
		}
	}
	times := sampleTimes([]*curve{c}, 0)
	if len(times) != 61 {
		t.Errorf("sampled two seconds into %d times, want every thirtieth of a second", len(times))
	}
}

func TestReadRefuses(t *testing.T) {
	for _, c := range []struct {
		file []byte
		want string
	}{
		{[]byte("; FBX 7.4.0 project file\nFBXHeaderExtension:  {\n"), "saved as text"},
		{append([]byte(binaryMagic+"\x1a\x00"), 0xb8, 0x1b, 0, 0), "FBX version 7096"},
		{[]byte("glTF"), "not an FBX file"},
		{zUpQuad(7400)[:200], "cut short"},
	} {
		if _, err := Read(c.file, nil); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("Read gave %v, want a mistake saying %q", err, c.want)
		}
	}
}

// TestReadBlender reads the tube Blender exported (testdata/tube.py): its
// bones, its skin, its shape key and its animation. package golib checks
// that it poses as the glTF export of the same scene does.
func TestReadBlender(t *testing.T) {
	file, err := os.ReadFile("testdata/tube.fbx")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Read(file, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Skins) != 1 || len(doc.Skins[0].Joints) != 3 {
		t.Fatalf("skins %+v, want one of three bones", doc.Skins)
	}
	mesh := doc.Meshes[0]
	if len(mesh.TargetNames) != 1 || mesh.TargetNames[0] != "Bulge" || len(mesh.Primitives[0].Targets) != 1 {
		t.Errorf("shape keys %v, targets %d", mesh.TargetNames, len(mesh.Primitives[0].Targets))
	}
	p := mesh.Primitives[0]
	for v := 0; v < len(p.Weights); v += 4 {
		if sum := p.Weights[v] + p.Weights[v+1] + p.Weights[v+2] + p.Weights[v+3]; math.Abs(float64(sum)-1) > 1e-4 {
			t.Fatalf("vertex %d's weights add up to %v", v/4, sum)
		}
	}
	if len(doc.Animations) != 1 || doc.Animations[0].Name != "Sway" {
		t.Fatalf("animations %+v, want Blender's action Sway", doc.Animations)
	}
	weights := false
	for _, c := range doc.Animations[0].Channels {
		weights = weights || c.Path == "weights"
	}
	if !weights {
		t.Error("the shape key's animation, in Sway's take, wasn't read")
	}
}

func TestTrimStillEnd(t *testing.T) {
	a := gltf.Animation{Channels: []gltf.Channel{
		{Times: []float32{0, 1, 2, 3, 4}, Values: []float32{0, 5, 7, 7, 7}},
		{Times: []float32{0, 1, 2, 3, 4}, Values: []float32{1, 1, 1, 1, 1, 2, 2, 2, 2, 2}},
	}}
	trimStillEnd(&a)
	// The second channel moves last, from 2 s to 3 s: both end there.
	for i, want := range []int{4, 4} {
		if len(a.Channels[i].Times) != want {
			t.Errorf("channel %d keeps %v, want its keys up to 3 s, where the last movement ends", i, a.Channels[i].Times)
		}
	}
}
