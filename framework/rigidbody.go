package golib

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync"
	"weak"

	"golib/internal/device"
)

// Rigid bodies fall, slide, roll, topple and stack on their own, simulated
// by Jolt Physics: Horizon Forbidden West's and Godot's engine, in GoLib's
// native library on the desktop and as JoltPhysics.js in a browser. They
// live in a PhysicsWorld with its still things, which Jolt gets a copy of,
// and each has its triangles there too, so rays hit them and characters
// stand on them and push them.

// BodyShape is the shape of a rigid body: a box, a ball, a capsule, a
// cylinder, or the shape around a model or a mesh. A Transform's Scale stretches it, as it
// stretches a mesh.
type BodyShape struct {
	kind    int
	size    Vector3   // a box's size; a ball's diameter in X; a capsule's or a cylinder's width in X, height in Y
	points  []Vector3 // a hull's points, as made
	corners []Vector3 // its triangles, for rays and characters: three corners each
}

// BoxShape is a box size meters wide, high and deep, centered on its body's
// position: a crate is golib.BoxShape(golib.Vector3{X: 1, Y: 1, Z: 1}).
func BoxShape(size Vector3) BodyShape {
	return BodyShape{kind: device.ShapeBox, size: size, corners: meshCorners(NewBoxMesh(), scaleMatrix(size), nil)}
}

// SphereShape is a ball radius meters across from its middle.
func SphereShape(radius float32) BodyShape {
	return BodyShape{kind: device.ShapeSphere, size: Vector3{X: 2 * radius, Y: 2 * radius, Z: 2 * radius}, corners: ballCorners(radius, 0)}
}

// CapsuleShape is a capsule height meters tall, round at both ends, radius
// meters from its middle, standing on Y and centered on its position, as a
// CharacterController's is but for its feet: a barrel on its end, a bowling
// pin.
func CapsuleShape(height, radius float32) BodyShape {
	height = max(height, 2*radius)
	return BodyShape{kind: device.ShapeCapsule, size: Vector3{X: 2 * radius, Y: height, Z: 2 * radius}, corners: ballCorners(radius, height/2-radius)}
}

// CylinderShape is a cylinder height meters tall and radius meters from its
// middle, flat at both ends, standing on Y and centered on its position, as
// NewCylinderMesh is scaled by golib.Vector3{X: 2 * radius, Y: height, Z:
// 2 * radius}: a barrel, a log, a coin on its side, a column.
func CylinderShape(height, radius float32) BodyShape {
	size := Vector3{X: 2 * radius, Y: height, Z: 2 * radius}
	return BodyShape{kind: device.ShapeCylinder, size: size, corners: meshCorners(NewCylinderMesh(), scaleMatrix(size), nil)}
}

// ModelShape is the shape around a model's meshes, at rest: what falls
// over and rolls as its model would, as long as the model has no holes or
// hollows that matter, which the shape fills in.
func ModelShape(model *Model) BodyShape {
	if model == nil {
		reportError(errors.New("golib: ModelShape got a nil model: make one with golib.NewModel"))
		return BoxShape(Vector3{X: 1, Y: 1, Z: 1})
	}
	model.mu.Lock()
	err := model.prepare()
	var corners []Vector3
	if err == nil {
		for _, part := range model.parts {
			if part.skin >= 0 {
				corners = append(corners, meshCorners(part.mesh, identityMatrix(), model.rest.joints[part.skin])...)
			} else {
				corners = append(corners, meshCorners(part.mesh, model.rest.globals[part.node], nil)...)
			}
		}
	}
	model.mu.Unlock()
	if err != nil {
		reportError(err)
		return BoxShape(Vector3{X: 1, Y: 1, Z: 1})
	}
	return hullShape(corners)
}

// MeshShape is the shape around a mesh, such as one made with NewMesh: a
// gem, a rock.
func MeshShape(mesh *Mesh) BodyShape {
	if mesh == nil {
		reportError(errors.New("golib: MeshShape got a nil mesh"))
		return BoxShape(Vector3{X: 1, Y: 1, Z: 1})
	}
	return hullShape(meshCorners(mesh, identityMatrix(), nil))
}

