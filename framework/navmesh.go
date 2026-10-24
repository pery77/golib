package golib

import (
	"container/heap"
	"errors"
	"math"
)

// NavMesh is where characters of one size can walk in a PhysicsWorld, for
// finding their way across it: an enemy that goes round walls to reach the
// player, a pet that follows. Make it once the world's solid things are
// added, for characters sized as a CharacterController:
//
//	s.nav = golib.NewNavMesh(s.physics, golib.CharacterController{Height: 1.8, Radius: 0.4})
//
//	// In Update, every so often, not every update:
//	if path, ok := s.nav.Path(enemy.body.Position, s.player.body.Position); ok {
//		enemy.path = path // walk to path[1], then the next...
//	}
//
// It is a grid of places, Radius meters apart, on ground no steeper than
// MaxSlope with room above for Height, joined where a character steps from
// one to the next: up or down no more than StepHeight, nothing solid
// between. Ground above ground, such as a bridge over a path, is walked on
// both. Make it again after the world changes where characters can go.
type NavMesh struct {
	world        *PhysicsWorld
	cell         float32
	origin       Vector3 // the corner of the grid, at the world's lowest
	across, deep int     // cells along X and Z
	places       []navPlace
	columns      map[int][]int32 // the places of each column, by x + z*across
	height       float32
	radius       float32
	step         float32
}

// navPlace is one place a character can stand.
type navPlace struct {
	position  Vector3
	neighbors []int32
}

// maxNavPlaces keeps a grid over a large world from taking too long to
// make: its cells grow until there are no more columns than this.
const maxNavPlaces = 1 << 20

// NewNavMesh finds where characters sized as character can walk in world:
// its Height, Radius, StepHeight and MaxSlope, with the same defaults. Its
// Position is ignored. The world's whole extent is searched.
func NewNavMesh(world *PhysicsWorld, character CharacterController) *NavMesh {
	if world == nil {
		reportError(errors.New("golib: NewNavMesh got a nil PhysicsWorld: make one with golib.NewPhysicsWorld"))
		return &NavMesh{}
	}
	height, radius, step, minGround := character.sizes()
	n := &NavMesh{world: world, cell: radius, height: height, radius: radius, step: step, columns: map[int][]int32{}}
	lo, hi, ok := world.bounds()
	if !ok {
		return n
	}
	for float64(hi.X-lo.X)*float64(hi.Z-lo.Z)/float64(n.cell*n.cell) > maxNavPlaces {
		n.cell *= 1.5
	}
	n.origin = Vector3{X: lo.X, Y: lo.Y, Z: lo.Z}
	n.across = int((hi.X-lo.X)/n.cell) + 1
	n.deep = int((hi.Z-lo.Z)/n.cell) + 1

	// Each column, from the top down: every ground a character fits on.
	for z := 0; z < n.deep; z++ {
		for x := 0; x < n.across; x++ {
			top := hi.Y + 1
			for {
				from := Vector3{X: n.origin.X + (float32(x)+0.5)*n.cell, Y: top, Z: n.origin.Z + (float32(z)+0.5)*n.cell}
				hit, ok := world.Raycast(from, Vector3{Y: -1}, top-lo.Y+1)
				if !ok {
					break
				}
				if hit.Normal.Y >= minGround && !n.blocked(world, hit.Position) {
					n.columns[x+z*n.across] = append(n.columns[x+z*n.across], int32(len(n.places)))
					n.places = append(n.places, navPlace{position: hit.Position})
				}
				top = hit.Position.Y - 0.05
			}
		}
	}

	// Each place joined to those around it a character steps to.
	for x := 0; x < n.across; x++ {
		for z := 0; z < n.deep; z++ {
			for _, i := range n.columns[x+z*n.across] {
				for dz := -1; dz <= 1; dz++ {
					for dx := -1; dx <= 1; dx++ {
						nx, nz := x+dx, z+dz
						if (dx == 0 && dz == 0) || nx < 0 || nz < 0 || nx >= n.across || nz >= n.deep {
							continue
						}
						for _, j := range n.columns[nx+nz*n.across] {
							if n.steps(world, n.places[i].position, n.places[j].position) {
								n.places[i].neighbors = append(n.places[i].neighbors, j)
							}
						}
					}
				}
			}
		}
	}
	return n
}

// blocked says whether a character standing at feet would be inside
// something solid.
func (n *NavMesh) blocked(world *PhysicsWorld, feet Vector3) bool {
	// Raised off the ground a little, so the ground itself doesn't count.
	bottom := feet.Add(Vector3{Y: n.radius + 0.05})
	top := feet.Add(Vector3{Y: max(n.height-n.radius, n.radius+0.05)})
	return world.touches(bottom, top, n.radius*0.9)
}

// steps says whether a character steps from a to b, beside it: no higher
// or lower than StepHeight, and nothing solid between them at its knees.
func (n *NavMesh) steps(world *PhysicsWorld, a, b Vector3) bool {
	if float32(math.Abs(float64(b.Y-a.Y))) > n.step {
		return false
	}
	// Each place has room for the character's capsule all round, so
	// between two places nearer than that room, nothing can stand.
	if n.cell*float32(math.Sqrt2)/2 < n.radius*0.9 {
		return true
	}
	return n.clear(world, a, b)
}

// clear says whether nothing solid stands between a and b, at a character's
// knees and its chest.
func (n *NavMesh) clear(world *PhysicsWorld, a, b Vector3) bool {
	for _, up := range []float32{n.step + 0.05, n.height * 0.75} {
		from, to := a.Add(Vector3{Y: up}), b.Add(Vector3{Y: up})
		if d := to.Sub(from).Length(); d > 1e-4 {
			if _, hit := world.SphereCast(from, to.Sub(from), n.radius*0.5, d); hit {
				return false
			}
		}
	}
	return true
}

