// Package gltf reads glTF 2.0 files, .gltf and .glb, into plain Go values:
// meshes as arrays of numbers, materials, pictures still encoded, and the
// node tree. It knows nothing of graphics cards; package golib turns what it
// reads into what a game draws. Like GoLib's readers for Tiled and Aseprite,
// it uses the standard library only.
//
// What it reads: triangles (lists, strips and fans), positions, normals
// (made flat when a file has none, as the specification asks), the first set
// of texture coordinates, the first vertex colors, indices of every size,
// sparse accessors, the quantized attributes of KHR_mesh_quantization; PBR
// metallic-roughness materials with their textures, emissive color and
// strength (KHR_materials_emissive_strength), unlit materials
// (KHR_materials_unlit), alpha modes and double sides; samplers; nodes with a
// matrix or a translation, rotation and scale, their names and their extras;
// and lights (KHR_lights_punctual). Skins, animations, morph targets and
// cameras are left for later; points and lines are skipped. A file that
// requires an extension it doesn't know, such as Draco compression, is
// refused with what to change in Blender.
package gltf

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Document is what a glTF file holds.
type Document struct {
	Meshes    []Mesh
	Materials []Material
	Textures  []Texture
	Images    []Image
	Nodes     []Node
	Roots     []int // the nodes of the scene to show, at the top of the tree
	Lights    []Light
}

// Mesh is a named list of primitives, drawn together.
type Mesh struct {
	Name       string
	Primitives []Primitive
}

// Primitive is one set of triangles with one material.
type Primitive struct {
	Positions []float32 // x, y, z for each vertex
	Normals   []float32 // x, y, z for each vertex, length 1
	Texcoords []float32 // u, v for each vertex, or nil
	Colors    []float32 // r, g, b, a for each vertex, linear, or nil
	Indices   []uint32  // three a triangle, counterclockwise seen from the front
	Material  int       // -1 for glTF's default material
}

// TextureRef is a material's use of a texture.
type TextureRef struct {
	Index    int // into Document.Textures, or -1 for none
	TexCoord int // which set of texture coordinates; GoLib reads the first
	Scale    float32
}

// Material is a PBR metallic-roughness material. Colors are linear.
type Material struct {
	Name                     string
	BaseColor                [4]float32
	BaseColorTexture         TextureRef
	Metallic, Roughness      float32
	MetallicRoughnessTexture TextureRef
	NormalTexture            TextureRef // Scale is the normal scale
	OcclusionTexture         TextureRef // Scale is the strength
	Emissive                 [3]float32 // already multiplied by its strength
	EmissiveTexture          TextureRef
	AlphaMode                string // "OPAQUE", "MASK" or "BLEND"
	AlphaCutoff              float32
	DoubleSided              bool
	Unlit                    bool
}

// Texture is a picture with how to sample it.
type Texture struct {
	Image          int  // into Document.Images, or -1
	Nearest        bool // magnified without smoothing, as pixel art wants
	RepeatS        bool // repeated across, rather than clamped or mirrored
	RepeatT        bool
	MirrorS        bool
	MirrorT        bool
	NearestMipmaps bool // minified without blending between mipmaps
}

// Image is a picture still in its file format.
type Image struct {
	Name string
	MIME string // "image/png" or "image/jpeg"
	Data []byte
}

// Node is a place in the tree: a transform relative to its parent, and what
// sits there.
type Node struct {
	Name     string
	Mesh     int // -1 for none
	Light    int // -1 for none
	Children []int
	Local    [16]float32 // column-major, relative to the parent
	Extras   map[string]string
	Joint    bool // a bone of a skin, which moves the mesh rather than marking a place
}

// Light is a light from KHR_lights_punctual. Its direction, for a spot and a
// directional light, is its node's -Z.
type Light struct {
	Name       string
	Type       string // "directional", "point" or "spot"
	Color      [3]float32
	Intensity  float32
	Range      float32 // 0 for no limit
	InnerAngle float32 // radians, for a spot
	OuterAngle float32
}

// known lists the extensions this package reads, or may safely ignore.
var known = map[string]bool{
	"KHR_mesh_quantization":           true,
	"KHR_materials_emissive_strength": true,
	"KHR_materials_unlit":             true,
	"KHR_lights_punctual":             true,
	"KHR_texture_transform":           true, // ignored for now: textures show untransformed
	"KHR_materials_specular":          true,
	"KHR_materials_ior":               true,
	"KHR_materials_transmission":      true,
	"KHR_materials_volume":            true,
	"KHR_materials_clearcoat":         true,
	"KHR_materials_sheen":             true,
}