// hullShape is the convex shape around corners, a hull Jolt works out from
// its points, the corners each once.
func hullShape(corners []Vector3) BodyShape {
	seen := map[Vector3]bool{}
	var points []Vector3
	lo, hi := Vector3{}, Vector3{}
	for i, c := range corners {
		if i == 0 {
			lo, hi = c, c
		}
		lo, hi = lowest(lo, c), highest(hi, c)
		if !seen[c] {
			seen[c] = true
			points = append(points, c)
		}
	}
	return BodyShape{kind: device.ShapeHull, size: hi.Sub(lo), points: points, corners: corners}
}

func scaleMatrix(s Vector3) matrix {
	m := identityMatrix()
	m[0], m[5], m[10] = s.X, s.Y, s.Z
	return m
}

// ballCorners are the triangles of a coarse ball of radius, or a capsule
// whose halves are stretch apart: enough for rays and characters to meet it
// about where Jolt does.
func ballCorners(radius, stretch float32) []Vector3 {
	const slices, rings = 12, 8
	point := func(ring, slice int) Vector3 {
		polar := float64(ring) / rings * math.Pi
		around := float64(slice) / slices * 2 * math.Pi
		p := Vector3{
			X: float32(math.Sin(polar)*math.Sin(around)) * radius,
			Y: float32(math.Cos(polar)) * radius,
			Z: float32(math.Sin(polar)*math.Cos(around)) * radius,
		}
		if ring <= rings/2 {
			p.Y += stretch
		} else {
			p.Y -= stretch
		}
		return p
	}
	var out []Vector3
	for ring := 0; ring < rings; ring++ {
		for slice := 0; slice < slices; slice++ {
			a, b := point(ring, slice), point(ring+1, slice)
			c, d := point(ring+1, slice+1), point(ring, slice+1)
			out = append(out, a, b, c, a, c, d)
		}
	}
	// The middle of a capsule, between its halves.
	if stretch > 0 {
		for slice := 0; slice < slices; slice++ {
			a, b := point(rings/2, slice), point(rings/2+1, slice)
			c, d := point(rings/2+1, slice+1), point(rings/2, slice+1)
			out = append(out, a, b, c, a, c, d)
		}
	}
	return out
}

// RigidBodyOptions say what a rigid body is made of and how it moves. The
// zero value is a body of light wood that falls, slides and bounces a
// little, as most things in a game do.
type RigidBodyOptions struct {
	// Mass is how heavy it is, in kilograms: when zero, its size of a light
	// wood, 250 kilograms a cubic meter, so a meter-wide crate weighs 250.
	// Heavier things push lighter ones aside, and are harder to push.
	Mass float32

	// Friction is how it grips what it slides on, from 0, ice, to 1,
	// rubber: 0.5 when zero. NoFriction makes it ice.
	Friction   float32
	NoFriction bool

	// Bounce is how much of its speed it keeps as it hits something, from
	// 0, a sack, to 1, a rubber ball that bounces as high as it fell.
	Bounce float32

	// Moved makes a body the game moves itself, with MoveTo, which pushes
	// what is in its way and carries what stands on it, and nothing pushes
	// back: a piston, a door that swings, a paddle.
	Moved bool

	// Fast checks the whole way the body moves each update, so a small
	// fast thing, a bullet or a ball hit hard, doesn't pass through a thin
	// wall. It costs more: use it for the few that need it.
	Fast bool

	// NoGravity leaves it floating where it is pushed: a balloon, a thing
	// in space.
	NoGravity bool
}

// lightWood is the density of a rigid body whose options give no mass:
// lighter than Jolt's water, so the crates and barrels of a game are pushed
// about by its characters as players expect.
const lightWood = 250

// RigidBody is a body in a PhysicsWorld that moves on its own: AddRigidBody
// adds it, PhysicsWorld.Update moves it, and Transform says where it is
// now, to draw it there.
type RigidBody struct {
	world   *PhysicsWorld
	id      int // its body in the world, which RayHit.Body names
	jolt    uint32
	shape   BodyShape
	scale   Vector3 // its transform's, which Transform gives back
	mass    float32
	moved   bool
	removed bool
	state   [device.BodyStateFloats]float32 // as of the last Update
	moveTo  *Transform                      // where MoveTo asked it to be at the next Update
}

