package main

import (
	"math"

	"golib"
)

// Tuning: the numbers that define how the game feels. Distances are in
// meters and times in seconds, as in golib's 3D: Y is up, and the courtyard
// lies on the ground around 0, 0, 0.
const (
	arenaHalf  = 11.0 // meters from the middle of the courtyard to the inside of its walls
	wellRadius = 1.15 // meters: the well in the middle, which nobody walks through
	bodyRadius = 0.32 // meters: every character's size on the ground

	walkSpeed   = 1.9 // meters per second at the top of a walk: a gentle push
	sprintSpeed = 5.2 // meters per second, pushed all the way
	walkPush    = 0.5 // how far the stick tilts before the walk turns into a sprint
	groundAccel = 32  // meters per second per second: how fast running starts and stops
	airAccel    = 12  // the same in the air
	turnSpeed   = 720 // degrees per second a character turns to face where it runs
	gravity     = 26  // meters per second per second
	jumpSpeed   = 7.6 // meters per second up: a jump rises jumpSpeed²/2g, about 1.1 m

	swingTime     = 0.36 // seconds a swing of the stick lasts, and the wait before the next
	swingHitAt    = 0.15 // seconds into a swing when the stick lands
	swingReach    = 1.35 // meters from the player's feet to an enemy's it can whack
	swingArc      = 80   // degrees either side of where the player faces that the stick sweeps
	aimReach      = 0.6  // meters beyond the stick's reach at which a swing turns the player to an enemy
	aimArc        = 180  // degrees either side of where the player faces it looks for one: all round
	knockback     = 5.5  // meters per second a whack sends an enemy flying
	knockHop      = 3.6  // meters per second up it bounces
	playerHealth  = 5
	hurtTime      = 1.0 // seconds after a punch in which the player can't be hurt again
	hurtKnockback = 4.0 // meters per second a punch pushes the player
	deathDelay    = 2.2 // seconds between the last heart and the game over screen

	firstWaveDelay = 1.6 // seconds before the first wave comes in
	waveBreak      = 3.4 // seconds between a cleared wave and the next
	nodDelay       = 0.5 // seconds after the last knockout before the player nods
	knockoutScore  = 100
	whackScore     = 10
	waveBonus      = 50 // times the wave's number, for clearing it
)

// The character's animations, as Kenney's Platformer Kit names them, and the
// bones they play on (see TestTheCharacterHasItsAnimations).
const (
	animIdle   = "idle"
	animWalk   = "walk"
	animSprint = "sprint"
	animJump   = "jump"
	animFall   = "fall"
	animDie    = "die"
	animNod    = "emote-yes"
	animHold   = "holding-right"      // the right arm holding something out
	animSwing  = "attack-melee-right" // a swing with the right arm
	animPunch  = "attack-melee-left"  // the enemies punch with the left

	boneTorso    = "torso"     // the body, the head and both arms: swings play here, over the legs
	boneRightArm = "arm-right" // the hand that holds the stick
)

// character is what the player and the enemies share: a body that moves on
// the ground and jumps, and the animations that show it.
type character struct {
	position golib.Vector3 // its feet
	velocity golib.Vector3 // meters per second
	yaw      float32       // where it faces, in degrees, as golib.Rotation's Yaw
	onGround bool
	animator *golib.Animator
}

// speed is how fast it moves across the ground, in meters per second.
func (c *character) speed() float32 {
	return golib.Vector3{X: c.velocity.X, Z: c.velocity.Z}.Length()
}

// steer moves the velocity on the ground towards target, with less grip in
// the air, and turns the character to face where it is told to go.
func (c *character) steer(target golib.Vector3, dt float32) {
	accel := float32(groundAccel)
	if !c.onGround {
		accel = airAccel
	}
	flat := golib.Vector3{X: c.velocity.X, Z: c.velocity.Z}.MoveTowards(golib.Vector3{X: target.X, Z: target.Z}, accel*dt)
	c.velocity.X, c.velocity.Z = flat.X, flat.Z
	if target.Length() > 0.1 {
		c.yaw = turnTowards(c.yaw, golib.RotationTowards(target).Yaw, turnSpeed*dt)
	}
}

