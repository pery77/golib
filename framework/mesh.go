package golib

import (
	"fmt"
	"math"
	"sync"

	"golib/internal/device"
)

// Mesh is a shape for the 3D world, made of triangles. GoLib makes the basic
// ones, each one meter across and centered on 0, 0, 0, so that a
// [Transform]'s Scale gives them their size:
//
//	box := golib.NewBoxMesh() // once, in the scene's constructor
//
//	// In Draw, between Screen.Begin3D and Screen.End3D:
//	screen.DrawMesh(box, golib.Transform{Position: wall.Position, Scale: golib.Vector3{X: 4, Y: 2, Z: 0.2}}, golib.Gray)
//
// A mesh goes to the graphics card the first time it is drawn, and Run frees
// it when the game ends. One mesh can be drawn any number of times a frame.
type Mesh struct {
	vertices []float32 // position, normal, texture coordinates, linear color: device.LayoutLit
	indices  []uint32  // three a triangle, counterclockwise seen from outside
	twoSided bool      // flat: seen from both sides, so its back isn't culled

	measured       bool    // center and radius are known
	center         Vector3 // the middle of its bounds
	radius         float32 // how far its farthest vertex is from center
	lowest, upmost Vector3 // its bounds: the least and the most of each axis

	gpu     device.Mesh // on the graphics card, once drawn
	edges   []uint32    // the wireframe's lines, two indices each, made when first drawn
	wireGPU device.Mesh

	// A model's mesh may be bent by bones and shape keys. skin holds four
	// joints and their four weights for each vertex, or nil; morphs, for
	// each of morphCount shape keys, how much each vertex moves and its
	// normal turns at weight 1, as two texels of four numbers.
	skin       []float32
	morphs     []float32
	morphCount int
	morphReach []float32 // for each shape key, the farthest it moves a vertex
	shapeKeys  []string  // their names, as Blender gave them
	morphGPU   device.DataTexture

	// lightcoords is where each vertex is on the lightmap a model's level
	// was baked into, u and v, or nil.
	lightcoords []float32
}

// sphereSlices and sphereRings are how finely spheres and cylinders are cut:
// enough to look round on a 1080p screen at the size games draw them.
const (
	roundSlices = 32
	sphereRings = 16
)

// NewBoxMesh returns a box one meter wide, high and deep, centered on 0, 0,
// 0. Scale it into walls, floors, crates and platforms.
func NewBoxMesh() *Mesh {
	m := &Mesh{}
	faces := []struct{ normal, across, up Vector3 }{
		{Vector3{X: 1}, Vector3{Z: -1}, Vector3{Y: 1}},
		{Vector3{X: -1}, Vector3{Z: 1}, Vector3{Y: 1}},
		{Vector3{Y: 1}, Vector3{X: 1}, Vector3{Z: -1}},
		{Vector3{Y: -1}, Vector3{X: 1}, Vector3{Z: 1}},
		{Vector3{Z: 1}, Vector3{X: 1}, Vector3{Y: 1}},
		{Vector3{Z: -1}, Vector3{X: -1}, Vector3{Y: 1}},
	}
	for _, face := range faces {
		first := m.vertexCount()
		for _, corner := range [4][2]float32{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
			p := face.normal.Scale(0.5).Add(face.across.Scale(corner[0] / 2)).Add(face.up.Scale(corner[1] / 2))
			m.addVertex(p, face.normal, (corner[0]+1)/2, (1-corner[1])/2)
		}
		m.addQuad(first, first+1, first+2, first+3)
	}
	return m
}

// NewSphereMesh returns a ball one meter across, centered on 0, 0, 0.
func NewSphereMesh() *Mesh {
	m := &Mesh{}
	for ring := 0; ring <= sphereRings; ring++ {
		v := float64(ring) / sphereRings
		polar := v * math.Pi // 0 at the top, pi at the bottom
		for slice := 0; slice <= roundSlices; slice++ {
			u := float64(slice) / roundSlices
			around := u * 2 * math.Pi
			normal := Vector3{
				X: float32(math.Sin(polar) * math.Sin(around)),
				Y: float32(math.Cos(polar)),
				Z: float32(math.Sin(polar) * math.Cos(around)),
			}
			m.addVertex(normal.Scale(0.5), normal, float32(u), float32(v))
		}
	}
	row := uint32(roundSlices + 1)
	for ring := uint32(0); ring < sphereRings; ring++ {
		for slice := uint32(0); slice < roundSlices; slice++ {
			top, bottom := ring*row+slice, (ring+1)*row+slice
			// The quad's corners counterclockwise seen from outside: down
			// the ring, then around the way the angle grows.
			m.addQuad(top, bottom, bottom+1, top+1)
		}
	}
	return m
}

