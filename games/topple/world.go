package main

import (
	"sort"

	"golib"
)

// Tuning: the numbers that define how the game feels.
const (
	yardHalf     = 10   // meters from the yard's middle to its fence
	fenceHeight  = 0.7  // meters
	fenceWidth   = 0.3  // meters
	walkSpeed    = 4.5  // meters per second
	fallGravity  = 20   // meters per second per second, for the player
	playerHeight = 1.5  // meters: the kit's character, drawn heroScale times its size
	playerRadius = 0.35 // meters
	heroScale    = 1.65 // the kit's character is 0.91 meters tall

	crateSize    = 0.6  // meters each way
	barrelWidth  = 0.5  // meters across
	barrelHeight = 0.75 // meters
	crateMass    = 10   // kilograms: empty crates
	barrelMass   = 9    // kilograms: empty barrels
	propSpacing  = 0.64 // meters between the middles of props in a row
	propGap      = 0.01 // meters of air under each prop when a level starts
	stoneMass    = 6000 // kilograms: nothing in the yard moves it

	ballRadius    = 0.16 // meters
	ballMass      = 5    // kilograms: a heavy leather ball
	ballBounce    = 0.35
	throwSpeed    = 19   // meters per second, towards where the mouse points
	throwMaxTime  = 0.9  // seconds: the longest arc a throw takes to reach its mark
	throwCooldown = 0.35 // seconds between throws
	throwHeight   = 1.25 // meters above the feet the ball leaves the hand
	throwAhead    = 0.6  // meters in front of the player
	ballLife      = 6    // seconds before a thrown ball is taken away
	ballIdle      = 1.0  // seconds a ball may roll slowly before it is taken away
	ballSlow      = 0.6  // meters per second
	maxBalls      = 4    // balls in the yard at once, the oldest going first: each costs (see DESIGN.md)

	downSpeed    = 1.2 // meters per second: a prop slower than this, off its platform, is down
	clearHold    = 1.0 // seconds every target must stay down to clear the level
	impactChange = 1.6 // meters per second of sudden change that sounds as a hit
	impactQuiet  = 0.12
	maxImpacts   = 3 // hit sounds in one update
)

// prop is a crate or barrel in the world.
type prop struct {
	kind    propKind
	body    *golib.RigidBody
	target  bool // it started on a platform
	down    bool // it has come off the platforms
	last    golib.Vector3
	quiet   float32 // seconds before it may sound again
	counted bool    // its fall has been cheered
}

// ball is a thrown ball.
type ball struct {
	body  *golib.RigidBody
	age   float32
	idle  float32 // seconds it has rolled slowly
	last  golib.Vector3
	quiet float32
}

// player is the thrower, walked by a CharacterController.
type player struct {
	body     golib.CharacterController
	velocity golib.Vector3
	yaw      float32 // where it faces
	walking  bool
	throwing float32 // seconds left of the throwing pose
}

// intent is what the player wants in one update, in the world's terms.
type intent struct {
	move  golib.Vector3 // the way to walk on the ground, at most length 1
	throw bool
	aimAt golib.Vector3 // origin and direction of the line the ball is aimed along
	aimBy golib.Vector3
}

// impact is something hitting something, for a sound.
type impact struct {
	position golib.Vector3
	strength float32 // 0 to 1
	ball     bool    // a ball hit, rather than wood
	barrel   bool
}

// events is what happened in one update, for the scene to play sounds.
type events struct {
	threw   bool
	impacts []impact
	fell    []golib.Vector3 // targets that just came down
	cleared bool
}

// world is one level being played.
type world struct {
	level     *level
	number    int // which level, from 0
	physics   *golib.PhysicsWorld
	player    player
	props     []*prop
	stones    []*golib.RigidBody
	balls     []*ball
	thrown    int
	cooldown  float32
	clearTime float32 // seconds every target has been down
	cleared   bool
	time      float32
}

// The still parts of the yard, as meshes placed in the world.
var (
	groundPlace = golib.Transform{Position: golib.Vector3{Y: -0.5}, Scale: golib.Vector3{X: 60, Y: 1, Z: 60}}
)

// fencePlaces are the four sides of the fence.
func fencePlaces() []golib.Transform {
	long := float32(2*yardHalf + fenceWidth)
	y := float32(fenceHeight / 2)
	return []golib.Transform{
		{Position: golib.Vector3{Y: y, Z: -yardHalf}, Scale: golib.Vector3{X: long, Y: fenceHeight, Z: fenceWidth}},
		{Position: golib.Vector3{Y: y, Z: yardHalf}, Scale: golib.Vector3{X: long, Y: fenceHeight, Z: fenceWidth}},
		{Position: golib.Vector3{Y: y, X: -yardHalf}, Scale: golib.Vector3{X: fenceWidth, Y: fenceHeight, Z: long}},
		{Position: golib.Vector3{Y: y, X: yardHalf}, Scale: golib.Vector3{X: fenceWidth, Y: fenceHeight, Z: long}},
	}
}