// move falls, moves the character by its velocity, and keeps it on the
// ground, inside the walls and out of the well. It reports the speed it
// landed at, or 0 if it didn't land.
func (c *character) move(dt float32) (landed float32) {
	c.velocity.Y -= gravity * dt
	c.position = c.position.Add(c.velocity.Scale(dt))
	if c.position.Y <= 0 {
		if !c.onGround {
			landed = -c.velocity.Y
		}
		c.position.Y, c.velocity.Y = 0, 0
		c.onGround = true
	} else {
		c.onGround = false
	}
	c.position = keepInside(c.position)
	return landed
}

// keepInside moves a point on the ground back inside the courtyard's walls
// and out of the well, as far as a character's body needs.
func keepInside(at golib.Vector3) golib.Vector3 {
	const edge = arenaHalf - bodyRadius
	at.X = golib.Clamp(at.X, -edge, edge)
	at.Z = golib.Clamp(at.Z, -edge, edge)
	flat := golib.Vector3{X: at.X, Z: at.Z}
	if d := flat.Length(); d < wellRadius+bodyRadius {
		out := flat.Normalize()
		if d < 0.001 {
			out = golib.Vector3{Z: -1}
		}
		out = out.Scale(wellRadius + bodyRadius)
		at.X, at.Z = out.X, out.Z
	}
	return at
}

// player is the astronaut with the stick.
type player struct {
	character
	health   int
	hurt     golib.Timer // runs after a punch: no punch hurts meanwhile
	swing    golib.Timer // runs while a swing lasts
	landed   bool        // this swing has reached the moment the stick lands
	nod      golib.Timer // runs while it nods after a cleared wave
	nodLater golib.Timer // runs from the last knockout to the nod
}

// alive says whether the player still has a heart left.
func (p *player) alive() bool {
	return p.health > 0
}

// intent is what the player wants in one update, from the keys or the
// gamepad; the rules never see the input itself.
type intent struct {
	move  golib.Vector3 // a direction on the ground; its length, up to 1, is how hard the player pushes
	jump  bool          // jump was pressed in this update
	swing bool          // swing was pressed in this update
}

// events is what happened in one step, for the scene's sounds and effects.
type events struct {
	jumped      bool
	landed      float32 // the speed the player landed at; 0 if it didn't land
	swung       bool
	whacks      int // enemies the stick hit
	knockouts   int // enemies knocked out
	hurt        bool
	died        bool
	waveStarted bool
	waveCleared bool
}

// world is the whole game state and its rules. It knows nothing about the
// input or the screen, so world_test.go can play it directly.
type world struct {
	player  player
	enemies []*enemy
	wave    int // the wave being fought, from 1; 0 before the first
	score   int
	next    golib.Timer // runs until the next wave comes in
	ending  golib.Timer // runs from the player's last heart to the game over screen
	over    bool        // the game over screen is due
	time    float32     // seconds since the game started
}

// playerStart is where the player starts, beside the well, facing +Z.
var playerStart = golib.Vector3{X: -4, Z: -5}

// newWorld returns the world at the start of a game: the player beside the
// well, facing the far wall, and the first wave on its way.
func newWorld() *world {
	w := &world{next: golib.NewTimer(firstWaveDelay)}
	w.player = player{
		character: character{position: playerStart, onGround: true, animator: golib.NewAnimator(characterModel)},
		health:    playerHealth,
	}
	// The right arm holds the stick out, on a layer of its own, whatever the
	// rest of the body plays.
	w.player.animator.PlayWith(animHold, golib.AnimationOptions{Bone: boneRightArm})
	w.poseAll()
	return w
}

// step advances the world by dt seconds with what the player wants.
func (w *world) step(want intent, dt float32) events {
	var happened events
	w.time += dt
	p := &w.player
	p.hurt.Tick(dt)

	if p.alive() {
		w.stepPlayer(want, dt, &happened)
	} else {
		p.steer(golib.Vector3{}, dt)
		p.move(dt)
		if w.ending.Tick(dt) {
			w.over = true
		}
	}
	for _, e := range w.enemies {
		w.stepEnemy(e, dt, &happened)
	}
	w.separate()
	w.removeFaded()
	w.stepWaves(dt, &happened)
	w.poseAll()
	w.animate(dt)
	return happened
}

