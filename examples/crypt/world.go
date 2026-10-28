package main

import (
	"math"
	"slices"

	"golib"
)

// Tuning: the numbers that define how the game feels. Distances are in
// meters and times in seconds, as in golib's 3D: Y is up, and the crypt runs
// north along +Z from the entry hall around 0, 0, 0.
const (
	playerHeight = 0.9  // meters: Kenney's astronaut is small
	playerRadius = 0.25 // meters
	stepHeight   = 0.27 // meters: a stair is 0.25 high
	runSpeed     = 3.6  // meters per second with the stick or keys all the way
	groundAccel  = 30   // meters per second per second: how fast running starts and stops
	airAccel     = 8    // the same while falling
	turnSpeed    = 720  // degrees per second the astronaut turns to face where it runs
	gravity      = 22   // meters per second per second
	maxFallSpeed = 18   // meters per second

	punchReach = 0.9  // meters between the astronaut and a crate's side for a punch to reach it
	punchTime  = 0.42 // seconds a punch lasts, and the wait before the next
	punchHitAt = 0.16 // seconds into it when the fist lands
	keyReach   = 0.9  // meters to a key to pick it up
	doorReach  = 0.4  // meters between the astronaut and a locked door to try it
	exitReach  = 1.0  // meters to the way out
	checkReach = 1.8  // meters to a checkpoint to reach it

	startHearts = 3
	caughtTime  = 1.6 // seconds the astronaut lies grabbed before it starts again at the checkpoint
	messageTime = 2.6 // seconds a message shows

	liftSpeed = 1.3 // meters per second
	liftWait  = 2.5 // seconds the lift waits at each floor

	stepLength  = 0.62 // meters between footsteps
	debrisLife  = 1.4  // seconds a splinter or a crumb of stone lasts
	doorFadeOut = 0.5  // seconds an opened door takes to vanish
)

// The astronaut's animations, as Kenney's Platformer Kit names them (see
// TestTheCharacterHasItsAnimations), and the bone the upper body's play on.
const (
	animIdle     = "idle"
	animWalk     = "walk"
	animSprint   = "sprint"
	animFall     = "fall"
	animDie      = "die"
	animCheer    = "emote-yes"
	animPunch    = "attack-melee-right"
	animPickUp   = "pick-up"
	animInteract = "interact-right"
	animReach    = "holding-both" // a guard's arms held out to grab
	animGrab     = "attack-melee-left"
	boneTorso    = "torso"
)

// intent is what the player wants in one update, from the keys, the mouse or
// the gamepad; the rules never see the input itself.
type intent struct {
	move      golib.Vector3 // a direction on the ground; its length, up to 1, is how hard the player pushes
	smash     int           // the crate the player clicked, to break it, or -1
	smashNear bool          // break the crate in front, with the keyboard or the gamepad
}

// noIntent is an update with nothing pressed.
var noIntent = intent{smash: -1}

// events is what happened in one step, for the scene's sounds and effects.
type events struct {
	punched     bool
	broke       []golib.Vector3 // crates that broke
	keyTaken    bool
	opened      []golib.Vector3 // doors that opened
	locked      bool            // the player tried a locked door with no key
	caught      bool
	gameOver    bool
	restarted   bool // back at the checkpoint after a grab
	checkpoint  bool
	won         bool
	playerStep  bool
	steps       []golib.Vector3 // the guards' footsteps
	spotted     []golib.Vector3 // guards that just saw the player
	liftMoving  bool            // the lift moves in this step
	liftStopped bool            // the lift arrived at a floor
}

// player is the astronaut.
type player struct {
	body     golib.CharacterController
	velocity golib.Vector3
	yaw      float32 // where it faces, in degrees, as golib.Rotation's Yaw
	animator *golib.Animator
	punch    golib.Timer // runs while a punch lasts
	target   int         // the crate the punch is for
	landed   bool        // the punch has reached the moment the fist lands
	caught   golib.Timer // runs while it lies grabbed
	walked   float32     // meters walked since the last footstep
}

