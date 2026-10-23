package golib

import (
	"errors"
	"math"
	"slices"
)

// PhysicsWorld is the solid part of a 3D world, as triangles: a level from
// Blender, the boxes and ramps a game makes, anything that stops a
// character or a ray. Make one in a scene, add what is solid once, and ask
// it where a ray hits or move a CharacterController through it:
//
//	s.physics = golib.NewPhysicsWorld()
//	s.physics.AddModel(level, golib.Transform{})
//
//	// What the mouse points at:
//	origin, direction := s.camera.Ray(input.MousePosition())
//	if hit, ok := s.physics.Raycast(origin, direction, 100); ok {
//		s.marker = hit.Position
//	}
//
// It holds still things and things the game moves itself, such as a lift,
// with SetTransform. Things that fall, roll and topple on their own, rigid
// bodies, come later in M7.
type PhysicsWorld struct {
	bodies    map[int]*physicsBody
	next      int
	triangles []physicsTriangle
	nodes     []bvhNode
	dirty     bool // the triangles or the tree are out of date
}

// physicsBody is one thing added to a PhysicsWorld: its triangles, three
// corners each, in its own space, and where it is.
type physicsBody struct {
	corners []Vector3
	place   matrix
}

// physicsTriangle is a triangle of a body, placed in the world.
type physicsTriangle struct {
	a, b, c Vector3
	normal  Vector3 // length 1, the side its corners go counterclockwise around
	body    int
}

// bvhNode is a node of the tree of boxes the world's triangles are sorted
// into, so a question looks at the few triangles near it rather than all of
// them: a leaf holds triangles from first, count of them; an inner node has
// its two children at left and left+1.
type bvhNode struct {
	lo, hi       Vector3
	left         int32
	first, count int32
}

// RayHit is where a ray, or a sphere swept along one, first meets the world.
type RayHit struct {
	// Position is where it hits: on the surface for a ray, and for a
	// sphere, the sphere's center when it touches.
	Position Vector3

	// Normal is the way the surface it hits faces, length 1.
	Normal Vector3

	// Distance is how far along the ray it hits, in meters.
	Distance float32

	// Body is what it hits: the number the Add method that added it
	// returned.
	Body int
}

// NewPhysicsWorld returns an empty PhysicsWorld.
func NewPhysicsWorld() *PhysicsWorld {
	return &PhysicsWorld{bodies: map[int]*physicsBody{}, next: 1}
}

// AddMesh adds mesh, placed by transform, as solid, and returns its number:
// what RayHit.Body says, and what Remove and SetTransform take.
func (w *PhysicsWorld) AddMesh(mesh *Mesh, transform Transform) int {
	if mesh == nil {
		reportError(errors.New("golib: PhysicsWorld.AddMesh got a nil mesh: make one with golib.NewBoxMesh or another New...Mesh"))
		return 0
	}
	return w.add(meshCorners(mesh, identityMatrix(), nil), transform.matrix())
}

// AddModel adds every mesh of model, at rest and placed by transform, as
// solid, and returns its number. A mesh whose object has the custom
// property collide set to false in Blender is left out: water, grass, the
// leaves of a tree.
func (w *PhysicsWorld) AddModel(model *Model, transform Transform) int {
	if model == nil {
		reportError(errors.New("golib: PhysicsWorld.AddModel got a nil model: make one with golib.NewModel"))
		return 0
	}
	model.mu.Lock()
	err := model.prepare()
	var corners []Vector3
	if err == nil {
		for _, part := range model.parts {
			if model.nodes[part.node].extras["collide"] == "false" {
				continue
			}
			if part.skin >= 0 {
				corners = meshCorners(part.mesh, identityMatrix(), model.rest.joints[part.skin])
			} else {
				corners = append(corners, meshCorners(part.mesh, model.rest.globals[part.node], nil)...)
			}
		}
	}
	model.mu.Unlock()
	if err != nil {
		reportError(err)
		return 0
	}
	return w.add(corners, transform.matrix())
}

