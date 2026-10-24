package main

import (
	"math"

	"golib"
)

// Tuning: the numbers that define how the game feels. Distances are in
// meters and times in seconds, as in golib's 3D: Y is up, and the island
// lies around 0, 0, 0 (see island.go).
const (
	walkSpeed   = 2.1  // meters per second, pushed all the way without sprinting
	sprintSpeed = 5.2  // meters per second, sprinting
	groundAccel = 30   // meters per second per second: how fast running starts and stops
	airAccel    = 14   // the same in the air
	turnSpeed   = 720  // degrees per second the astronaut turns to face where it runs
	gravity     = 26   // meters per second per second
	jumpSpeed   = 8.4  // meters per second up: a jump rises jumpSpeed²/2g, about 1.35 m, onto a block
	stepUp      = 0.3  // meters the astronaut steps up without jumping
	bodyRadius  = 0.26 // meters: the astronaut's size on the ground, against blocks, trunks and rocks
	footHalf    = 0.1  // meters: how far over an edge the astronaut stands before it falls

	seaLevel     = 0.35 // meters: the sea's surface; the blocks stand on its floor at 0
	sinkDepth    = 0.5  // meters under the surface at which a fall becomes a splash
	splashTime   = 0.9  // seconds before the astronaut is back on the last ground it stood on
	splashCost   = 3    // seconds off the clock for falling in the sea
	startingTime = 60   // seconds on the clock at the start
	coinTime     = 1.5  // seconds a coin adds to the clock
	coinReach    = 0.65 // meters from the astronaut's middle to a coin's it picks up
	coinScore    = 100  // points a coin
	secondScore  = 20   // points a second left on the clock at the flag
	flagReach    = 1.2  // meters from the flag's foot that count as reaching it

	punchTime   = 0.4  // seconds a punch lasts, and the wait before the next
	punchHitAt  = 0.2  // seconds into a punch when the fist is furthest out (see TestThePunchLandsWhenTheFistIsOut)
	punchReach  = 1.05 // meters from the astronaut's feet to a rock's middle it can punch
	punchArc    = 60   // degrees either side of where it faces that a punch reaches
	aimArc      = 110  // degrees either side in which a punch turns the astronaut to a rock
	rockHits    = 3    // punches that break a rock open
	rockCoins   = 3    // coins inside each rock
	coinPopUp   = 6    // meters per second up a coin flies out of a broken rock
	coinPopOut  = 1.6  // meters per second outwards
	coinWait    = 0.35 // seconds before a coin out of a rock can be picked up
	shakeTime   = 0.25 // seconds a punched rock shakes
	debrisTime  = 0.9  // seconds the pieces of a broken rock fly
	debrisCount = 6
)

// coin is a coin: floating where the island put it, or flying out of a
// broken rock until it lands.
type coin struct {
	position golib.Vector3 // its middle
	velocity golib.Vector3 // meters per second, while it flies
	home     golib.Vector3 // where a flying coin starts again if it falls in the sea
	flying   bool
	wait     golib.Timer // runs while it can't be picked up yet
	taken    bool
}

// rock is a rock with coins inside.
type rock struct {
	position golib.Vector3 // its foot
	hits     int
	shake    golib.Timer // runs while it shakes from a punch
	broken   bool
}

// piece is a bit of a broken rock, flying and fading: for the eye only.
type piece struct {
	position, velocity golib.Vector3
	spin               golib.Rotation
	life               golib.Timer
}

// outcome is how the run ended.
type outcome int

const (
	running outcome = iota
	won             // the astronaut reached the flag
	timeUp          // the clock ran out
)

// player is the astronaut.
type player struct {
	position golib.Vector3 // its feet
	velocity golib.Vector3 // meters per second
	yaw      float32       // where it faces, in degrees, as golib.Rotation's Yaw
	onGround bool
	safe     golib.Vector3 // the last ground it stood on, well away from the edge
	splash   golib.Timer   // runs while it is under the sea, before it comes back
	punch    golib.Timer   // runs while a punch lasts
	landed   bool          // this punch has reached the moment the fist is out
	animator *golib.Animator
}