// touches says whether anything solid is within radius of the segment from
// bottom to top.
func (w *PhysicsWorld) touches(bottom, top Vector3, radius float32) bool {
	pad := Vector3{X: radius, Y: radius, Z: radius}
	found := false
	w.near(lowest(bottom, top).Sub(pad), highest(bottom, top).Add(pad), func(t *physicsTriangle) {
		if found {
			return
		}
		s, p := closestSegmentTriangle(bottom, top, t)
		found = s.Distance(p) < radius
	})
	return found
}

// nearest returns the place nearest p that a character there would stand
// on: in its column or around it, on ground below it or a step above.
func (n *NavMesh) nearest(p Vector3) (int32, bool) {
	if len(n.places) == 0 {
		return 0, false
	}
	cx := int((p.X - n.origin.X) / n.cell)
	cz := int((p.Z - n.origin.Z) / n.cell)
	best, bestDistance := int32(-1), float32(math.MaxFloat32)
	for r := 0; r <= 3 && best < 0; r++ {
		for z := cz - r; z <= cz+r; z++ {
			for x := cx - r; x <= cx+r; x++ {
				if x < 0 || z < 0 || x >= n.across || z >= n.deep {
					continue
				}
				for _, i := range n.columns[x+z*n.across] {
					q := n.places[i].position
					if q.Y > p.Y+n.step+0.01 {
						continue // ground above it
					}
					// Height counts more than distance across: the floor
					// it stands on, not the one below.
					d := Vector3{X: q.X - p.X, Z: q.Z - p.Z}.Length() + 4*float32(math.Abs(float64(q.Y-p.Y)))
					if d < bestDistance {
						best, bestDistance = i, d
					}
				}
			}
		}
	}
	return best, best >= 0
}

// Path returns the way a character walks from from to to, as the points it
// walks through in turn, on the ground: the first is from, the last is to,
// and between them it walks straight. When to is off the ground, in the air
// or on something the character can't stand on, the path ends on the
// ground nearest below it. It is false when there is no way from from to
// to, and the points are then nil.
func (n *NavMesh) Path(from, to Vector3) ([]Vector3, bool) {
	start, ok := n.nearest(from)
	if !ok {
		return nil, false
	}
	goal, ok := n.nearest(to)
	if !ok {
		return nil, false
	}
	// A* over the places, by distance walked.
	cost := map[int32]float32{start: 0}
	came := map[int32]int32{}
	open := &navQueue{{place: start, estimate: n.places[start].position.Distance(n.places[goal].position)}}
	done := map[int32]bool{}
	for open.Len() > 0 {
		at := heap.Pop(open).(navEntry).place
		if at == goal {
			break
		}
		if done[at] {
			continue
		}
		done[at] = true
		for _, next := range n.places[at].neighbors {
			c := cost[at] + n.places[at].position.Distance(n.places[next].position)
			if old, seen := cost[next]; seen && old <= c {
				continue
			}
			cost[next] = c
			came[next] = at
			heap.Push(open, navEntry{place: next, estimate: c + n.places[next].position.Distance(n.places[goal].position)})
		}
	}
	if _, reached := cost[goal]; !reached {
		return nil, false
	}
	var places []Vector3
	for at := goal; ; at = came[at] {
		places = append(places, n.places[at].position)
		if at == start {
			break
		}
	}
	for i, j := 0, len(places)-1; i < j; i, j = i+1, j-1 {
		places[i], places[j] = places[j], places[i]
	}
	// The ends are from and to themselves when they stand there; one in the
	// air, such as a jumping player, gives the ground under it.
	if float32(math.Abs(float64(from.Y-places[0].Y))) <= n.step {
		places[0] = from
	}
	if float32(math.Abs(float64(to.Y-places[len(places)-1].Y))) <= n.step {
		places[len(places)-1] = to
	}
	return n.straighten(places), true
}

// straighten leaves out the points of a path a character can walk past in a
// straight line: the grid's zigzags.
func (n *NavMesh) straighten(points []Vector3) []Vector3 {
	out := []Vector3{points[0]}
	for i := 0; i < len(points)-1; {
		k := i + 1
		for k+1 < len(points) && n.walkable(points[i], points[k+1]) {
			k++
		}
		out = append(out, points[k])
		i = k
	}
	return out
}

// walkable says whether a character walks straight from a to b: nothing in
// the way, and ground all along, never more than a step from the line
// between them.
func (n *NavMesh) walkable(a, b Vector3) bool {
	if !n.clear(n.world, a, b) {
		return false
	}
	length := b.Sub(a).Length()
	samples := int(length/(n.cell*0.5)) + 1
	for s := 1; s < samples; s++ {
		p := a.Lerp(b, float32(s)/float32(samples))
		if _, ok := n.world.Raycast(p.Add(Vector3{Y: n.step}), Vector3{Y: -1}, 2*n.step); !ok {
			return false
		}
	}
	return true
}

// navEntry and navQueue are A*'s places to look at next, nearest first.
type navEntry struct {
	place    int32
	estimate float32
}

type navQueue []navEntry

func (q navQueue) Len() int           { return len(q) }
func (q navQueue) Less(i, j int) bool { return q[i].estimate < q[j].estimate }
func (q navQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *navQueue) Push(x any)        { *q = append(*q, x.(navEntry)) }
func (q *navQueue) Pop() any          { old := *q; e := old[len(old)-1]; *q = old[:len(old)-1]; return e }
