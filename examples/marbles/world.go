package main

import (
	"math"

	"golib"
)

// Tuning: the numbers that define how the game feels. Distances are in
// meters, as in golib's 3D; the platform's top is at Y = 0, centered on 0, 0.
const (
	platformSize = 28 // meters a side

	marbleRadius   = 0.5 // meters
	marbleAccel    = 11  // meters per second per second, at full tilt
	marbleDrag     = 1.0 // share of its speed the marble loses per second
	marbleMaxSpeed = 9   // meters per second
	gravity        = 20  // meters per second per second, once off the edge
	fallDepth      = 14  // meters below the platform where a fall costs a life

	startLives   = 3
	respawnGrace = 2.0 // seconds in which the cubes can't hurt a marble that starts again

	cubeCount     = 3
	cubeSize      = 1.0  // meters each way
	cubeReach     = 0.55 // meters from its center at which a cube touches: about its half-size
	cubeBaseSpeed = 2.4  // meters per second
	cubeSpeedStep = 0.6  // meters per second added every level
	cubeMaxSpeed  = 7.5  // meters per second
	cubeGrip      = 5    // meters per second per second: how fast a cube changes course
	cubeSpacing   = 1.3  // meters the cubes keep between their centers

	gemCount        = 3
	gemsPerLevel    = 5
	gemHeight       = 1.0 // meters from the platform to a gem's center
	gemReach        = 0.4 // meters: a gem's size, for collecting it
	gemAwayFromBall = 4   // meters at least from the marble where a new gem appears
	gemAwayFromGem  = 3   // meters at least between gems
	gemMargin       = 1.5 // meters at least from the platform's edge
)

// half is half the platform's side: its edges are at -half and half on X and Z.
const half = platformSize / 2

// marble is the player.
type marble struct {
	position golib.Vector3 // its center
	velocity golib.Vector3 // meters per second
	falling  bool          // it rolled off the edge
	// axes are two of the marble's own directions, turned as it rolls, so
	// the marks drawn on it roll too.
	axes [2]golib.Vector3
}

// cube is an enemy that slides after the marble.
type cube struct {
	position golib.Vector3 // its center, on the platform
	velocity golib.Vector3
	home     golib.Vector3 // where it starts, and goes back to when a life is lost
}

// gem is something to collect, floating above the platform.
type gem struct {
	position golib.Vector3 // its center, gemHeight above the platform
	phase    float32       // seconds, so the gems don't bob and spin in step
}

// events is what happened in one step, for the scene's sounds and effects.
type events struct {
	collected int  // gems collected
	levelUp   bool // the cubes got faster
	fell      bool // the marble went over the edge
	hit       bool // a cube touched the marble
	lostLife  bool // a life went, by a fall or a cube
	gameOver  bool // the last life went
}

// world is the whole game state and its rules. It knows nothing about the
// input or the screen, so world_test.go can play it directly.
type world struct {
	marble marble
	cubes  []cube
	gems   []gem
	score  int
	lives  int
	grace  golib.Timer // runs while the cubes can't hurt the marble
	time   float32     // seconds played, for the gems' bobbing
	over   bool        // no lives left
}

// newWorld returns the world at the start of a game. It uses random numbers
// for the gems' places.
func newWorld() *world {
	w := &world{lives: startLives}
	homes := []golib.Vector3{{X: -10, Z: 10}, {X: 10, Z: 10}, {X: -10, Z: -10}}
	for i := range cubeCount {
		home := homes[i%len(homes)]
		home.Y = cubeSize / 2
		w.cubes = append(w.cubes, cube{position: home, home: home})
	}
	w.resetMarble()
	for i := range gemCount {
		w.gems = append(w.gems, gem{phase: float32(i) * 0.7})
		w.placeGem(i)
	}
	return w
}

// level is how many times the cubes have got faster: one every gemsPerLevel gems.
func (w *world) level() int {
	return w.score / gemsPerLevel
}

// cubeSpeed is the cubes' top speed at the current level.
func (w *world) cubeSpeed() float32 {
	return min(cubeMaxSpeed, cubeBaseSpeed+cubeSpeedStep*float32(w.level()))
}

// resetMarble puts the marble at rest in the middle of the platform.
func (w *world) resetMarble() {
	w.marble = marble{
		position: golib.Vector3{Y: marbleRadius},
		axes:     [2]golib.Vector3{{Y: 1}, {Z: 1}},
	}
}

// step advances the world by dt seconds. move is where the player wants to
// roll, flat on the ground, no longer than 1: the scene turns the keys and the
// stick into it, from where the camera looks.
func (w *world) step(move golib.Vector3, dt float32) events {
	var happened events
	w.time += dt
	if w.over {
		return happened
	}
	w.grace.Tick(dt)

	w.moveMarble(move, dt, &happened)
	if happened.lostLife {
		return happened
	}
	w.moveCubes(dt)

	if !w.marble.falling && !w.grace.Running() {
		for _, c := range w.cubes {
			if touches(w.marble.position, c.position) {
				happened.hit = true
				w.loseLife(&happened)
				return happened
			}
		}
	}

	if w.marble.position.Y > 0 { // still on the platform, or only just off it
		for i, g := range w.gems {
			if flat(g.position).Distance(flat(w.marble.position)) < marbleRadius+gemReach {
				levelBefore := w.level()
				w.score++
				happened.collected++
				happened.levelUp = happened.levelUp || w.level() > levelBefore
				w.placeGem(i)
			}
		}
	}
	return happened
}