// speed is how fast it moves across the ground, in meters per second.
func (p *player) speed() float32 {
	return flat(p.velocity).Length()
}

// door is a locked door, solid until a key opens it.
type door struct {
	spec boxSpec
	body int
	open bool
	gone golib.Timer // runs while it vanishes
}

// crate is a crate in the way; one hides a key.
type crate struct {
	spec   boxSpec
	body   int
	broken bool
}

// key is a key to pick up: lying in the open, or out of a broken crate.
type key struct {
	name     string
	position golib.Vector3 // where it lies: the floor under it
	taken    bool
	popped   float32 // seconds since it came out of a crate; it hops out
	fromBox  bool
}

// debris is a splinter of a broken crate or a crumb of a vanished door.
type debris struct {
	position golib.Vector3
	velocity golib.Vector3
	turn     golib.Rotation
	spin     float32 // degrees per second
	size     float32
	floor    float32 // the height it rests on
	life     float32 // seconds left
	stone    bool
}

// liftState is what the lift is doing.
type liftState int

const (
	liftDown liftState = iota // waiting at the bottom
	liftRising
	liftUp // waiting at the top
	liftSinking
)

// lift is the stone lift: it waits at each floor, then goes to the other.
type lift struct {
	spec   liftSpec
	body   int
	height float32 // meters above the bottom of its shaft
	state  liftState
	wait   golib.Timer
}

// progress is what the game remembers between runs: where the player got
// to, saved at each checkpoint, so they can carry on from there.
type progress struct {
	Checkpoint int      // the last checkpoint reached, by its place in the level
	KeysFound  int      // keys picked up
	KeysHeld   int      // keys picked up and not yet used on a door
	Hearts     int      // hearts left
	Opened     []string // the doors opened
	Broken     []string // the crates broken
	Taken      []string // the keys picked up, by the name of their marker or crate
	Time       float32  // seconds played
}

// world is the whole game state and its rules. It knows nothing about the
// input or the screen, so the tests can play it directly.
type world struct {
	level      *level
	physics    *golib.PhysicsWorld
	nav        *golib.NavMesh
	navDirty   bool // something changed where the guards can walk
	levelBody  int
	player     player
	guards     []*guard
	doors      []*door
	crates     []*crate
	keys       []*key
	lift       lift
	debris     []debris
	keysFound  int
	keysHeld   int
	hearts     int
	checkpoint int
	time       float32 // seconds played
	message    string
	messageFor golib.Timer
	won        bool
	over       bool // no hearts left
}

// newWorld returns the crypt at the start, or as saved, when from isn't nil.
func newWorld(l *level, from *progress) *world {
	w := &world{level: l, physics: golib.NewPhysicsWorld(), hearts: startHearts}
	w.levelBody = w.physics.AddModel(levelModel, golib.Transform{})
	if from != nil {
		w.checkpoint = min(max(from.Checkpoint, 0), len(l.checkpoints)-1)
		w.keysFound, w.keysHeld, w.time = from.KeysFound, from.KeysHeld, from.Time
		if from.Hearts > 0 {
			w.hearts = from.Hearts
		}
	}
	has := func(list []string, name string) bool {
		return from != nil && slices.Contains(list, name)
	}
	for _, spec := range l.doors {
		d := &door{spec: spec, open: has(from.openedList(), spec.name)}
		if !d.open {
			d.body = w.physics.AddMesh(boxMesh, golib.Transform{Position: spec.center, Scale: spec.size})
		}
		w.doors = append(w.doors, d)
	}
	for _, spec := range l.crates {
		c := &crate{spec: spec, broken: has(from.brokenList(), spec.name)}
		if !c.broken {
			c.body = w.physics.AddMesh(boxMesh, golib.Transform{Position: spec.center, Scale: spec.size})
		} else if spec.key && !has(from.takenList(), spec.name) {
			w.keys = append(w.keys, &key{name: spec.name, position: crateFloor(spec), fromBox: true, popped: 10})
		}
		w.crates = append(w.crates, c)
	}
	for _, spec := range l.keys {
		w.keys = append(w.keys, &key{name: spec.name, position: spec.center, taken: has(from.takenList(), spec.name)})
	}
	w.lift = lift{spec: l.lift, wait: golib.NewTimer(liftWait)}
	w.lift.body = w.physics.AddMesh(boxMesh, w.lift.transform())
	w.nav = golib.NewNavMesh(w.physics, guardBody(golib.Vector3{}))

	cp := l.checkpoints[w.checkpoint]
	w.player = player{animator: golib.NewAnimator(characterModel), target: -1}
	w.placePlayer(cp.position, cp.yaw)
	for _, spec := range l.guards {
		w.guards = append(w.guards, newGuard(spec))
	}
	w.poseAll()
	return w
}