// AddRigidBody adds a body of shape, placed by transform, that moves as
// options say, and returns it. Its triangles join the world, so rays hit it
// and characters stand on it and push it.
//
//	crate := s.physics.AddRigidBody(golib.BoxShape(golib.Vector3{X: 1, Y: 1, Z: 1}), golib.Transform{Position: at}, golib.RigidBodyOptions{})
//
//	// In Update:
//	s.physics.Update(dt)
//
//	// In Draw:
//	screen.DrawMesh(s.box, crate.Transform(), golib.Brown)
func (w *PhysicsWorld) AddRigidBody(shape BodyShape, transform Transform, options RigidBodyOptions) *RigidBody {
	scale := onesFor(transform.Scale)
	b := &RigidBody{world: w, shape: shape, scale: transform.Scale, moved: options.Moved, jolt: device.NoBody}
	// Its triangles, scaled, placed where it is.
	corners := make([]Vector3, len(shape.corners))
	for i, c := range shape.corners {
		corners[i] = Vector3{X: c.X * scale.X, Y: c.Y * scale.Y, Z: c.Z * scale.Z}
	}
	place := Transform{Position: transform.Position, Rotation: transform.Rotation}
	b.id = w.next
	w.next++
	w.bodies[b.id] = &physicsBody{corners: corners, place: place.matrix(), transform: place, moving: true, rigid: b, joltID: device.NoBody}
	w.movingDirty = true
	w.rigid = append(w.rigid, b)

	size := Vector3{X: shape.size.X * scale.X, Y: shape.size.Y * scale.Y, Z: shape.size.Z * scale.Z}
	b.mass = options.Mass
	if b.mass <= 0 {
		b.mass = lightWood * volumeOf(shape.kind, size)
	}
	q := turnOf(place.Rotation)
	b.state = [device.BodyStateFloats]float32{transform.Position.X, transform.Position.Y, transform.Position.Z, q[0], q[1], q[2], q[3]}
	if !w.startJolt() {
		return b
	}
	d := make([]float32, device.BodyFloats, device.BodyFloats+3*len(shape.points))
	d[device.BodyShape] = float32(shape.kind)
	switch shape.kind {
	case device.ShapeBox:
		d[device.BodySizeX], d[device.BodySizeY], d[device.BodySizeZ] = size.X/2, size.Y/2, size.Z/2
	case device.ShapeSphere:
		d[device.BodySizeX] = max(size.X, size.Y, size.Z) / 2
	case device.ShapeCapsule:
		radius := max(size.X, size.Z) / 2
		d[device.BodySizeX], d[device.BodySizeY] = radius, max(0.001, size.Y/2-radius)
	case device.ShapeCylinder:
		d[device.BodySizeX], d[device.BodySizeY] = max(size.X, size.Z)/2, size.Y/2
	case device.ShapeHull:
		d[device.BodyPoints] = float32(len(shape.points))
		for _, p := range shape.points {
			d = append(d, p.X*scale.X, p.Y*scale.Y, p.Z*scale.Z)
		}
	}
	d[device.BodyPositionX], d[device.BodyPositionY], d[device.BodyPositionZ] = transform.Position.X, transform.Position.Y, transform.Position.Z
	d[device.BodyRotationX], d[device.BodyRotationY], d[device.BodyRotationZ], d[device.BodyRotationW] = q[0], q[1], q[2], q[3]
	d[device.BodyMotion] = device.MotionFree
	if options.Moved {
		d[device.BodyMotion] = device.MotionMoved
	}
	d[device.BodyMass] = b.mass
	d[device.BodyFriction] = 0.5
	if options.Friction > 0 {
		d[device.BodyFriction] = options.Friction
	}
	if options.NoFriction {
		d[device.BodyFriction] = 0
	}
	d[device.BodyRestitution] = Clamp(options.Bounce, 0, 1)
	d[device.BodyLinearDamping], d[device.BodyAngularDamping] = 0.05, 0.05
	d[device.BodyGravity] = 1
	if options.NoGravity {
		d[device.BodyGravity] = 0
	}
	if options.Fast {
		d[device.BodyFast] = 1
	}
	b.jolt = device.AddPhysicsBody(w.jolt, d)
	if b.jolt == device.NoBody {
		reportError(errors.New("golib: AddRigidBody: Jolt couldn't make the shape: it has no size, or a ModelShape or MeshShape of a flat thing"))
	}
	return b
}

