package golib

import (
	"errors"
	"fmt"
	"math"
	"path"
	"strings"
	"sync"

	"golib/internal/device"
	"golib/internal/fbx"
	"golib/internal/gltf"
)

// Model is a 3D model from a glTF file, .glb or .gltf, as Blender and most
// other 3D tools export it, in the game's assets folder: its meshes with
// their materials and textures, placed as the file places them. Make it
// once, like a sprite, and draw it as many times as needed:
//
//	var crate = golib.NewModel("models/crate.glb")
//
//	// In Draw, between Screen.Begin3D and Screen.End3D:
//	screen.DrawModel(crate, golib.Transform{Position: c.Position}, golib.White)
//
// The file is read the first time the model is drawn or asked about, and its
// textures go to the graphics card the first time it is drawn. A model made
// in Blender as a level carries its markers: the empties placed in it, with
// their names and custom properties (see [Model.Markers]).
type Model struct {
	name string

	mu       sync.Mutex
	read     bool
	err      error
	parts    []modelPart
	textures []*texture3D
	markers  []ModelMarker
	lowest   Vector3
	upmost   Vector3
	decoded  bool // its textures are decoded, ready for the graphics card

	nodes      []modelNode
	order      []int // the nodes, each after its parent
	skins      []modelSkin
	rootSkin   int // the skin whose top bone root motion follows
	animations []modelAnimation
	rest       modelPose // the model as the file has it, which DrawModel draws
	extra      []string  // files of more animations, from AddAnimations
}

// modelPart is one mesh of a model, with its material, on the node that
// places it, or bent by a skin.
type modelPart struct {
	mesh     *Mesh
	material *material3D
	node     int
	skin     int // -1 for a mesh no bones bend

	// reach is, for a skinned part, how far its vertices are from each of
	// the skin's joints they follow, as bound: the sphere culling tests
	// grows from where the joints are. -1 for a joint that moves none.
	reach []float32
}

// ModelMarker is a point placed in a model in Blender, as an empty, or in
// any other node with no mesh but a character's bones: where a level's player starts, where enemies
// appear, where a door goes. Position, Rotation and Scale are in the model's
// own space, as Blender shows them once exported; place them with
// [Transform.Child] when the model is drawn moved.
type ModelMarker struct {
	Name     string
	Position Vector3
	Rotation Rotation
	Scale    Vector3

	// Properties are the custom properties the object had in Blender, as
	// text: numbers and true or false as written, lists and groups as JSON.
	// Blender exports them with Include > Custom Properties ticked.
	Properties Properties
}

// NewModel returns the glTF model in the game's assets folder named name,
// such as "models/ship.glb". A .gltf file's own files, its .bin buffer and
// its textures, are read from the folder it is in.
func NewModel(name string) *Model {
	return &Model{name: name}
}

func (m *Model) call() string {
	return fmt.Sprintf("golib.NewModel(%q)", m.name)
}

// Err reads the model, if it has not been read yet, and returns what stopped
// it from loading: a missing file, a format GoLib doesn't read, or a mistake
// in the file. It returns nil when the model is ready. Drawing or asking a
// model that didn't load stops Run with the same mistake, so a game needs
// Err only to handle it itself, or to check its models in a test.
func (m *Model) Err() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.prepare()
}

// Markers returns the model's markers, in the order of the file's nodes.
func (m *Model) Markers() []ModelMarker {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.prepare() != nil {
		return nil
	}
	return m.markers
}

// Marker returns the first marker named name, and whether there is one: the
// place a level's player starts, for example.
func (m *Model) Marker(name string) (ModelMarker, bool) {
	for _, marker := range m.Markers() {
		if marker.Name == name {
			return marker, true
		}
	}
	return ModelMarker{}, false
}

// Bounds returns the box around the model, in its own space, as its least
// and its most X, Y and Z: how big it is, and where its feet are.
//
//	lowest, upmost := tree.Bounds()
//	height := upmost.Y - lowest.Y
func (m *Model) Bounds() (lowest, upmost Vector3) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.prepare() != nil {
		return Vector3{}, Vector3{}
	}
	return m.lowest, m.upmost
}

// prepare reads the file the first time, and returns what stopped it.
func (m *Model) prepare() error {
	if !m.read {
		m.read = true
		if err := m.load(); err != nil {
			m.err = fmt.Errorf("%s: %w", m.call(), err)
		}
	}
	return m.err
}