// platformPlace is where a platform's box is drawn and solid.
func platformPlace(p platform) golib.Transform {
	return golib.Transform{Position: golib.Vector3{X: p.X, Y: p.Height / 2, Z: p.Z}, Scale: golib.Vector3{X: p.Width, Y: p.Height, Z: p.Depth}}
}

// Shapes, made once.
var (
	crateShape  = golib.BoxShape(golib.Vector3{X: 1, Y: 1, Z: 1})
	ballShape   = golib.SphereShape(0.5)
	barrelShape = golib.CylinderShape(1, 0.5) // scaled to a barrel by its transform
	stoneShape  = crateShape
)

// newWorld builds level number n of levels.
func newWorld(levels []*level, n int) *world {
	l := levels[n]
	w := &world{level: l, number: n, physics: golib.NewPhysicsWorld()}
	w.physics.AddMesh(boxMesh, groundPlace)
	for _, f := range fencePlaces() {
		w.physics.AddMesh(boxMesh, f)
	}
	for _, p := range l.Platforms {
		w.physics.AddMesh(boxMesh, platformPlace(p))
	}
	for _, s := range l.Stones {
		size := golib.Vector3{X: s.Width, Y: s.Height, Z: s.Depth}
		at := golib.Transform{Position: golib.Vector3{X: s.X, Y: s.Height/2 + propGap, Z: s.Z}, Scale: size}
		w.stones = append(w.stones, w.physics.AddRigidBody(stoneShape, at, golib.RigidBodyOptions{Mass: stoneMass, Friction: 0.9}))
	}
	for _, p := range l.props {
		w.props = append(w.props, w.addProp(p))
	}
	start := golib.Vector3{X: l.Start.X, Z: l.Start.Z}
	w.player.body = golib.CharacterController{Position: start, Height: playerHeight, Radius: playerRadius, StepHeight: 0.25}
	w.player.yaw = golib.RotationTowards(w.focus().Sub(start)).Yaw
	return w
}

// addProp puts a crate or barrel in the world.
func (w *world) addProp(p placedProp) *prop {
	at := golib.Transform{Position: p.position, Scale: sizeOf(p.kind)}
	var body *golib.RigidBody
	if p.kind == barrelKind {
		body = w.physics.AddRigidBody(barrelShape, at, golib.RigidBodyOptions{Mass: barrelMass, Friction: 0.6})
	} else {
		body = w.physics.AddRigidBody(crateShape, at, golib.RigidBodyOptions{Mass: crateMass, Friction: 0.6})
	}
	return &prop{kind: p.kind, body: body, target: p.target, down: !p.target}
}

// focus is the point the camera keeps in view, on the ground.
func (w *world) focus() golib.Vector3 {
	return golib.Vector3{X: w.level.Focus.X, Z: w.level.Focus.Z}
}

// left is how many targets are still up.
func (w *world) left() int {
	n := 0
	for _, p := range w.props {
		if p.target && !p.down {
			n++
		}
	}
	return n
}

// targets is how many props have to come down.
func (w *world) targets() int {
	n := 0
	for _, p := range w.props {
		if p.target {
			n++
		}
	}
	return n
}

// step moves the world on by dt.
func (w *world) step(want intent, dt float32) events {
	var happened events
	w.time += dt
	w.cooldown = max(0, w.cooldown-dt)
	if want.throw && w.cooldown == 0 && !w.cleared {
		w.throw(want.aimAt, want.aimBy)
		happened.threw = true
	}

	w.physics.Update(dt) // the rigid bodies, before the player moves
	w.walk(want.move, dt)
	w.ageBalls(dt)
	happened.impacts = w.listen(dt)

	if !w.cleared {
		allDown := true
		for _, p := range w.props {
			if !p.target {
				continue
			}
			p.down = w.isDown(p)
			if p.down && !p.counted {
				p.counted = true
				happened.fell = append(happened.fell, p.body.Position())
			}
			allDown = allDown && p.down
		}
		if allDown {
			w.clearTime += dt
		} else {
			w.clearTime = 0
		}
		if w.clearTime >= clearHold {
			w.cleared = true
			happened.cleared = true
		}
	}
	return happened
}

// walk moves the player along move, at most length 1, and turns it there.
func (w *world) walk(move golib.Vector3, dt float32) {
	p := &w.player
	move = golib.Vector3{X: move.X, Z: move.Z}.ClampLength(1)
	p.velocity.X, p.velocity.Z = move.X*walkSpeed, move.Z*walkSpeed
	p.velocity.Y -= fallGravity * dt
	p.body.Move(w.physics, p.velocity.Scale(dt))
	if p.body.OnGround && p.velocity.Y < 0 {
		p.velocity.Y = 0
	}
	p.walking = move.Length() > 0.1
	if p.walking && p.throwing <= 0 {
		p.yaw = golib.RotationTowards(move).Yaw
	}
	p.throwing = max(0, p.throwing-dt)
}