// NewCylinderMesh returns a cylinder one meter across and one meter high,
// standing on the Y axis, centered on 0, 0, 0, with its ends closed.
func NewCylinderMesh() *Mesh {
	m := &Mesh{}
	first := m.vertexCount()
	for slice := 0; slice <= roundSlices; slice++ {
		u := float64(slice) / roundSlices
		around := u * 2 * math.Pi
		normal := Vector3{X: float32(math.Sin(around)), Z: float32(math.Cos(around))}
		m.addVertex(normal.Scale(0.5).Add(Vector3{Y: 0.5}), normal, float32(u), 0)
		m.addVertex(normal.Scale(0.5).Add(Vector3{Y: -0.5}), normal, float32(u), 1)
	}
	for slice := uint32(0); slice < roundSlices; slice++ {
		top, bottom := first+slice*2, first+slice*2+1
		m.addQuad(top, bottom, bottom+2, top+2)
	}
	for _, end := range []float32{0.5, -0.5} {
		normal := Vector3{Y: end * 2}
		center := m.vertexCount()
		m.addVertex(Vector3{Y: end}, normal, 0.5, 0.5)
		for slice := 0; slice <= roundSlices; slice++ {
			around := float64(slice) / roundSlices * 2 * math.Pi
			sin, cos := float32(math.Sin(around)), float32(math.Cos(around))
			m.addVertex(Vector3{X: sin / 2, Y: end, Z: cos / 2}, normal, 0.5+sin/2, 0.5-cos/2)
		}
		for slice := uint32(0); slice < roundSlices; slice++ {
			a, b := center+1+slice, center+2+slice
			if end > 0 {
				m.indices = append(m.indices, center, a, b)
			} else {
				m.indices = append(m.indices, center, b, a)
			}
		}
	}
	return m
}

// NewPlaneMesh returns a flat square one meter on each side, lying on the
// ground at 0, 0, 0 and facing up. It is seen from below too. Scale its X and
// Z into a floor, a road or water.
func NewPlaneMesh() *Mesh {
	m := &Mesh{twoSided: true}
	up := Vector3{Y: 1}
	m.addVertex(Vector3{X: -0.5, Z: -0.5}, up, 0, 0)
	m.addVertex(Vector3{X: -0.5, Z: 0.5}, up, 0, 1)
	m.addVertex(Vector3{X: 0.5, Z: 0.5}, up, 1, 1)
	m.addVertex(Vector3{X: 0.5, Z: -0.5}, up, 1, 0)
	m.addQuad(0, 1, 2, 3)
	return m
}

func (m *Mesh) vertexCount() uint32 {
	return uint32(len(m.vertices) / device.LayoutFloats[device.LayoutLit])
}

func (m *Mesh) addVertex(position, normal Vector3, u, v float32) {
	m.vertices = append(m.vertices, position.X, position.Y, position.Z, normal.X, normal.Y, normal.Z, u, v, 1, 1, 1, 1)
}

// bounds measures the mesh, the first time it is asked: the box around it,
// and the sphere around that, which culling tests against what the camera
// sees.
func (m *Mesh) bounds() {
	if m.measured {
		return
	}
	m.measured = true
	stride := device.LayoutFloats[device.LayoutLit]
	if len(m.vertices) < stride {
		return
	}
	lo := Vector3{X: m.vertices[0], Y: m.vertices[1], Z: m.vertices[2]}
	hi := lo
	for i := stride; i+2 < len(m.vertices); i += stride {
		p := Vector3{X: m.vertices[i], Y: m.vertices[i+1], Z: m.vertices[i+2]}
		lo = Vector3{X: min(lo.X, p.X), Y: min(lo.Y, p.Y), Z: min(lo.Z, p.Z)}
		hi = Vector3{X: max(hi.X, p.X), Y: max(hi.Y, p.Y), Z: max(hi.Z, p.Z)}
	}
	m.lowest, m.upmost = lo, hi
	m.center = lo.Lerp(hi, 0.5)
	m.radius = hi.Distance(m.center)
}