// meshCorners returns the corners of mesh's triangles, placed by place, or
// bent at rest by joints for a skinned mesh.
func meshCorners(mesh *Mesh, place matrix, joints []float32) []Vector3 {
	stride := 12
	corner := func(i uint32) Vector3 {
		v := mesh.vertices[int(i)*stride:]
		p := Vector3{X: v[0], Y: v[1], Z: v[2]}
		if joints == nil || mesh.skin == nil {
			return place.transformPoint(p)
		}
		var bent Vector3
		for k := range 4 {
			weight := mesh.skin[int(i)*8+4+k]
			if weight > 0 {
				j := matrix(joints[int(mesh.skin[int(i)*8+k])*16:])
				bent = bent.Add(j.transformPoint(p).Scale(weight))
			}
		}
		return bent
	}
	out := make([]Vector3, 0, len(mesh.indices))
	for _, i := range mesh.indices {
		out = append(out, corner(i))
	}
	return out
}

func (w *PhysicsWorld) add(corners []Vector3, place matrix) int {
	id := w.next
	w.next++
	w.bodies[id] = &physicsBody{corners: corners, place: place}
	w.dirty = true
	return id
}

// Remove takes the body numbered id out of the world: a door opened, a
// wall broken.
func (w *PhysicsWorld) Remove(id int) {
	delete(w.bodies, id)
	w.dirty = true
}

// SetTransform moves the body numbered id to transform, as a lift or a
// drawbridge moves: what is added with a transform is moved by this one in
// its place.
func (w *PhysicsWorld) SetTransform(id int, transform Transform) {
	if b := w.bodies[id]; b != nil {
		b.place = transform.matrix()
		w.dirty = true
	}
}

// build places every body's triangles and sorts them into the tree, when
// something changed since the last question.
func (w *PhysicsWorld) build() {
	if !w.dirty {
		return
	}
	w.dirty = false
	w.triangles = w.triangles[:0]
	ids := make([]int, 0, len(w.bodies))
	for id := range w.bodies {
		ids = append(ids, id)
	}
	slices.Sort(ids) // the same tree every time, so questions answer alike
	for _, id := range ids {
		b := w.bodies[id]
		for i := 0; i+2 < len(b.corners); i += 3 {
			t := physicsTriangle{
				a: b.place.transformPoint(b.corners[i]), b: b.place.transformPoint(b.corners[i+1]),
				c: b.place.transformPoint(b.corners[i+2]), body: id,
			}
			n := t.b.Sub(t.a).Cross(t.c.Sub(t.a))
			if n.Length() < 1e-12 {
				continue // no area: nothing to hit
			}
			t.normal = n.Normalize()
			w.triangles = append(w.triangles, t)
		}
	}
	w.nodes = w.nodes[:0]
	if len(w.triangles) > 0 {
		w.nodes = append(w.nodes, bvhNode{})
		w.split(0, 0, len(w.triangles))
	}
}

// split makes node the box around triangles first to first+count, and,
// when there are more than a few, splits them in two along the box's
// longest side.
func (w *PhysicsWorld) split(node, first, count int) {
	lo, hi := w.triangles[first].bounds()
	for _, t := range w.triangles[first+1 : first+count] {
		l, h := t.bounds()
		lo, hi = lowest(lo, l), highest(hi, h)
	}
	w.nodes[node] = bvhNode{lo: lo, hi: hi, first: int32(first), count: int32(count)}
	if count <= 4 {
		return
	}
	size := hi.Sub(lo)
	axis := func(v Vector3) float32 { return v.X }
	if size.Y > size.X && size.Y >= size.Z {
		axis = func(v Vector3) float32 { return v.Y }
	} else if size.Z > size.X && size.Z > size.Y {
		axis = func(v Vector3) float32 { return v.Z }
	}
	part := w.triangles[first : first+count]
	slices.SortStableFunc(part, func(p, q physicsTriangle) int {
		a, b := axis(p.a.Add(p.b).Add(p.c)), axis(q.a.Add(q.b).Add(q.c))
		switch {
		case a < b:
			return -1
		case a > b:
			return 1
		}
		return 0
	})
	half := count / 2
	left := len(w.nodes)
	w.nodes = append(w.nodes, bvhNode{}, bvhNode{})
	w.nodes[node].left, w.nodes[node].count = int32(left), 0
	w.split(left, first, half)
	w.split(left+1, first+half, count-half)
}

