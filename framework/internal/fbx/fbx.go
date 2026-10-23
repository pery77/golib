// Package fbx reads FBX files, binary FBX 7.1 and later, as Autodesk's
// tools, Blender, Mixamo and most model stores write them, into the same
// document package gltf reads glTF files into, so package golib draws and
// animates both alike. Like GoLib's other readers, it uses the standard
// library only.
//
// What it reads: the node tree with FBX's transforms (pivots, pre- and
// post-rotations, rotation orders, geometric transforms), turned into
// meters, Y up and +Z forward from the file's own units and axes; meshes
// with their normals, first texture coordinates, first vertex colors and
// materials, cut into triangles; Phong and Lambert materials with their
// textures, embedded or beside the file; skins, with the four strongest
// bones of each vertex; blend shapes, Blender's shape keys, with their
// names; user properties, Blender's custom properties, as extras; and
// animation stacks, each a glTF animation, sampled into translations,
// rotations and scales. FBX files saved as text, and files older than FBX
// 7.1 (2011), are refused with what to do instead.
package fbx

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"path"
	"sort"
	"strconv"
	"strings"

	"golib/internal/gltf"
)

// element is one record of the file: a name, its values, and the records
// inside it.
type element struct {
	name     string
	values   []any // int64, float64, bool, string, []byte, []float64 or []int64
	children []*element
}

func (e *element) child(name string) *element {
	for _, c := range e.children {
		if c.name == name {
			return c
		}
	}
	return nil
}

// float returns value i as a number, or 0.
func (e *element) float(i int) float64 {
	if e == nil || i >= len(e.values) {
		return 0
	}
	return toFloat(e.values[i])
}

func (e *element) text(i int) string {
	if e == nil || i >= len(e.values) {
		return ""
	}
	s, _ := e.values[i].(string)
	return s
}

// floats returns the array in the first value of the child called name.
func (e *element) floats(name string) []float64 {
	c := e.child(name)
	if c == nil || len(c.values) == 0 {
		return nil
	}
	switch v := c.values[0].(type) {
	case []float64:
		return v
	case []int64:
		out := make([]float64, len(v))
		for i, x := range v {
			out[i] = float64(x)
		}
		return out
	}
	return nil
}

func (e *element) ints(name string) []int64 {
	c := e.child(name)
	if c == nil || len(c.values) == 0 {
		return nil
	}
	switch v := c.values[0].(type) {
	case []int64:
		return v
	case []float64:
		out := make([]int64, len(v))
		for i, x := range v {
			out[i] = int64(x)
		}
		return out
	}
	return nil
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case int64:
		return float64(x)
	case float64:
		return x
	case bool:
		if x {
			return 1
		}
	}
	return 0
}

// binaryMagic starts every binary FBX file.
const binaryMagic = "Kaydara FBX Binary  \x00"

// parse reads the file's records.
func parse(file []byte) (*element, error) {
	if !bytes.HasPrefix(file, []byte(binaryMagic)) {
		if bytes.Contains(file[:min(len(file), 4096)], []byte("FBXHeaderExtension")) {
			return nil, errors.New("an FBX file saved as text (ASCII): export it again as binary FBX, or as glTF")
		}
		return nil, errors.New("not an FBX file")
	}
	if len(file) < 27 {
		return nil, errors.New("an FBX file too short for its header")
	}
	version := int(binary.LittleEndian.Uint32(file[23:]))
	if version < 7100 {
		return nil, fmt.Errorf("FBX version %d: GoLib reads FBX 7.1 (2011) and later; export it again as FBX 7.4 or glTF", version)
	}
	p := parser{data: file, wide: version >= 7500}
	root := &element{name: "", values: []any{int64(version)}}
	at := 27
	for {
		e, next, err := p.record(at)
		if err != nil {
			return nil, err
		}
		if e == nil {
			break
		}
		root.children = append(root.children, e)
		at = next
	}
	return root, nil
}

type parser struct {
	data []byte
	wide bool // 64-bit offsets, from FBX 7.5
}

var errShort = errors.New("the FBX file is cut short")

// record reads the record at at, and returns where the next one starts. It
// returns nil at the empty record that ends a list.
func (p *parser) record(at int) (*element, int, error) {
	d := p.data
	header := 13
	if p.wide {
		header = 25
	}
	if at+header > len(d) {
		return nil, at, nil // the end of the file's records
	}
	var end, count int
	if p.wide {
		end, count = int(binary.LittleEndian.Uint64(d[at:])), int(binary.LittleEndian.Uint64(d[at+8:]))
	} else {
		end, count = int(binary.LittleEndian.Uint32(d[at:])), int(binary.LittleEndian.Uint32(d[at+4:]))
	}
	nameLength := int(d[at+header-1])
	if end == 0 {
		return nil, at + header + nameLength, nil
	}
	if end > len(d) || end < at {
		return nil, 0, errShort
	}
	at += header
	if at+nameLength > end {
		return nil, 0, errShort
	}
	e := &element{name: string(d[at : at+nameLength])}
	at += nameLength
	for range count {
		v, next, err := p.value(at, end)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", e.name, err)
		}
		e.values = append(e.values, v)
		at = next
	}
	for at < end {
		c, next, err := p.record(at)
		if err != nil {
			return nil, 0, err
		}
		at = next
		if c == nil {
			break
		}
		e.children = append(e.children, c)
	}
	return e, end, nil
}

