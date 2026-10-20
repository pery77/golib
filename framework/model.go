package golib

import (
	"errors"
	"fmt"
	"math"
	"path"
	"sync"

	"golib/internal/device"
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
}

// modelPart is one mesh of a model, with its material, where the model's
// nodes put it.
type modelPart struct {
	mesh     *Mesh
	material *material3D
	place    matrix
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
	dir := path.Dir(m.name)
	doc, err := gltf.Read(file, func(uri string) ([]byte, error) {
		return ReadAsset(path.Join(dir, uri))
	})
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
			baseColor:       mat.BaseColor,
			baseTexture:     texture(mat.BaseColorTexture, false),
			emissive:        mat.Emissive,
			emissiveTexture: texture(mat.EmissiveTexture, false),
			alphaCutoff:     mat.AlphaCutoff,
			twoSided:        mat.DoubleSided,
			unlit:           mat.Unlit,
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
			meshes[i] = append(meshes[i], meshOf(p))
		}
	}

	first := true
	var visit func(node int, parent matrix)
	visit = func(node int, parent matrix) {
		n := doc.Nodes[node]
		place := parent.mul(matrix(n.Local))
		if n.Mesh >= 0 {
			for k, mesh := range meshes[n.Mesh] {
				material := plainMaterial
				if at := doc.Meshes[n.Mesh].Primitives[k].Material; at >= 0 {
					material = materials[at]
				}
				m.parts = append(m.parts, modelPart{mesh: mesh, material: material, place: place})
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
					p = place.transformPoint(p)
					if first {
						m.lowest, m.upmost, first = p, p, false
					}
					m.lowest = Vector3{X: min(m.lowest.X, p.X), Y: min(m.lowest.Y, p.Y), Z: min(m.lowest.Z, p.Z)}
					m.upmost = Vector3{X: max(m.upmost.X, p.X), Y: max(m.upmost.Y, p.Y), Z: max(m.upmost.Z, p.Z)}
				}
			}
		} else if !n.Joint {
			position, rotation, scale := place.decompose()
			m.markers = append(m.markers, ModelMarker{
				Name: n.Name, Position: position, Rotation: rotation, Scale: scale,
				Properties: Properties(n.Extras),
			})
		}
		for _, child := range n.Children {
			visit(child, place)
		}
	}
	for _, root := range doc.Roots {
		visit(root, identityMatrix())
	}
	if len(m.parts) == 0 && len(m.markers) == 0 {
		return errors.New("the file has nothing in its scene: export it again with the objects selected, or with Include > Selected Objects unticked")
	}
	return nil
}

func boolIndex(b bool) int {
	if b {
		return 1
	}
	return 0
}

// meshOf turns a glTF primitive into a Mesh.
func meshOf(p gltf.Primitive) *Mesh {
	count := len(p.Positions) / 3
	mesh := &Mesh{indices: p.Indices}
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
	model.decodeTextures()
	if model.err != nil {
		reportError(model.err)
		return
	}
	place := transform.matrix()
	for _, part := range model.parts {
		s.addItem(part.mesh, part.material, place.mul(part.place), tint)
	}
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