// Vertex is a corner of a mesh made with [NewMesh].
type Vertex struct {
	Position Vector3

	// Normal is the way the surface faces at the corner, length 1. Leave
	// it zero in every vertex and NewMesh works the normals out from the
	// triangles: shared corners make a smooth surface, corners repeated for
	// each triangle a faceted one.
	Normal Vector3

	// U and V place a texture on the mesh: 0, 0 is the picture's top-left
	// corner, 1, 1 its bottom-right one.
	U, V float32

	// Color tints the mesh at this corner, blending towards the next ones;
	// the color DrawMesh gets multiplies it. The zero Color counts as White.
	Color Color
}

// NewMesh returns a mesh made of triangles given in code: terrain from a
// height map, a ramp, a gem, any shape the New...Mesh functions don't make.
// indices name three vertices for each triangle, counterclockwise as seen
// from the side the triangle faces; that side is drawn, and the other is
// culled.
//
//	// A slope rising towards -Z, 4 meters wide and long, 1 high.
//	ramp := golib.NewMesh([]golib.Vertex{
//		{Position: golib.Vector3{X: -2, Z: 2}}, {Position: golib.Vector3{X: 2, Z: 2}},
//		{Position: golib.Vector3{X: 2, Y: 1, Z: -2}}, {Position: golib.Vector3{X: -2, Y: 1, Z: -2}},
//	}, []int{0, 1, 2, 0, 2, 3})
func NewMesh(vertices []Vertex, indices []int) *Mesh {
	m := &Mesh{}
	if len(indices)%3 != 0 {
		reportError(fmt.Errorf("golib.NewMesh got %d indices: give three for each triangle", len(indices)))
		return m
	}
	smooth := true
	for _, v := range vertices {
		if v.Normal != (Vector3{}) {
			smooth = false
			break
		}
	}
	normals := make([]Vector3, len(vertices))
	for i := range vertices {
		normals[i] = vertices[i].Normal
	}
	for t := 0; t < len(indices); t += 3 {
		a, b, c := indices[t], indices[t+1], indices[t+2]
		for _, i := range []int{a, b, c} {
			if i < 0 || i >= len(vertices) {
				reportError(fmt.Errorf("golib.NewMesh: triangle %d names vertex %d, but there are %d", t/3, i, len(vertices)))
				return &Mesh{}
			}
		}
		if smooth {
			// Each triangle adds its normal, weighted by its area, to its
			// corners'.
			face := vertices[b].Position.Sub(vertices[a].Position).Cross(vertices[c].Position.Sub(vertices[a].Position))
			normals[a], normals[b], normals[c] = normals[a].Add(face), normals[b].Add(face), normals[c].Add(face)
		}
		m.indices = append(m.indices, uint32(a), uint32(b), uint32(c))
	}
	for i, v := range vertices {
		color := [4]float32{1, 1, 1, 1}
		if v.Color != (Color{}) {
			color = linearColor(v.Color)
		}
		n := normals[i].Normalize()
		m.vertices = append(m.vertices, v.Position.X, v.Position.Y, v.Position.Z, n.X, n.Y, n.Z, v.U, v.V,
			color[0], color[1], color[2], color[3])
	}
	return m
}

// linearColor returns c as four numbers from 0 to 1, its red, green and blue
// turned from sRGB, as colors are given, into the linear light shaders mix.
func linearColor(c Color) [4]float32 {
	toLinear := func(v uint8) float32 { return float32(math.Pow(float64(v)/255, 2.2)) }
	return [4]float32{toLinear(c.R), toLinear(c.G), toLinear(c.B), float32(c.A) / 255}
}