// volumeOf returns how many cubic meters a shape of kind and size holds.
func volumeOf(kind int, size Vector3) float32 {
	switch kind {
	case device.ShapeSphere:
		r := max(size.X, size.Y, size.Z) / 2
		return 4.0 / 3 * math.Pi * r * r * r
	case device.ShapeCapsule:
		r := max(size.X, size.Z) / 2
		return math.Pi*r*r*max(0, size.Y-2*r) + 4.0/3*math.Pi*r*r*r
	case device.ShapeCylinder:
		r := max(size.X, size.Z) / 2
		return math.Pi * r * r * size.Y
	case device.ShapeHull:
		return size.X * size.Y * size.Z * 0.6 // about what a rock fills of its box
	}
	return size.X * size.Y * size.Z
}

func onesFor(v Vector3) Vector3 {
	for _, part := range []*float32{&v.X, &v.Y, &v.Z} {
		if *part == 0 {
			*part = 1
		}
	}
	return v
}

// turnOf returns a rotation as a quaternion x, y, z, w.
func turnOf(r Rotation) quaternion {
	m := r.matrix()
	m00, m11, m22 := float64(m[0]), float64(m[5]), float64(m[10])
	m21, m12 := float64(m[6]), float64(m[9])
	m02, m20 := float64(m[8]), float64(m[2])
	m10, m01 := float64(m[1]), float64(m[4])
	var x, y, z, w float64
	switch trace := m00 + m11 + m22; {
	case trace > 0:
		k := 0.5 / math.Sqrt(trace+1)
		w, x, y, z = 0.25/k, (m21-m12)*k, (m02-m20)*k, (m10-m01)*k
	case m00 > m11 && m00 > m22:
		k := 2 * math.Sqrt(1+m00-m11-m22)
		w, x, y, z = (m21-m12)/k, 0.25*k, (m01+m10)/k, (m02+m20)/k
	case m11 > m22:
		k := 2 * math.Sqrt(1+m11-m00-m22)
		w, x, y, z = (m02-m20)/k, (m01+m10)/k, 0.25*k, (m12+m21)/k
	default:
		k := 2 * math.Sqrt(1+m22-m00-m11)
		w, x, y, z = (m10-m01)/k, (m02+m20)/k, (m12+m21)/k, 0.25*k
	}
	return quaternion{float32(x), float32(y), float32(z), float32(w)}.normalize()
}

// startJolt makes the world's Jolt world, the first time a rigid body is
// added, with a copy of everything still and moved already in the world,
// and reports whether it is there.
func (w *PhysicsWorld) startJolt() bool {
	if w.jolt.ID != 0 {
		return true
	}
	if w.joltFailed {
		return false
	}
	freeOrphanedPhysics()
	p, err := device.NewPhysics(65536)
	if err != nil {
		w.joltFailed = true
		reportError(fmt.Errorf("golib: PhysicsWorld.AddRigidBody: %w", err))
		return false
	}
	w.jolt = p
	joltWorlds.Lock()
	if joltWorlds.live == nil {
		joltWorlds.live = map[uintptr]weak.Pointer[PhysicsWorld]{}
	}
	joltWorlds.live[p.ID] = weak.Make(w)
	joltWorlds.Unlock()
	runtime.AddCleanup(w, orphanPhysics, p)
	for _, id := range w.sortedIDs() {
		if b := w.bodies[id]; b.rigid == nil {
			w.mirror(b)
		}
	}
	return true
}

func (w *PhysicsWorld) sortedIDs() []int {
	ids := make([]int, 0, len(w.bodies))
	for id := range w.bodies {
		ids = append(ids, id)
	}
	for i := 1; i < len(ids); i++ {
		for k := i; k > 0 && ids[k] < ids[k-1]; k-- {
			ids[k], ids[k-1] = ids[k-1], ids[k]
		}
	}
	return ids
}