// speed is how fast the astronaut moves across the ground, in meters per second.
func (p *player) speed() float32 {
	return golib.Vector3{X: p.velocity.X, Z: p.velocity.Z}.Length()
}

// intent is what the player wants in one update, from the keys or the
// gamepad; the rules never see the input itself.
type intent struct {
	move   golib.Vector3 // a direction on the ground; its length, up to 1, is how hard the player pushes
	sprint bool          // held
	jump   bool          // pressed in this update
	punch  bool          // pressed in this update
}

// events is what happened in one step, for the scene's sounds and effects.
type events struct {
	jumped    bool
	landed    float32 // the speed the astronaut landed at; 0 if it didn't land
	coins     int     // coins picked up
	punched   bool
	rockHit   bool
	rockBroke bool
	splashed  bool
	won       bool
	timeUp    bool
	tick      bool // the clock passed a whole second in its last ten
}

// world is the whole game state and its rules. It knows nothing about the
// input or the screen, so the tests can play it directly.
type world struct {
	island   *island
	player   player
	coins    []*coin
	rocks    []*rock
	pieces   []piece
	clock    float32 // seconds left
	taken    int     // coins picked up
	outcome  outcome
	time     float32 // seconds since the run started, for things that bob and spin
	clockRun bool    // the clock counts down; not on the title screen
}

// newWorld returns the world at the start of a run: the astronaut at the
// start facing up the island, the coins and rocks where the island has them.
func newWorld(l *island) *world {
	w := &world{island: l, clock: startingTime, clockRun: true}
	w.player = player{position: l.start, safe: l.start, onGround: true, animator: golib.NewAnimator(astronautModel)}
	for _, at := range l.coins {
		w.coins = append(w.coins, &coin{position: at})
	}
	for _, at := range l.rocks {
		w.rocks = append(w.rocks, &rock{position: at})
	}
	w.pose()
	return w
}

// score is what the run is worth: its coins, and the seconds left once the
// flag is reached.
func (w *world) score() int {
	score := w.taken * coinScore
	if w.outcome == won {
		score += int(math.Ceil(float64(w.clock))) * secondScore
	}
	return score
}

// step advances the world by dt seconds with what the player wants.
func (w *world) step(want intent, dt float32) events {
	var happened events
	w.time += dt
	if w.outcome == running && w.clockRun {
		before := w.clock
		w.clock = max(w.clock-dt, 0)
		if before <= 10 && math.Ceil(float64(before)) != math.Ceil(float64(w.clock)) {
			happened.tick = true
		}
		if w.clock <= 0 {
			w.outcome = timeUp
			happened.timeUp = true
			w.player.punch.Stop()
		}
	}
	if w.outcome != running {
		want = intent{}
	}
	w.stepPlayer(want, dt, &happened)
	if w.outcome == won {
		w.player.yaw = turnTowards(w.player.yaw, w.inwardYaw(), turnSpeed/3*dt) // it turns round to cheer at the island
	}
	w.stepCoins(dt, &happened)
	w.stepRocks(dt)
	if w.outcome == running && w.reachedFlag() {
		w.outcome = won
		happened.won = true
		w.player.punch.Stop()
	}
	w.pose()
	w.player.animator.Update(dt)
	return happened
}