// NewConeMesh returns a cone one meter across at its base and one meter high,
// pointing up the Y axis, centered on 0, 0, 0, its base closed.
func NewConeMesh() *Mesh {
	m := &Mesh{}
	slope := float32(1 / math.Sqrt(5)) // a side rises 1 for every 0.5 out: its normal leans up by this
	across := float32(2 / math.Sqrt(5))
	for slice := 0; slice <= roundSlices; slice++ {
		u := float64(slice) / roundSlices
		around := u * 2 * math.Pi
		sin, cos := float32(math.Sin(around)), float32(math.Cos(around))
		normal := Vector3{X: sin * across, Y: slope, Z: cos * across}
		m.addVertex(Vector3{Y: 0.5}, normal, float32(u), 0)
		m.addVertex(Vector3{X: sin / 2, Y: -0.5, Z: cos / 2}, normal, float32(u), 1)
	}
	for slice := uint32(0); slice < roundSlices; slice++ {
		tip, edge := slice*2, slice*2+1
		m.indices = append(m.indices, tip, edge, edge+2)
	}
	center := m.vertexCount()
	m.addVertex(Vector3{Y: -0.5}, Vector3{Y: -1}, 0.5, 0.5)
	for slice := 0; slice <= roundSlices; slice++ {
		around := float64(slice) / roundSlices * 2 * math.Pi
		sin, cos := float32(math.Sin(around)), float32(math.Cos(around))
		m.addVertex(Vector3{X: sin / 2, Y: -0.5, Z: cos / 2}, Vector3{Y: -1}, 0.5+sin/2, 0.5-cos/2)
	}
	for slice := uint32(0); slice < roundSlices; slice++ {
		m.indices = append(m.indices, center, center+2+slice, center+1+slice)
	}
	return m
}

// NewCapsuleMesh returns a capsule, the usual shape of a character: one meter
// high and half a meter across, standing on the Y axis, centered on 0, 0, 0.
// Scale it the same way in X, Y and Z, or its round ends stretch.
func NewCapsuleMesh() *Mesh {
	m := &Mesh{}
	const radius, middle = 0.25, 0.25 // the round ends' radius, and how far their centers are from 0
	// A sphere's rows of vertices, split at its equator into two halves: the
	// top half's last row and the bottom half's first are both on the
	// equator, and the straight side runs between them.
	half := sphereRings / 2
	rings := sphereRings + 1 // rows of quads
	row := uint32(roundSlices + 1)
	for ring := 0; ring <= rings; ring++ {
		step, center := ring, float32(middle)
		if ring > half {
			step, center = ring-1, -middle
		}
		polar := float64(step) / float64(sphereRings) * math.Pi
		for slice := 0; slice <= roundSlices; slice++ {
			u := float64(slice) / roundSlices
			around := u * 2 * math.Pi
			normal := Vector3{
				X: float32(math.Sin(polar) * math.Sin(around)),
				Y: float32(math.Cos(polar)),
				Z: float32(math.Sin(polar) * math.Cos(around)),
			}
			position := normal.Scale(radius).Add(Vector3{Y: center})
			m.addVertex(position, normal, float32(u), 0.5-position.Y)
		}
	}
	for ring := uint32(0); ring < uint32(rings); ring++ {
		for slice := uint32(0); slice < roundSlices; slice++ {
			top, bottom := ring*row+slice, (ring+1)*row+slice
			m.addQuad(top, bottom, bottom+1, top+1)
		}
	}
	return m
}

// addQuad adds the quad a, b, c, d, counterclockwise, as two triangles.
func (m *Mesh) addQuad(a, b, c, d uint32) {
	m.indices = append(m.indices, a, b, c, a, c, d)
}

// upload puts the mesh on the graphics card, the first time it is drawn: a
// skinned one with its joints and weights after each vertex, a lightmapped
// one with where it is on its lightmap.
func (m *Mesh) upload() device.Mesh {
	if m.gpu.ID == 0 {
		switch {
		case m.skin == nil && m.lightcoords != nil:
			stride := device.LayoutFloats[device.LayoutLit]
			count := int(m.vertexCount())
			vertices := make([]float32, 0, count*device.LayoutFloats[device.LayoutLightmapped])
			for i := 0; i < count; i++ {
				vertices = append(vertices, m.vertices[i*stride:(i+1)*stride]...)
				vertices = append(vertices, m.lightcoords[i*2:i*2+2]...)
			}
			m.gpu = device.NewMesh(device.LayoutLightmapped, vertices, m.indices, false)
		case m.skin == nil:
			m.gpu = device.NewMesh(device.LayoutLit, m.vertices, m.indices, false)
		default:
			stride := device.LayoutFloats[device.LayoutLit]
			count := int(m.vertexCount())
			vertices := make([]float32, 0, count*device.LayoutFloats[device.LayoutSkinned])
			for i := 0; i < count; i++ {
				vertices = append(vertices, m.vertices[i*stride:(i+1)*stride]...)
				vertices = append(vertices, m.skin[i*8:i*8+8]...)
			}
			m.gpu = device.NewMesh(device.LayoutSkinned, vertices, m.indices, false)
		}
		loadedMeshes.add(m)
	}
	return m.gpu
}