// value reads one value of a record, at at, before end.
func (p *parser) value(at, end int) (any, int, error) {
	d := p.data
	if at >= end {
		return nil, 0, errShort
	}
	kind := d[at]
	at++
	need := func(n int) error {
		if at+n > end {
			return errShort
		}
		return nil
	}
	switch kind {
	case 'Y':
		if err := need(2); err != nil {
			return nil, 0, err
		}
		return int64(int16(binary.LittleEndian.Uint16(d[at:]))), at + 2, nil
	case 'C':
		if err := need(1); err != nil {
			return nil, 0, err
		}
		return d[at] != 0, at + 1, nil
	case 'I':
		if err := need(4); err != nil {
			return nil, 0, err
		}
		return int64(int32(binary.LittleEndian.Uint32(d[at:]))), at + 4, nil
	case 'F':
		if err := need(4); err != nil {
			return nil, 0, err
		}
		return float64(math.Float32frombits(binary.LittleEndian.Uint32(d[at:]))), at + 4, nil
	case 'D':
		if err := need(8); err != nil {
			return nil, 0, err
		}
		return math.Float64frombits(binary.LittleEndian.Uint64(d[at:])), at + 8, nil
	case 'L':
		if err := need(8); err != nil {
			return nil, 0, err
		}
		return int64(binary.LittleEndian.Uint64(d[at:])), at + 8, nil
	case 'S', 'R':
		if err := need(4); err != nil {
			return nil, 0, err
		}
		n := int(binary.LittleEndian.Uint32(d[at:]))
		at += 4
		if err := need(n); err != nil {
			return nil, 0, err
		}
		if kind == 'S' {
			return string(d[at : at+n]), at + n, nil
		}
		return d[at : at+n], at + n, nil
	case 'f', 'd', 'l', 'i', 'b':
		if err := need(12); err != nil {
			return nil, 0, err
		}
		count := int(binary.LittleEndian.Uint32(d[at:]))
		encoding := binary.LittleEndian.Uint32(d[at+4:])
		size := int(binary.LittleEndian.Uint32(d[at+8:]))
		at += 12
		if err := need(size); err != nil {
			return nil, 0, err
		}
		raw := d[at : at+size]
		width := map[byte]int{'f': 4, 'd': 8, 'l': 8, 'i': 4, 'b': 1}[kind]
		if encoding == 1 {
			r, err := zlib.NewReader(bytes.NewReader(raw))
			if err != nil {
				return nil, 0, fmt.Errorf("a compressed array: %w", err)
			}
			raw, err = io.ReadAll(io.LimitReader(r, int64(count*width)))
			if err != nil {
				return nil, 0, fmt.Errorf("a compressed array: %w", err)
			}
		}
		if len(raw) < count*width {
			return nil, 0, errShort
		}
		at += size
		switch kind {
		case 'f':
			out := make([]float64, count)
			for i := range out {
				out[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:])))
			}
			return out, at, nil
		case 'd':
			out := make([]float64, count)
			for i := range out {
				out[i] = math.Float64frombits(binary.LittleEndian.Uint64(raw[i*8:]))
			}
			return out, at, nil
		case 'l':
			out := make([]int64, count)
			for i := range out {
				out[i] = int64(binary.LittleEndian.Uint64(raw[i*8:]))
			}
			return out, at, nil
		case 'i':
			out := make([]int64, count)
			for i := range out {
				out[i] = int64(int32(binary.LittleEndian.Uint32(raw[i*4:])))
			}
			return out, at, nil
		default:
			out := make([]int64, count)
			for i := range out {
				out[i] = int64(raw[i])
			}
			return out, at, nil
		}
	}
	return nil, 0, fmt.Errorf("a value of unknown type %q", kind)
}

// properties reads a Properties70 list: each property's values after its
// name and types, and which are a user's own, Blender's custom properties.
type properties struct {
	values map[string][]any
	user   []string // in the file's order
}

func readProperties(e *element) properties {
	p := properties{values: map[string][]any{}}
	list := e.child("Properties70")
	if list == nil {
		return p
	}
	for _, prop := range list.children {
		if prop.name != "P" || len(prop.values) < 4 {
			continue
		}
		name := prop.text(0)
		p.values[name] = prop.values[4:]
		if strings.Contains(prop.text(3), "U") {
			p.user = append(p.user, name)
		}
	}
	return p
}

// number returns the property's value i, or fallback when it has none.
func (p properties) number(name string, i int, fallback float64) float64 {
	v, ok := p.values[name]
	if !ok || i >= len(v) {
		return fallback
	}
	return toFloat(v[i])
}

// vector returns a property of three numbers.
func (p properties) vector(name string, fallback [3]float64) [3]float64 {
	return [3]float64{p.number(name, 0, fallback[0]), p.number(name, 1, fallback[1]), p.number(name, 2, fallback[2])}
}

func (p properties) has(name string) bool {
	_, ok := p.values[name]
	return ok
}

// object is an entry of the file's Objects.
type object struct {
	id    int64
	kind  string // Model, Geometry, Material, Deformer, AnimationCurve...
	name  string
	sub   string // Mesh, LimbNode, Null, Skin, Cluster, Shape...
	e     *element
	props properties
}

// link is a connection between two objects, with the property it goes to.
type link struct {
	id       int64
	property string
}

// scene is what the reader works from: the objects and how they connect.
type scene struct {
	objects  map[int64]*object
	order    []*object // as the file lists them
	children map[int64][]link
	parents  map[int64][]link
	readFile func(string) ([]byte, error)
}

// Read reads an FBX file. readFile reads the files it names, such as
// textures that aren't embedded in it, by their path relative to the file.
func Read(file []byte, readFile func(name string) ([]byte, error)) (*gltf.Document, error) {
	root, err := parse(file)
	if err != nil {
		return nil, err
	}
	s := &scene{objects: map[int64]*object{}, children: map[int64][]link{}, parents: map[int64][]link{}, readFile: readFile}
	if objects := root.child("Objects"); objects != nil {
		for _, e := range objects.children {
			if len(e.values) < 3 {
				continue
			}
			id, _ := e.values[0].(int64)
			name := e.text(1)
			if cut := strings.Index(name, "\x00\x01"); cut >= 0 {
				name = name[:cut]
			} else if strings.Contains(name, "::") {
				name = name[strings.Index(name, "::")+2:]
			}
			o := &object{id: id, kind: e.name, name: name, sub: e.text(2), e: e, props: readProperties(e)}
			s.objects[id] = o
			s.order = append(s.order, o)
		}
	}
	if connections := root.child("Connections"); connections != nil {
		for _, c := range connections.children {
			if c.name != "C" || len(c.values) < 3 {
				continue
			}
			child, _ := c.values[1].(int64)
			parent, _ := c.values[2].(int64)
			property := c.text(3)
			s.children[parent] = append(s.children[parent], link{child, property})
			s.parents[child] = append(s.parents[child], link{parent, property})
		}
	}
	b := builder{scene: s, doc: &gltf.Document{}, nodeOf: map[int64]int{}}
	b.settings(root.child("GlobalSettings"))
	if err := b.build(); err != nil {
		return nil, err
	}
	return b.doc, nil
}

