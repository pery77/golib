package main

import "golib"

// Tuning for the skeleton guards: more of the same astronaut, tinted pale.
const (
	guardRadius     = 0.3  // meters: a little wider than the astronaut, so their paths keep clear of corners
	patrolSpeed     = 1.3  // meters per second on their rounds
	chaseSpeed      = 3.0  // meters per second after the player: a little slower than the player's run
	sightRange      = 9.0  // meters a guard sees
	sightArc        = 60   // degrees either side of where it faces
	senseRange      = 1.5  // meters: closer than this, it notices the player whichever way it faces
	eyeHeight       = 0.7  // meters above its feet
	searchTime      = 3.5  // seconds a guard keeps after the player once it can't see them
	repathTime      = 0.35 // seconds between new paths while it chases
	catchReach      = 0.75 // meters between a guard's middle and the player's to grab them
	routeWait       = 1.0  // seconds a guard stands at each point of its round
	waypointReach   = 0.3  // meters from a point of its path to count it reached
	guardStepLength = 0.7  // meters between a guard's footsteps
	alertTime       = 0.9  // seconds the "!" shows over a guard that saw the player
)

// guardState is what a guard is doing.
type guardState int

const (
	patrolling guardState = iota // walking its round
	chasing                      // after the player, or where it last saw them
)

// guard is a skeleton guard.
type guard struct {
	spec     guardSpec
	body     golib.CharacterController
	velocity golib.Vector3
	yaw      float32
	animator *golib.Animator
	state    guardState
	next     int             // the point of its round it walks to
	path     []golib.Vector3 // the way it follows, from the crypt's NavMesh
	pathAt   int             // the point of path it walks to
	repath   golib.Timer     // runs until it looks for a new path to the player
	wait     golib.Timer     // runs while it stands at a point of its round
	lastSeen golib.Vector3   // where it last saw the player
	lost     golib.Timer     // runs while it chases the player out of sight
	alert    golib.Timer     // runs while the "!" shows
	grabbing golib.Timer     // runs while it grabs the player
	walked   float32         // meters walked since its last footstep
}

// guardBody is a guard's body standing at feet, and the size the crypt's
// NavMesh is made for.
func guardBody(feet golib.Vector3) golib.CharacterController {
	return golib.CharacterController{Position: feet, Height: playerHeight, Radius: guardRadius, StepHeight: stepHeight}
}

// newGuard is a guard at the start of its round, facing its first point.
func newGuard(spec guardSpec) *guard {
	g := &guard{spec: spec, body: guardBody(spec.start), animator: golib.NewAnimator(characterModel)}
	g.next = 1 % len(spec.route)
	g.yaw = golib.RotationTowards(flat(spec.route[g.next].Sub(spec.start))).Yaw
	g.animator.Play(animIdle)
	return g
}

// speed is how fast it moves across the ground, in meters per second.
func (g *guard) speed() float32 {
	return flat(g.velocity).Length()
}

// stepGuard moves one guard on: it walks its round, looks for the player,
// chases them along the crypt's paths, and grabs them.
func (w *world) stepGuard(g *guard, dt float32, happened *events) {
	p := &w.player
	g.alert.Tick(dt)
	g.grabbing.Tick(dt)
	var target golib.Vector3 // the velocity it wants on the ground
	if p.caught.Running() {
		target = golib.Vector3{} // everyone stops while the player lies grabbed
	} else {
		sees := w.sees(g)
		switch g.state {
		case patrolling:
			if sees {
				g.startChase(p.body.Position)
				happened.spotted = append(happened.spotted, g.body.Position)
				break
			}
			target = w.patrol(g, dt)
		case chasing:
			if sees {
				g.lastSeen = p.body.Position
				g.lost.Start(searchTime)
			} else if g.lost.Tick(dt) {
				g.stopChase()
				break
			}
			target = w.chase(g, sees, dt)
			if w.catches(g) {
				w.grab(happened)
				g.grabbing.Start(caughtTime)
				g.animator.PlayWith(animGrab, golib.AnimationOptions{Bone: boneTorso, Once: true, Restart: true})
			}
		}
	}
	steer(&g.velocity, &g.yaw, target, g.body.OnGround, dt)
	if g.state == chasing && w.sees(g) {
		// Face the player while closing in, even when the path turns.
		g.yaw = turnTowards(g.yaw, golib.RotationTowards(flat(p.body.Position.Sub(g.body.Position))).Yaw, turnSpeed*dt)
	}
	before := g.body.Position
	moveBody(&g.body, &g.velocity, w.physics, dt)
	if g.body.OnGround {
		g.walked += flat(g.body.Position.Sub(before)).Length()
		if g.walked >= guardStepLength {
			g.walked = 0
			happened.steps = append(happened.steps, g.body.Position)
		}
	}
}