// The lists of a progress that may be nil.
func (p *progress) openedList() []string {
	if p == nil {
		return nil
	}
	return p.Opened
}

func (p *progress) brokenList() []string {
	if p == nil {
		return nil
	}
	return p.Broken
}

func (p *progress) takenList() []string {
	if p == nil {
		return nil
	}
	return p.Taken
}

// progress is what to save of the world, at a checkpoint.
func (w *world) progress() progress {
	p := progress{Checkpoint: w.checkpoint, KeysFound: w.keysFound, KeysHeld: w.keysHeld, Hearts: w.hearts, Time: w.time}
	for _, d := range w.doors {
		if d.open {
			p.Opened = append(p.Opened, d.spec.name)
		}
	}
	for _, c := range w.crates {
		if c.broken {
			p.Broken = append(p.Broken, c.spec.name)
		}
	}
	for _, k := range w.keys {
		if k.taken {
			p.Taken = append(p.Taken, k.name)
		}
	}
	return p
}

// placePlayer stands the astronaut at feet, facing yaw, at rest.
func (w *world) placePlayer(feet golib.Vector3, yaw float32) {
	p := &w.player
	p.body = golib.CharacterController{Position: feet, Height: playerHeight, Radius: playerRadius, StepHeight: stepHeight}
	p.velocity = golib.Vector3{}
	p.yaw = yaw
}

// crateFloor is the floor under a crate's middle, where its key falls.
func crateFloor(spec boxSpec) golib.Vector3 {
	return golib.Vector3{X: spec.center.X, Y: spec.center.Y - spec.size.Y/2, Z: spec.center.Z}
}

// say shows a message for a moment, and reports whether it wasn't showing
// already.
func (w *world) say(text string) bool {
	if w.message == text && w.messageFor.Running() {
		return false
	}
	w.message = text
	w.messageFor.Start(messageTime)
	return true
}

// step advances the world by dt seconds with what the player wants.
func (w *world) step(want intent, dt float32) events {
	var happened events
	if w.won || w.over {
		w.animate(dt)
		return happened
	}
	w.time += dt
	w.messageFor.Tick(dt)

	// The lift moves first: golib carries whoever stands on it at their
	// next Move.
	w.stepLift(dt, &happened)
	w.stepPlayer(want, dt, &happened)
	for _, g := range w.guards {
		w.stepGuard(g, dt, &happened)
	}
	w.separateGuards()
	w.stepDebris(dt)
	for _, d := range w.doors {
		d.gone.Tick(dt)
	}
	if w.navDirty {
		w.nav = golib.NewNavMesh(w.physics, guardBody(golib.Vector3{}))
		w.navDirty = false
	}
	w.poseAll()
	w.animate(dt)
	return happened
}