func (m *Model) load() error {
	file, err := ReadAsset(m.name)
	if err != nil {
		return err
	}
	doc, err := readModelFile(m.name, file)
	if err != nil {
		return err
	}

	// Textures are made when a material uses them: a texture for color is
	// sRGB, one for data is linear.
	made := map[[2]int]*texture3D{}
	texture := func(ref gltf.TextureRef, linear bool) *texture3D {
		if ref.Index < 0 || ref.Index >= len(doc.Textures) {
			return nil
		}
		t := doc.Textures[ref.Index]
		if t.Image < 0 || t.Image >= len(doc.Images) {
			return nil
		}
		key := [2]int{ref.Index, boolIndex(linear)}
		if made[key] == nil {
			made[key] = &texture3D{data: doc.Images[t.Image].Data, options: device.TextureOptions{
				Linear: linear, Nearest: t.Nearest, NearestMipmaps: t.NearestMipmaps,
				ClampS: !t.RepeatS && !t.MirrorS, ClampT: !t.RepeatT && !t.MirrorT,
				MirrorS: t.MirrorS, MirrorT: t.MirrorT,
			}}
			m.textures = append(m.textures, made[key])
		}
		return made[key]
	}
	materials := make([]*material3D, len(doc.Materials))
	for i, mat := range doc.Materials {
		out := &material3D{
			baseColor:         mat.BaseColor,
			baseTexture:       texture(mat.BaseColorTexture, false),
			emissive:          mat.Emissive,
			emissiveTexture:   texture(mat.EmissiveTexture, false),
			metallic:          mat.Metallic,
			roughness:         mat.Roughness,
			metalRoughTexture: texture(mat.MetallicRoughnessTexture, true),
			normalTexture:     texture(mat.NormalTexture, true),
			normalScale:       mat.NormalTexture.Scale,
			occlusionTexture:  texture(mat.OcclusionTexture, true),
			occlusionStrength: mat.OcclusionTexture.Scale,
			alphaCutoff:       mat.AlphaCutoff,
			twoSided:          mat.DoubleSided,
			unlit:             mat.Unlit,
		}
		switch mat.AlphaMode {
		case "MASK":
			out.alpha = alphaMask
		case "BLEND":
			out.alpha = alphaBlend
		}
		materials[i] = out
	}

	meshes := make([][]*Mesh, len(doc.Meshes))
	for i, mesh := range doc.Meshes {
		for _, p := range mesh.Primitives {
			meshes[i] = append(meshes[i], meshOf(p, mesh.TargetNames))
		}
	}

	m.readTree(doc)
	if err := m.readSkins(doc); err != nil {
		return err
	}
	m.readAnimations(doc, func(i int) int { return i }, "")
	for _, name := range m.extra {
		if err := m.readAnimationFile(name); err != nil {
			return err
		}
	}

	first := true
	grow := func(p Vector3) {
		if first {
			m.lowest, m.upmost, first = p, p, false
		}
		m.lowest = Vector3{X: min(m.lowest.X, p.X), Y: min(m.lowest.Y, p.Y), Z: min(m.lowest.Z, p.Z)}
		m.upmost = Vector3{X: max(m.upmost.X, p.X), Y: max(m.upmost.Y, p.Y), Z: max(m.upmost.Z, p.Z)}
	}
	var failed error
	var visit func(node int)
	visit = func(node int) {
		n := doc.Nodes[node]
		place := m.rest.globals[node]
		if n.Mesh >= 0 {
			for k, mesh := range meshes[n.Mesh] {
				material := plainMaterial
				if at := doc.Meshes[n.Mesh].Primitives[k].Material; at >= 0 {
					material = materials[at]
				}
				part := modelPart{mesh: mesh, material: material, node: node, skin: -1}
				if n.Skin >= 0 && mesh.skin != nil {
					part.skin = n.Skin
					if err := m.bind(&part); err != nil {
						failed = fmt.Errorf("mesh %q: %w", doc.Meshes[n.Mesh].Name, err)
						return
					}
					m.skinnedCorners(&part, grow)
				} else {
					mesh.bounds()
					for corner := 0; corner < 8; corner++ {
						p := mesh.lowest
						if corner&1 != 0 {
							p.X = mesh.upmost.X
						}
						if corner&2 != 0 {
							p.Y = mesh.upmost.Y
						}
						if corner&4 != 0 {
							p.Z = mesh.upmost.Z
						}
						grow(place.transformPoint(p))
					}
				}
				m.parts = append(m.parts, part)
			}
		} else if !n.Joint {
			position, rotation, scale := place.decompose()
			m.markers = append(m.markers, ModelMarker{
				Name: n.Name, Position: position, Rotation: rotation, Scale: scale,
				Properties: Properties(n.Extras),
			})
		}
		for _, child := range n.Children {
			visit(child)
		}
	}
	for _, root := range doc.Roots {
		visit(root)
	}
	if failed != nil {
		return failed
	}
	if len(m.parts) == 0 && len(m.markers) == 0 {
		return errors.New("the file has nothing in its scene: export it again with the objects selected, or with Include > Selected Objects unticked")
	}
	return nil
}