// stepPlayer runs, jumps and swings the stick as the player wants.
func (w *world) stepPlayer(want intent, dt float32, happened *events) {
	p := &w.player
	push := golib.Clamp(want.move.Length(), 0, 1)
	p.steer(want.move.Normalize().Scale(pushSpeed(push)), dt)
	if want.jump && p.onGround {
		p.velocity.Y = jumpSpeed
		p.onGround = false
		happened.jumped = true
	}
	happened.landed = p.move(dt)

	// A swing starts when the last one is over, whatever the legs do.
	if want.swing && !p.swing.Running() {
		w.aim()
		p.swing.Start(swingTime)
		p.landed = false
		happened.swung = true
		p.animator.PlayWith(animSwing, golib.AnimationOptions{Bone: boneTorso, Once: true, Restart: true, Fade: 0.05, Speed: swingSpeed})
	}
	if p.swing.Running() && !p.landed && swingTime-p.swing.Left() >= swingHitAt {
		p.landed = true
		w.whack(happened)
	}
	p.swing.Tick(dt)

	if p.nodLater.Tick(dt) {
		p.nod.Start(nodLength)
		p.animator.PlayWith(animNod, golib.AnimationOptions{Bone: boneTorso, Once: true, Restart: true})
	}
	p.nod.Tick(dt)
}

// The lengths of the animations the rules time, in seconds, as the kit's
// file has them: golib says how far into the animation the whole model
// plays is, but not how long an animation lasts.
const (
	swingLength = 0.417 // attack-melee-right
	nodLength   = 0.667 // emote-yes
	dieLength   = 0.333 // die
	punchLength = 0.417 // attack-melee-left
)

// swingSpeed plays the kit's swing in swingTime seconds.
const swingSpeed = swingLength / swingTime

// aim turns the player, as a swing starts, to the nearest enemy close enough
// to whack and roughly in front, so a swing that is nearly right lands.
func (w *world) aim() {
	p := &w.player
	var nearest *enemy
	for _, e := range w.enemies {
		if e.down() || !inArc(p.character, e.position, swingReach+aimReach, aimArc) {
			continue
		}
		if nearest == nil || e.position.Distance(p.position) < nearest.position.Distance(p.position) {
			nearest = e
		}
	}
	if nearest != nil {
		p.yaw = golib.RotationTowards(golib.Vector3{X: nearest.position.X - p.position.X, Z: nearest.position.Z - p.position.Z}).Yaw
	}
}

// pushSpeed is how fast the player runs for a push from 0 to 1: up to the
// walk's top speed for a gentle push, then up to a sprint.
func pushSpeed(push float32) float32 {
	if push < 0.05 {
		return 0
	}
	if push <= walkPush {
		return walkSpeed * push / walkPush
	}
	return golib.Lerp(walkSpeed, sprintSpeed, (push-walkPush)/(1-walkPush))
}

// whack hits every enemy in front of the player within the stick's reach.
func (w *world) whack(happened *events) {
	p := &w.player
	for _, e := range w.enemies {
		if e.down() || !inArc(p.character, e.position, swingReach, swingArc) {
			continue
		}
		happened.whacks++
		w.score += whackScore
		if e.hit(p.position) {
			happened.knockouts++
			w.score += knockoutScore
		}
	}
}

// inArc says whether a point is within reach of c, measured on the ground,
// and no more than arc degrees to either side of where c faces.
func inArc(c character, at golib.Vector3, reach, arc float32) bool {
	way := golib.Vector3{X: at.X - c.position.X, Z: at.Z - c.position.Z}
	if way.Length() > reach || at.Y-c.position.Y > 1.2 {
		return false
	}
	if way.Length() < bodyRadius {
		return true // close enough to hit whichever way it faces
	}
	turn := angleBetween(c.yaw, golib.RotationTowards(way).Yaw)
	return turn >= -arc && turn <= arc
}