// connected returns the objects of kind connected below id, in the file's
// order of connections, with the property each is connected to.
func (s *scene) connected(id int64, kind string) []link {
	var out []link
	for _, l := range s.children[id] {
		if o := s.objects[l.id]; o != nil && o.kind == kind {
			out = append(out, l)
		}
	}
	return out
}

// above returns the objects of kind that id is connected to.
func (s *scene) above(id int64, kind string) []link {
	var out []link
	for _, l := range s.parents[id] {
		if o := s.objects[l.id]; o != nil && o.kind == kind {
			out = append(out, l)
		}
	}
	return out
}

// builder turns the scene into a glTF document.
type builder struct {
	*scene
	doc        *gltf.Document
	nodeOf     map[int64]int // a Model's node
	models     []*model
	convert    mat // from the file's units and axes into meters, Y up, +Z forward
	materialOf map[int64]int
	textureOf  map[int64]int
	imageOf    map[int64]int
}

// model is a Model object with its transform's parts.
type model struct {
	*object
	node      int
	parent    int // its node's parent node, -1 at the top
	geometry  *object
	order     int // the Euler order of its rotation
	pre       mat // its pre-rotation, rotation offset and pivot, as matrices
	rotation  [3]float64
	trans     [3]float64
	scale     [3]float64
	geometric mat
}

// settings reads the file's units and axes.
func (b *builder) settings(e *element) {
	p := properties{values: map[string][]any{}}
	if e != nil {
		p = readProperties(e)
	}
	unit := p.number("UnitScaleFactor", 0, 1) / 100 // centimeters, unless the file says
	axis := func(name string, fallback float64) (int, float64) {
		a := int(p.number(name, 0, fallback))
		sign := p.number(name+"Sign", 0, 1)
		if a < 0 || a > 2 {
			a = int(fallback)
		}
		if sign == 0 {
			sign = 1
		}
		return a, sign
	}
	coord, coordSign := axis("CoordAxis", 0)
	up, upSign := axis("UpAxis", 1)
	front, frontSign := axis("FrontAxis", 2)
	// GoLib's X, Y and Z are the file's coordinate, up and front axes.
	var m mat
	m[15] = 1
	m[coord*4+0] = coordSign * unit
	m[up*4+1] = upSign * unit
	m[front*4+2] = frontSign * unit
	b.convert = m
}

func (b *builder) build() error {
	if err := b.buildMaterials(); err != nil {
		return err
	}
	// The nodes: every Model, in the file's order.
	for _, o := range b.order {
		if o.kind != "Model" {
			continue
		}
		m := &model{object: o, node: len(b.doc.Nodes), parent: -1}
		b.nodeOf[o.id] = m.node
		b.models = append(b.models, m)
		b.doc.Nodes = append(b.doc.Nodes, gltf.Node{Name: o.name, Mesh: -1, Light: -1, Skin: -1, Joint: o.sub == "LimbNode" || o.sub == "Root"})
	}
	for _, m := range b.models {
		for _, l := range b.above(m.id, "Model") {
			if parent, ok := b.nodeOf[l.id]; ok {
				m.parent = parent
				b.doc.Nodes[parent].Children = append(b.doc.Nodes[parent].Children, m.node)
				break
			}
		}
		if m.parent < 0 {
			b.doc.Roots = append(b.doc.Roots, m.node)
		}
		m.read()
		node := &b.doc.Nodes[m.node]
		local := b.local(m, m.trans, m.rotation, m.scale)
		node.Local = local.float32s()
		t, q, s := local.decompose()
		node.Translation, node.Rotation, node.Scale = t, q, s
		node.Extras = m.extras()
	}
	if err := checkTree(b.doc); err != nil {
		return err
	}
	for _, m := range b.models {
		for _, l := range b.connected(m.id, "Geometry") {
			if g := b.objects[l.id]; g.sub == "Mesh" {
				m.geometry = g
				if err := b.buildMesh(m); err != nil {
					return fmt.Errorf("mesh %q: %w", m.name, err)
				}
				break
			}
		}
	}
	return b.buildAnimations()
}

// read takes the Model's transform out of its properties.
func (m *model) read() {
	p := m.props
	m.trans = p.vector("Lcl Translation", [3]float64{})
	m.rotation = p.vector("Lcl Rotation", [3]float64{})
	m.scale = p.vector("Lcl Scaling", [3]float64{1, 1, 1})
	m.order = int(p.number("RotationOrder", 0, 0))
	if m.order < 0 || m.order > 5 {
		m.order = 0 // the spheric order is read as XYZ
	}
	m.geometric = translation(p.vector("GeometricTranslation", [3]float64{})).
		mul(euler(p.vector("GeometricRotation", [3]float64{}), 0)).
		mul(scaling(p.vector("GeometricScaling", [3]float64{1, 1, 1})))
}

// local returns where the model is relative to its parent with translation
// t, rotation r and scale s, as FBX composes it, turned into GoLib's space
// for a node at the top: T * Roff * Rp * Rpre * R * Rpost^-1 * Rp^-1 * Soff
// * Sp * S * Sp^-1.
func (b *builder) local(m *model, t, r, s [3]float64) mat {
	p := m.props
	rp := p.vector("RotationPivot", [3]float64{})
	sp := p.vector("ScalingPivot", [3]float64{})
	post := euler(p.vector("PostRotation", [3]float64{}), 0)
	l := translation(t).
		mul(translation(p.vector("RotationOffset", [3]float64{}))).
		mul(translation(rp)).
		mul(euler(p.vector("PreRotation", [3]float64{}), 0)).
		mul(euler(r, m.order)).
		mul(post.inverse()).
		mul(translation(neg(rp))).
		mul(translation(p.vector("ScalingOffset", [3]float64{}))).
		mul(translation(sp)).
		mul(scaling(s)).
		mul(translation(neg(sp)))
	if m.parent < 0 {
		l = b.convert.mul(l)
	}
	return l
}

