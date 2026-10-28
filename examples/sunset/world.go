package main

import (
	"math"

	"golib"
)

// Tuning: the numbers that define how the game feels. Distances are in
// meters, on the water, which is the ground at Y 0; speeds in meters per
// second.
const (
	roundTime = 90 // seconds a run lasts

	thrust      = 16   // m/s² forward at full throttle
	reverse     = 9    // m/s² backwards, braking first
	maxSpeed    = 17   // m/s
	turnSpeed   = 120  // degrees per second at full steer
	forwardDrag = 0.7  // share of the speed along the hull lost per second
	sideGrip    = 2.2  // share of the sideways slide lost per second: lower drifts more
	craftRadius = 1.3  // meters: the hovercraft, as a circle on the water
	bounce      = 0.45 // share of the speed kept, bounced, after hitting something

	lagoonRadius = 55  // meters from the middle to where the lagoon starts pushing back
	lagoonPush   = 30  // m/s² pushing back towards the middle, per meter past lagoonRadius
	lagoonEdge   = 1.5 // meters past lagoonRadius that the hovercraft can't go, inside the buoys

	crystalCount     = 7    // crystals on the water at a time
	crystalReach     = 1.9  // meters from the hovercraft's middle to a crystal's that collects it
	crystalHeight    = 1.3  // meters over the water
	crystalMinFar    = 14   // meters: new crystals appear at least this far from the hovercraft
	crystalSpacing   = 7    // meters between crystals, at least
	crystalSpawnArea = 48   // meters from the middle, at most
	goldenEvery      = 5    // every fifth crystal that appears is golden
	goldenLife       = 12.0 // seconds a golden crystal lasts
	goldenPoints     = 3
)

// obstacle is something solid in the water: an islet or a cluster of
// mooring poles, a circle on the water.
type obstacle struct {
	position golib.Vector3 // its middle, at the water
	radius   float32       // meters
	islet    bool          // an islet of rock; otherwise mooring poles
}

// obstacles are the lagoon's islets and mooring poles, the same in every run.
var obstacles = []obstacle{
	{position: golib.Vector3{X: -18, Z: 24}, radius: 6, islet: true},
	{position: golib.Vector3{X: 26, Z: -14}, radius: 7.5, islet: true},
	{position: golib.Vector3{X: -30, Z: -26}, radius: 5, islet: true},
	{position: golib.Vector3{X: 14, Z: 36}, radius: 4, islet: true},
	{position: golib.Vector3{X: 8, Z: 12}, radius: 0.8},
	{position: golib.Vector3{X: -6, Z: -16}, radius: 0.8},
	{position: golib.Vector3{X: -26, Z: 4}, radius: 0.8},
	{position: golib.Vector3{X: 34, Z: 14}, radius: 0.8},
	{position: golib.Vector3{X: 2, Z: -36}, radius: 0.8},
	{position: golib.Vector3{X: -12, Z: 42}, radius: 0.8},
	{position: golib.Vector3{X: 40, Z: -34}, radius: 0.8},
}

// craft is the hovercraft.
type craft struct {
	position golib.Vector3 // on the water, Y 0
	velocity golib.Vector3 // m/s, on the water
	yaw      float32       // degrees: where its nose points, as golib.Rotation's Yaw
	steer    float32       // the last steer, -1 to 1, for leaning into turns
	throttle float32       // the last throttle, -1 to 1
}

// heading is the way the hovercraft's nose points, on the water.
func (c craft) heading() golib.Vector3 {
	return golib.Rotation{Yaw: c.yaw}.Forward()
}

// crystal is a crystal floating over the water.
type crystal struct {
	position golib.Vector3 // over the water, crystalHeight up
	golden   bool
	life     golib.Timer // a golden crystal fades when it runs out
	phase    float32     // seconds, so crystals don't bob together
	age      float32     // seconds since it appeared, for growing in
}