// startChase sets the guard after the player, seen at at.
func (g *guard) startChase(at golib.Vector3) {
	g.state = chasing
	g.lastSeen = at
	g.lost.Start(searchTime)
	g.path, g.pathAt = nil, 0
	g.repath.Stop()
	g.alert.Start(alertTime)
	g.animator.PlayWith(animReach, golib.AnimationOptions{Bone: boneTorso})
}

// stopChase sends the guard back to its round.
func (g *guard) stopChase() {
	g.state = patrolling
	g.path, g.pathAt = nil, 0
	g.wait.Start(routeWait)
	g.animator.Stop(boneTorso)
}

// patrol walks the guard round its points, standing a moment at each, and
// returns the velocity it wants.
func (w *world) patrol(g *guard, dt float32) golib.Vector3 {
	if g.wait.Running() {
		g.wait.Tick(dt)
		return golib.Vector3{}
	}
	if g.path == nil {
		path, ok := w.nav.Path(g.body.Position, g.spec.route[g.next])
		if !ok {
			// No way there, such as a guard that fell off its floor: it stands.
			return golib.Vector3{}
		}
		g.path, g.pathAt = path, 0
	}
	way, arrived := g.followPath()
	if arrived {
		g.path = nil
		g.next = (g.next + 1) % len(g.spec.route)
		g.wait.Start(routeWait)
		return golib.Vector3{}
	}
	return way.Scale(patrolSpeed)
}

// chase runs the guard along a path to where it last saw the player, and
// returns the velocity it wants.
func (w *world) chase(g *guard, sees bool, dt float32) golib.Vector3 {
	p := &w.player
	toPlayer := flat(p.body.Position.Sub(g.body.Position))
	if sees && toPlayer.Length() < 2 && abs(p.body.Position.Y-g.body.Position.Y) < 0.5 {
		return toPlayer.Normalize().Scale(chaseSpeed) // close and in sight: straight at them
	}
	if g.repath.Tick(dt) || g.path == nil {
		g.repath.Start(repathTime)
		path, ok := w.nav.Path(g.body.Position, g.lastSeen)
		if ok {
			g.path, g.pathAt = path, 0
		} else {
			g.path = []golib.Vector3{}
		}
	}
	way, arrived := g.followPath()
	if arrived {
		return golib.Vector3{} // where it last saw the player: it looks about until it gives up
	}
	return way.Scale(chaseSpeed)
}

// followPath is the way to the next point of the guard's path, on the
// ground, length 1, and whether it has reached the path's end.
func (g *guard) followPath() (golib.Vector3, bool) {
	for g.pathAt < len(g.path) && flat(g.path[g.pathAt].Sub(g.body.Position)).Length() < waypointReach {
		g.pathAt++
	}
	if g.pathAt >= len(g.path) {
		return golib.Vector3{}, true
	}
	return flat(g.path[g.pathAt].Sub(g.body.Position)).Normalize(), false
}

// sees says whether a guard sees the player: close enough, in front of it,
// or right behind it, and with nothing solid in between.
func (w *world) sees(g *guard) bool {
	p := &w.player
	if p.caught.Running() {
		return false
	}
	eye := g.body.Position.Add(golib.Vector3{Y: eyeHeight})
	chest := p.body.Position.Add(golib.Vector3{Y: playerHeight * 0.6})
	way := chest.Sub(eye)
	distance := way.Length()
	if distance > sightRange {
		return false
	}
	if distance > senseRange && abs(angleBetween(g.yaw, golib.RotationTowards(flat(way)).Yaw)) > sightArc {
		return false
	}
	hit, blocked := w.physics.Raycast(eye, way.Normalize(), distance)
	return !blocked || hit.Distance >= distance-0.05
}

// catches says whether a guard is close enough to grab the player.
func (w *world) catches(g *guard) bool {
	p := &w.player
	return flat(p.body.Position.Sub(g.body.Position)).Length() < catchReach &&
		abs(p.body.Position.Y-g.body.Position.Y) < 0.8
}

// separateGuards pushes guards that stand inside each other apart, through
// the crypt, so they don't walk as one.
func (w *world) separateGuards() {
	const apart = 2 * guardRadius
	for i, a := range w.guards {
		for _, b := range w.guards[i+1:] {
			way := flat(b.body.Position.Sub(a.body.Position))
			d := way.Length()
			if d >= apart || abs(a.body.Position.Y-b.body.Position.Y) > 1 {
				continue
			}
			if d < 0.001 {
				way, d = golib.Vector3{X: 1}, 1
			}
			push := way.Scale((apart - d) / d / 2)
			a.body.Move(w.physics, push.Scale(-1))
			b.body.Move(w.physics, push)
		}
	}
}