// mirror gives Jolt a still or moved body of the world, once Jolt is
// there, so rigid bodies meet it.
func (w *PhysicsWorld) mirror(b *physicsBody) {
	if w.jolt.ID == 0 || b.rigid != nil || b.joltID != device.NoBody || len(b.corners) < 3 {
		return
	}
	scale := onesFor(b.transform.Scale)
	corners := make([]float32, 0, len(b.corners)*3)
	for _, c := range b.corners {
		corners = append(corners, c.X*scale.X, c.Y*scale.Y, c.Z*scale.Z)
	}
	q := turnOf(b.transform.Rotation)
	p := b.transform.Position
	b.joltID = device.AddPhysicsMesh(w.jolt, corners, 0.6, 0, [3]float32{p.X, p.Y, p.Z}, [4]float32(q), b.moving)
	b.joltMoved = b.moving
}

// Update moves the rigid bodies on by dt seconds, as Scene.Update gets it,
// pushed by what the game moved since: call it in every Update, after
// moving lifts and before moving characters. A world without rigid bodies
// needs none.
func (w *PhysicsWorld) Update(dt float32) {
	freeOrphanedPhysics()
	if w.jolt.ID == 0 || dt <= 0 {
		return
	}
	gravity := w.Gravity
	if gravity == (Vector3{}) {
		gravity = Vector3{Y: -9.81}
	}
	if gravity != w.gravity {
		w.gravity = gravity
		device.SetPhysicsGravity(w.jolt, [3]float32{gravity.X, gravity.Y, gravity.Z})
	}
	// What the game moved, moved in Jolt over the update, so it pushes.
	for _, id := range w.sortedIDs() {
		b := w.bodies[id]
		if b.rigid == nil && b.joltTo && b.joltID != device.NoBody {
			b.joltTo = false
			q := turnOf(b.transform.Rotation)
			p := b.transform.Position
			device.SetPhysicsBody(w.jolt, b.joltID, device.SetMoveTo, []float32{p.X, p.Y, p.Z, q[0], q[1], q[2], q[3], dt})
		}
	}
	// The characters, where they moved; a character that didn't move since
	// the last Update has gone from the game, or stands still for now: its
	// capsule goes, and comes back at its next Move.
	kept := w.characters[:0]
	for _, j := range w.characters {
		if j.moved < w.updates {
			device.RemovePhysicsBody(w.jolt, j.id)
			continue
		}
		device.SetPhysicsBody(w.jolt, j.id, device.SetMoveTo, []float32{j.to.X, j.to.Y, j.to.Z, 0, 0, 0, 1, dt})
		kept = append(kept, j)
	}
	clear(w.characters[len(kept):])
	w.characters = kept
	w.updates++
	for _, r := range w.rigid {
		if r.moveTo != nil && r.jolt != device.NoBody {
			q := turnOf(r.moveTo.Rotation)
			p := r.moveTo.Position
			device.SetPhysicsBody(w.jolt, r.jolt, device.SetMoveTo, []float32{p.X, p.Y, p.Z, q[0], q[1], q[2], q[3], dt})
			r.moveTo = nil
		}
	}
	device.StepPhysics(w.jolt, dt, 1)

	// Where every body is now, read at once, and their triangles moved.
	var ids []uint32
	var bodies []*RigidBody
	for _, r := range w.rigid {
		if r.jolt != device.NoBody {
			ids = append(ids, r.jolt)
			bodies = append(bodies, r)
		}
	}
	if len(ids) == 0 {
		return
	}
	states := make([]float32, len(ids)*device.BodyStateFloats)
	device.ReadPhysicsBodies(w.jolt, ids, states)
	for i, r := range bodies {
		copy(r.state[:], states[i*device.BodyStateFloats:])
		b := w.bodies[r.id]
		place := nodePose{translation: r.Position(), rotation: quaternion(r.state[3:7]), scale: Vector3{X: 1, Y: 1, Z: 1}}.matrix()
		if place == b.place {
			continue // at rest: its triangles stay as they are
		}
		b.moveTo(place)
		b.moves++
	}
}