// stepPlayer runs, jumps, punches and falls as the player wants.
func (w *world) stepPlayer(want intent, dt float32, happened *events) {
	p := &w.player
	if p.splash.Running() {
		if p.splash.Tick(dt) {
			p.position, p.velocity, p.onGround = p.safe, golib.Vector3{}, true
		}
		return
	}
	push := golib.Clamp(want.move.Length(), 0, 1)
	top := float32(walkSpeed)
	if want.sprint {
		top = sprintSpeed
	}
	p.steer(want.move.Normalize().Scale(push*top), dt)
	if want.jump && p.onGround {
		p.velocity.Y = jumpSpeed
		p.onGround = false
		happened.jumped = true
		p.animator.PlayWith(clipJump.name, golib.AnimationOptions{Once: true, Restart: true, Fade: 0.08})
	}
	happened.landed = w.move(dt)
	if p.position.Y < seaLevel-sinkDepth {
		p.splash.Start(splashTime)
		happened.splashed = true
		if w.outcome == running && w.clockRun {
			w.clock = max(w.clock-splashCost, 0.01)
		}
		return
	}
	if p.onGround && w.island.highestAround(p.position.X, p.position.Z, 0.6) == p.position.Y {
		p.safe = p.position // all the ground around is as high: well away from any edge
	}

	if want.punch && !p.punch.Running() {
		w.aim()
		p.punch.Start(punchTime)
		p.landed = false
		happened.punched = true
		layerOver(p.animator, clipPunch, clipPunch.length()/punchTime)
	}
	if p.punch.Running() && !p.landed && punchTime-p.punch.Left() >= punchHitAt {
		p.landed = true
		w.hitRocks(happened)
	}
	p.punch.Tick(dt)
}

// steer moves the velocity on the ground towards target, with less grip in
// the air, and turns the astronaut to face where it is told to go.
func (p *player) steer(target golib.Vector3, dt float32) {
	accel := float32(groundAccel)
	if !p.onGround {
		accel = airAccel
	}
	flat := golib.Vector3{X: p.velocity.X, Z: p.velocity.Z}.MoveTowards(golib.Vector3{X: target.X, Z: target.Z}, accel*dt)
	p.velocity.X, p.velocity.Z = flat.X, flat.Z
	if target.Length() > 0.1 {
		p.yaw = turnTowards(p.yaw, golib.RotationTowards(target).Yaw, turnSpeed*dt)
	}
}

// move falls and moves the astronaut by its velocity, one way across the
// ground at a time, so it slides along a block's side instead of sticking to
// it, and keeps it out of trunks and rocks. It reports the speed it landed
// at, or 0 if it didn't land.
func (w *world) move(dt float32) (landed float32) {
	p := &w.player
	l := w.island
	feet := p.position.Y
	// Across the ground: each way on its own, undone if it walks into a
	// block higher than a step.
	next := p.position
	next.X += p.velocity.X * dt
	if l.highestAround(next.X, next.Z, bodyRadius) > feet+stepUp {
		next.X, p.velocity.X = p.position.X, 0
	}
	next.Z += p.velocity.Z * dt
	if l.highestAround(next.X, next.Z, bodyRadius) > feet+stepUp {
		next.Z, p.velocity.Z = p.position.Z, 0
	}
	next = w.pushOut(next)
	p.position.X, p.position.Z = next.X, next.Z

	// Up and down: fall onto the highest ground under the feet that isn't
	// above them by more than a step.
	ground := l.highestUnder(p.position.X, p.position.Z, footHalf, feet+stepUp)
	p.velocity.Y -= gravity * dt
	p.position.Y += p.velocity.Y * dt
	if ground > 0 && p.position.Y <= ground && (p.velocity.Y <= 0 || p.onGround) {
		if !p.onGround {
			landed = -p.velocity.Y
		}
		p.position.Y, p.velocity.Y, p.onGround = ground, 0, true
	} else {
		p.onGround = false
	}
	return landed
}

