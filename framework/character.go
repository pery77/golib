package golib

import (
	"errors"
	"math"
	"slices"
)

// CharacterController moves a character through a PhysicsWorld as a
// capsule standing on its feet: it slides along walls, walks up slopes up
// to MaxSlope and steps up ledges up to StepHeight, stands on the ground and
// stops under a ceiling. The game keeps its velocity and its gravity, and
// moves it by velocity times dt in every Update:
//
//	p.body = &golib.CharacterController{Position: start, Height: 1.8, Radius: 0.3}
//
//	// In Update:
//	p.velocity.Y -= gravity * dt
//	if p.body.OnGround && input.KeyPressed(golib.KeySpace) {
//		p.velocity.Y = jumpSpeed
//	}
//	p.body.Move(s.physics, p.velocity.Scale(dt))
//	if p.body.OnGround || p.body.OnCeiling {
//		p.velocity.Y = 0
//	}
//
// Draw the character's model at Position: its feet.
type CharacterController struct {
	// Position is where the character's feet are: the bottom of the
	// capsule.
	Position Vector3

	// Height is the capsule's height, 1.8 meters when zero, and Radius how
	// far it reaches from its middle, 0.3 meters when zero.
	Height, Radius float32

	// StepHeight is the tallest ledge it steps up onto as it walks, 0.3
	// meters when zero. MaxSlope is the steepest ground it stands and walks
	// up on, in degrees, 45 when zero; steeper ground is a wall it slides
	// down.
	StepHeight, MaxSlope float32

	// OnGround says whether the last Move ended standing on the ground,
	// GroundNormal the way that ground faces, OnCeiling whether it bumped
	// its head, and OnWall whether a wall stopped it.
	OnGround     bool
	GroundNormal Vector3
	OnCeiling    bool
	OnWall       bool

	// GroundBody is the number of what it stands on, as the PhysicsWorld's
	// Add method returned it, or 0. A body moved with SetTransform carries
	// what stands on it: a lift, a raft.
	GroundBody int

	// Moved is how far the last Move took it, ground carrying it included:
	// divided by dt, the velocity it really had, for Camera3D.Follow.
	Moved Vector3

	groundMoves int // how many times GroundBody had moved, as last seen
}

func (c *CharacterController) sizes() (height, radius, step, minGround float32) {
	height, radius, step = c.Height, c.Radius, c.StepHeight
	if height <= 0 {
		height = 1.8
	}
	if radius <= 0 {
		radius = 0.3
	}
	radius = min(radius, height/2)
	if step <= 0 {
		step = 0.3
	}
	slope := c.MaxSlope
	if slope <= 0 {
		slope = 45
	}
	return height, radius, step, float32(math.Cos(float64(min(slope, 89)) * math.Pi / 180))
}