// events is what happened in one step, for the scenes to play sounds and
// show effects.
type events struct {
	collected []crystal // crystals collected in this step
	faded     []crystal // golden crystals that faded away
	bumped    float32   // the speed of a hit against an islet, a pole or the ring, 0 for none
	tick      int       // a whole second of the last ten passed: the seconds left, else 0
	timeUp    bool      // the run ended in this step
}

// world is the whole game state and its rules. It knows nothing about the
// input or the screen, so world_test.go can play it directly.
type world struct {
	craft    craft
	crystals []crystal
	score    int
	clock    golib.Timer // the time left in the run
	spawned  int         // crystals that have appeared, to make every fifth golden
	over     bool
	time     float32 // seconds since the run began, for bobbing and spinning
}

// newWorld returns the world at the start of a run: the hovercraft in the
// middle of the lagoon facing the sunset, and the crystals around it.
func newWorld() *world {
	w := &world{
		craft: craft{yaw: sunsetYaw},
		clock: golib.NewTimer(roundTime),
	}
	for range crystalCount {
		w.spawnCrystal()
	}
	for i := range w.crystals {
		w.crystals[i].age = 1 // the first crystals are there from the start, grown
	}
	return w
}

// sunsetYaw is the yaw that faces the setting sun, against the way its
// light travels (sunDirection, in main.go).
var sunsetYaw = golib.RotationTowards(golib.Vector3{X: -sunDirection.X, Z: -sunDirection.Z}).Yaw

// step advances the world by dt seconds. throttle goes from -1, reverse, to
// 1, full ahead; steer from -1, left, to 1, right.
func (w *world) step(throttle, steer, dt float32) events {
	var happened events
	w.time += dt
	if w.over {
		w.glide(0, 0, dt)
		return happened
	}
	before := int(math.Ceil(float64(w.clock.Left())))
	if w.clock.Tick(dt) {
		w.over = true
		happened.timeUp = true
	}
	if after := int(math.Ceil(float64(w.clock.Left()))); after < before && after < 10 && after > 0 {
		happened.tick = after
	}
	happened.bumped = w.glide(throttle, steer, dt)
	if !w.over {
		w.collect(&happened)
	}
	w.age(&happened, dt)
	return happened
}

// glide moves the hovercraft by its controls: it turns, thrusts along its
// nose and slides on, losing its sideways slide faster than its speed ahead,
// as a hovercraft drifts. It returns the speed of a hit, or 0.
func (w *world) glide(throttle, steer, dt float32) float32 {
	c := &w.craft
	throttle = golib.Clamp(throttle, -1, 1)
	steer = golib.Clamp(steer, -1, 1)
	c.throttle, c.steer = throttle, steer
	c.yaw -= steer * turnSpeed * dt // a larger yaw turns left
	c.yaw = float32(math.Mod(float64(c.yaw), 360))

	heading := c.heading()
	right := golib.Rotation{Yaw: c.yaw}.Right()
	ahead := c.velocity.Dot(heading)
	side := c.velocity.Dot(right)
	switch {
	case throttle > 0:
		ahead += throttle * thrust * dt
	case throttle < 0 && ahead > 0:
		ahead = max(0, ahead+throttle*thrust*dt) // braking is as strong as thrust
	case throttle < 0:
		ahead += throttle * reverse * dt
	}
	ahead *= 1 - min(1, forwardDrag*dt)
	side *= 1 - min(1, sideGrip*dt)
	c.velocity = heading.Scale(ahead).Add(right.Scale(side)).ClampLength(maxSpeed)
	c.position = c.position.Add(c.velocity.Scale(dt))
	return w.keepOut(dt)
}