// hurtPlayer takes a heart from the player, punched from from, unless a
// punch hurt it a moment ago.
func (w *world) hurtPlayer(from golib.Vector3, happened *events) {
	p := &w.player
	if !p.alive() || p.hurt.Running() {
		return
	}
	p.health--
	p.hurt.Start(hurtTime)
	away := golib.Vector3{X: p.position.X - from.X, Z: p.position.Z - from.Z}.Normalize()
	p.velocity = away.Scale(hurtKnockback).Add(golib.Vector3{Y: 2.5})
	p.onGround = false
	happened.hurt = true
	if !p.alive() {
		happened.died = true
		w.ending.Start(deathDelay)
		p.swing.Stop()
		p.animator.Stop(boneTorso)
		p.animator.Stop(boneRightArm)
	}
}

// stepWaves sends the next wave in when it is due, and starts the break,
// and the player's nod, when the last enemy of a wave is down.
func (w *world) stepWaves(dt float32, happened *events) {
	if w.next.Running() {
		if w.next.Tick(dt) {
			w.wave++
			w.spawnWave()
			happened.waveStarted = true
		}
		return
	}
	if w.wave == 0 || !w.player.alive() {
		return
	}
	for _, e := range w.enemies {
		if !e.down() {
			return
		}
	}
	happened.waveCleared = true
	w.score += waveBonus * w.wave
	w.next.Start(waveBreak)
	w.player.nodLater.Start(nodDelay)
}

// waveSize is how many enemies come in wave n, from 1.
func waveSize(n int) int {
	return min(1+n, 12)
}

// The four gates, in the middle of each wall, where the enemies come in.
var gates = []golib.Vector3{{Z: arenaHalf}, {X: arenaHalf}, {Z: -arenaHalf}, {X: -arenaHalf}}

// spawnWave brings in the enemies of the wave just begun, through the gates,
// each a little inside its gate, facing the middle of the courtyard.
func (w *world) spawnWave() {
	for i := range waveSize(w.wave) {
		gate := gates[(i+w.wave)%len(gates)]
		in := gate.Normalize().Scale(-1)
		side := golib.Vector3{X: in.Z, Z: -in.X}
		at := gate.Add(in.Scale(0.9 + float32(i/len(gates))*1.1)).Add(side.Scale(golib.RandomFloat(-0.7, 0.7)))
		w.enemies = append(w.enemies, newEnemy(at, golib.RotationTowards(in).Yaw, w.wave))
	}
}

// separate pushes characters that stand inside each other apart, so a crowd
// spreads round the player instead of piling into one spot. Enemies that are
// down are out of the way.
func (w *world) separate() {
	const apart = 2 * bodyRadius
	for i, a := range w.enemies {
		if a.down() {
			continue
		}
		for _, b := range w.enemies[i+1:] {
			if b.down() {
				continue
			}
			push := pushApart(a.position, b.position, apart)
			a.position = keepInside(a.position.Sub(push.Scale(0.5)))
			b.position = keepInside(b.position.Add(push.Scale(0.5)))
		}
		if w.player.alive() {
			a.position = keepInside(a.position.Sub(pushApart(a.position, w.player.position, apart)))
		}
	}
}

// pushApart is how far b must move away from a, across the ground, for the
// two to be apart meters from each other; zero when they already are.
func pushApart(a, b golib.Vector3, apart float32) golib.Vector3 {
	way := golib.Vector3{X: b.X - a.X, Z: b.Z - a.Z}
	d := way.Length()
	if d >= apart {
		return golib.Vector3{}
	}
	if d < 0.001 {
		way, d = golib.Vector3{X: 1}, 1
	}
	return way.Scale((apart - d) / d)
}

// removeFaded takes out the enemies that have faded away.
func (w *world) removeFaded() {
	kept := w.enemies[:0]
	for _, e := range w.enemies {
		if !e.faded() {
			kept = append(kept, e)
		}
	}
	w.enemies = kept
}

// animate moves every character's animations on.
func (w *world) animate(dt float32) {
	w.player.animator.Update(dt)
	for _, e := range w.enemies {
		e.animator.Update(dt)
	}
}

// turnTowards turns the yaw from towards to by at most maxTurn degrees, the
// shortest way round.
func turnTowards(from, to, maxTurn float32) float32 {
	turn := angleBetween(from, to)
	if float32(math.Abs(float64(turn))) <= maxTurn {
		return to
	}
	if turn < 0 {
		return from - maxTurn
	}
	return from + maxTurn
}

// angleBetween returns the shortest turn from one yaw to another, from -180 to
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