// advice says what to do about an extension GoLib can't read, in the
// exporters people use.
var advice = map[string]string{
	"KHR_draco_mesh_compression": "turn off Compression in Blender's glTF export",
	"EXT_meshopt_compression":    "export without meshopt compression",
	"KHR_texture_basisu":         "export the textures as PNG or JPEG",
	"EXT_texture_webp":           "export the textures as PNG or JPEG, not WebP",
}

// Read reads a .glb or .gltf file. readFile reads the files it names, such as
// a .bin buffer or a .png texture, by the URI written in it.
func Read(file []byte, readFile func(uri string) ([]byte, error)) (*Document, error) {
	jsonPart, binPart := file, []byte(nil)
	if bytes.HasPrefix(file, []byte("glTF")) {
		var err error
		if jsonPart, binPart, err = splitGLB(file); err != nil {
			return nil, err
		}
	}
	var raw rawDocument
	if err := json.Unmarshal(jsonPart, &raw); err != nil {
		return nil, fmt.Errorf("not a glTF file: %w", err)
	}
	if !strings.HasPrefix(raw.Asset.Version, "2.") {
		return nil, fmt.Errorf("glTF version %q: GoLib reads glTF 2.0; export it again from a current Blender", raw.Asset.Version)
	}
	for _, name := range raw.ExtensionsRequired {
		if !known[name] {
			if fix, ok := advice[name]; ok {
				return nil, fmt.Errorf("it needs %s, which GoLib doesn't read: %s", name, fix)
			}
			return nil, fmt.Errorf("it needs the extension %s, which GoLib doesn't read", name)
		}
	}
	r := reader{raw: &raw, bin: binPart, readFile: readFile}
	return r.document()
}

func splitGLB(file []byte) (jsonPart, binPart []byte, err error) {
	if len(file) < 20 {
		return nil, nil, errors.New("a .glb file too short for its header")
	}
	if version := binary.LittleEndian.Uint32(file[4:]); version != 2 {
		return nil, nil, fmt.Errorf("a version %d .glb file: GoLib reads version 2", version)
	}
	for at := 12; at+8 <= len(file); {
		length := int(binary.LittleEndian.Uint32(file[at:]))
		kind := string(file[at+4 : at+8])
		if length < 0 || at+8+length > len(file) {
			return nil, nil, errors.New("a .glb chunk runs past the end of the file: it may be cut short")
		}
		chunk := file[at+8 : at+8+length]
		switch kind {
		case "JSON":
			jsonPart = chunk
		case "BIN\x00":
			binPart = chunk
		}
		at += 8 + length
	}
	if jsonPart == nil {
		return nil, nil, errors.New("a .glb file without its JSON chunk")
	}
	return jsonPart, binPart, nil
}

type rawTextureRef struct {
	Index    *int     `json:"index"`
	TexCoord int      `json:"texCoord"`
	Scale    *float32 `json:"scale"`
	Strength *float32 `json:"strength"`
}