// hand is where a ball leaves the player's hand, facing along way.
func (w *world) hand(way golib.Vector3) golib.Vector3 {
	flat := golib.Vector3{X: way.X, Z: way.Z}.Normalize()
	return w.player.body.Position.Add(golib.Vector3{Y: throwHeight}).Add(flat.Scale(throwAhead))
}

// aimPoint is where a line from origin along direction first meets
// something solid, or a point far along it.
func (w *world) aimPoint(origin, direction golib.Vector3) golib.Vector3 {
	if hit, ok := w.physics.Raycast(origin, direction, 80); ok {
		return hit.Position
	}
	return origin.Add(direction.Normalize().Scale(30))
}

// throwVelocity is the velocity that carries a ball from from to to, under
// the rigid bodies' gravity, in the time a throw of throwSpeed takes,
// at most throwMaxTime.
func throwVelocity(from, to golib.Vector3) golib.Vector3 {
	t := golib.Clamp(to.Distance(from)/throwSpeed, 0.08, throwMaxTime)
	v := to.Sub(from).Scale(1 / t)
	v.Y += 0.5 * 9.81 * t // what gravity takes off on the way
	return v
}

// throw throws a ball at what the line from origin along direction points to.
func (w *world) throw(origin, direction golib.Vector3) {
	mark := w.aimPoint(origin, direction)
	from := w.hand(mark.Sub(w.player.body.Position))
	velocity := throwVelocity(from, mark)
	size := golib.Vector3{X: 2 * ballRadius, Y: 2 * ballRadius, Z: 2 * ballRadius}
	body := w.physics.AddRigidBody(ballShape, golib.Transform{Position: from, Scale: size},
		golib.RigidBodyOptions{Mass: ballMass, Bounce: ballBounce, Fast: true, Friction: 0.5})
	body.SetVelocity(velocity)
	w.balls = append(w.balls, &ball{body: body, last: velocity})
	if len(w.balls) > maxBalls {
		w.balls[0].body.Remove()
		w.balls = w.balls[1:]
	}
	w.thrown++
	w.cooldown = throwCooldown
	w.player.yaw = golib.RotationTowards(golib.Vector3{X: mark.X - from.X, Z: mark.Z - from.Z}).Yaw
	w.player.throwing = 0.4
}

// ageBalls takes away balls that are old or have left the yard.
func (w *world) ageBalls(dt float32) {
	kept := w.balls[:0]
	for _, b := range w.balls {
		b.age += dt
		if b.body.Velocity().Length() < ballSlow {
			b.idle += dt
		} else {
			b.idle = 0
		}
		at := b.body.Position()
		if b.age > ballLife || b.idle > ballIdle || at.Y < -5 || abs(at.X) > 28 || abs(at.Z) > 28 {
			b.body.Remove()
			continue
		}
		kept = append(kept, b)
	}
	clear(w.balls[len(kept):])
	w.balls = kept
}

// listen finds the bodies whose speed changed suddenly in the last update:
// things hitting things. The strongest few are kept.
func (w *world) listen(dt float32) []impact {
	var heard []impact
	expected := float32(9.81) * dt * 1.5 // falling changes speed this much on its own
	for _, p := range w.props {
		v := p.body.Velocity()
		change := v.Sub(p.last).Length()
		p.last = v
		p.quiet = max(0, p.quiet-dt)
		if change > impactChange+expected && p.quiet == 0 {
			p.quiet = impactQuiet
			heard = append(heard, impact{position: p.body.Position(), strength: golib.Clamp(change/8, 0.15, 1), barrel: p.kind == barrelKind})
		}
	}
	for _, b := range w.balls {
		v := b.body.Velocity()
		change := v.Sub(b.last).Length()
		b.last = v
		b.quiet = max(0, b.quiet-dt)
		if change > impactChange+expected && b.quiet == 0 {
			b.quiet = impactQuiet
			heard = append(heard, impact{position: b.body.Position(), strength: golib.Clamp(change/12, 0.15, 1), ball: true})
		}
	}
	sort.Slice(heard, func(i, j int) bool { return heard[i].strength > heard[j].strength })
	if len(heard) > maxImpacts {
		heard = heard[:maxImpacts]
	}
	return heard
}

// isDown says whether a prop has come off the platforms: it isn't over any
// platform's top, and it has nearly stopped.
func (w *world) isDown(p *prop) bool {
	at := p.body.Position()
	if p.body.Velocity().Length() > downSpeed {
		return false
	}
	for _, pl := range w.level.Platforms {
		over := abs(at.X-pl.X) < pl.Width/2+0.05 && abs(at.Z-pl.Z) < pl.Depth/2+0.05
		if over && at.Y > pl.Height {
			return false
		}
	}
	return true
}

func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}