// readTree keeps the file's nodes, each with its rest pose, in an order with
// parents first, and places them at rest.
func (m *Model) readTree(doc *gltf.Document) {
	m.nodes = make([]modelNode, len(doc.Nodes))
	for i, n := range doc.Nodes {
		node := modelNode{
			name: n.Name, parent: -1, local: matrix(n.Local), byMatrix: n.HasMatrix, mesh: n.Mesh >= 0,
			rest: nodePose{
				translation: Vector3{X: n.Translation[0], Y: n.Translation[1], Z: n.Translation[2]},
				rotation:    quaternion(n.Rotation).normalize(),
				scale:       Vector3{X: n.Scale[0], Y: n.Scale[1], Z: n.Scale[2]},
			},
		}
		if n.Mesh >= 0 {
			if weights := doc.Meshes[n.Mesh].Weights; len(weights) > 0 {
				node.weights = append([]float32(nil), weights...)
				if n.Weights != nil {
					copy(node.weights, n.Weights)
				}
			}
		}
		m.nodes[i] = node
	}
	for i, n := range doc.Nodes {
		for _, c := range n.Children {
			m.nodes[c].parent = i
		}
	}
	seen := make([]bool, len(m.nodes))
	var visit func(int)
	visit = func(n int) {
		if seen[n] {
			return
		}
		seen[n] = true
		m.order = append(m.order, n)
		for _, c := range doc.Nodes[n].Children {
			visit(c)
		}
	}
	for _, root := range doc.Roots {
		visit(root)
	}
	// Nodes of other scenes, which a skin may still name as its bones.
	for n, node := range m.nodes {
		if node.parent < 0 {
			visit(n)
		}
	}

	r := &m.rest
	r.locals = make([]nodePose, len(m.nodes))
	r.weights = make([][]float32, len(m.nodes))
	r.globals = make([]matrix, len(m.nodes))
	for n, node := range m.nodes {
		r.locals[n] = node.rest
		r.weights[n] = node.weights
	}
	// At rest, each node is where the file's own matrix puts it, exactly.
	for _, n := range m.order {
		if parent := m.nodes[n].parent; parent >= 0 {
			r.globals[n] = r.globals[parent].mul(m.nodes[n].local)
		} else {
			r.globals[n] = identityMatrix().mul(m.nodes[n].local)
		}
	}
}

// readSkins keeps the file's skins, with the top bone of each, and bends
// them at rest.
func (m *Model) readSkins(doc *gltf.Document) error {
	for _, s := range doc.Skins {
		skin := modelSkin{joints: s.Joints, root: -1}
		joint := map[int]bool{}
		for k, j := range s.Joints {
			joint[j] = true
			skin.inverseBind = append(skin.inverseBind, matrix(s.InverseBind[k]))
		}
		// The top bone: the first with no bone of the skin above it.
		for _, j := range s.Joints {
			top := true
			for p := m.nodes[j].parent; p >= 0; p = m.nodes[p].parent {
				if joint[p] {
					top = false
					break
				}
			}
			if top {
				skin.root = j
				break
			}
		}
		if skin.root >= 0 {
			under := make([]bool, len(m.nodes))
			for _, n := range m.order {
				if n == skin.root || (m.nodes[n].parent >= 0 && under[m.nodes[n].parent]) {
					under[n] = true
					skin.under = append(skin.under, n)
				}
			}
		}
		m.skins = append(m.skins, skin)
	}
	m.rest.joints = make([][]float32, len(m.skins))
	for s, skin := range m.skins {
		m.rest.joints[s] = make([]float32, 16*len(skin.joints))
	}
	m.bend(&m.rest)
	return nil
}

