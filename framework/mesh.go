package golib

import (
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
	vertices []float32 // position, normal, texture coordinates: device.LayoutLit
	indices  []uint32  // three a triangle, counterclockwise seen from outside
	twoSided bool      // flat: seen from both sides, so its back isn't culled

	gpu     device.Mesh // on the graphics card, once drawn
	edges   []uint32    // the wireframe's lines, two indices each, made when first drawn
	wireGPU device.Mesh
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
	m.vertices = append(m.vertices, position.X, position.Y, position.Z, normal.X, normal.Y, normal.Z, u, v)
}

// addQuad adds the quad a, b, c, d, counterclockwise, as two triangles.
func (m *Mesh) addQuad(a, b, c, d uint32) {
	m.indices = append(m.indices, a, b, c, a, c, d)
}

// upload puts the mesh on the graphics card, the first time it is drawn.
func (m *Mesh) upload() device.Mesh {
	if m.gpu.ID == 0 {
		m.gpu = device.NewMesh(device.LayoutLit, m.vertices, m.indices, false)
		loadedMeshes.add(m)
	}
	return m.gpu
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
	m.gpu, m.wireGPU = device.Mesh{}, device.Mesh{}
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