// pushOut moves a point on the ground out of the trees' trunks and the rocks
// that still stand, as far as the astronaut's body needs.
func (w *world) pushOut(at golib.Vector3) golib.Vector3 {
	push := func(from golib.Vector3, radius float32) {
		if at.Y > from.Y+1.2 || at.Y < from.Y-0.5 {
			return // far above or below it
		}
		way := golib.Vector3{X: at.X - from.X, Z: at.Z - from.Z}
		apart := radius + bodyRadius
		d := way.Length()
		if d >= apart {
			return
		}
		if d < 0.001 {
			way, d = golib.Vector3{Z: -1}, 1
		}
		out := way.Scale(apart / d)
		at.X, at.Z = from.X+out.X, from.Z+out.Z
	}
	for _, t := range w.island.trees {
		push(t.position, treeRadius)
	}
	for _, r := range w.rocks {
		if !r.broken {
			push(r.position, rockRadius)
		}
	}
	return at
}

// aim turns the astronaut, as a punch starts, to the nearest rock close
// enough to punch and roughly in front, so a punch that is nearly right lands.
func (w *world) aim() {
	p := &w.player
	var nearest *rock
	for _, r := range w.rocks {
		if r.broken || !inArc(p, r.position, punchReach, aimArc) {
			continue
		}
		if nearest == nil || r.position.Distance(p.position) < nearest.position.Distance(p.position) {
			nearest = r
		}
	}
	if nearest != nil {
		p.yaw = golib.RotationTowards(golib.Vector3{X: nearest.position.X - p.position.X, Z: nearest.position.Z - p.position.Z}).Yaw
	}
}

// hitRocks lands the punch on every rock in front of the astronaut, within
// reach, and breaks open the ones that have taken enough.
func (w *world) hitRocks(happened *events) {
	p := &w.player
	for _, r := range w.rocks {
		if r.broken || !inArc(p, r.position, punchReach, punchArc) {
			continue
		}
		r.hits++
		r.shake.Start(shakeTime)
		happened.rockHit = true
		if r.hits >= rockHits {
			w.breakRock(r)
			happened.rockBroke = true
		}
	}
}

// breakRock breaks a rock open: its coins fly out, away from the astronaut
// and to the sides, and its pieces fly everywhere.
func (w *world) breakRock(r *rock) {
	r.broken = true
	away := golib.Vector3{X: r.position.X - w.player.position.X, Z: r.position.Z - w.player.position.Z}.Normalize()
	if away.Length() == 0 {
		away = golib.Vector3{Z: 1}
	}
	for i := range rockCoins {
		turn := float32(i-(rockCoins-1)/2) * 50 // degrees: one ahead, the others to the sides
		out := away.Rotate(golib.Rotation{Yaw: turn}).Scale(coinPopOut)
		w.coins = append(w.coins, &coin{
			position: r.position.Add(golib.Vector3{Y: 0.4}),
			home:     r.position.Add(golib.Vector3{Y: 0.4}),
			velocity: golib.Vector3{X: out.X, Y: coinPopUp, Z: out.Z},
			flying:   true,
			wait:     golib.NewTimer(coinWait),
		})
	}
	for range debrisCount {
		way := golib.Vector3{X: golib.RandomFloat(-1, 1), Z: golib.RandomFloat(-1, 1)}.Normalize().Scale(golib.RandomFloat(1.5, 3))
		w.pieces = append(w.pieces, piece{
			position: r.position.Add(golib.Vector3{Y: 0.3}),
			velocity: golib.Vector3{X: way.X, Y: golib.RandomFloat(3, 6), Z: way.Z},
			spin:     golib.Rotation{Yaw: golib.RandomFloat(0, 360), Pitch: golib.RandomFloat(-60, 60)},
			life:     golib.NewTimer(debrisTime),
		})
	}
}

// inArc says whether a point is within reach of the astronaut, measured on
// the ground, and no more than arc degrees to either side of where it faces.
func inArc(p *player, at golib.Vector3, reach, arc float32) bool {
	way := golib.Vector3{X: at.X - p.position.X, Z: at.Z - p.position.Z}
	if way.Length() > reach || abs(at.Y-p.position.Y) > 0.6 {
		return false
	}
	turn := angleBetween(p.yaw, golib.RotationTowards(way).Yaw)
	return turn >= -arc && turn <= arc
}