// joltCharacter is a CharacterController as Jolt sees it: a kinematic
// capsule a tenth thinner than the character, so rigid bodies land on its
// head and bounce off its legs instead of passing into it, while what it
// walks into is pushed as heavy as a person, by push, and no harder.
type joltCharacter struct {
	of             *CharacterController
	id             uint32
	height, radius float32
	to             Vector3 // its middle, where Move left it
	moved          int     // the Update it last moved after
}

// placeCharacter moves c's capsule in Jolt to where c is, by the next
// Update, making it the first time.
func (w *PhysicsWorld) placeCharacter(c *CharacterController, height, radius float32) {
	if w.jolt.ID == 0 {
		return
	}
	var j *joltCharacter
	for _, k := range w.characters {
		if k.of == c {
			j = k
		}
	}
	if j != nil && (j.height != height || j.radius != radius) {
		j.moved = -1 // Update takes it out
		j = nil
	}
	middle := c.Position.Add(Vector3{Y: height / 2})
	if j == nil {
		d := make([]float32, device.BodyFloats)
		d[device.BodyShape] = device.ShapeCapsule
		d[device.BodySizeX], d[device.BodySizeY] = radius*0.9, max(0.001, height/2-radius)
		d[device.BodyPositionX], d[device.BodyPositionY], d[device.BodyPositionZ] = middle.X, middle.Y, middle.Z
		d[device.BodyRotationW] = 1
		d[device.BodyMotion] = device.MotionMoved
		d[device.BodyMass] = characterPush
		d[device.BodyFriction] = 0.5
		id := device.AddPhysicsBody(w.jolt, d)
		if id == device.NoBody {
			return
		}
		j = &joltCharacter{of: c, id: id, height: height, radius: radius}
		w.characters = append(w.characters, j)
	}
	j.to, j.moved = middle, w.updates
}

// joltWorlds are the Jolt worlds made and not yet freed, for Run to free
// when it ends, and the orphaned ones, whose PhysicsWorld the game keeps no
// longer, as when a scene ends: the next Update or AddRigidBody frees them,
// on the game's goroutine, so a game that makes a world each level doesn't
// keep every one.
var joltWorlds struct {
	sync.Mutex
	live     map[uintptr]weak.Pointer[PhysicsWorld]
	orphaned []device.Physics
}

// orphanPhysics is called once the PhysicsWorld that made p is gone.
func orphanPhysics(p device.Physics) {
	joltWorlds.Lock()
	defer joltWorlds.Unlock()
	if _, live := joltWorlds.live[p.ID]; live {
		delete(joltWorlds.live, p.ID)
		joltWorlds.orphaned = append(joltWorlds.orphaned, p)
	}
}

// freeOrphanedPhysics frees the Jolt worlds no PhysicsWorld uses any more.
func freeOrphanedPhysics() {
	joltWorlds.Lock()
	orphaned := joltWorlds.orphaned
	joltWorlds.orphaned = nil
	joltWorlds.Unlock()
	for _, p := range orphaned {
		device.FreePhysics(p)
	}
}

// freePhysicsWorlds frees every Jolt world, when Run ends.
func freePhysicsWorlds() {
	freeOrphanedPhysics()
	joltWorlds.Lock()
	live := joltWorlds.live
	joltWorlds.live = nil
	joltWorlds.Unlock()
	for id, pointer := range live {
		if w := pointer.Value(); w != nil {
			w.jolt = device.Physics{}
			for _, b := range w.bodies {
				b.joltID = device.NoBody
			}
			for _, r := range w.rigid {
				r.jolt = device.NoBody
			}
			w.characters = nil
		}
		device.FreePhysics(device.Physics{ID: id})
	}
}

// ID returns the body's number in its world: what RayHit.Body and
// CharacterController.GroundBody say when they meet it.
func (b *RigidBody) ID() int { return b.id }

// Position returns where the body's middle is now.
func (b *RigidBody) Position() Vector3 {
	return Vector3{X: b.state[0], Y: b.state[1], Z: b.state[2]}
}