// Move moves the character by motion through world: velocity times dt, in
// meters. It goes as far as the world lets it, sliding along what is in the
// way, and sets OnGround, GroundNormal, OnCeiling and OnWall.
func (c *CharacterController) Move(world *PhysicsWorld, motion Vector3) {
	if world == nil {
		reportError(errors.New("golib: CharacterController.Move got a nil PhysicsWorld: make one with golib.NewPhysicsWorld"))
		return
	}
	height, radius, step, minGround := c.sizes()
	wasOnGround := c.OnGround
	before := c.Position
	// Standing on something the game moved since: carried with it.
	if wasOnGround && c.GroundBody != 0 {
		if b := world.bodies[c.GroundBody]; b != nil && b.moves != c.groundMoves {
			c.Position = b.place.mul(b.previous.inverse()).transformPoint(c.Position)
		}
	}
	start := c.Position
	moved := c.sweep(world, start, motion, height, radius, step, minGround)

	// Blocked by a wall while walking: try again a step higher, and keep
	// that if it gets further.
	across := Vector3{X: motion.X, Z: motion.Z}
	if moved.wall && wasOnGround && across.Length() > 1e-6 {
		up := c.sweep(world, start, Vector3{Y: step}, height, radius, step, minGround)
		over := c.sweep(world, up.position, across, height, radius, step, minGround)
		down := c.sweep(world, over.position, Vector3{Y: -step - 0.01 + min(0, motion.Y)}, height, radius, step, minGround)
		if down.ground && flat(down.position.Sub(start)) > flat(moved.position.Sub(start))+1e-4 {
			moved = down
			moved.wall = over.wall
		}
	}

	// Walking down a slope or off a step: keep to the ground rather than
	// leaving it for a fall of a few centimeters.
	if wasOnGround && !moved.ground && motion.Y <= 0 {
		down := c.sweep(world, moved.position, Vector3{Y: -step}, height, radius, step, minGround)
		if down.ground {
			moved.position, moved.ground, moved.groundNormal, moved.groundBody = down.position, true, down.groundNormal, down.groundBody
		}
	}

	c.Position = moved.position
	c.Moved = c.Position.Sub(before)
	c.push(world, moved.pushed, motion)
	world.placeCharacter(c, height, radius)
	c.OnGround, c.GroundNormal = moved.ground, moved.groundNormal
	c.OnCeiling, c.OnWall = moved.ceiling, moved.wall
	c.GroundBody = 0
	if c.OnGround {
		c.GroundBody = moved.groundBody
		if b := world.bodies[c.GroundBody]; b != nil {
			c.groundMoves = b.moves
		}
	} else {
		c.GroundNormal = Vector3{}
	}
}

// characterPush is how heavy a character pushes as, in kilograms: what it
// walks into moves at its pace when lighter than this, and slower the
// heavier it is. characterForce is the hardest it pushes, in newtons: a
// meter-wide crate of light wood slides briskly, two stacked slowly, three
// not at all.
const (
	characterPush  = 80
	characterForce = 3000
)

// fixedStep is how long an update lasts, which turns a Move's motion into a
// speed.
const fixedStep = 1.0 / updatesPerSecond

// push shoves the rigid bodies the character walked into, across the
// ground, towards the speed it walks at.
func (c *CharacterController) push(world *PhysicsWorld, pushed map[int]Vector3, motion Vector3) {
	for id, away := range pushed {
		b := world.bodies[id]
		if b == nil || b.rigid == nil {
			continue
		}
		away = Vector3{X: away.X, Z: away.Z}.Normalize()
		speed := Vector3{X: motion.X, Z: motion.Z}.Dot(away) / fixedStep
		if speed <= 0 {
			continue
		}
		r := b.rigid
		short := speed - r.Velocity().Dot(away)
		if short <= 0 {
			continue
		}
		// Its strength shared among what it pushes at once.
		r.Push(away.Scale(min(short*min(r.mass, characterPush), characterForce*fixedStep/float32(len(pushed)))))
	}
}

func flat(v Vector3) float32 {
	return Vector3{X: v.X, Z: v.Z}.Length()
}

// moveResult is where a sweep ended, and what it touched.
type moveResult struct {
	position              Vector3
	ground, ceiling, wall bool
	groundNormal          Vector3
	groundBody            int
	pushed                map[int]Vector3 // rigid bodies it walked into, and which way
	motion                Vector3         // the sweep's last step
}

// sweep moves a capsule from position by motion, a little at a time so it
// doesn't pass through thin walls, pushing it out of whatever it overlaps
// after each step.
func (c *CharacterController) sweep(world *PhysicsWorld, position, motion Vector3, height, radius, step, minGround float32) moveResult {
	r := moveResult{position: position}
	length := motion.Length()
	steps := max(1, int(math.Ceil(float64(length/(radius*0.5)))))
	part := motion.Scale(1 / float32(steps))
	r.motion = part
	for range steps {
		r.position = r.position.Add(part)
		c.resolve(world, &r, height, radius, step, minGround)
	}
	return r
}