// stepCoins flies the coins out of broken rocks until they land, and picks
// up the ones the astronaut touches.
func (w *world) stepCoins(dt float32, happened *events) {
	p := &w.player
	middle := p.position.Add(golib.Vector3{Y: 0.4})
	for _, c := range w.coins {
		if c.taken {
			continue
		}
		c.wait.Tick(dt)
		if c.flying {
			c.velocity.Y -= gravity * dt
			next := c.position.Add(c.velocity.Scale(dt))
			// A coin stops against the side of a block rather than going into it.
			bottom := c.position.Y - coinHover
			if w.island.highestAround(next.X, next.Z, 0.1) > bottom+0.05 {
				next.X, next.Z = c.position.X, c.position.Z
				c.velocity.X, c.velocity.Z = 0, 0
			}
			ground := w.island.highestUnder(next.X, next.Z, 0.1, bottom+0.05)
			if ground > 0 && next.Y <= ground+coinHover && c.velocity.Y <= 0 {
				next.Y, c.velocity, c.flying = ground+coinHover, golib.Vector3{}, false
			}
			c.position = next
			if c.position.Y < seaLevel-1 { // into the sea: it drops again where its rock stood
				c.position, c.velocity = c.home, golib.Vector3{}
			}
		}
		if w.outcome != running || c.wait.Running() || p.splash.Running() {
			continue
		}
		if c.position.Distance(middle) < coinReach {
			c.taken = true
			w.taken++
			w.clock += coinTime
			happened.coins++
			if !p.punch.Running() {
				layerOver(p.animator, clipPickUp, 1)
			}
		}
	}
}

// stepRocks moves the shakes on, and the pieces of broken rocks.
func (w *world) stepRocks(dt float32) {
	for _, r := range w.rocks {
		r.shake.Tick(dt)
	}
	kept := w.pieces[:0]
	for _, p := range w.pieces {
		p.life.Tick(dt)
		if !p.life.Running() {
			continue
		}
		p.velocity.Y -= gravity * dt
		p.position = p.position.Add(p.velocity.Scale(dt))
		p.spin = p.spin.Turn(golib.Vector3{X: 1}, 400*dt)
		kept = append(kept, p)
	}
	w.pieces = kept
}

// inwardYaw is the yaw that faces from the astronaut to the middle of the
// island, where the end's camera looks at it from.
func (w *world) inwardYaw() float32 {
	return golib.RotationTowards(golib.Vector3{X: -w.player.position.X, Z: -w.player.position.Z}).Yaw
}

// reachedFlag says whether the astronaut stands at the flag.
func (w *world) reachedFlag() bool {
	p := &w.player
	flag := w.island.flag
	way := golib.Vector3{X: flag.X - p.position.X, Z: flag.Z - p.position.Z}
	return way.Length() < flagReach && abs(p.position.Y-flag.Y) < 0.5
}

// coinsLeft is how many coins can still be picked up, the ones inside rocks
// that still stand included.
func (w *world) coinsLeft() int {
	left := 0
	for _, c := range w.coins {
		if !c.taken {
			left++
		}
	}
	for _, r := range w.rocks {
		if !r.broken {
			left += rockCoins
		}
	}
	return left
}

// turnTowards turns the yaw from towards to by at most maxTurn degrees, the
// shortest way round.
func turnTowards(from, to, maxTurn float32) float32 {
	turn := angleBetween(from, to)
	if abs(turn) <= maxTurn {
		return to
	}
	if turn < 0 {
		return from - maxTurn
	}
	return from + maxTurn
}

// angleBetween returns the shortest turn from one yaw to another, from -180
// to 180 degrees.
func angleBetween(from, to float32) float32 {
	turn := float32(math.Mod(float64(to-from), 360))
	if turn > 180 {
		turn -= 360
	}
	if turn < -180 {
		turn += 360
	}
	return turn
}

func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}
