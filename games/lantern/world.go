package main

import (
	"math"

	"golib"
)

// The tuning: how the explorer moves and what the temple asks of them.
const (
	walkSpeed      = 3.6  // meters per second on dry stone
	wadeSpeed      = 1.9  // meters per second with the feet under water
	turnRate       = 540  // degrees per second the explorer turns to face where they walk
	gravity        = 20   // meters per second per second
	explorerHeight = 1.7  // meters
	explorerRadius = 0.32 // meters
	waterLevel     = -0.08
	wadeDepth      = 0.05 // meters under the water's surface the feet must be to wade
	gemReach       = 1.1  // meters from the explorer's middle to a gem to take it
	exitReach      = 1.4  // meters from the exit to leave
	gateOpenTime   = 2.5  // seconds the gate takes to sink into the floor
	runeFadeIn     = 1.2  // seconds a rune takes to glow fully
)

// pool is the sunken floor of the flooded hall, under the water, from x0, z0
// to x1, z1: where the water is drawn.
var pool = struct{ x0, x1, z0, z1 float32 }{-4, 4, 15, 25}

// gem is one of the six gems, and the rune it leaves once taken.
type gem struct {
	position golib.Vector3 // where it floats
	floor    golib.Vector3 // the floor under it, where its rune glows
	color    int           // which of gemColors
	taken    bool
	takenAt  float32 // the world's time when it was taken
}

// explorer is the player: a capsule walked through the temple.
type explorer struct {
	body     golib.CharacterController
	velocity golib.Vector3
	yaw      float32 // degrees, where they face
	stride   float32 // meters walked, for the bob of their steps
	wading   bool
}

// gate is the stone gate in the far wall, solid until the six gems open it.
type gate struct {
	position golib.Vector3 // its middle when closed
	size     golib.Vector3
	body     int     // its number in the physics world while it is solid
	open     bool    // the six gems were taken
	opening  float32 // from 0, closed, to 1, sunk into the floor
}

// intent is what the player wants in one update: the way to walk on the
// ground, as long as the push, at most 1.
type intent struct {
	move golib.Vector3
}

// event says what happened in an update, for the scene to play sounds and
// throw particles.
type event struct {
	took      int // the gem taken, or -1
	gateOpens bool
	escaped   bool
}

// world is the temple and everything in it: the rules, with no input or
// drawing.
type world struct {
	physics *golib.PhysicsWorld
	player  explorer
	start   golib.Vector3
	gems    []gem
	gate    gate
	exit    golib.Vector3
	torches []golib.Vector3 // the flames, where fire and smoke rise
	time      float32         // seconds played
	escaped   bool
	escapedAt float32 // the time the explorer left
}

// newWorld reads the level's markers and builds the world from them. The
// level must be loaded: call it from main or a scene, never from a package
// variable's initializer.
func newWorld() *world {
	w := &world{physics: golib.NewPhysicsWorld()}
	w.physics.AddModel(level, golib.Transform{})
	for _, m := range level.Markers() {
		switch m.Properties.String("kind") {
		case "start":
			w.start = m.Position
			w.player.yaw = m.Rotation.Yaw
		case "gem":
			w.gems = append(w.gems, gem{position: m.Position, color: len(w.gems) % len(gemColors)})
		case "torch":
			w.torches = append(w.torches, m.Position)
		case "gate":
			w.gate.position, w.gate.size = m.Position, m.Scale
		case "exit":
			w.exit = m.Position
		}
	}
	// The runes glow on whatever is under each gem: the floor, the pool.
	for i := range w.gems {
		g := &w.gems[i]
		g.floor = golib.Vector3{X: g.position.X, Z: g.position.Z}
		if hit, ok := w.physics.Raycast(g.position, golib.Vector3{Y: -1}, 5); ok {
			g.floor = hit.Position
		}
	}
	w.gate.body = w.physics.AddMesh(boxMesh, w.gate.transform())
	w.player.body = golib.CharacterController{Position: w.start, Height: explorerHeight, Radius: explorerRadius}
	return w
}

// transform is where the gate is drawn now, sinking as it opens.
func (g gate) transform() golib.Transform {
	at := g.position
	at.Y -= g.size.Y * golib.EaseInOut(g.opening)
	return golib.Transform{Position: at, Scale: g.size}
}

// gemsTaken counts the gems taken so far.
func (w *world) gemsTaken() int {
	n := 0
	for _, g := range w.gems {
		if g.taken {
			n++
		}
	}
	return n
}

// step moves the world on by dt seconds, as the player wants.
func (w *world) step(want intent, dt float32) event {
	ev := event{took: -1}
	w.time += dt
	p := &w.player

	speed := float32(walkSpeed)
	if p.wading {
		speed = wadeSpeed
	}
	p.velocity.X, p.velocity.Z = want.move.X*speed, want.move.Z*speed
	p.velocity.Y -= gravity * dt
	p.body.Move(w.physics, p.velocity.Scale(dt))
	if p.body.OnGround && p.velocity.Y < 0 {
		p.velocity.Y = 0
	}
	p.wading = p.body.Position.Y < waterLevel-wadeDepth

	// Face where the explorer really went, turning at turnRate.
	moved := golib.Vector3{X: p.body.Moved.X, Z: p.body.Moved.Z}
	if moved.Length() > 0.2*speed*dt {
		p.yaw = turnTowards(p.yaw, golib.RotationTowards(moved).Yaw, turnRate*dt)
		p.stride += moved.Length()
	}

	middle := p.body.Position.Add(golib.Vector3{Y: explorerHeight / 2})
	for i := range w.gems {
		g := &w.gems[i]
		if !g.taken && g.position.Distance(middle) < gemReach {
			g.taken, g.takenAt = true, w.time
			ev.took = i
		}
	}
	if !w.gate.open && w.gemsTaken() == len(w.gems) {
		w.gate.open = true
		w.physics.Remove(w.gate.body)
		ev.gateOpens = true
	}
	if w.gate.open && w.gate.opening < 1 {
		w.gate.opening = min(1, w.gate.opening+dt/gateOpenTime)
	}
	if w.gate.open && !w.escaped && flatDistance(p.body.Position, w.exit) < exitReach {
		w.escaped, w.escapedAt = true, w.time
		ev.escaped = true
	}
	return ev
}

// runeGlow is how brightly a taken gem's rune glows, from 0 as it is taken
// to 1.
func (w *world) runeGlow(g gem) float32 {
	if !g.taken {
		return 0
	}
	return golib.EaseOut(golib.Clamp((w.time-g.takenAt)/runeFadeIn, 0, 1))
}

// flatDistance is the distance between a and b across the ground.
func flatDistance(a, b golib.Vector3) float32 {
	return golib.Vector2{X: a.X - b.X, Y: a.Z - b.Z}.Length()
}

// turnTowards turns from yaw towards target by at most step degrees, the
// short way round.
func turnTowards(yaw, target, step float32) float32 {
	diff := float32(math.Remainder(float64(target-yaw), 360))
	if diff > step {
		diff = step
	} else if diff < -step {
		diff = -step
	}
	return yaw + diff
}