// readAnimations keeps the animations of doc, the model's own file when
// named is "", and returns how many it kept. nodeOf gives the model's node
// for each node of doc, or -1. named names another file's only animation,
// and that file's animations that move none of the model are left out.
func (m *Model) readAnimations(doc *gltf.Document, nodeOf func(int) int, named string) int {
	root := m.rootBone()
	kept := 0
	for _, a := range doc.Animations {
		anim := modelAnimation{name: a.Name}
		if named != "" && len(doc.Animations) == 1 {
			anim.name = named
		}
		if anim.name == "" {
			anim.name = fmt.Sprintf("animation %d", len(m.animations))
		}
		for _, c := range a.Channels {
			node := nodeOf(c.Node)
			if node < 0 {
				continue
			}
			channel := animationChannel{
				node: node, step: c.Interpolation == "STEP", cubic: c.Interpolation == "CUBICSPLINE",
				times: c.Times, values: c.Values,
			}
			switch c.Path {
			case "translation":
				channel.path, channel.width = pathTranslation, 3
			case "rotation":
				channel.path, channel.width = pathRotation, 4
			case "scale":
				channel.path, channel.width = pathScale, 3
			case "weights":
				channel.path, channel.width = pathWeights, len(m.nodes[node].weights)
				per := len(c.Values) / len(c.Times)
				if channel.cubic {
					per /= 3
				}
				if per != channel.width {
					continue // shape keys this model's mesh doesn't have
				}
			}
			if channel.width == 0 || m.nodes[node].byMatrix {
				continue
			}
			if channel.path == pathTranslation && node == root {
				anim.root = append(anim.root, len(anim.channels))
			}
			anim.length = max(anim.length, c.Times[len(c.Times)-1])
			anim.channels = append(anim.channels, channel)
		}
		if len(anim.channels) > 0 || named == "" {
			m.animations = append(m.animations, anim)
			kept++
		}
	}
	return kept
}

// AddAnimations reads the animations of other files into the model, the way
// Mixamo gives a character's: each in an FBX file of its own, with the
// skeleton and without the character. A glTF file of animations alone
// works too. They move the model's bones, and its other nodes, by name. A
// file with a single animation names it after the file, so
// "models/run.fbx" plays as "run"; one with several keeps their names.
// It returns the model, to add them where it is made:
//
//	var hero = golib.NewModel("models/hero.fbx").AddAnimations("models/idle.fbx", "models/run.fbx")
//
// The files are read with the model, the first time it is drawn or asked
// about, and a mistake in one stops it loading as a mistake in the model's
// own file does.
func (m *Model) AddAnimations(names ...string) *Model {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.extra = append(m.extra, names...)
	if m.read && m.err == nil {
		// Read already: the new files are read now.
		for _, name := range names {
			if err := m.readAnimationFile(name); err != nil {
				m.err = fmt.Errorf("%s: %w", m.call(), err)
				break
			}
		}
	}
	return m
}

// readAnimationFile reads the animations of the file called name into the
// model, matching its nodes to the model's by name.
func (m *Model) readAnimationFile(name string) error {
	file, err := ReadAsset(name)
	if err != nil {
		return err
	}
	doc, err := readModelFile(name, file)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	byName := map[string]int{}
	for i, n := range m.nodes {
		if _, ok := byName[n.name]; !ok {
			byName[n.name] = i
		}
	}
	nodeOf := func(i int) int {
		if at, ok := byName[doc.Nodes[i].Name]; ok {
			return at
		}
		return -1
	}
	base := strings.TrimSuffix(path.Base(name), path.Ext(name))
	if len(doc.Animations) == 0 {
		return fmt.Errorf("%s has no animations: export it with Animation ticked, or with Mixamo's Format FBX Binary", name)
	}
	if m.readAnimations(doc, nodeOf, base) == 0 {
		return fmt.Errorf("%s moves no bone of the model: its bones are named %s, and the model's %s", name, nameList(fileBones(doc), "nothing"), m.boneList())
	}
	return nil
}

