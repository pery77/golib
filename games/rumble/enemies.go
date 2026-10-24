package main

import "golib"

// Tuning for the enemies: more of the same astronaut, tinted red.
const (
	enemyHealth     = 3   // whacks to knock one out
	enemyWalkSpeed  = 1.1 // meters per second while it wanders
	enemyChaseSpeed = 2.7 // meters per second while it chases, in the first wave
	enemyChaseGain  = 0.2 // meters per second faster in each wave after
	enemyChaseMax   = 4.3 // meters per second: never faster than this, still slower than a sprint
	noticeRange     = 5.5 // meters: closer than this, a wandering enemy sees the player and chases
	noticeLeast     = 2.5 // seconds an enemy wanders at least before it chases anyway
	noticeMost      = 5.5 // seconds it wanders at most
	wanderTurnLeast = 1.2 // seconds between turns while it wanders
	wanderTurnMost  = 2.6
	punchRange      = 0.85 // meters to the player at which an enemy stops and punches
	punchTime       = 0.6  // seconds a punch lasts
	punchHitAt      = 0.32 // seconds into a punch when the fist lands
	punchReach      = 1.1  // meters the fist reaches
	punchArc        = 60   // degrees either side of where the enemy faces
	punchCooldown   = 1.1  // seconds between an enemy's punches
	staggerTime     = 0.55 // seconds an enemy reels after a whack before it chases again
	flashTime       = 0.12 // seconds an enemy flashes white after a whack
	fallOverTime    = 0.7  // seconds a knocked out enemy lies there before it fades
	fadeTime        = 1.3  // seconds it takes to fade away
)

// enemyState is what an enemy is doing.
type enemyState int

const (
	wandering  enemyState = iota // walking about, not after the player yet
	chasing                      // running at the player
	punching                     // standing still, throwing a punch
	staggered                    // reeling from a whack
	knockedOut                   // down: its death animation, then it fades away
)

// enemy is one of the red astronauts.
type enemy struct {
	character
	health      int
	state       enemyState
	chaseSpeed  float32
	heading     golib.Vector3 // where it wanders, on the ground
	turn        golib.Timer   // runs until it picks a new way to wander
	notice      golib.Timer   // runs until it chases the player anyway
	punch       golib.Timer   // runs while a punch lasts
	punchLanded bool          // this punch has reached the moment the fist lands
	cooldown    golib.Timer   // runs until it may punch again
	stagger     golib.Timer   // runs while it reels
	flash       golib.Timer   // runs while it flashes white
	downFor     float32       // seconds since it was knocked out
}

// newEnemy returns an enemy of wave n standing at at, facing yaw, about to
// wander in.
func newEnemy(at golib.Vector3, yaw float32, wave int) *enemy {
	e := &enemy{
		character:  character{position: at, yaw: yaw, onGround: true, animator: golib.NewAnimator(characterModel)},
		health:     enemyHealth,
		state:      wandering,
		chaseSpeed: min(enemyChaseSpeed+enemyChaseGain*float32(wave-1), enemyChaseMax),
		heading:    golib.Rotation{Yaw: yaw}.Forward(),
		turn:       golib.NewTimer(golib.RandomFloat(wanderTurnLeast, wanderTurnMost)),
		notice:     golib.NewTimer(golib.RandomFloat(noticeLeast, noticeMost)),
	}
	e.animator.Play(animWalk)
	return e
}

// down says whether it has been knocked out.
func (e *enemy) down() bool {
	return e.state == knockedOut
}

// faded says whether it has faded away, and can be taken out of the world.
func (e *enemy) faded() bool {
	return e.down() && e.downFor >= fallOverTime+fadeTime
}

// opacity is how solid it is drawn, from 1 down to 0 as it fades away.
func (e *enemy) opacity() float32 {
	if !e.down() {
		return 1
	}
	return golib.Clamp(1-(e.downFor-fallOverTime)/fadeTime, 0, 1)
}

// hit is a whack of the stick from a player standing at from. It knocks the
// enemy back and up, and reports whether this whack knocked it out.
func (e *enemy) hit(from golib.Vector3) bool {
	away := golib.Vector3{X: e.position.X - from.X, Z: e.position.Z - from.Z}.Normalize()
	e.velocity = away.Scale(knockback).Add(golib.Vector3{Y: knockHop})
	e.onGround = false
	e.flash.Start(flashTime)
	e.punch.Stop()
	e.animator.Stop(boneTorso)
	e.health--
	if e.health <= 0 {
		e.state = knockedOut
		e.yaw = golib.RotationTowards(away.Scale(-1)).Yaw // it falls facing the one who hit it
		return true
	}
	e.state = staggered
	e.stagger.Start(staggerTime)
	return false
}

// stepEnemy moves one enemy on: it wanders, notices the player, chases and
// punches, reels from whacks, and lies down and fades once knocked out.
func (w *world) stepEnemy(e *enemy, dt float32, happened *events) {
	e.flash.Tick(dt)
	p := &w.player
	toPlayer := golib.Vector3{X: p.position.X - e.position.X, Z: p.position.Z - e.position.Z}
	switch e.state {
	case wandering:
		if e.turn.Tick(dt) {
			e.heading = golib.Rotation{Yaw: golib.RandomFloat(-180, 180)}.Forward()
			e.turn.Start(golib.RandomFloat(wanderTurnLeast, wanderTurnMost))
		}
		// Away from the walls: towards the middle when it gets near one.
		if max(abs(e.position.X), abs(e.position.Z)) > arenaHalf-1.5 {
			e.heading = golib.Vector3{X: -e.position.X, Z: -e.position.Z}.Normalize()
		}
		e.steer(e.heading.Scale(enemyWalkSpeed), dt)
		if (e.notice.Tick(dt) || toPlayer.Length() < noticeRange) && p.alive() {
			e.state = chasing
		}
	case chasing:
		if !p.alive() {
			e.state = wandering
			break
		}
		if toPlayer.Length() < punchRange && !e.cooldown.Running() {
			e.state = punching
			e.punch.Start(punchTime)
			e.punchLanded = false
			e.animator.PlayWith(animPunch, golib.AnimationOptions{Bone: boneTorso, Once: true, Restart: true, Speed: punchLength / punchTime})
			break
		}
		target := toPlayer.Normalize().Scale(e.chaseSpeed)
		if toPlayer.Length() < punchRange {
			target = golib.Vector3{} // waiting for its next punch, close by
			e.yaw = turnTowards(e.yaw, golib.RotationTowards(toPlayer).Yaw, turnSpeed*dt)
		}
		e.steer(target, dt)
	case punching:
		e.steer(golib.Vector3{}, dt)
		e.yaw = turnTowards(e.yaw, golib.RotationTowards(toPlayer).Yaw, turnSpeed/3*dt)
		if !e.punchLanded && punchTime-e.punch.Left() >= punchHitAt {
			e.punchLanded = true
			if inArc(e.character, p.position, punchReach, punchArc) {
				w.hurtPlayer(e.position, happened)
			}
		}
		if e.punch.Tick(dt) {
			e.state = chasing
			e.cooldown.Start(punchCooldown)
		}
	case staggered:
		if e.onGround {
			e.steer(golib.Vector3{}, dt)
		}
		if e.stagger.Tick(dt) {
			e.state = chasing
		}
	case knockedOut:
		if e.onGround {
			e.steer(golib.Vector3{}, dt)
		}
		e.downFor += dt
	}
	e.cooldown.Tick(dt)
	e.move(dt)
}

func abs(x float32) float32 {
	return max(x, -x)
}