// resolve pushes the capsule at r.position out of the triangles it
// overlaps: up off ground it can stand on, as far as it sinks into it and
// straight up, so it doesn't slide down a slope or off a ledge's edge; and
// away from walls and ceilings. Ground goes first, so the edge of a step is
// stood on rather than pushed away from.
func (c *CharacterController) resolve(world *PhysicsWorld, r *moveResult, height, radius, step, minGround float32) {
	var touching []*physicsTriangle
	for range 4 {
		bottom := r.position.Add(Vector3{Y: radius})
		top := r.position.Add(Vector3{Y: height - radius})
		pad := Vector3{X: radius, Y: radius, Z: radius}
		touching = touching[:0]
		world.near(lowest(bottom, top).Sub(pad), highest(bottom, top).Add(pad), func(t *physicsTriangle) {
			touching = append(touching, t)
		})
		slices.SortStableFunc(touching, func(a, b *physicsTriangle) int {
			return int(boolInt(b.normal.Y >= minGround)) - int(boolInt(a.normal.Y >= minGround))
		})
		pushed := false
		for _, t := range touching {
			bottom := r.position.Add(Vector3{Y: radius})
			top := r.position.Add(Vector3{Y: height - radius})
			onSegment, onTriangle := closestSegmentTriangle(bottom, top, t)
			away := onSegment.Sub(onTriangle)
			distance := away.Length()
			if distance >= radius-1e-5 {
				continue
			}
			n := t.normal
			if distance > 1e-6 {
				n = away.Scale(1 / distance)
			} else if n.Dot(onSegment.Sub(r.position.Add(Vector3{Y: height / 2}))) > 0 {
				n = n.Scale(-1) // the triangle runs through the capsule's middle: out the way it faces
			}
			depth := radius - distance
			// Ground higher above the feet than a step is a ledge: a wall.
			low := onTriangle.Y-r.position.Y <= step+1e-3
			switch {
			case low && t.normal.Y >= minGround && onTriangle.Y <= bottom.Y+1e-4:
				// Raised until the bottom of the capsule just touches it.
				across := Vector3{X: onSegment.X - onTriangle.X, Z: onSegment.Z - onTriangle.Z}.Length()
				lift := onTriangle.Y + float32(math.Sqrt(float64(max(0, radius*radius-across*across)))) - onSegment.Y
				r.position.Y += max(0, lift)
				r.ground = true
				r.groundNormal, r.groundBody = t.normal, t.body
			case low && n.Y >= minGround:
				r.position.Y += depth / n.Y
				r.ground = true
				r.groundNormal, r.groundBody = n, t.body
			case n.Y <= -0.7:
				// A rigid body on its head rests on the character's capsule
				// in Jolt, a little inside this one: not a ceiling to be
				// pushed down by, into the ground.
				if b := world.bodies[t.body]; b == nil || b.rigid == nil || b.rigid.moved {
					r.position = r.position.Add(n.Scale(depth))
				}
				r.ceiling = true
			default:
				if b := world.bodies[t.body]; b != nil && b.rigid != nil && !b.rigid.moved {
					// A rigid body that came to the character leans on its
					// capsule in Jolt, a little inside this one: only
					// walking into one steps the character back from it.
					if (Vector3{X: r.motion.X, Z: r.motion.Z}).Dot(Vector3{X: n.X, Z: n.Z}) >= 0 {
						continue
					}
					if r.pushed == nil {
						r.pushed = map[int]Vector3{}
					}
					r.pushed[t.body] = n.Scale(-1)
				}
				// Pushed back across the ground, not up a ledge's edge.
				if flat := (Vector3{X: n.X, Z: n.Z}); n.Y > 0 && flat.Length() > 1e-3 {
					n = flat.Normalize()
				}
				r.position = r.position.Add(n.Scale(depth))
				r.wall = true
			}
			pushed = true
		}
		if !pushed {
			return
		}
	}
}