func (t *physicsTriangle) bounds() (lo, hi Vector3) {
	return lowest(lowest(t.a, t.b), t.c), highest(highest(t.a, t.b), t.c)
}

func lowest(a, b Vector3) Vector3 {
	return Vector3{X: min(a.X, b.X), Y: min(a.Y, b.Y), Z: min(a.Z, b.Z)}
}

func highest(a, b Vector3) Vector3 {
	return Vector3{X: max(a.X, b.X), Y: max(a.Y, b.Y), Z: max(a.Z, b.Z)}
}

// near calls visit with every triangle whose box overlaps the box from lo
// to hi.
func (w *PhysicsWorld) near(lo, hi Vector3, visit func(t *physicsTriangle)) {
	w.build()
	if len(w.nodes) == 0 {
		return
	}
	stack := []int32{0}
	for len(stack) > 0 {
		n := &w.nodes[stack[len(stack)-1]]
		stack = stack[:len(stack)-1]
		if n.hi.X < lo.X || n.lo.X > hi.X || n.hi.Y < lo.Y || n.lo.Y > hi.Y || n.hi.Z < lo.Z || n.lo.Z > hi.Z {
			continue
		}
		if n.count > 0 {
			for i := n.first; i < n.first+n.count; i++ {
				visit(&w.triangles[i])
			}
			continue
		}
		stack = append(stack, n.left, n.left+1)
	}
}

// Raycast returns where the ray from origin along direction first hits the
// world, within distance meters, and whether it hits anything. Triangles
// are hit from either side. direction needn't be length 1.
//
//	// The ground under the player, up to 50 meters down:
//	if hit, ok := s.physics.Raycast(p.position.Add(golib.Vector3{Y: 0.5}), golib.Vector3{Y: -1}, 50); ok {
//		shadowAt = hit.Position
//	}
func (w *PhysicsWorld) Raycast(origin, direction Vector3, distance float32) (RayHit, bool) {
	return w.SphereCast(origin, direction, 0, distance)
}

// SphereCast sweeps a sphere radius meters across from origin along
// direction, and returns where it first touches the world, within distance
// meters: Position is then the sphere's center. A sphere already touching
// something at origin hits it at once. A sphere is a fat ray: a thrown ball,
// a camera that mustn't go into a wall.
func (w *PhysicsWorld) SphereCast(origin, direction Vector3, radius, distance float32) (RayHit, bool) {
	direction = direction.Normalize()
	if direction == (Vector3{}) || distance <= 0 {
		return RayHit{}, false
	}
	radius = max(0, radius)
	end := origin.Add(direction.Scale(distance))
	pad := Vector3{X: radius, Y: radius, Z: radius}
	best := RayHit{Distance: distance}
	found := false
	// The tree's boxes along the way: the box around the whole sweep, then
	// each triangle's own test.
	w.near(lowest(origin, end).Sub(pad), highest(origin, end).Add(pad), func(t *physicsTriangle) {
		if d, n, ok := sweepTriangle(origin, direction, radius, t); ok && d <= best.Distance {
			best = RayHit{Distance: d, Normal: n, Body: t.body}
			found = true
		}
	})
	if !found {
		return RayHit{}, false
	}
	best.Position = origin.Add(direction.Scale(best.Distance))
	return best, true
}