// fileBones names the first few nodes an animation file's animations move.
func fileBones(doc *gltf.Document) []string {
	seen := map[int]bool{}
	var names []string
	for _, a := range doc.Animations {
		for _, c := range a.Channels {
			if !seen[c.Node] && len(names) < 5 {
				seen[c.Node] = true
				names = append(names, doc.Nodes[c.Node].Name)
			}
		}
	}
	return names
}

// bind checks that a skinned part's vertices name bones its skin has, and
// measures how far they are from each, for culling.
func (m *Model) bind(part *modelPart) error {
	skin := &m.skins[part.skin]
	mesh := part.mesh
	part.reach = make([]float32, len(skin.joints))
	for k := range part.reach {
		part.reach[k] = -1
	}
	stride := device.LayoutFloats[device.LayoutLit]
	for v := 0; v*8 < len(mesh.skin); v++ {
		position := Vector3{X: mesh.vertices[v*stride], Y: mesh.vertices[v*stride+1], Z: mesh.vertices[v*stride+2]}
		for k := range 4 {
			joint, weight := int(mesh.skin[v*8+k]), mesh.skin[v*8+4+k]
			if weight <= 0 {
				continue
			}
			if joint >= len(skin.joints) {
				return fmt.Errorf("a vertex is bound to bone %d, but its skin has %d bones", joint, len(skin.joints))
			}
			// Where the joint was when the mesh was bound to it.
			at := skin.inverseBind[joint].inverse()
			part.reach[joint] = max(part.reach[joint], position.Distance(Vector3{X: at[12], Y: at[13], Z: at[14]}))
		}
	}
	return nil
}

// skinnedCorners grows the model's bounds by a skinned part's vertices at
// rest.
func (m *Model) skinnedCorners(part *modelPart, grow func(Vector3)) {
	joints := m.rest.joints[part.skin]
	mesh := part.mesh
	stride := device.LayoutFloats[device.LayoutLit]
	for v := 0; v*8 < len(mesh.skin); v++ {
		position := Vector3{X: mesh.vertices[v*stride], Y: mesh.vertices[v*stride+1], Z: mesh.vertices[v*stride+2]}
		var bent Vector3
		for k := range 4 {
			weight := mesh.skin[v*8+4+k]
			if weight <= 0 {
				continue
			}
			j := matrix(joints[int(mesh.skin[v*8+k])*16:])
			bent = bent.Add(j.transformPoint(position).Scale(weight))
		}
		grow(bent)
	}
}

// readModelFile reads a glTF or FBX file, by its name's extension, and the
// files it names from its folder.
func readModelFile(name string, file []byte) (*gltf.Document, error) {
	dir := path.Dir(name)
	beside := func(uri string) ([]byte, error) {
		return ReadAsset(path.Join(dir, uri))
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".fbx":
		return fbx.Read(file, beside)
	case ".glb", ".gltf":
		return gltf.Read(file, beside)
	}
	return nil, fmt.Errorf("%s isn't a model file GoLib reads: it reads glTF (.glb, .gltf) and FBX (.fbx)", path.Base(name))
}

func boolIndex(b bool) int {
	if b {
		return 1
	}
	return 0
}