// uploadMorphs puts the mesh's shape keys on the graphics card, the first
// time it is drawn with them.
func (m *Mesh) uploadMorphs() device.DataTexture {
	if m.morphGPU.ID == 0 && len(m.morphs) > 0 {
		m.morphGPU = device.NewDataTexture(m.morphs)
	}
	return m.morphGPU
}

// uploadWire puts the mesh's wireframe on the graphics card, the first time
// it is drawn: its edges, in white, for a program to tint.
func (m *Mesh) uploadWire() device.Mesh {
	if m.wireGPU.ID == 0 {
		if m.edges == nil {
			m.edges = m.outlineEdges()
		}
		count := int(m.vertexCount())
		vertices := make([]float32, 0, count*device.LayoutFloats[device.LayoutColored])
		stride := device.LayoutFloats[device.LayoutLit]
		if count == 0 {
			return device.Mesh{}
		}
		for i := 0; i < count; i++ {
			p := m.vertices[i*stride:]
			vertices = append(vertices, p[0], p[1], p[2], 1, 1, 1, 1)
		}
		m.wireGPU = device.NewMesh(device.LayoutColored, vertices, m.edges, true)
		loadedMeshes.add(m)
	}
	return m.wireGPU
}

// outlineEdges returns the edges of the mesh's triangles, each once, without
// the ones between two triangles that lie flat against each other, such as
// the diagonal across each side of a box.
func (m *Mesh) outlineEdges() []uint32 {
	type edge struct{ a, b Vector3 }
	stride := device.LayoutFloats[device.LayoutLit]
	position := func(i uint32) Vector3 {
		p := m.vertices[int(i)*stride:]
		return Vector3{X: p[0], Y: p[1], Z: p[2]}
	}
	key := func(a, b uint32) edge {
		pa, pb := position(a), position(b)
		// Vertices at the same place, such as a box's corner seen by three
		// faces, are one corner for the outline.
		if pa.X > pb.X || pa.X == pb.X && (pa.Y > pb.Y || pa.Y == pb.Y && pa.Z > pb.Z) {
			pa, pb = pb, pa
		}
		return edge{pa, pb}
	}
	type seen struct {
		a, b    uint32
		normals []Vector3
	}
	edges := map[edge]*seen{}
	var order []edge
	for t := 0; t+2 < len(m.indices); t += 3 {
		i := m.indices[t : t+3]
		normal := position(i[1]).Sub(position(i[0])).Cross(position(i[2]).Sub(position(i[0]))).Normalize()
		for k := 0; k < 3; k++ {
			a, b := i[k], i[(k+1)%3]
			e := key(a, b)
			if edges[e] == nil {
				edges[e] = &seen{a: a, b: b}
				order = append(order, e)
			}
			edges[e].normals = append(edges[e].normals, normal)
		}
	}
	var lines []uint32
	for _, e := range order {
		s := edges[e]
		if len(s.normals) == 2 && s.normals[0].Dot(s.normals[1]) > 0.9999 {
			continue // inside a flat face
		}
		lines = append(lines, s.a, s.b)
	}
	return lines
}

// unload frees what the mesh holds on the graphics card, so that it goes
// there again if a game runs again.
func (m *Mesh) unload() {
	device.UnloadMesh(m.gpu)
	device.UnloadMesh(m.wireGPU)
	device.UnloadDataTexture(m.morphGPU)
	m.gpu, m.wireGPU, m.morphGPU = device.Mesh{}, device.Mesh{}, device.DataTexture{}
}

// loadedMeshes are the meshes on the graphics card, for Run to free when it
// ends.
var loadedMeshes meshList

type meshList struct {
	sync.Mutex
	meshes []*Mesh
}

func (l *meshList) add(m *Mesh) {
	l.Lock()
	defer l.Unlock()
	l.meshes = append(l.meshes, m)
}

// unloadAll frees every mesh on the graphics card. The window must still be
// open.
func (l *meshList) unloadAll() {
	l.Lock()
	meshes := l.meshes
	l.meshes = nil
	l.Unlock()
	for _, m := range meshes {
		m.unload()
	}
}