type rawDocument struct {
	Asset struct {
		Version string `json:"version"`
	} `json:"asset"`
	ExtensionsRequired []string `json:"extensionsRequired"`
	Scene              *int     `json:"scene"`
	Scenes             []struct {
		Nodes []int `json:"nodes"`
	} `json:"scenes"`
	Nodes []struct {
		Name        string          `json:"name"`
		Mesh        *int            `json:"mesh"`
		Children    []int           `json:"children"`
		Matrix      []float32       `json:"matrix"`
		Translation []float32       `json:"translation"`
		Rotation    []float32       `json:"rotation"`
		Scale       []float32       `json:"scale"`
		Extras      json.RawMessage `json:"extras"`
		Extensions  struct {
			Lights *struct {
				Light int `json:"light"`
			} `json:"KHR_lights_punctual"`
		} `json:"extensions"`
	} `json:"nodes"`
	Skins []struct {
		Joints []int `json:"joints"`
	} `json:"skins"`
	Meshes []struct {
		Name       string `json:"name"`
		Primitives []struct {
			Attributes map[string]int `json:"attributes"`
			Indices    *int           `json:"indices"`
			Material   *int           `json:"material"`
			Mode       *int           `json:"mode"`
		} `json:"primitives"`
	} `json:"meshes"`
	Materials []struct {
		Name string `json:"name"`
		PBR  *struct {
			BaseColorFactor          []float32      `json:"baseColorFactor"`
			BaseColorTexture         *rawTextureRef `json:"baseColorTexture"`
			MetallicFactor           *float32       `json:"metallicFactor"`
			RoughnessFactor          *float32       `json:"roughnessFactor"`
			MetallicRoughnessTexture *rawTextureRef `json:"metallicRoughnessTexture"`
		} `json:"pbrMetallicRoughness"`
		NormalTexture    *rawTextureRef `json:"normalTexture"`
		OcclusionTexture *rawTextureRef `json:"occlusionTexture"`
		EmissiveTexture  *rawTextureRef `json:"emissiveTexture"`
		EmissiveFactor   []float32      `json:"emissiveFactor"`
		AlphaMode        string         `json:"alphaMode"`
		AlphaCutoff      *float32       `json:"alphaCutoff"`
		DoubleSided      bool           `json:"doubleSided"`
		Extensions       struct {
			Strength *struct {
				EmissiveStrength float32 `json:"emissiveStrength"`
			} `json:"KHR_materials_emissive_strength"`
			Unlit *struct{} `json:"KHR_materials_unlit"`
		} `json:"extensions"`
	} `json:"materials"`
	Textures []struct {
		Sampler *int `json:"sampler"`
		Source  *int `json:"source"`
	} `json:"textures"`
	Samplers []struct {
		MagFilter int `json:"magFilter"`
		MinFilter int `json:"minFilter"`
		WrapS     int `json:"wrapS"`
		WrapT     int `json:"wrapT"`
	} `json:"samplers"`
	Images []struct {
		Name       string `json:"name"`
		URI        string `json:"uri"`
		MimeType   string `json:"mimeType"`
		BufferView *int   `json:"bufferView"`
	} `json:"images"`
	Accessors []struct {
		BufferView    *int   `json:"bufferView"`
		ByteOffset    int    `json:"byteOffset"`
		ComponentType int    `json:"componentType"`
		Normalized    bool   `json:"normalized"`
		Count         int    `json:"count"`
		Type          string `json:"type"`
		Sparse        *struct {
			Count   int `json:"count"`
			Indices struct {
				BufferView    int `json:"bufferView"`
				ByteOffset    int `json:"byteOffset"`
				ComponentType int `json:"componentType"`
			} `json:"indices"`
			Values struct {
				BufferView int `json:"bufferView"`
				ByteOffset int `json:"byteOffset"`
			} `json:"values"`
		} `json:"sparse"`
	} `json:"accessors"`
	BufferViews []struct {
		Buffer     int `json:"buffer"`
		ByteOffset int `json:"byteOffset"`
		ByteLength int `json:"byteLength"`
		ByteStride int `json:"byteStride"`
	} `json:"bufferViews"`
	Buffers []struct {
		URI        string `json:"uri"`
		ByteLength int    `json:"byteLength"`
	} `json:"buffers"`
	Extensions struct {
		Lights *struct {
			Lights []struct {
				Name      string    `json:"name"`
				Type      string    `json:"type"`
				Color     []float32 `json:"color"`
				Intensity *float32  `json:"intensity"`
				Range     float32   `json:"range"`
				Spot      *struct {
					InnerConeAngle float32  `json:"innerConeAngle"`
					OuterConeAngle *float32 `json:"outerConeAngle"`
				} `json:"spot"`
			} `json:"lights"`
		} `json:"KHR_lights_punctual"`
	} `json:"extensions"`
}

type reader struct {
	raw      *rawDocument
	bin      []byte
	readFile func(uri string) ([]byte, error)
	buffers  [][]byte
}