// Transform returns where the body is and how it is turned now, with the
// Scale it was added with: draw its mesh or model with it.
func (b *RigidBody) Transform() Transform {
	turn := nodePose{rotation: quaternion(b.state[3:7]), scale: Vector3{X: 1, Y: 1, Z: 1}}.matrix()
	return Transform{Position: b.Position(), Rotation: turn.rotation(), Scale: b.scale}
}

// Velocity returns how fast it moves, in meters per second, each way.
func (b *RigidBody) Velocity() Vector3 {
	return Vector3{X: b.state[7], Y: b.state[8], Z: b.state[9]}
}

// Resting says whether it has come to rest, so Jolt has stopped working it
// out until something touches it: a crate that has landed.
func (b *RigidBody) Resting() bool {
	return b.state[13] == 0
}

// Mass returns how heavy it is, in kilograms.
func (b *RigidBody) Mass() float32 { return b.mass }

// SetVelocity sets how fast it moves, at once: a ball thrown, a crate
// stopped.
func (b *RigidBody) SetVelocity(velocity Vector3) {
	if b.jolt != device.NoBody && b.world.jolt.ID != 0 {
		device.SetPhysicsBody(b.world.jolt, b.jolt, device.SetVelocity, []float32{velocity.X, velocity.Y, velocity.Z, b.state[10], b.state[11], b.state[12]})
	}
	b.state[7], b.state[8], b.state[9] = velocity.X, velocity.Y, velocity.Z
}

// Push gives it a blow through its middle, in kilograms times meters per
// second: its velocity changes by impulse divided by its Mass, so the same
// kick sends a ball flying and nudges a car. An explosion pushes everything
// near it away from where it is.
func (b *RigidBody) Push(impulse Vector3) {
	b.PushAt(impulse, b.Position())
}

// PushAt gives it a blow at point, a point of the world on or in it: off
// its middle, it spins too, as a crate hit at a corner does.
func (b *RigidBody) PushAt(impulse, point Vector3) {
	if b.jolt != device.NoBody && b.world.jolt.ID != 0 {
		device.SetPhysicsBody(b.world.jolt, b.jolt, device.SetImpulse, []float32{impulse.X, impulse.Y, impulse.Z, point.X, point.Y, point.Z})
	}
}

// Teleport puts it at transform's position and rotation at once, still
// moving as it moved: a respawned ball, a crate put back.
func (b *RigidBody) Teleport(transform Transform) {
	q := turnOf(transform.Rotation)
	p := transform.Position
	if b.jolt != device.NoBody && b.world.jolt.ID != 0 {
		device.SetPhysicsBody(b.world.jolt, b.jolt, device.SetPlace, []float32{p.X, p.Y, p.Z, q[0], q[1], q[2], q[3]})
	}
	copy(b.state[:7], []float32{p.X, p.Y, p.Z, q[0], q[1], q[2], q[3]})
	if pb := b.world.bodies[b.id]; pb != nil {
		pb.moveTo(Transform{Position: p, Rotation: transform.Rotation}.matrix())
	}
}

// MoveTo moves a body added with RigidBodyOptions.Moved to transform's
// position and rotation by the next Update, pushing what is in its way.
// Call it in every Update the body moves.
func (b *RigidBody) MoveTo(transform Transform) {
	if !b.moved {
		reportError(errors.New("golib: RigidBody.MoveTo moves a body added with RigidBodyOptions{Moved: true}; push the others with Push or SetVelocity"))
		return
	}
	b.moveTo = &transform
}

// Remove takes the body out of its world: a crate broken, a ball lost.
func (b *RigidBody) Remove() {
	if b.removed {
		return
	}
	b.removed = true
	w := b.world
	if b.jolt != device.NoBody && w.jolt.ID != 0 {
		device.RemovePhysicsBody(w.jolt, b.jolt)
	}
	b.jolt = device.NoBody
	delete(w.bodies, b.id)
	for i, r := range w.rigid {
		if r == b {
			w.rigid = append(w.rigid[:i], w.rigid[i+1:]...)
			break
		}
	}
	w.movingDirty = true
}