// stepPlayer runs as the player wants, punches crates, picks up keys, tries
// doors and reaches checkpoints and the way out.
func (w *world) stepPlayer(want intent, dt float32, happened *events) {
	p := &w.player
	if p.caught.Running() {
		if p.caught.Tick(dt) {
			w.startAgain()
			happened.restarted = !w.over
		}
		return
	}
	if p.punch.Running() {
		want.move = golib.Vector3{} // a punch roots the astronaut to the spot
	}
	push := golib.Clamp(want.move.Length(), 0, 1)
	steer(&p.velocity, &p.yaw, want.move.Normalize().Scale(push*runSpeed), p.body.OnGround, dt)
	if p.punch.Running() {
		if c := w.crateAt(p.target); c != nil {
			p.yaw = turnTowards(p.yaw, golib.RotationTowards(flat(c.spec.center.Sub(p.body.Position))).Yaw, turnSpeed*dt)
		}
	}
	before := p.body.Position
	moveBody(&p.body, &p.velocity, w.physics, dt)
	if p.body.OnGround {
		p.walked += flat(p.body.Position.Sub(before)).Length()
		if p.walked >= stepLength {
			p.walked = 0
			happened.playerStep = true
		}
	}

	w.stepPunch(want, dt, happened)
	w.pickUpKeys(happened)
	w.tryDoors(happened)
	w.reachCheckpoints(happened)
	if p.body.Position.Distance(w.level.exit) < exitReach {
		if w.keysFound >= w.level.keysInLevel() {
			w.won = true
			happened.won = true
		} else {
			w.say("The way out is sealed: find every key")
		}
	}
}

// steer moves a velocity on the ground towards target, with less grip in the
// air, and turns yaw to face where it is told to go.
func steer(velocity *golib.Vector3, yaw *float32, target golib.Vector3, onGround bool, dt float32) {
	accel := float32(groundAccel)
	if !onGround {
		accel = airAccel
	}
	moved := flat(*velocity).MoveTowards(flat(target), accel*dt)
	velocity.X, velocity.Z = moved.X, moved.Z
	if flat(target).Length() > 0.1 {
		*yaw = turnTowards(*yaw, golib.RotationTowards(flat(target)).Yaw, turnSpeed*dt)
	}
}

// moveBody falls and moves a character through the crypt by its velocity;
// standing on the lift, golib carries it up or down. Its velocity across
// the ground becomes how it really moved, so a character pushing into a
// wall slides along it rather than running on the spot, and the camera
// swings round behind where it goes, not where it pushes.
func moveBody(body *golib.CharacterController, velocity *golib.Vector3, physics *golib.PhysicsWorld, dt float32) {
	velocity.Y = max(velocity.Y-gravity*dt, -maxFallSpeed)
	body.Move(physics, velocity.Scale(dt))
	moved := flat(body.Moved).Scale(1 / dt)
	velocity.X, velocity.Z = moved.X, moved.Z
	if body.OnGround && velocity.Y < 0 || body.OnCeiling && velocity.Y > 0 {
		velocity.Y = 0
	}
}

// stepPunch starts a punch at the crate the player clicked, or at the one in
// front, and breaks it when the fist lands.
func (w *world) stepPunch(want intent, dt float32, happened *events) {
	p := &w.player
	if !p.punch.Running() {
		target := -1
		switch {
		case want.smash >= 0:
			if w.inReach(want.smash) {
				target = want.smash
			} else {
				w.say("Too far: walk up to the crate")
			}
		case want.smashNear:
			target = w.nearestCrate()
		}
		if target >= 0 {
			p.punch.Start(punchTime)
			p.target, p.landed = target, false
			happened.punched = true
			p.animator.PlayWith(animPunch, golib.AnimationOptions{Bone: boneTorso, Once: true, Restart: true, Fade: 0.05,
				Speed: characterModel.AnimationLength(animPunch) / punchTime})
		}
	}
	if p.punch.Running() && !p.landed && punchTime-p.punch.Left() >= punchHitAt {
		p.landed = true
		if c := w.crateAt(p.target); c != nil {
			w.breakCrate(c, happened)
		}
	}
	p.punch.Tick(dt)
}

// pickUpKeys picks up the keys the astronaut walks over.
func (w *world) pickUpKeys(happened *events) {
	p := &w.player
	for _, k := range w.keys {
		if k.taken || (k.fromBox && k.popped < 0.5) {
			continue
		}
		if flat(k.position.Sub(p.body.Position)).Length() < keyReach && abs(k.position.Y-p.body.Position.Y) < 1 {
			k.taken = true
			w.keysFound++
			w.keysHeld++
			happened.keyTaken = true
			p.animator.PlayWith(animPickUp, golib.AnimationOptions{Bone: boneTorso, Once: true, Restart: true})
			left := w.level.keysInLevel() - w.keysFound
			switch left {
			case 0:
				w.say("The last key! Find the way out")
			case 1:
				w.say("A key! One more to find")
			default:
				w.say("A key! It opens a locked door")
			}
		}
	}
}