func (r *reader) document() (*Document, error) {
	raw := r.raw
	r.buffers = make([][]byte, len(raw.Buffers))
	for i, b := range raw.Buffers {
		var err error
		switch {
		case b.URI == "":
			if r.bin == nil {
				return nil, fmt.Errorf("buffer %d has no file and the file has no binary chunk", i)
			}
			r.buffers[i] = r.bin
		default:
			if r.buffers[i], err = r.uri(b.URI); err != nil {
				return nil, err
			}
		}
		if len(r.buffers[i]) < b.ByteLength {
			return nil, fmt.Errorf("buffer %d holds %d bytes, but the file says %d: it may be cut short", i, len(r.buffers[i]), b.ByteLength)
		}
	}

	doc := &Document{}
	for i, img := range raw.Images {
		image := Image{Name: img.Name, MIME: img.MimeType}
		var err error
		if img.BufferView != nil {
			image.Data, err = r.view(*img.BufferView)
		} else {
			image.Data, err = r.uri(img.URI)
		}
		if err != nil {
			return nil, fmt.Errorf("image %d: %w", i, err)
		}
		switch {
		case bytes.HasPrefix(image.Data, []byte("\x89PNG")):
			image.MIME = "image/png"
		case bytes.HasPrefix(image.Data, []byte("\xff\xd8")):
			image.MIME = "image/jpeg"
		default:
			return nil, fmt.Errorf("image %d (%s) is neither PNG nor JPEG: export the textures as PNG or JPEG", i, img.Name)
		}
		doc.Images = append(doc.Images, image)
	}

	for _, t := range raw.Textures {
		texture := Texture{Image: -1, RepeatS: true, RepeatT: true}
		if t.Source != nil {
			texture.Image = *t.Source
		}
		if t.Sampler != nil && *t.Sampler < len(raw.Samplers) {
			s := raw.Samplers[*t.Sampler]
			texture.Nearest = s.MagFilter == 9728
			texture.NearestMipmaps = s.MinFilter == 9728 || s.MinFilter == 9984 || s.MinFilter == 9985
			texture.RepeatS = s.WrapS == 0 || s.WrapS == 10497
			texture.RepeatT = s.WrapT == 0 || s.WrapT == 10497
			texture.MirrorS = s.WrapS == 33648
			texture.MirrorT = s.WrapT == 33648
		}
		doc.Textures = append(doc.Textures, texture)
	}

	ref := func(t *rawTextureRef) TextureRef {
		out := TextureRef{Index: -1, Scale: 1}
		if t == nil || t.Index == nil || *t.Index < 0 || *t.Index >= len(raw.Textures) {
			return out
		}
		out.Index, out.TexCoord = *t.Index, t.TexCoord
		if t.Scale != nil {
			out.Scale = *t.Scale
		}
		if t.Strength != nil {
			out.Scale = *t.Strength
		}
		return out
	}
	for _, m := range raw.Materials {
		mat := Material{
			Name: m.Name, BaseColor: [4]float32{1, 1, 1, 1}, Metallic: 1, Roughness: 1,
			BaseColorTexture: ref(nil), MetallicRoughnessTexture: ref(nil),
			NormalTexture: ref(m.NormalTexture), OcclusionTexture: ref(m.OcclusionTexture),
			EmissiveTexture: ref(m.EmissiveTexture),
			AlphaMode:       "OPAQUE", AlphaCutoff: 0.5, DoubleSided: m.DoubleSided,
			Unlit: m.Extensions.Unlit != nil,
		}
		if p := m.PBR; p != nil {
			if len(p.BaseColorFactor) == 4 {
				copy(mat.BaseColor[:], p.BaseColorFactor)
			}
			mat.BaseColorTexture = ref(p.BaseColorTexture)
			if p.MetallicFactor != nil {
				mat.Metallic = *p.MetallicFactor
			}
			if p.RoughnessFactor != nil {
				mat.Roughness = *p.RoughnessFactor
			}
			mat.MetallicRoughnessTexture = ref(p.MetallicRoughnessTexture)
		}
		if len(m.EmissiveFactor) == 3 {
			copy(mat.Emissive[:], m.EmissiveFactor)
		}
		if s := m.Extensions.Strength; s != nil {
			for i := range mat.Emissive {
				mat.Emissive[i] *= s.EmissiveStrength
			}
		}
		if m.AlphaMode != "" {
			mat.AlphaMode = m.AlphaMode
		}
		if m.AlphaCutoff != nil {
			mat.AlphaCutoff = *m.AlphaCutoff
		}
		doc.Materials = append(doc.Materials, mat)
	}

	for mi, m := range raw.Meshes {
		mesh := Mesh{Name: m.Name}
		for pi, p := range m.Primitives {
			prim, ok, err := r.primitive(p.Attributes, p.Indices, p.Mode)
			if err != nil {
				return nil, fmt.Errorf("mesh %d (%s), primitive %d: %w", mi, m.Name, pi, err)
			}
			if !ok {
				continue
			}
			prim.Material = -1
			if p.Material != nil && *p.Material >= 0 && *p.Material < len(doc.Materials) {
				prim.Material = *p.Material
			}
			mesh.Primitives = append(mesh.Primitives, prim)
		}
		doc.Meshes = append(doc.Meshes, mesh)
	}

	if l := raw.Extensions.Lights; l != nil {
		for _, light := range l.Lights {
			out := Light{Name: light.Name, Type: light.Type, Color: [3]float32{1, 1, 1}, Intensity: 1, Range: light.Range}
			if len(light.Color) == 3 {
				copy(out.Color[:], light.Color)
			}
			if light.Intensity != nil {
				out.Intensity = *light.Intensity
			}
			if s := light.Spot; s != nil {
				out.InnerAngle, out.OuterAngle = s.InnerConeAngle, math.Pi/4
				if s.OuterConeAngle != nil {
					out.OuterAngle = *s.OuterConeAngle
				}
			}
			doc.Lights = append(doc.Lights, out)
		}
	}

	for i, n := range raw.Nodes {
		node := Node{Name: n.Name, Mesh: -1, Light: -1, Children: n.Children, Local: identity}
		if n.Mesh != nil && *n.Mesh >= 0 && *n.Mesh < len(doc.Meshes) {
			node.Mesh = *n.Mesh
		}
		if l := n.Extensions.Lights; l != nil && l.Light >= 0 && l.Light < len(doc.Lights) {
			node.Light = l.Light
		}
		switch {
		case len(n.Matrix) == 16:
			copy(node.Local[:], n.Matrix)
		default:
			t, q, s := [3]float32{}, [4]float32{0, 0, 0, 1}, [3]float32{1, 1, 1}
			if len(n.Translation) == 3 {
				copy(t[:], n.Translation)
			}
			if len(n.Rotation) == 4 {
				copy(q[:], n.Rotation)
			}
			if len(n.Scale) == 3 {
				copy(s[:], n.Scale)
			}
			node.Local = trs(t, q, s)
		}
		extras, err := flattenExtras(n.Extras)
		if err != nil {
			return nil, fmt.Errorf("node %d (%s): its extras: %w", i, n.Name, err)
		}
		node.Extras = extras
		for _, c := range n.Children {
			if c < 0 || c >= len(raw.Nodes) {
				return nil, fmt.Errorf("node %d (%s) has a child %d that doesn't exist", i, n.Name, c)
			}
		}
		doc.Nodes = append(doc.Nodes, node)
	}

	for _, skin := range raw.Skins {
		for _, joint := range skin.Joints {
			if joint >= 0 && joint < len(doc.Nodes) {
				doc.Nodes[joint].Joint = true
			}
		}
	}

	switch {
	case raw.Scene != nil && *raw.Scene >= 0 && *raw.Scene < len(raw.Scenes):
		doc.Roots = raw.Scenes[*raw.Scene].Nodes
	case len(raw.Scenes) > 0:
		doc.Roots = raw.Scenes[0].Nodes
	default:
		// No scene: every node that is nobody's child.
		child := map[int]bool{}
		for _, n := range raw.Nodes {
			for _, c := range n.Children {
				child[c] = true
			}
		}
		for i := range raw.Nodes {
			if !child[i] {
				doc.Roots = append(doc.Roots, i)
			}
		}
	}
	for _, root := range doc.Roots {
		if root < 0 || root >= len(doc.Nodes) {
			return nil, fmt.Errorf("the scene names node %d, which doesn't exist", root)
		}
	}
	if err := checkTree(doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// checkTree refuses a node tree with a loop, which would never end.
func checkTree(doc *Document) error {
	state := make([]byte, len(doc.Nodes)) // 0 not seen, 1 being visited, 2 done
	var visit func(int) error
	visit = func(i int) error {
		switch state[i] {
		case 1:
			return fmt.Errorf("node %d (%s) is its own ancestor", i, doc.Nodes[i].Name)
		case 2:
			return nil
		}
		state[i] = 1
		for _, c := range doc.Nodes[i].Children {
			if err := visit(c); err != nil {
				return err
			}
		}
		state[i] = 2
		return nil
	}
	for i := range doc.Nodes {
		if err := visit(i); err != nil {
			return err
		}
	}
	return nil
}

// flattenExtras turns a node's extras, Blender's custom properties, into
// text values: numbers and true or false as they are written, objects and
// lists as JSON.
func flattenExtras(raw json.RawMessage) (map[string]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, nil // extras that aren't an object carry no properties
	}
	out := make(map[string]string, len(values))
	for name, value := range values {
		var text string
		if json.Unmarshal(value, &text) == nil {
			out[name] = text
			continue
		}
		var number float64
		if json.Unmarshal(value, &number) == nil {
			out[name] = strconv.FormatFloat(number, 'g', -1, 64)
			continue
		}
		out[name] = string(bytes.TrimSpace(value))
	}
	return out, nil
}

// uri reads a file the glTF names, or the data written inside a data URI.
func (r *reader) uri(uri string) ([]byte, error) {
	if rest, ok := strings.CutPrefix(uri, "data:"); ok {
		_, encoded, found := strings.Cut(rest, ";base64,")
		if !found {
			return nil, errors.New("a data URI that isn't base64")
		}
		return base64.StdEncoding.DecodeString(encoded)
	}
	if r.readFile == nil {
		return nil, fmt.Errorf("it names the file %s, and nothing was given to read it", uri)
	}
	return r.readFile(unescape(uri))
}

// unescape turns %20 and the like in a URI back into characters.
func unescape(uri string) string {
	if !strings.Contains(uri, "%") {
		return uri
	}
	var b strings.Builder
	for i := 0; i < len(uri); i++ {
		if uri[i] == '%' && i+2 < len(uri) {
			if v, err := strconv.ParseUint(uri[i+1:i+3], 16, 8); err == nil {
				b.WriteByte(byte(v))
				i += 2
				continue
			}
		}
		b.WriteByte(uri[i])
	}
	return b.String()
}

func (r *reader) view(i int) ([]byte, error) {
	if i < 0 || i >= len(r.raw.BufferViews) {
		return nil, fmt.Errorf("buffer view %d doesn't exist", i)
	}
	v := r.raw.BufferViews[i]
	if v.Buffer < 0 || v.Buffer >= len(r.buffers) {
		return nil, fmt.Errorf("buffer view %d names buffer %d, which doesn't exist", i, v.Buffer)
	}
	b := r.buffers[v.Buffer]
	if v.ByteOffset < 0 || v.ByteLength < 0 || v.ByteOffset+v.ByteLength > len(b) {
		return nil, fmt.Errorf("buffer view %d runs past the end of its buffer", i)
	}
	return b[v.ByteOffset : v.ByteOffset+v.ByteLength], nil
}

var componentSizes = map[int]int{5120: 1, 5121: 1, 5122: 2, 5123: 2, 5125: 4, 5126: 4}
var typeSizes = map[string]int{"SCALAR": 1, "VEC2": 2, "VEC3": 3, "VEC4": 4, "MAT2": 4, "MAT3": 9, "MAT4": 16}

// floats reads accessor i as count elements of want numbers each, as floats:
// normalized integers become 0 to 1, or -1 to 1. Elements with fewer numbers
// than want leave the rest at fill.
func (r *reader) floats(i, want int, fill float32) ([]float32, int, error) {
	if i < 0 || i >= len(r.raw.Accessors) {
		return nil, 0, fmt.Errorf("accessor %d doesn't exist", i)
	}
	a := r.raw.Accessors[i]
	comps, size := typeSizes[a.Type], componentSizes[a.ComponentType]
	if comps == 0 || size == 0 {
		return nil, 0, fmt.Errorf("accessor %d has type %s of component %d, which glTF doesn't have", i, a.Type, a.ComponentType)
	}
	out := make([]float32, a.Count*want)
	for k := range out {
		if k%want >= comps {
			out[k] = fill
		}
	}
	read := func(data []byte, stride int, element, outElement int) {
		for c := 0; c < min(comps, want); c++ {
			out[outElement*want+c] = number(data[element*stride+c*size:], a.ComponentType, a.Normalized)
		}
	}
	if a.BufferView != nil {
		data, err := r.view(*a.BufferView)
		if err != nil {
			return nil, 0, fmt.Errorf("accessor %d: %w", i, err)
		}
		stride := r.raw.BufferViews[*a.BufferView].ByteStride
		if stride == 0 {
			stride = comps * size
		}
		if a.Count > 0 && a.ByteOffset+(a.Count-1)*stride+comps*size > len(data) {
			return nil, 0, fmt.Errorf("accessor %d runs past the end of its buffer view", i)
		}
		data = data[a.ByteOffset:]
		for e := 0; e < a.Count; e++ {
			read(data, stride, e, e)
		}
	}
	if s := a.Sparse; s != nil && s.Count > 0 {
		indexData, err := r.view(s.Indices.BufferView)
		if err != nil {
			return nil, 0, fmt.Errorf("accessor %d, sparse indices: %w", i, err)
		}
		valueData, err := r.view(s.Values.BufferView)
		if err != nil {
			return nil, 0, fmt.Errorf("accessor %d, sparse values: %w", i, err)
		}
		indexSize := componentSizes[s.Indices.ComponentType]
		if indexSize == 0 || s.Indices.ByteOffset+s.Count*indexSize > len(indexData) || s.Values.ByteOffset+s.Count*comps*size > len(valueData) {
			return nil, 0, fmt.Errorf("accessor %d's sparse values run past their buffer views", i)
		}
		indexData, valueData = indexData[s.Indices.ByteOffset:], valueData[s.Values.ByteOffset:]
		for k := 0; k < s.Count; k++ {
			target := int(integer(indexData[k*indexSize:], s.Indices.ComponentType))
			if target >= a.Count {
				return nil, 0, fmt.Errorf("accessor %d's sparse values change element %d of %d", i, target, a.Count)
			}
			read(valueData, comps*size, k, target)
		}
	}
	return out, a.Count, nil
}

// indices reads accessor i as whole numbers, of any size: read as floats,
// indices past 16,777,216 would lose their last digits.
func (r *reader) indices(i int) ([]uint32, error) {
	if i < 0 || i >= len(r.raw.Accessors) {
		return nil, fmt.Errorf("accessor %d doesn't exist", i)
	}
	a := r.raw.Accessors[i]
	size := componentSizes[a.ComponentType]
	if a.Type != "SCALAR" || (a.ComponentType != 5121 && a.ComponentType != 5123 && a.ComponentType != 5125) {
		return nil, fmt.Errorf("accessor %d, used for indices, is %s of component %d: indices are unsigned whole numbers", i, a.Type, a.ComponentType)
	}
	out := make([]uint32, a.Count)
	if a.BufferView == nil || a.Sparse != nil {
		// Rare for indices; floats reads both, exactly enough for any mesh
		// a game draws.
		values, _, err := r.floats(i, 1, 0)
		for k, v := range values {
			out[k] = uint32(v)
		}
		return out, err
	}
	data, err := r.view(*a.BufferView)
	if err != nil {
		return nil, fmt.Errorf("accessor %d: %w", i, err)
	}
	stride := r.raw.BufferViews[*a.BufferView].ByteStride
	if stride == 0 {
		stride = size
	}
	if a.Count > 0 && a.ByteOffset+(a.Count-1)*stride+size > len(data) {
		return nil, fmt.Errorf("accessor %d runs past the end of its buffer view", i)
	}
	data = data[a.ByteOffset:]
	for k := range out {
		out[k] = integer(data[k*stride:], a.ComponentType)
	}
	return out, nil
}

func number(b []byte, kind int, normalized bool) float32 {
	switch kind {
	case 5126:
		return math.Float32frombits(binary.LittleEndian.Uint32(b))
	case 5121:
		if normalized {
			return float32(b[0]) / 255
		}
		return float32(b[0])
	case 5123:
		v := binary.LittleEndian.Uint16(b)
		if normalized {
			return float32(v) / 65535
		}
		return float32(v)
	case 5125:
		return float32(binary.LittleEndian.Uint32(b))
	case 5120:
		v := float32(int8(b[0]))
		if normalized {
			return max(v/127, -1)
		}
		return v
	case 5122:
		v := float32(int16(binary.LittleEndian.Uint16(b)))
		if normalized {
			return max(v/32767, -1)
		}
		return v
	}
	return 0
}

func integer(b []byte, kind int) uint32 {
	switch kind {
	case 5121:
		return uint32(b[0])
	case 5123:
		return uint32(binary.LittleEndian.Uint16(b))
	default:
		return binary.LittleEndian.Uint32(b)
	}
}

// primitive reads one primitive, and says false for one it skips: points and
// lines, or one without positions.
func (r *reader) primitive(attributes map[string]int, indicesAt *int, modeAt *int) (Primitive, bool, error) {
	mode := 4
	if modeAt != nil {
		mode = *modeAt
	}
	if mode < 4 || mode > 6 {
		return Primitive{}, false, nil
	}
	at, ok := attributes["POSITION"]
	if !ok {
		return Primitive{}, false, nil
	}
	var p Primitive
	var count int
	var err error
	if p.Positions, count, err = r.floats(at, 3, 0); err != nil {
		return p, false, err
	}
	if at, ok := attributes["NORMAL"]; ok {
		var n int
		if p.Normals, n, err = r.floats(at, 3, 0); err != nil {
			return p, false, err
		}
		if n != count {
			return p, false, fmt.Errorf("%d normals for %d positions", n, count)
		}
	}
	if at, ok := attributes["TEXCOORD_0"]; ok {
		var n int
		if p.Texcoords, n, err = r.floats(at, 2, 0); err != nil {
			return p, false, err
		}
		if n != count {
			return p, false, fmt.Errorf("%d texture coordinates for %d positions", n, count)
		}
	}
	if at, ok := attributes["COLOR_0"]; ok {
		var n int
		if p.Colors, n, err = r.floats(at, 4, 1); err != nil {
			return p, false, err
		}
		if n != count {
			return p, false, fmt.Errorf("%d colors for %d positions", n, count)
		}
	}

	var indices []uint32
	if indicesAt != nil {
		if indices, err = r.indices(*indicesAt); err != nil {
			return p, false, err
		}
		for k, v := range indices {
			if int(v) >= count {
				return p, false, fmt.Errorf("index %d names vertex %d of %d", k, v, count)
			}
		}
	} else {
		indices = make([]uint32, count)
		for k := range indices {
			indices[k] = uint32(k)
		}
	}
	switch mode {
	case 5: // a strip: every vertex after the second makes a triangle with the two before
		var list []uint32
		for k := 2; k < len(indices); k++ {
			if k%2 == 0 {
				list = append(list, indices[k-2], indices[k-1], indices[k])
			} else {
				list = append(list, indices[k-1], indices[k-2], indices[k])
			}
		}
		indices = list
	case 6: // a fan: every vertex after the second makes a triangle with the first
		var list []uint32
		for k := 2; k < len(indices); k++ {
			list = append(list, indices[0], indices[k-1], indices[k])
		}
		indices = list
	}
	indices = indices[:len(indices)/3*3]
	p.Indices = indices
	if p.Normals == nil {
		p.flatten()
	}
	return p, true, nil
}

// flatten gives every triangle its own three vertices, with the triangle's
// own normal: the flat normals glTF asks for when a file has none.
func (p *Primitive) flatten() {
	var positions, normals, texcoords, colors []float32
	for _, i := range p.Indices {
		positions = append(positions, p.Positions[i*3:i*3+3]...)
		if p.Texcoords != nil {
			texcoords = append(texcoords, p.Texcoords[i*2:i*2+2]...)
		}
		if p.Colors != nil {
			colors = append(colors, p.Colors[i*4:i*4+4]...)
		}
	}
	for t := 0; t+8 < len(positions); t += 9 {
		ax, ay, az := positions[t+3]-positions[t], positions[t+4]-positions[t+1], positions[t+5]-positions[t+2]
		bx, by, bz := positions[t+6]-positions[t], positions[t+7]-positions[t+1], positions[t+8]-positions[t+2]
		nx, ny, nz := ay*bz-az*by, az*bx-ax*bz, ax*by-ay*bx
		if l := float32(math.Sqrt(float64(nx*nx + ny*ny + nz*nz))); l > 0 {
			nx, ny, nz = nx/l, ny/l, nz/l
		}
		normals = append(normals, nx, ny, nz, nx, ny, nz, nx, ny, nz)
	}
	p.Positions, p.Normals, p.Texcoords, p.Colors = positions, normals, texcoords, colors
	p.Indices = make([]uint32, len(positions)/3)
	for k := range p.Indices {
		p.Indices[k] = uint32(k)
	}
}

var identity = [16]float32{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}

// trs builds translation times rotation times scale, with the rotation as a
// quaternion x, y, z, w, in column-major order.
func trs(t [3]float32, q [4]float32, s [3]float32) [16]float32 {
	length := float32(math.Sqrt(float64(q[0]*q[0] + q[1]*q[1] + q[2]*q[2] + q[3]*q[3])))
	if length == 0 {
		q, length = [4]float32{0, 0, 0, 1}, 1
	}
	x, y, z, w := q[0]/length, q[1]/length, q[2]/length, q[3]/length
	xx, yy, zz := x*x, y*y, z*z
	xy, xz, yz := x*y, x*z, y*z
	wx, wy, wz := w*x, w*y, w*z
	return [16]float32{
		(1 - 2*(yy+zz)) * s[0], 2 * (xy + wz) * s[0], 2 * (xz - wy) * s[0], 0,
		2 * (xy - wz) * s[1], (1 - 2*(xx+zz)) * s[1], 2 * (yz + wx) * s[1], 0,
		2 * (xz + wy) * s[2], 2 * (yz - wx) * s[2], (1 - 2*(xx+yy)) * s[2], 0,
		t[0], t[1], t[2], 1,
	}
}