// meshOf turns a glTF primitive into a Mesh, with its skin and its shape
// keys, named by names, its mesh's morph targets' names.
func meshOf(p gltf.Primitive, names []string) *Mesh {
	count := len(p.Positions) / 3
	mesh := &Mesh{indices: p.Indices}
	if p.Joints != nil {
		mesh.skin = make([]float32, 0, count*8)
		for i := 0; i < count; i++ {
			mesh.skin = append(mesh.skin, p.Joints[i*4:i*4+4]...)
			mesh.skin = append(mesh.skin, p.Weights[i*4:i*4+4]...)
		}
	}
	if len(p.Targets) > 0 {
		mesh.morphCount = len(p.Targets)
		mesh.morphs = make([]float32, 0, len(p.Targets)*count*8)
		mesh.morphReach = make([]float32, len(p.Targets))
		for t, target := range p.Targets {
			for i := 0; i < count; i++ {
				d := target.Positions[i*3 : i*3+3]
				var n []float32
				if target.Normals != nil {
					n = target.Normals[i*3 : i*3+3]
				} else {
					n = []float32{0, 0, 0}
				}
				mesh.morphs = append(mesh.morphs, d[0], d[1], d[2], 0, n[0], n[1], n[2], 0)
				mesh.morphReach[t] = max(mesh.morphReach[t], Vector3{X: d[0], Y: d[1], Z: d[2]}.Length())
			}
		}
		mesh.shapeKeys = make([]string, len(p.Targets))
		for t := range mesh.shapeKeys {
			if t < len(names) {
				mesh.shapeKeys[t] = names[t]
			}
		}
	}
	mesh.vertices = make([]float32, 0, count*device.LayoutFloats[device.LayoutLit])
	for i := 0; i < count; i++ {
		u, v := float32(0), float32(0)
		if p.Texcoords != nil {
			u, v = p.Texcoords[i*2], p.Texcoords[i*2+1]
		}
		color := [4]float32{1, 1, 1, 1}
		if p.Colors != nil {
			copy(color[:], p.Colors[i*4:i*4+4])
		}
		mesh.vertices = append(mesh.vertices,
			p.Positions[i*3], p.Positions[i*3+1], p.Positions[i*3+2],
			p.Normals[i*3], p.Normals[i*3+1], p.Normals[i*3+2],
			u, v, color[0], color[1], color[2], color[3])
	}
	return mesh
}

// decodeTextures decodes the model's pictures, all at the same time, the
// first time it is drawn: decoding is most of what loading a model costs.
func (m *Model) decodeTextures() {
	if m.decoded {
		return
	}
	m.decoded = true
	var wg sync.WaitGroup
	for _, t := range m.textures {
		if t.gpu.ID != 0 || t.decoded.Width != 0 {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.decoded, t.err = device.DecodeImage(t.data)
		}()
	}
	wg.Wait()
	for _, t := range m.textures {
		if t.err != nil && m.err == nil {
			m.err = fmt.Errorf("%s: a texture: %w", m.call(), t.err)
		}
	}
}

// DrawModel draws model placed by transform, tinted by tint: White draws it
// as it was made, and a color with A below 255 makes it see-through.
func (s *Screen) DrawModel(model *Model, transform Transform, tint Color) {
	if !s.in3D("DrawModel") {
		return
	}
	if model == nil {
		reportError(errors.New("golib: Screen.DrawModel got a nil model: make one with golib.NewModel"))
		return
	}
	model.mu.Lock()
	defer model.mu.Unlock()
	if err := model.prepare(); err != nil {
		reportError(err)
		return
	}
	if !model.readyToDraw() {
		return
	}
	s.drawPose(model, &model.rest, transform.matrix(), tint)
}

// readyToDraw decodes the model's textures, the first time, and reports what
// stopped them. The model is locked.
func (m *Model) readyToDraw() bool {
	if err := m.prepare(); err != nil {
		reportError(err)
		return false
	}
	m.decodeTextures()
	if m.err != nil {
		reportError(m.err)
		return false
	}
	return true
}

// drawPose draws model posed as pose, placed by place: each part where its
// node is, or bent by its skin and its shape keys.
func (s *Screen) drawPose(model *Model, pose *modelPose, place matrix, tint Color) {
	for _, part := range model.parts {
		weights := pose.weights[part.node]
		shaped := false
		for _, w := range weights {
			shaped = shaped || w != 0
		}
		if part.skin < 0 && !shaped {
			s.addItem(part.mesh, part.material, place.mul(pose.globals[part.node]), tint)
			continue
		}
		at := len(s.pass.animation)
		joints := 0
		if part.skin >= 0 {
			s.pass.animation = append(s.pass.animation, pose.joints[part.skin]...)
			joints = len(model.skins[part.skin].joints)
		}
		morphs := 0
		if part.mesh.morphCount > 0 {
			morphs = part.mesh.morphCount
			s.pass.animation = append(s.pass.animation, weights[:min(morphs, len(weights))]...)
			for len(s.pass.animation)%4 != 0 || len(s.pass.animation)-at < joints*16+morphs {
				s.pass.animation = append(s.pass.animation, 0)
			}
		}
		// The sphere culling tests: around the joints, as far as their
		// vertices reach, or around the mesh, grown by its shape keys.
		var center Vector3
		var radius float32
		var shapes float32
		for k, w := range weights {
			if k < len(part.mesh.morphReach) {
				shapes += float32(math.Abs(float64(w))) * part.mesh.morphReach[k]
			}
		}
		model3D := place
		if part.skin >= 0 {
			center, radius = model.skinSphere(part, pose)
		} else {
			part.mesh.bounds()
			local := pose.globals[part.node]
			model3D = place.mul(local)
			center, radius = local.transformPoint(part.mesh.center), part.mesh.radius*local.largestScale()
		}
		s.addAnimated(part.mesh, part.material, model3D, tint, itemAnimation{
			at: at, floats: len(s.pass.animation) - at, joints: joints, morphs: morphs,
		}, place.transformPoint(center), (radius+shapes)*place.largestScale())
	}
}