// tryDoors opens a locked door the astronaut walks up to, with a key, or
// says it is locked.
func (w *world) tryDoors(happened *events) {
	p := &w.player
	for _, d := range w.doors {
		if d.open || gapToBox(p.body.Position, playerHeight, d.spec) > doorReach {
			continue
		}
		if w.keysHeld == 0 {
			happened.locked = w.say("Locked. Find a key") || happened.locked
			continue
		}
		w.keysHeld--
		d.open = true
		d.gone.Start(doorFadeOut)
		w.physics.Remove(d.body)
		w.navDirty = true
		happened.opened = append(happened.opened, d.spec.center)
		w.burst(d.spec.center, d.spec.size.Y, 18, true)
		p.animator.PlayWith(animInteract, golib.AnimationOptions{Bone: boneTorso, Once: true, Restart: true})
		w.say("The key turns, and the door crumbles away")
	}
}

// reachCheckpoints moves the checkpoint on when the astronaut reaches the
// next one.
func (w *world) reachCheckpoints(happened *events) {
	p := &w.player
	for i, cp := range w.level.checkpoints {
		if i > w.checkpoint && p.body.Position.Distance(cp.position) < checkReach {
			w.checkpoint = i
			happened.checkpoint = true
		}
	}
}

// grab is a guard catching the astronaut: a heart lost, and back to the
// checkpoint, unless that was the last heart.
func (w *world) grab(happened *events) {
	p := &w.player
	if p.caught.Running() {
		return
	}
	w.hearts--
	happened.caught = true
	p.caught.Start(caughtTime)
	p.punch.Stop()
	p.velocity = golib.Vector3{}
	p.animator.Stop(boneTorso)
	if w.hearts <= 0 {
		happened.gameOver = true
	}
}

// startAgain puts the astronaut back at the last checkpoint and the guards
// back on their rounds, after a grab.
func (w *world) startAgain() {
	if w.hearts <= 0 {
		w.over = true
		return
	}
	cp := w.level.checkpoints[w.checkpoint]
	w.placePlayer(cp.position, cp.yaw)
	w.player.animator.PlayWith(animIdle, golib.AnimationOptions{NoFade: true})
	for i, g := range w.guards {
		w.guards[i] = newGuard(g.spec)
	}
}

// animate moves every character's animations on.
func (w *world) animate(dt float32) {
	w.player.animator.Update(dt)
	for _, g := range w.guards {
		g.animator.Update(dt)
	}
	for _, k := range w.keys {
		k.popped += dt
	}
}

// keysLeft is how many keys are still to be found.
func (w *world) keysLeft() int {
	return w.level.keysInLevel() - w.keysFound
}

// Small geometry.

// flat is a vector laid on the ground.
func flat(v golib.Vector3) golib.Vector3 {
	return golib.Vector3{X: v.X, Z: v.Z}
}

// gapToBox is how far a character standing at feet, height tall, is from a
// box's sides, across the ground; a large number when it is above or below.
func gapToBox(feet golib.Vector3, height float32, box boxSpec) float32 {
	half := box.size.Scale(0.5)
	if feet.Y > box.center.Y+half.Y+0.3 || feet.Y+height < box.center.Y-half.Y {
		return math.MaxFloat32
	}
	dx := max(abs(feet.X-box.center.X)-half.X, 0)
	dz := max(abs(feet.Z-box.center.Z)-half.Z, 0)
	return golib.Vector3{X: dx, Z: dz}.Length() - playerRadius
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

// angleBetween is the shortest turn from one yaw to another, from -180 to
// 180 degrees.
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
	return max(x, -x)
}