// extras returns the Model's user properties, Blender's custom properties,
// as text.
func (m *model) extras() map[string]string {
	if len(m.props.user) == 0 {
		return nil
	}
	out := map[string]string{}
	for _, name := range m.props.user {
		var parts []string
		for _, v := range m.props.values[name] {
			switch x := v.(type) {
			case string:
				parts = append(parts, x)
			case float64:
				parts = append(parts, strconv.FormatFloat(x, 'g', -1, 64))
			case int64:
				parts = append(parts, strconv.FormatInt(x, 10))
			case bool:
				parts = append(parts, strconv.FormatBool(x))
			}
		}
		out[name] = strings.Join(parts, " ")
	}
	return out
}

func neg(v [3]float64) [3]float64 { return [3]float64{-v[0], -v[1], -v[2]} }

// checkTree refuses Models that are their own ancestors.
func checkTree(doc *gltf.Document) error {
	state := make([]byte, len(doc.Nodes))
	var visit func(int) error
	visit = func(i int) error {
		switch state[i] {
		case 1:
			return fmt.Errorf("node %q is its own ancestor", doc.Nodes[i].Name)
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

// buildMaterials turns the file's materials, textures and pictures into
// glTF's.
func (b *builder) buildMaterials() error {
	b.materialOf, b.textureOf, b.imageOf = map[int64]int{}, map[int64]int{}, map[int64]int{}
	for _, o := range b.order {
		if o.kind != "Video" {
			continue
		}
		data, err := b.picture(o)
		if err != nil {
			return err
		}
		if data == nil {
			continue
		}
		b.imageOf[o.id] = len(b.doc.Images)
		mime := "image/png"
		if bytes.HasPrefix(data, []byte("\xff\xd8")) {
			mime = "image/jpeg"
		}
		b.doc.Images = append(b.doc.Images, gltf.Image{Name: o.name, MIME: mime, Data: data})
	}
	for _, o := range b.order {
		if o.kind != "Texture" {
			continue
		}
		texture := gltf.Texture{Image: -1, RepeatS: true, RepeatT: true}
		for _, l := range b.connected(o.id, "Video") {
			if i, ok := b.imageOf[l.id]; ok {
				texture.Image = i
				break
			}
		}
		if texture.Image < 0 {
			// The picture's file may be named by the texture alone.
			data, err := b.pictureFile(o.e.child("RelativeFilename").text(0), o.e.child("FileName").text(0))
			if err != nil {
				return err
			}
			if data != nil {
				texture.Image = len(b.doc.Images)
				b.doc.Images = append(b.doc.Images, gltf.Image{Name: o.name, MIME: "image/png", Data: data})
			}
		}
		b.textureOf[o.id] = len(b.doc.Textures)
		b.doc.Textures = append(b.doc.Textures, texture)
	}
	for _, o := range b.order {
		if o.kind != "Material" {
			continue
		}
		b.materialOf[o.id] = len(b.doc.Materials)
		b.doc.Materials = append(b.doc.Materials, b.material(o))
	}
	return nil
}

// material turns a Phong or Lambert material into a metallic-roughness one.
func (b *builder) material(o *object) gltf.Material {
	p := o.props
	none := gltf.TextureRef{Index: -1, Scale: 1}
	m := gltf.Material{
		Name: o.name, BaseColor: [4]float32{1, 1, 1, 1}, Roughness: 1,
		BaseColorTexture: none, MetallicRoughnessTexture: none, NormalTexture: none,
		OcclusionTexture: none, EmissiveTexture: none, AlphaMode: "OPAQUE", AlphaCutoff: 0.5,
	}
	textures := map[string]int{}
	for _, l := range b.connected(o.id, "Texture") {
		if t, ok := b.textureOf[l.id]; ok {
			textures[l.property] = t
		}
	}
	diffuse := p.vector("DiffuseColor", [3]float64{0.8, 0.8, 0.8})
	if !p.has("DiffuseColor") && p.has("Diffuse") {
		diffuse = p.vector("Diffuse", diffuse)
	}
	factor := p.number("DiffuseFactor", 0, 1)
	if t, ok := textures["DiffuseColor"]; ok {
		m.BaseColorTexture.Index = t
		diffuse, factor = [3]float64{1, 1, 1}, 1
	}
	for i := range 3 {
		m.BaseColor[i] = float32(diffuse[i] * factor)
	}
	opacity := p.number("Opacity", 0, 1)
	if !p.has("Opacity") && p.has("TransparencyFactor") {
		opacity = 1 - p.number("TransparencyFactor", 0, 0)
	}
	if opacity < 0.999 {
		m.BaseColor[3] = float32(max(0, opacity))
		m.AlphaMode = "BLEND"
	}
	emissive := p.vector("EmissiveColor", [3]float64{})
	strength := p.number("EmissiveFactor", 0, 1)
	for i := range 3 {
		m.Emissive[i] = float32(emissive[i] * strength)
	}
	if t, ok := textures["EmissiveColor"]; ok {
		m.EmissiveTexture.Index = t
	}
	for _, name := range []string{"NormalMap", "Bump"} {
		if t, ok := textures[name]; ok {
			m.NormalTexture.Index = t
			break
		}
	}
	// A Phong highlight's exponent, as the roughness that looks alike.
	if shininess := p.number("ShininessExponent", 0, p.number("Shininess", 0, -1)); shininess >= 0 {
		m.Roughness = float32(max(0.05, min(1, math.Sqrt(2/(shininess+2)))))
	}
	return m
}

// picture returns a Video's picture: embedded in the file, or the file it
// names, or nil when there is neither.
func (b *builder) picture(o *object) ([]byte, error) {
	if content := o.e.child("Content"); content != nil && len(content.values) > 0 {
		if data, ok := content.values[0].([]byte); ok && len(data) > 0 {
			return data, nil
		}
	}
	return b.pictureFile(o.e.child("RelativeFilename").text(0), o.e.child("FileName").text(0))
}

// pictureFile reads a picture the file names, relative to it, or by its
// base name beside it, since an absolute path is another machine's. It
// returns nil when there is no such file.
func (b *builder) pictureFile(relative, absolute string) ([]byte, error) {
	if b.readFile == nil {
		return nil, nil
	}
	var tried []string
	for _, name := range []string{relative, absolute} {
		name = strings.ReplaceAll(name, "\\", "/")
		if name == "" {
			continue
		}
		for _, candidate := range []string{name, path.Base(name), "textures/" + path.Base(name), "Textures/" + path.Base(name)} {
			if strings.HasPrefix(candidate, "/") || strings.Contains(candidate, ":") {
				continue
			}
			tried = append(tried, candidate)
			if data, err := b.readFile(candidate); err == nil {
				if !bytes.HasPrefix(data, []byte("\x89PNG")) && !bytes.HasPrefix(data, []byte("\xff\xd8")) {
					return nil, fmt.Errorf("the texture %s is neither PNG nor JPEG: save it as PNG or JPEG", candidate)
				}
				return data, nil
			}
		}
	}
	if len(tried) > 0 {
		return nil, fmt.Errorf("its texture %s isn't beside it: put it next to the model, or export with the textures embedded (Path Mode: Copy, with the box beside it pressed, in Blender)", tried[0])
	}
	return nil, nil
}

// buildMesh turns a Model's geometry into a glTF mesh, cut by material,
// with its skin and its shape keys.
func (b *builder) buildMesh(m *model) error {
	g := m.geometry.e
	points := g.floats("Vertices")
	polygons := g.ints("PolygonVertexIndex")
	if len(points) < 3 || len(polygons) < 3 {
		return nil
	}
	count := len(points) / 3
	geometric := m.geometric
	normalTurn := geometric.normalMatrix()

	normals := g.child("LayerElementNormal")
	uvs := g.child("LayerElementUV")
	colors := g.child("LayerElementColor")
	materials := g.child("LayerElementMaterial")
	var used []int // the model's materials, in the order they connect
	for _, l := range b.connected(m.id, "Material") {
		used = append(used, b.materialOf[l.id])
	}

	// The skin: for each control point, its joints and weights.
	var skin *pointWeights
	if s, err := b.buildSkin(m, count); err != nil {
		return err
	} else if s != nil {
		skin = s
	}
	shapes, names, weights := b.shapes(m, count)

	type key struct {
		point            int
		nx, ny, nz, u, v float32
		color            [4]float32
	}
	type output struct {
		prim   gltf.Primitive
		lookup map[key]uint32
	}
	var outputs []*output
	byMaterial := map[int]*output{}
	primitive := func(material int) *output {
		if o := byMaterial[material]; o != nil {
			return o
		}
		o := &output{lookup: map[key]uint32{}}
		o.prim.Material = material
		if skin != nil {
			o.prim.Joints, o.prim.Weights = []float32{}, []float32{}
		}
		o.prim.Targets = make([]gltf.MorphTarget, len(shapes))
		byMaterial[material] = o
		outputs = append(outputs, o)
		return o
	}

	var corner []int // the polygon-vertex numbers of the polygon being read
	polygon := 0
	for pv, index := range polygons {
		corner = append(corner, pv)
		if index >= 0 {
			continue
		}
		// The polygon is done: a fan of triangles from its first corner.
		material := -1
		if materials != nil {
			if v := layer(materials, "Materials", "", 1, polygon, pv, 0); v != nil {
				if i := int(v[0]); i >= 0 && i < len(used) {
					material = used[i]
				}
			}
		} else if len(used) > 0 {
			material = used[0]
		}
		out := primitive(material)
		vertex := func(pv int) (uint32, error) {
			point := int(polygons[pv])
			if point < 0 {
				point = -point - 1
			}
			if point >= count {
				return 0, fmt.Errorf("a polygon names point %d of %d", point, count)
			}
			k := key{point: point, color: [4]float32{1, 1, 1, 1}}
			if n := layer(normals, "Normals", "NormalsIndex", 3, polygon, pv, point); n != nil {
				turned := normalTurn.direction([3]float64{n[0], n[1], n[2]})
				k.nx, k.ny, k.nz = float32(turned[0]), float32(turned[1]), float32(turned[2])
			}
			if uv := layer(uvs, "UV", "UVIndex", 2, polygon, pv, point); uv != nil {
				k.u, k.v = float32(uv[0]), float32(1-uv[1]) // FBX's V grows up the picture
			}
			if c := layer(colors, "Colors", "ColorIndex", 4, polygon, pv, point); c != nil {
				for i := range 3 {
					k.color[i] = float32(math.Pow(max(0, c[i]), 2.2))
				}
				k.color[3] = float32(c[3])
			}
			if at, ok := out.lookup[k]; ok {
				return at, nil
			}
			at := uint32(len(out.prim.Positions) / 3)
			out.lookup[k] = at
			p := geometric.point([3]float64{points[point*3], points[point*3+1], points[point*3+2]})
			out.prim.Positions = append(out.prim.Positions, float32(p[0]), float32(p[1]), float32(p[2]))
			out.prim.Normals = append(out.prim.Normals, k.nx, k.ny, k.nz)
			if uvs != nil {
				out.prim.Texcoords = append(out.prim.Texcoords, k.u, k.v)
			}
			if colors != nil {
				out.prim.Colors = append(out.prim.Colors, k.color[:]...)
			}
			if skin != nil {
				out.prim.Joints = append(out.prim.Joints, skin.joints[point*4:point*4+4]...)
				out.prim.Weights = append(out.prim.Weights, skin.weights[point*4:point*4+4]...)
			}
			for t, shape := range shapes {
				d := shape.positions[point*3 : point*3+3]
				n := shape.normals[point*3 : point*3+3]
				out.prim.Targets[t].Positions = append(out.prim.Targets[t].Positions, d...)
				out.prim.Targets[t].Normals = append(out.prim.Targets[t].Normals, n...)
			}
			return at, nil
		}
		first, err := vertex(corner[0])
		if err != nil {
			return err
		}
		for k := 1; k+1 < len(corner); k++ {
			a, err := vertex(corner[k])
			if err != nil {
				return err
			}
			c, err := vertex(corner[k+1])
			if err != nil {
				return err
			}
			out.prim.Indices = append(out.prim.Indices, first, a, c)
		}
		corner = corner[:0]
		polygon++
	}
	mesh := gltf.Mesh{Name: m.name, Weights: weights, TargetNames: names}
	for _, o := range outputs {
		if len(o.prim.Indices) == 0 {
			continue
		}
		if normals == nil {
			flatNormals(&o.prim)
		}
		mesh.Primitives = append(mesh.Primitives, o.prim)
	}
	if len(mesh.Primitives) == 0 {
		return nil
	}
	node := &b.doc.Nodes[m.node]
	node.Mesh = len(b.doc.Meshes)
	b.doc.Meshes = append(b.doc.Meshes, mesh)
	if skin != nil {
		node.Skin = skin.skin
	}
	return nil
}

// layer returns the value of a layer element (normals, texture
// coordinates, colors, materials) for a polygon vertex, width numbers, or
// nil when there is no such element or value.
func layer(e *element, data, index string, width, polygon, pv, point int) []float64 {
	if e == nil {
		return nil
	}
	values := e.floats(data)
	at := 0
	switch e.child("MappingInformationType").text(0) {
	case "ByPolygonVertex":
		at = pv
	case "ByVertice", "ByVertex", "ByControlPoint":
		at = point
	case "ByPolygon":
		at = polygon
	case "AllSame":
		at = 0
	default:
		return nil
	}
	if ref := e.child("ReferenceInformationType").text(0); ref == "IndexToDirect" || ref == "Index" {
		indices := e.ints(index)
		if data == "Materials" {
			indices = nil // a material layer's own values are the indices
		}
		if indices != nil {
			if at >= len(indices) {
				return nil
			}
			at = int(indices[at])
		}
	}
	if at < 0 || (at+1)*width > len(values) {
		return nil
	}
	return values[at*width : (at+1)*width]
}

// flatNormals gives each triangle's corners the triangle's own normal, for
// a mesh whose file has none.
func flatNormals(p *gltf.Primitive) {
	normals := make([]float32, len(p.Positions))
	for t := 0; t+2 < len(p.Indices); t += 3 {
		i := p.Indices[t : t+3]
		a := p.Positions[i[0]*3 : i[0]*3+3]
		b := p.Positions[i[1]*3 : i[1]*3+3]
		c := p.Positions[i[2]*3 : i[2]*3+3]
		ux, uy, uz := b[0]-a[0], b[1]-a[1], b[2]-a[2]
		vx, vy, vz := c[0]-a[0], c[1]-a[1], c[2]-a[2]
		nx, ny, nz := uy*vz-uz*vy, uz*vx-ux*vz, ux*vy-uy*vx
		for _, k := range i {
			normals[k*3] += nx
			normals[k*3+1] += ny
			normals[k*3+2] += nz
		}
	}
	for k := 0; k+2 < len(normals); k += 3 {
		l := float32(math.Sqrt(float64(normals[k]*normals[k] + normals[k+1]*normals[k+1] + normals[k+2]*normals[k+2])))
		if l > 0 {
			normals[k], normals[k+1], normals[k+2] = normals[k]/l, normals[k+1]/l, normals[k+2]/l
		}
	}
	p.Normals = normals
}

// pointWeights is a skin read for a mesh's control points.
type pointWeights struct {
	skin    int       // in the document's skins
	joints  []float32 // four for each point
	weights []float32
}

// buildSkin reads the skin deforming m's geometry, if it has one, and adds
// it to the document: a joint for each of its clusters.
func (b *builder) buildSkin(m *model, count int) (*pointWeights, error) {
	var skinDeformer *object
	for _, l := range b.connected(m.geometry.id, "Deformer") {
		if d := b.objects[l.id]; d.sub == "Skin" {
			skinDeformer = d
			break
		}
	}
	if skinDeformer == nil {
		return nil, nil
	}
	type influence struct {
		joint  int
		weight float32
	}
	influences := make([][]influence, count)
	var skin gltf.Skin
	skin.Name, skin.Skeleton = skinDeformer.name, -1
	for _, l := range b.connected(skinDeformer.id, "Deformer") {
		cluster := b.objects[l.id]
		if cluster.sub != "Cluster" {
			continue
		}
		bone := -1
		for _, c := range b.connected(cluster.id, "Model") {
			if n, ok := b.nodeOf[c.id]; ok {
				bone = n
				break
			}
		}
		if bone < 0 {
			continue
		}
		joint := len(skin.Joints)
		skin.Joints = append(skin.Joints, bone)
		b.doc.Nodes[bone].Joint = true
		// Transform takes the mesh's space, as bound, into the bone's: the
		// FBX SDK shows it as the mesh's place in the world, but stores it
		// in the bone's space, as Blender writes it and ufbx reads it.
		skin.InverseBind = append(skin.InverseBind, matOf(cluster.e.floats("Transform")).float32s())
		indices := cluster.e.ints("Indexes")
		weights := cluster.e.floats("Weights")
		for k, point := range indices {
			if k < len(weights) && point >= 0 && int(point) < count && weights[k] > 0 {
				influences[point] = append(influences[point], influence{joint, float32(weights[k])})
			}
		}
	}
	if len(skin.Joints) == 0 {
		return nil, nil
	}
	out := &pointWeights{skin: len(b.doc.Skins), joints: make([]float32, count*4), weights: make([]float32, count*4)}
	for point, list := range influences {
		sort.SliceStable(list, func(i, j int) bool { return list[i].weight > list[j].weight })
		var sum float32
		for k := 0; k < len(list) && k < 4; k++ {
			sum += list[k].weight
		}
		if sum <= 0 {
			out.weights[point*4] = 1 // a point no bone moves follows the first
			continue
		}
		for k := 0; k < len(list) && k < 4; k++ {
			out.joints[point*4+k] = float32(list[k].joint)
			out.weights[point*4+k] = list[k].weight / sum
		}
	}
	b.doc.Skins = append(b.doc.Skins, skin)
	return out, nil
}

// shape is a blend shape's moves for every control point of a mesh.
type shape struct {
	positions, normals []float32
	channel            *object
}

// shapes reads the blend shapes of m's geometry: for each channel, Blender's
// shape key, how far it moves each point at full weight, its name and its
// weight at rest.
func (b *builder) shapes(m *model, count int) ([]shape, []string, []float32) {
	var out []shape
	var names []string
	var weights []float32
	turn := m.geometric.normalMatrix()
	for _, l := range b.connected(m.geometry.id, "Deformer") {
		blend := b.objects[l.id]
		if blend.sub != "BlendShape" {
			continue
		}
		for _, c := range b.connected(blend.id, "Deformer") {
			channel := b.objects[c.id]
			if channel.sub != "BlendShapeChannel" {
				continue
			}
			// The channel's last shape is the one at full weight;
			// in-between shapes are left out.
			geometries := b.connected(channel.id, "Geometry")
			if len(geometries) == 0 {
				continue
			}
			g := b.objects[geometries[len(geometries)-1].id].e
			s := shape{positions: make([]float32, count*3), normals: make([]float32, count*3), channel: channel}
			indices := g.ints("Indexes")
			moves := g.floats("Vertices")
			turns := g.floats("Normals")
			for k, point := range indices {
				if point < 0 || int(point) >= count || (k+1)*3 > len(moves) {
					continue
				}
				d := m.geometric.direction([3]float64{moves[k*3], moves[k*3+1], moves[k*3+2]})
				s.positions[point*3], s.positions[point*3+1], s.positions[point*3+2] = float32(d[0]), float32(d[1]), float32(d[2])
				if (k+1)*3 <= len(turns) {
					n := turn.direction([3]float64{turns[k*3], turns[k*3+1], turns[k*3+2]})
					s.normals[point*3], s.normals[point*3+1], s.normals[point*3+2] = float32(n[0]), float32(n[1]), float32(n[2])
				}
			}
			out = append(out, s)
			names = append(names, channel.name)
			weights = append(weights, float32(channel.e.child("DeformPercent").float(0)/100))
		}
	}
	return out, names, weights
}

// ticks is how many units of FBX time make a second.
const ticks = 46186158000

// curve is an animation curve: keys, and how to go between them.
type curve struct {
	times  []float64 // seconds
	values []float64
	steps  []bool // each key holds until the next, rather than sliding
}

func readCurve(e *element) *curve {
	raw := e.ints("KeyTime")
	values := e.floats("KeyValueFloat")
	if len(raw) == 0 || len(values) < len(raw) {
		return nil
	}
	c := &curve{times: make([]float64, len(raw)), values: values[:len(raw)], steps: make([]bool, len(raw))}
	for i, t := range raw {
		c.times[i] = float64(t) / ticks
	}
	// Each flag holds for as many keys as its count says.
	flags := e.ints("KeyAttrFlags")
	counts := e.ints("KeyAttrRefCount")
	k := 0
	for i, flag := range flags {
		n := len(raw)
		if i < len(counts) {
			n = int(counts[i])
		}
		for ; n > 0 && k < len(raw); n, k = n-1, k+1 {
			c.steps[k] = flag&0x02 != 0
		}
	}
	return c
}

// at returns the curve's value at time t.
func (c *curve) at(t float64) float64 {
	n := len(c.times)
	if t <= c.times[0] {
		return c.values[0]
	}
	if t >= c.times[n-1] {
		return c.values[n-1]
	}
	k := sort.SearchFloat64s(c.times, t)
	if c.times[k] == t {
		return c.values[k]
	}
	k-- // times[k] < t < times[k+1]
	if c.steps[k] {
		return c.values[k]
	}
	u := (t - c.times[k]) / (c.times[k+1] - c.times[k])
	return c.values[k] + (c.values[k+1]-c.values[k])*u
}

// animatedProperty is one property of one object an animation moves, with a
// curve for each of its parts that moves.
type animatedProperty struct {
	defaults [3]float64
	curves   [3]*curve
}

func (p *animatedProperty) at(t float64, rest [3]float64) [3]float64 {
	out := rest
	for i, c := range p.curves {
		if c != nil {
			out[i] = c.at(t)
		}
	}
	return out
}

// sampleGap is the longest a sampled animation goes between two samples:
// FBX turns by three angles, which a turn between quaternions only follows
// closely when the steps are short.
const sampleGap = 1.0 / 30

// buildAnimations turns each animation stack into a glTF animation, its
// first layer sampled at its keys, and between them often enough.
func (b *builder) buildAnimations() error {
	for _, stack := range b.order {
		if stack.kind != "AnimationStack" {
			continue
		}
		layers := b.connected(stack.id, "AnimationLayer")
		if len(layers) == 0 {
			continue
		}
		start := stack.props.number("LocalStart", 0, 0) / ticks
		moved := map[int64]map[string]*animatedProperty{}
		var targets []int64 // in the order first seen
		for _, l := range b.connected(layers[0].id, "AnimationCurveNode") {
			node := b.objects[l.id]
			var target link
			for _, up := range b.parents[node.id] {
				if up.property != "" {
					target = up
					break
				}
			}
			if target.property == "" {
				continue
			}
			p := &animatedProperty{defaults: [3]float64{
				node.props.number("d|X", 0, 0), node.props.number("d|Y", 0, 0), node.props.number("d|Z", 0, 0),
			}}
			if target.property == "DeformPercent" {
				p.defaults[0] = node.props.number("d|DeformPercent", 0, 0)
			}
			found := false
			for _, c := range b.connected(node.id, "AnimationCurve") {
				part := map[string]int{"d|X": 0, "d|Y": 1, "d|Z": 2, "d|DeformPercent": 0}
				if i, ok := part[c.property]; ok {
					if cv := readCurve(b.objects[c.id].e); cv != nil {
						p.curves[i] = cv
						found = true
					}
				}
			}
			if !found {
				continue
			}
			if moved[target.id] == nil {
				moved[target.id] = map[string]*animatedProperty{}
				targets = append(targets, target.id)
			}
			moved[target.id][target.property] = p
		}
		name := stack.name
		if cut := strings.LastIndex(name, "|"); cut >= 0 {
			name = name[cut+1:] // Blender's "Armature|Run" is the action "Run"
		}
		animation := gltf.Animation{Name: name}
		shapes := map[int64]*animatedProperty{}
		for _, id := range targets {
			o := b.objects[id]
			properties := moved[id]
			switch o.kind {
			case "Model":
				if channels := b.sampleModel(b.models[b.nodeOf[id]], properties, start); channels != nil {
					animation.Channels = append(animation.Channels, channels...)
				}
			case "Deformer":
				if p := properties["DeformPercent"]; p != nil {
					shapes[id] = p
				}
			}
		}
		animation.Channels = append(animation.Channels, b.sampleShapes(shapes, start)...)
		trimStillEnd(&animation)
		if len(animation.Channels) > 0 {
			b.doc.Animations = append(b.doc.Animations, animation)
		}
	}
	return nil
}

// trimStillEnd ends an animation where its last movement does. Blender
// exports each action over the scene's whole timeline, 250 frames unless
// it was set, so a short punch would hold still for seconds before it ended
// or looped. Still is closer to the last pose than half a millimeter, or a
// few hundredths of a degree: Blender's curves drift by less than that past
// their last key. An animation meant to pause at its end loses the pause.
func trimStillEnd(a *gltf.Animation) {
	const still = 5e-4
	var end float32
	for _, c := range a.Channels {
		n := len(c.Times)
		if n == 0 {
			continue
		}
		width := len(c.Values) / n
		last := c.Values[(n-1)*width:]
		for k := n - 1; k >= 0; k-- {
			moved := false
			for i := range width {
				if math.Abs(float64(c.Values[k*width+i]-last[i])) > still {
					moved = true
					break
				}
			}
			if moved {
				// It is still from the key after k on.
				end = max(end, c.Times[min(k+1, n-1)])
				break
			}
		}
	}
	for i := range a.Channels {
		c := &a.Channels[i]
		n := len(c.Times)
		if n == 0 {
			continue
		}
		width := len(c.Values) / n
		keep := sort.Search(n, func(k int) bool { return c.Times[k] >= end }) + 1
		if keep < n {
			c.Times, c.Values = c.Times[:keep], c.Values[:keep*width]
		}
	}
}

// sampleTimes returns the keys of every curve given, from start, and times
// between them so that none is further than sampleGap from the next.
func sampleTimes(curves []*curve, start float64) []float64 {
	var times []float64
	for _, c := range curves {
		if c != nil {
			times = append(times, c.times...)
		}
	}
	if len(times) == 0 {
		return nil
	}
	sort.Float64s(times)
	var out []float64
	for _, t := range times {
		if len(out) > 0 && t-out[len(out)-1] < 1e-6 {
			continue
		}
		if len(out) > 0 {
			last := out[len(out)-1]
			steps := int(math.Ceil((t - last) / sampleGap))
			for k := 1; k < steps; k++ {
				out = append(out, last+(t-last)*float64(k)/float64(steps))
			}
		}
		out = append(out, t)
	}
	for i := range out {
		out[i] = max(0, out[i]-start)
	}
	return out
}

// sampleModel samples a Model's animated translation, rotation and scale
// into glTF channels.
func (b *builder) sampleModel(m *model, properties map[string]*animatedProperty, start float64) []gltf.Channel {
	var curves []*curve
	for _, p := range properties {
		curves = append(curves, p.curves[:]...)
	}
	times := sampleTimes(curves, start)
	if len(times) == 0 {
		return nil
	}
	t := gltf.Channel{Node: m.node, Path: "translation", Interpolation: "LINEAR"}
	r := gltf.Channel{Node: m.node, Path: "rotation", Interpolation: "LINEAR"}
	s := gltf.Channel{Node: m.node, Path: "scale", Interpolation: "LINEAR"}
	var last [4]float32
	for i, time := range times {
		at := time + start
		trans, rot, scale := m.trans, m.rotation, m.scale
		if p := properties["Lcl Translation"]; p != nil {
			trans = p.at(at, trans)
		}
		if p := properties["Lcl Rotation"]; p != nil {
			rot = p.at(at, rot)
		}
		if p := properties["Lcl Scaling"]; p != nil {
			scale = p.at(at, scale)
		}
		pt, pq, ps := b.local(m, trans, rot, scale).decompose()
		// Keep each turn on the same side as the last, so that turning
		// between them goes the short way.
		if i > 0 && pq[0]*last[0]+pq[1]*last[1]+pq[2]*last[2]+pq[3]*last[3] < 0 {
			pq = [4]float32{-pq[0], -pq[1], -pq[2], -pq[3]}
		}
		last = pq
		for _, c := range []*gltf.Channel{&t, &r, &s} {
			c.Times = append(c.Times, float32(time))
		}
		t.Values = append(t.Values, pt[:]...)
		r.Values = append(r.Values, pq[:]...)
		s.Values = append(s.Values, ps[:]...)
	}
	return []gltf.Channel{t, r, s}
}

// sampleShapes turns the blend shape channels an animation moves into glTF
// channels of the weights of the meshes they shape, one for each mesh, its
// other shape keys held at rest.
func (b *builder) sampleShapes(moved map[int64]*animatedProperty, start float64) []gltf.Channel {
	if len(moved) == 0 {
		return nil
	}
	var out []gltf.Channel
	for _, m := range b.models {
		node := b.doc.Nodes[m.node]
		if node.Mesh < 0 || m.geometry == nil {
			continue
		}
		mesh := b.doc.Meshes[node.Mesh]
		// The mesh's channels, in the order its shape keys were read.
		var channels []*animatedProperty
		var curves []*curve
		for _, l := range b.connected(m.geometry.id, "Deformer") {
			if b.objects[l.id].sub != "BlendShape" {
				continue
			}
			for _, c := range b.connected(l.id, "Deformer") {
				if b.objects[c.id].sub != "BlendShapeChannel" {
					continue
				}
				p := moved[c.id]
				channels = append(channels, p)
				if p != nil {
					curves = append(curves, p.curves[0])
				}
			}
		}
		if len(curves) == 0 || len(channels) != len(mesh.Weights) {
			continue
		}
		channel := gltf.Channel{Node: m.node, Path: "weights", Interpolation: "LINEAR"}
		for _, time := range sampleTimes(curves, start) {
			channel.Times = append(channel.Times, float32(time))
			for w, p := range channels {
				value := float64(mesh.Weights[w])
				if p != nil && p.curves[0] != nil {
					value = p.curves[0].at(time+start) / 100
				}
				channel.Values = append(channel.Values, float32(value))
			}
		}
		out = append(out, channel)
	}
	return out
}