// sweepTriangle returns how far along direction, length 1, a sphere of
// radius from origin first touches t, and the touching surface's normal.
func sweepTriangle(origin, direction Vector3, radius float32, t *physicsTriangle) (float32, Vector3, bool) {
	n := t.normal
	if origin.Sub(t.a).Dot(n) < 0 {
		n = n.Scale(-1) // the side the sphere is on
	}
	// Touching already.
	if closest := closestOnTriangle(origin, t); closest.Distance(origin) < radius {
		away := origin.Sub(closest)
		if away.Length() > 1e-6 {
			n = away.Normalize()
		}
		return 0, n, true
	}
	best := float32(math.MaxFloat32)
	var normal Vector3
	hit := false
	// The face: where the sphere's front reaches the plane, inside the
	// triangle.
	if towards := direction.Dot(n); towards < -1e-8 {
		d := (origin.Sub(t.a).Dot(n) - radius) / -towards
		if d >= 0 {
			p := origin.Add(direction.Scale(d)).Sub(n.Scale(radius))
			if insideTriangle(p, t) {
				best, normal, hit = d, n, true
			}
		}
	}
	if hit {
		return best, normal, true
	}
	// The edges and the corners, which a sphere can graze past the face.
	for _, e := range [3][2]Vector3{{t.a, t.b}, {t.b, t.c}, {t.c, t.a}} {
		if d, ok := sweepSegment(origin, direction, radius, e[0], e[1]); ok && d < best {
			best, hit = d, true
			center := origin.Add(direction.Scale(d))
			normal = center.Sub(closestOnSegment(center, e[0], e[1])).Normalize()
		}
	}
	return best, normal, hit
}

// sweepSegment returns how far along direction a sphere from origin first
// touches the segment from a to b: its sides, a cylinder, or its ends.
func sweepSegment(origin, direction Vector3, radius float32, a, b Vector3) (float32, bool) {
	best := float32(math.MaxFloat32)
	hit := false
	edge := b.Sub(a)
	length2 := edge.Dot(edge)
	if length2 > 1e-12 {
		// The cylinder: the parts of origin and direction across the edge.
		m := origin.Sub(a)
		dd, md := direction.Dot(edge)/length2, m.Dot(edge)/length2
		da, ma := direction.Sub(edge.Scale(dd)), m.Sub(edge.Scale(md))
		qa, qb, qc := da.Dot(da), 2*da.Dot(ma), ma.Dot(ma)-radius*radius
		if qa > 1e-12 {
			if d, ok := firstRoot(qa, qb, qc); ok {
				if along := md + dd*d; along >= 0 && along <= 1 {
					best, hit = d, true
				}
			}
		}
	}
	for _, p := range []Vector3{a, b} {
		m := origin.Sub(p)
		if d, ok := firstRoot(1, 2*m.Dot(direction), m.Dot(m)-radius*radius); ok && d < best {
			best, hit = d, true
		}
	}
	return best, hit
}

// firstRoot returns the smaller root of a x² + b x + c that isn't negative.
func firstRoot(a, b, c float32) (float32, bool) {
	disc := b*b - 4*a*c
	if disc < 0 {
		return 0, false
	}
	s := float32(math.Sqrt(float64(disc)))
	d := (-b - s) / (2 * a)
	if d < 0 {
		return 0, false // behind, or already inside, which the caller checks first
	}
	return d, true
}

// insideTriangle says whether p, on t's plane, is inside it.
func insideTriangle(p Vector3, t *physicsTriangle) bool {
	n := t.normal
	return t.b.Sub(t.a).Cross(p.Sub(t.a)).Dot(n) >= 0 &&
		t.c.Sub(t.b).Cross(p.Sub(t.b)).Dot(n) >= 0 &&
		t.a.Sub(t.c).Cross(p.Sub(t.c)).Dot(n) >= 0
}

// closestOnSegment returns the point of the segment from a to b nearest p.
func closestOnSegment(p, a, b Vector3) Vector3 {
	ab := b.Sub(a)
	l := ab.Dot(ab)
	if l < 1e-12 {
		return a
	}
	return a.Add(ab.Scale(Clamp(p.Sub(a).Dot(ab)/l, 0, 1)))
}

