package main

import (
	"math"

	"golib"
)

// Crates: picked with the mouse or the nearest in reach, punched, broken
// into splinters, and the key inside let out.

// crateAt is the crate numbered i, if it is still whole.
func (w *world) crateAt(i int) *crate {
	if i < 0 || i >= len(w.crates) || w.crates[i].broken {
		return nil
	}
	return w.crates[i]
}

// crateHit is the crate whose body a ray hit, by its number, or -1.
func (w *world) crateHit(body int) int {
	for i, c := range w.crates {
		if !c.broken && c.body == body {
			return i
		}
	}
	return -1
}

// pickCrate is the crate a ray from origin along direction hits first, such
// as one from the camera through the mouse pointer, or -1.
func (w *world) pickCrate(origin, direction golib.Vector3) int {
	hit, ok := w.physics.Raycast(origin, direction, 30)
	if !ok {
		return -1
	}
	return w.crateHit(hit.Body)
}

// inReach says whether the astronaut can punch crate i from where it stands.
func (w *world) inReach(i int) bool {
	c := w.crateAt(i)
	if c == nil {
		return false
	}
	return gapToBox(w.player.body.Position, playerHeight, c.spec) <= punchReach
}

// nearestCrate is the closest crate in reach, preferring one in front, or -1.
func (w *world) nearestCrate() int {
	p := &w.player
	best, bestScore := -1, float32(math.MaxFloat32)
	for i := range w.crates {
		if !w.inReach(i) {
			continue
		}
		way := flat(w.crates[i].spec.center.Sub(p.body.Position))
		score := way.Length() + abs(angleBetween(p.yaw, golib.RotationTowards(way).Yaw))/90
		if score < bestScore {
			best, bestScore = i, score
		}
	}
	return best
}

// breakCrate smashes a crate into splinters, out of the way, and lets out
// its key, if it has one.
func (w *world) breakCrate(c *crate, happened *events) {
	c.broken = true
	w.physics.Remove(c.body)
	// The guards' paths keep going round where it stood: making the NavMesh
	// again takes about 80 ms, a stutter, which only an opened door is worth.
	happened.broke = append(happened.broke, c.spec.center)
	w.burst(c.spec.center, c.spec.size.X, 14, false)
	if c.spec.key {
		w.keys = append(w.keys, &key{name: c.spec.name, position: crateFloor(c.spec), fromBox: true})
		w.say("A key fell out of the crate!")
	}
}

// burst throws pieces out from at.
func (w *world) burst(at golib.Vector3, size float32, count int, stone bool) {
	floor := at.Y - size/2
	for range count {
		out := golib.Vector3{X: golib.RandomFloat(-1, 1), Y: golib.RandomFloat(0.4, 1.2), Z: golib.RandomFloat(-1, 1)}
		w.debris = append(w.debris, debris{
			position: at.Add(golib.Vector3{X: golib.RandomFloat(-0.3, 0.3) * size, Y: golib.RandomFloat(-0.3, 0.3) * size, Z: golib.RandomFloat(-0.3, 0.3) * size}),
			velocity: out.Scale(golib.RandomFloat(1.5, 3.5)),
			turn:     golib.Rotation{Yaw: golib.RandomFloat(0, 360), Pitch: golib.RandomFloat(-90, 90)},
			spin:     golib.RandomFloat(-400, 400),
			size:     golib.RandomFloat(0.08, 0.22),
			floor:    floor,
			life:     debrisLife * golib.RandomFloat(0.7, 1),
			stone:    stone,
		})
	}
}

// stepDebris lets the pieces fall, bounce and fade.
func (w *world) stepDebris(dt float32) {
	kept := w.debris[:0]
	for _, d := range w.debris {
		d.life -= dt
		if d.life <= 0 {
			continue
		}
		d.velocity.Y -= gravity * dt
		d.position = d.position.Add(d.velocity.Scale(dt))
		if d.position.Y < d.floor+d.size/2 {
			d.position.Y = d.floor + d.size/2
			d.velocity = golib.Vector3{X: d.velocity.X * 0.5, Y: -d.velocity.Y * 0.3, Z: d.velocity.Z * 0.5}
			d.spin *= 0.5
		}
		d.turn.Yaw += d.spin * dt
		d.turn.Roll += d.spin * 0.7 * dt
		kept = append(kept, d)
	}
	w.debris = kept
}