// moveMarble rolls the marble, or lets it fall once it is off the edge.
func (w *world) moveMarble(move golib.Vector3, dt float32, happened *events) {
	m := &w.marble
	if !m.falling {
		move = flat(move).ClampLength(1)
		m.velocity = m.velocity.Add(move.Scale(marbleAccel * dt))
		m.velocity = m.velocity.Scale(max(0, 1-marbleDrag*dt)).ClampLength(marbleMaxSpeed)
	} else {
		m.velocity.Y -= gravity * dt
	}
	m.position = m.position.Add(m.velocity.Scale(dt))
	m.roll(dt)

	if !m.falling && offPlatform(m.position) {
		m.falling = true
		happened.fell = true
	}
	if m.position.Y < -fallDepth {
		w.loseLife(happened)
	}
}

// roll turns the marble's axes as a ball rolling without slipping turns: by
// its distance over its radius, around the line across its path.
func (m *marble) roll(dt float32) {
	ground := flat(m.velocity)
	speed := ground.Length()
	if speed == 0 {
		return
	}
	axis := golib.Vector3{Y: 1}.Cross(ground).Normalize() // the top rolls the way it goes
	angle := speed * dt / marbleRadius
	a := rotateAround(m.axes[0], axis, angle).Normalize()
	b := rotateAround(m.axes[1], axis, angle)
	b = b.Sub(a.Scale(b.Dot(a))).Normalize() // kept at right angles to a
	m.axes = [2]golib.Vector3{a, b}
}

// rotateAround turns v by angle radians around axis, which has length 1,
// counterclockwise seen from where axis points (Rodrigues' formula).
// golib.Rotation turns only by yaw, pitch and roll, so a marble that rolls in
// any direction keeps its turn this way.
func rotateAround(v, axis golib.Vector3, angle float32) golib.Vector3 {
	sin, cos := math.Sincos(float64(angle))
	s, c := float32(sin), float32(cos)
	return v.Scale(c).Add(axis.Cross(v).Scale(s)).Add(axis.Scale(axis.Dot(v) * (1 - c)))
}

// moveCubes steers every cube towards the marble, with limited grip, keeps
// them apart and on the platform.
func (w *world) moveCubes(dt float32) {
	speed := w.cubeSpeed()
	for i := range w.cubes {
		c := &w.cubes[i]
		var wanted golib.Vector3
		if !w.marble.falling {
			wanted = flat(w.marble.position.Sub(c.position)).Normalize().Scale(speed)
		}
		c.velocity = c.velocity.MoveTowards(wanted, cubeGrip*dt)
		c.position = c.position.Add(c.velocity.Scale(dt))
	}
	// Push cubes that overlap apart, half each.
	for i := range w.cubes {
		for j := i + 1; j < len(w.cubes); j++ {
			a, b := &w.cubes[i], &w.cubes[j]
			way := flat(b.position.Sub(a.position))
			distance := way.Length()
			if distance >= cubeSpacing {
				continue
			}
			if distance == 0 {
				way = golib.Vector3{X: 1}
			}
			push := way.Normalize().Scale((cubeSpacing - distance) / 2)
			a.position = a.position.Sub(push)
			b.position = b.position.Add(push)
		}
	}
	// Keep them on the platform: a cube at the edge stops going over it.
	limit := float32(half - cubeSize/2.0)
	for i := range w.cubes {
		c := &w.cubes[i]
		if c.position.X < -limit || c.position.X > limit {
			c.position.X = golib.Clamp(c.position.X, -limit, limit)
			c.velocity.X = 0
		}
		if c.position.Z < -limit || c.position.Z > limit {
			c.position.Z = golib.Clamp(c.position.Z, -limit, limit)
			c.velocity.Z = 0
		}
	}
}

// loseLife takes a life, and starts again from the middle, or ends the game.
func (w *world) loseLife(happened *events) {
	w.lives--
	happened.lostLife = true
	if w.lives <= 0 {
		w.lives = 0
		w.over = true
		happened.gameOver = true
		return
	}
	w.resetMarble()
	for i := range w.cubes {
		w.cubes[i].position, w.cubes[i].velocity = w.cubes[i].home, golib.Vector3{}
	}
	w.grace.Start(respawnGrace)
}

// placeGem moves gem i to a random place on the platform, away from the
// marble, the other gems and the cubes. After many tries it takes the last
// place it found, so it never gives up.
func (w *world) placeGem(i int) {
	limit := float32(half - gemMargin)
	var at golib.Vector3
	for try := 0; try < 100; try++ {
		at = golib.Vector3{X: golib.RandomFloat(-limit, limit), Y: gemHeight, Z: golib.RandomFloat(-limit, limit)}
		if w.goodGemPlace(i, at) {
			break
		}
	}
	w.gems[i].position = at
}

func (w *world) goodGemPlace(i int, at golib.Vector3) bool {
	if flat(at).Distance(flat(w.marble.position)) < gemAwayFromBall {
		return false
	}
	for j, g := range w.gems {
		if j != i && flat(at).Distance(flat(g.position)) < gemAwayFromGem {
			return false
		}
	}
	for _, c := range w.cubes {
		if flat(at).Distance(flat(c.position)) < cubeSpacing {
			return false
		}
	}
	return true
}

// touches says whether the marble, with its center at ball, touches a cube
// centered at box. The cubes turn to where they slide, so the check treats
// them as round, cubeReach across from the middle.
func touches(ball, box golib.Vector3) bool {
	return flat(ball).Distance(flat(box)) < marbleRadius+cubeReach
}

// offPlatform says whether a point is beyond the platform's edge, seen from above.
func offPlatform(p golib.Vector3) bool {
	return p.X < -half || p.X > half || p.Z < -half || p.Z > half
}

// flat returns v without its height: where it is on the ground.
func flat(v golib.Vector3) golib.Vector3 {
	return golib.Vector3{X: v.X, Z: v.Z}
}