// keepOut pushes the hovercraft out of islets and poles, bouncing it off
// them, and back towards the lagoon from past its ring of buoys. It returns
// the speed of the hardest hit, or 0.
func (w *world) keepOut(dt float32) float32 {
	c := &w.craft
	var hit float32
	for _, o := range obstacles {
		away := flat(c.position.Sub(o.position))
		gap := away.Length() - o.radius - craftRadius
		if gap >= 0 {
			continue
		}
		normal := away.Normalize()
		if normal == (golib.Vector3{}) {
			normal = golib.Vector3{X: 1}
		}
		c.position = c.position.Add(normal.Scale(-gap))
		if into := c.velocity.Dot(normal); into < 0 {
			hit = max(hit, -into)
			c.velocity = c.velocity.Sub(normal.Scale((1 + bounce) * into)).Scale(0.8)
		}
	}
	if past := flat(c.position).Length() - lagoonRadius; past > 0 {
		inward := flat(c.position).Normalize().Scale(-1)
		c.velocity = c.velocity.Add(inward.Scale(lagoonPush * past * dt))
		if past > lagoonEdge {
			c.position = c.position.Add(inward.Scale(past - lagoonEdge))
			if out := c.velocity.Dot(inward); out < 0 {
				hit = max(hit, -out)
				c.velocity = c.velocity.Sub(inward.Scale((1 + bounce) * out))
			}
		}
	}
	c.position.Y = 0
	return hit
}

// collect takes the crystals the hovercraft touches, scores them, and puts
// new ones elsewhere.
func (w *world) collect(happened *events) {
	kept := w.crystals[:0]
	for _, k := range w.crystals {
		if flat(k.position.Sub(w.craft.position)).Length() <= crystalReach {
			happened.collected = append(happened.collected, k)
			w.score += k.points()
			continue
		}
		kept = append(kept, k)
	}
	w.crystals = kept
	for range happened.collected {
		w.spawnCrystal()
	}
}

// age grows new crystals in and fades golden ones out.
func (w *world) age(happened *events, dt float32) {
	kept := w.crystals[:0]
	for _, k := range w.crystals {
		k.age += dt
		if k.golden && k.life.Tick(dt) {
			happened.faded = append(happened.faded, k)
			continue
		}
		kept = append(kept, k)
	}
	w.crystals = kept
	for range happened.faded {
		w.spawnCrystal()
	}
}

// points is what collecting k scores.
func (k crystal) points() int {
	if k.golden {
		return goldenPoints
	}
	return 1
}

// spawnCrystal puts a new crystal somewhere on the lagoon: away from the
// hovercraft, the islets, the poles and the other crystals. Every
// goldenEvery-th is golden.
func (w *world) spawnCrystal() {
	w.spawned++
	k := crystal{golden: w.spawned%goldenEvery == 0, phase: golib.RandomFloat(0, 10)}
	if k.golden {
		k.life = golib.NewTimer(goldenLife)
	}
	best, bestRoom := golib.Vector3{}, float32(-1)
	for range 60 {
		angle := golib.RandomFloat(0, 2*math.Pi)
		reach := crystalSpawnArea * float32(math.Sqrt(float64(golib.RandomFloat(0.02, 1))))
		at := golib.Vector3{X: reach * float32(math.Cos(float64(angle))), Z: reach * float32(math.Sin(float64(angle)))}
		room := w.room(at)
		if room > bestRoom {
			best, bestRoom = at, room
		}
		if room >= 1 {
			break
		}
	}
	k.position = best.Add(golib.Vector3{Y: crystalHeight})
	w.crystals = append(w.crystals, k)
}

// room says how well a spot suits a new crystal: 1 or more when it is far
// enough from everything, less the more it crowds something.
func (w *world) room(at golib.Vector3) float32 {
	room := flat(at.Sub(w.craft.position)).Length() / crystalMinFar
	for _, o := range obstacles {
		room = min(room, (flat(at.Sub(o.position)).Length()-o.radius)/3)
	}
	for _, k := range w.crystals {
		room = min(room, flat(at.Sub(k.position)).Length()/crystalSpacing)
	}
	return room
}

// timeLeft is the seconds left in the run.
func (w *world) timeLeft() float32 {
	return w.clock.Left()
}

// flat returns v on the water, with no height.
func flat(v golib.Vector3) golib.Vector3 {
	return golib.Vector3{X: v.X, Z: v.Z}
}