// skinSphere returns a sphere around a skinned part as pose bends it, in the
// model's space: around every joint that moves a vertex, as far as its
// vertices were from it, scaled as the joint is.
func (m *Model) skinSphere(part modelPart, pose *modelPose) (Vector3, float32) {
	joints := pose.joints[part.skin]
	skin := &m.skins[part.skin]
	var lo, hi Vector3
	first := true
	for k, reach := range part.reach {
		if reach < 0 {
			continue
		}
		at := pose.globals[skin.joints[k]]
		center := Vector3{X: at[12], Y: at[13], Z: at[14]}
		r := reach * matrix(joints[k*16:]).largestScale()
		low, high := center.Sub(Vector3{X: r, Y: r, Z: r}), center.Add(Vector3{X: r, Y: r, Z: r})
		if first {
			lo, hi, first = low, high, false
		}
		lo = Vector3{X: min(lo.X, low.X), Y: min(lo.Y, low.Y), Z: min(lo.Z, low.Z)}
		hi = Vector3{X: max(hi.X, high.X), Y: max(hi.Y, high.Y), Z: max(hi.Z, high.Z)}
	}
	center := lo.Lerp(hi, 0.5)
	return center, hi.Distance(center)
}

// decompose splits m into the position, rotation and scale that make it,
// as nearly as a Transform can say it: exactly unless m shears, which only a
// stretched parent turning its child does.
func (m matrix) decompose() (position Vector3, rotation Rotation, scale Vector3) {
	position = Vector3{X: m[12], Y: m[13], Z: m[14]}
	columns := [3]Vector3{{X: m[0], Y: m[1], Z: m[2]}, {X: m[4], Y: m[5], Z: m[6]}, {X: m[8], Y: m[9], Z: m[10]}}
	scale = Vector3{X: columns[0].Length(), Y: columns[1].Length(), Z: columns[2].Length()}
	if columns[0].Cross(columns[1]).Dot(columns[2]) < 0 {
		scale.X = -scale.X // mirrored
	}
	turn := identityMatrix()
	for i, c := range columns {
		length := []float32{scale.X, scale.Y, scale.Z}[i]
		if length != 0 {
			c = c.Scale(1 / length)
		}
		turn[i*4], turn[i*4+1], turn[i*4+2] = c.X, c.Y, c.Z
	}
	return position, turn.rotation(), scale
}

// rotation returns the Rotation of a matrix that only turns: the inverse of
// Rotation.matrix, Yaw, then Pitch, then Roll.
func (m matrix) rotation() Rotation {
	toDegrees := 180 / math.Pi
	// The matrix is Ry(yaw) Rx(-pitch) Rz(roll): its row 1, column 2 is
	// sin(pitch), and the rest give yaw and roll, except looking straight up
	// or down, where only their difference counts and roll is taken as 0.
	sinPitch := math.Max(-1, math.Min(1, float64(m[9])))
	pitch := math.Asin(sinPitch)
	var yaw, roll float64
	if math.Abs(sinPitch) < 0.99999 {
		yaw = math.Atan2(float64(m[8]), float64(m[10]))
		roll = math.Atan2(float64(m[1]), float64(m[5]))
	} else {
		yaw = math.Atan2(float64(-m[2]), float64(m[0]))
	}
	return Rotation{Yaw: float32(yaw * toDegrees), Pitch: float32(pitch * toDegrees), Roll: float32(roll * toDegrees)}
}