// closestOnTriangle returns the point of t nearest p (Ericson, Real-Time
// Collision Detection, 5.1.5).
func closestOnTriangle(p Vector3, t *physicsTriangle) Vector3 {
	a, b, c := t.a, t.b, t.c
	ab, ac, ap := b.Sub(a), c.Sub(a), p.Sub(a)
	d1, d2 := ab.Dot(ap), ac.Dot(ap)
	if d1 <= 0 && d2 <= 0 {
		return a
	}
	bp := p.Sub(b)
	d3, d4 := ab.Dot(bp), ac.Dot(bp)
	if d3 >= 0 && d4 <= d3 {
		return b
	}
	if vc := d1*d4 - d3*d2; vc <= 0 && d1 >= 0 && d3 <= 0 {
		return a.Add(ab.Scale(d1 / (d1 - d3)))
	}
	cp := p.Sub(c)
	d5, d6 := ab.Dot(cp), ac.Dot(cp)
	if d6 >= 0 && d5 <= d6 {
		return c
	}
	if vb := d5*d2 - d1*d6; vb <= 0 && d2 >= 0 && d6 <= 0 {
		return a.Add(ac.Scale(d2 / (d2 - d6)))
	}
	if va := d3*d6 - d5*d4; va <= 0 && d4-d3 >= 0 && d5-d6 >= 0 {
		return b.Add(c.Sub(b).Scale((d4 - d3) / ((d4 - d3) + (d5 - d6))))
	}
	va, vb, vc := d3*d6-d5*d4, d5*d2-d1*d6, d1*d4-d3*d2
	denom := 1 / (va + vb + vc)
	return a.Add(ab.Scale(vb * denom)).Add(ac.Scale(vc * denom))
}

// closestSegmentTriangle returns the points of the segment from p to q and
// of t nearest each other.
func closestSegmentTriangle(p, q Vector3, t *physicsTriangle) (onSegment, onTriangle Vector3) {
	// Where the segment crosses the triangle, they meet.
	dp, dq := p.Sub(t.a).Dot(t.normal), q.Sub(t.a).Dot(t.normal)
	if (dp < 0) != (dq < 0) && dp != dq {
		x := p.Add(q.Sub(p).Scale(dp / (dp - dq)))
		if insideTriangle(x, t) {
			return x, x
		}
	}
	best := float32(math.MaxFloat32)
	try := func(s, tri Vector3) {
		if d := s.Sub(tri).Dot(s.Sub(tri)); d < best {
			best, onSegment, onTriangle = d, s, tri
		}
	}
	try(p, closestOnTriangle(p, t))
	try(q, closestOnTriangle(q, t))
	for _, e := range [3][2]Vector3{{t.a, t.b}, {t.b, t.c}, {t.c, t.a}} {
		s, tri := closestSegments(p, q, e[0], e[1])
		try(s, tri)
	}
	return onSegment, onTriangle
}

// closestSegments returns the points of the segments p1-q1 and p2-q2
// nearest each other (Ericson, 5.1.9).
func closestSegments(p1, q1, p2, q2 Vector3) (Vector3, Vector3) {
	d1, d2, r := q1.Sub(p1), q2.Sub(p2), p1.Sub(p2)
	a, e, f := d1.Dot(d1), d2.Dot(d2), d2.Dot(r)
	var s, t float32
	switch {
	case a <= 1e-12 && e <= 1e-12:
		return p1, p2
	case a <= 1e-12:
		t = Clamp(f/e, 0, 1)
	default:
		c := d1.Dot(r)
		if e <= 1e-12 {
			s = Clamp(-c/a, 0, 1)
		} else {
			b := d1.Dot(d2)
			denom := a*e - b*b
			if denom != 0 {
				s = Clamp((b*f-c*e)/denom, 0, 1)
			}
			t = (b*s + f) / e
			if t < 0 {
				t, s = 0, Clamp(-c/a, 0, 1)
			} else if t > 1 {
				t, s = 1, Clamp((b-c)/a, 0, 1)
			}
		}
	}
	return p1.Add(d1.Scale(s)), p2.Add(d2.Scale(t))
}
