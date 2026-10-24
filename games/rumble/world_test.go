package main

import (
	"math"
	"slices"
	"testing"

	"golib"
)

const dt = 1.0 / 60 // the step golib.Run passes to Update

// swingUpdates is how many updates a swing lasts, rounded down.
var swingTimeVar float32 = swingTime
var swingUpdates = int(swingTimeVar * 60)

// quietWorld is a new world with no wave on its way, so a test can bring in
// the enemies it wants.
func quietWorld() *world {
	golib.SetRandomSeed(1)
	w := newWorld()
	w.next.Stop()
	return w
}

// run steps the world for seconds with the same intent in every update, and
// collects what happened.
func run(w *world, want intent, seconds float32) events {
	var all events
	for range int(seconds * 60) {
		h := w.step(want, dt)
		all.jumped = all.jumped || h.jumped
		all.swung = all.swung || h.swung
		all.whacks += h.whacks
		all.knockouts += h.knockouts
		all.hurt = all.hurt || h.hurt
		all.died = all.died || h.died
		all.waveStarted = all.waveStarted || h.waveStarted
		all.waveCleared = all.waveCleared || h.waveCleared
	}
	return all
}

// forward is a push towards +Z, the way the player starts facing.
func forward(push float32) intent {
	return intent{move: golib.Vector3{Z: push}}
}

// enemyAt puts an enemy at at, standing still, and returns it.
func enemyAt(w *world, at golib.Vector3) *enemy {
	e := newEnemy(at, 180, 1)
	w.enemies = append(w.enemies, e)
	return e
}

func TestTheModelsLoad(t *testing.T) {
	for _, m := range []*golib.Model{characterModel, grassModel, flowersModel, rocksModel, treeModel, pineModel} {
		if err := m.Err(); err != nil {
			t.Error(err)
		}
	}
	// The astronaut stands on its feet and is under a meter tall: the
	// camera's distances and the reaches are tuned for that.
	lowest, upmost := characterModel.Bounds()
	if math.Abs(float64(lowest.Y)) > 0.01 || upmost.Y < 0.8 || upmost.Y > 1.0 {
		t.Errorf("the astronaut's bounds are %v to %v, want its feet at 0 and about 0.9 m tall", lowest, upmost)
	}
}

func TestTheCharacterHasItsAnimations(t *testing.T) {
	have := characterModel.Animations()
	for _, name := range []string{animIdle, animWalk, animSprint, animJump, animFall, animDie, animNod, animHold, animSwing, animPunch} {
		if !slices.Contains(have, name) {
			t.Errorf("the astronaut has no animation %q; it has %v", name, have)
		}
	}
	// The stick's hand: the right arm, on the astronaut's right, which is -X
	// for a model facing +Z, at about shoulder height.
	arm := golib.NewAnimator(characterModel).Bone(boneRightArm)
	if arm.Position.X > -0.05 || arm.Position.Y < 0.2 || arm.Position.Y > 0.5 {
		t.Errorf("the right arm is at %v, want it on the right (-X) at shoulder height", arm.Position)
	}
}

func TestThePlayerIdlesWalksAndSprintsByHowHardItIsPushed(t *testing.T) {
	for _, c := range []struct {
		push  float32
		plays string
	}{{0, animIdle}, {0.3, animWalk}, {walkPush, animWalk}, {0.8, animSprint}, {1, animSprint}} {
		w := quietWorld()
		run(w, forward(c.push), 1)
		if got := w.player.animator.Playing(); got != c.plays {
			t.Errorf("pushed %v for a second, the player plays %q at %.2f m/s, want %q", c.push, got, w.player.speed(), c.plays)
		}
	}
	// A full push sprints, a gentle one walks.
	w := quietWorld()
	run(w, forward(1), 1)
	if speed := w.player.speed(); math.Abs(float64(speed-sprintSpeed)) > 0.05 {
		t.Errorf("pushed all the way, the player runs at %.2f m/s, want %v", speed, sprintSpeed)
	}
	w = quietWorld()
	run(w, forward(walkPush/2), 1)
	if speed := w.player.speed(); math.Abs(float64(speed-walkSpeed/2)) > 0.05 {
		t.Errorf("pushed a quarter of the way, the player walks at %.2f m/s, want %v", speed, walkSpeed/2)
	}
}

func TestThePlayerJumpsAndFalls(t *testing.T) {
	w := quietWorld()
	happened := w.step(intent{jump: true}, dt)
	if !happened.jumped || w.player.onGround {
		t.Fatal("jump pressed on the ground didn't leave it")
	}
	run(w, intent{}, 0.1)
	if got := w.player.animator.Playing(); got != animJump {
		t.Errorf("rising from a jump, the player plays %q, want %q", got, animJump)
	}
	for w.player.velocity.Y > -1 {
		w.step(intent{}, dt)
	}
	if got := w.player.animator.Playing(); got != animFall {
		t.Errorf("coming down from a jump, the player plays %q, want %q", got, animFall)
	}
	top := float32(jumpSpeed * jumpSpeed / (2 * gravity))
	if top < 0.9 || top > 1.4 {
		t.Errorf("a jump rises %.2f m; the astronaut is 0.9 m tall, so want about a meter", top)
	}
	run(w, intent{}, 1)
	if !w.player.onGround || w.player.animator.Playing() != animIdle {
		t.Errorf("a second after a jump the player is on the ground: %v, playing %q, want idling there", w.player.onGround, w.player.animator.Playing())
	}
	// No second jump in the air.
	w.step(intent{jump: true}, dt)
	run(w, intent{}, 0.1)
	if w.step(intent{jump: true}, dt).jumped {
		t.Error("jump pressed in the air jumped again")
	}
}

func TestSwingingWhileSprintingKeepsTheLegsRunning(t *testing.T) {
	w := quietWorld()
	run(w, forward(1), 1)
	if !w.step(intent{move: golib.Vector3{Z: 1}, swing: true}, dt).swung {
		t.Fatal("swing pressed while sprinting didn't swing")
	}
	for range swingUpdates - 2 {
		w.step(forward(1), dt)
		if got := w.player.animator.Playing(); got != animSprint {
			t.Fatalf("during a swing the player's body plays %q, want the legs still on %q", got, animSprint)
		}
		if w.player.speed() < sprintSpeed-0.05 {
			t.Fatalf("during a swing the player slowed to %.2f m/s", w.player.speed())
		}
	}
	if !w.player.swing.Running() {
		t.Error("the swing ended before swingTime")
	}
	run(w, forward(1), 0.1)
	if w.player.swing.Running() {
		t.Error("the swing goes on after swingTime")
	}
}

func TestThreeWhacksKnockAnEnemyOutAndItFadesAway(t *testing.T) {
	w := quietWorld()
	in := w.player.position.Add(golib.Vector3{Z: 1})
	e := enemyAt(w, in)
	var happened events
	for i := range enemyHealth {
		e.position, e.velocity, e.state = in, golib.Vector3{}, staggered // back in front of the stick
		e.cooldown.Start(10)                                             // no punch back meanwhile
		h := run(w, intent{swing: true}, swingTime+0.05)
		if h.whacks != 1 {
			t.Fatalf("swing %d whacked %d enemies, want the one in front", i+1, h.whacks)
		}
		happened.knockouts += h.knockouts
		if i < enemyHealth-1 && (e.down() || e.health != enemyHealth-1-i) {
			t.Fatalf("after %d whacks the enemy has %d health and is down: %v", i+1, e.health, e.down())
		}
	}
	if !e.down() || happened.knockouts != 1 {
		t.Fatalf("after %d whacks the enemy isn't knocked out", enemyHealth)
	}
	if w.score != enemyHealth*whackScore+knockoutScore {
		t.Errorf("the score is %d, want %d", w.score, enemyHealth*whackScore+knockoutScore)
	}
	run(w, intent{}, 0.2)
	if got := e.animator.Playing(); got != animDie {
		t.Errorf("knocked out, the enemy plays %q, want %q", got, animDie)
	}
	if e.opacity() != 1 {
		t.Errorf("just knocked out, the enemy is %.2f solid, want 1 until it has fallen over", e.opacity())
	}
	for e.downFor < fallOverTime+fadeTime/2 {
		w.step(intent{}, dt)
	}
	if o := e.opacity(); o < 0.45 || o > 0.55 {
		t.Errorf("half way through fading, the enemy is %.2f solid", o)
	}
	run(w, intent{}, fadeTime/2+0.05)
	if len(w.enemies) != 0 {
		t.Error("the enemy is still in the world after fading away")
	}
}

func TestTheStickOnlyHitsWithinReach(t *testing.T) {
	w := quietWorld()
	far := enemyAt(w, w.player.position.Add(golib.Vector3{Z: swingReach + aimReach + 1}))
	far.state, far.stagger = staggered, golib.NewTimer(10)
	if h := run(w, intent{swing: true}, swingTime); h.whacks != 0 || far.health != enemyHealth {
		t.Errorf("a swing whacked %d enemies, with the only one out of reach", h.whacks)
	}
	// An enemy behind the player, or off to its side, is turned to as the
	// swing starts, and whacked.
	for _, at := range []golib.Vector3{{Z: -1}, {X: 1, Z: -0.3}} {
		w = quietWorld()
		e := enemyAt(w, w.player.position.Add(at))
		e.state, e.stagger = staggered, golib.NewTimer(10)
		if h := run(w, intent{swing: true}, swingTime); h.whacks != 1 {
			t.Errorf("a swing at an enemy at %v from the player whacked %d, want the swing to turn to it", at, h.whacks)
		}
	}
	// Only once a swing: the next one waits until this one is over.
	w = quietWorld()
	e := enemyAt(w, w.player.position.Add(golib.Vector3{Z: 0.8}))
	e.state, e.stagger = staggered, golib.NewTimer(10)
	if h := run(w, intent{swing: true}, swingTime-0.05); h.whacks != 1 {
		t.Errorf("swing held down for a swing's length whacked %d times, want once", h.whacks)
	}
}

func TestEnemiesWanderAndThenChase(t *testing.T) {
	w := quietWorld()
	e := enemyAt(w, golib.Vector3{X: 6, Z: 6})
	run(w, intent{}, 0.5)
	if e.state != wandering || e.animator.Playing() != animWalk {
		t.Errorf("an enemy far from the player is %v, playing %q, want it wandering and walking", e.state, e.animator.Playing())
	}
	run(w, intent{}, noticeMost)
	if e.state != chasing && e.state != punching {
		t.Errorf("after %v seconds the enemy is %v, want it chasing", noticeMost+0.5, e.state)
	}
	run(w, intent{}, 0.5)
	if e.speed() > runsAbove && e.animator.Playing() != animSprint {
		t.Errorf("chasing at %.2f m/s, the enemy plays %q, want %q", e.speed(), e.animator.Playing(), animSprint)
	}
	// One that comes close sees the player at once.
	w = quietWorld()
	near := enemyAt(w, w.player.position.Add(golib.Vector3{X: noticeRange - 1}))
	w.step(intent{}, dt)
	if near.state != chasing {
		t.Errorf("an enemy %v m from the player is %v, want it chasing", noticeRange-1, near.state)
	}
}

func TestPunchesTakeHeartsUntilThePlayerIsKnockedOut(t *testing.T) {
	w := quietWorld()
	e := enemyAt(w, w.player.position.Add(golib.Vector3{Z: 0.7}))
	e.state = chasing
	var happened events
	for range 30 * 60 {
		e.position = w.player.position.Add(golib.Vector3{Z: 0.7}) // stays at the player's face
		h := w.step(intent{}, dt)
		happened.hurt = happened.hurt || h.hurt
		if h.died {
			happened.died = true
			break
		}
		if h.hurt && w.player.alive() && !w.player.hurt.Running() {
			t.Fatal("a punch hurt the player without the moment it can't be hurt again")
		}
	}
	if !happened.hurt || !happened.died || w.player.health != 0 {
		t.Fatalf("an enemy punching the player for 30 seconds left it with %d hearts", w.player.health)
	}
	run(w, intent{}, 0.2)
	if got := w.player.animator.Playing(); got != animDie {
		t.Errorf("knocked out, the player plays %q, want %q", got, animDie)
	}
	if w.over {
		t.Error("the game is over at once; want a moment to see the player fall")
	}
	run(w, intent{}, deathDelay)
	if !w.over {
		t.Error("the game isn't over after the player's death and deathDelay")
	}
}

func TestClearingAWaveNodsAndBringsABiggerOne(t *testing.T) {
	golib.SetRandomSeed(1)
	w := newWorld()
	h := run(w, intent{}, firstWaveDelay+0.05)
	if !h.waveStarted || w.wave != 1 || len(w.enemies) != waveSize(1) {
		t.Fatalf("after %v seconds the wave is %d with %d enemies, want wave 1 with %d", firstWaveDelay, w.wave, len(w.enemies), waveSize(1))
	}
	for _, e := range w.enemies {
		for range enemyHealth {
			e.hit(e.position.Add(golib.Vector3{X: 1}))
		}
	}
	scored := w.score
	h = w.step(intent{}, dt)
	if !h.waveCleared || w.score != scored+waveBonus {
		t.Fatalf("with every enemy knocked out, the wave isn't cleared: %v, score %d", h.waveCleared, w.score)
	}
	run(w, intent{}, nodDelay+0.1)
	if !w.player.nod.Running() {
		t.Error("the player doesn't nod after clearing a wave")
	}
	h = run(w, intent{}, waveBreak)
	if !h.waveStarted || w.wave != 2 {
		t.Fatalf("after the break the wave is %d, want 2", w.wave)
	}
	if len(w.enemies) != waveSize(2) || waveSize(2) <= waveSize(1) {
		t.Errorf("wave 2 has %d enemies, want %d, more than wave 1's %d", len(w.enemies), waveSize(2), waveSize(1))
	}
	for n := 2; n < 20; n++ {
		if waveSize(n+1) < waveSize(n) {
			t.Errorf("wave %d is smaller than wave %d", n+1, n)
		}
	}
	// Later waves chase faster, up to a cap below the player's sprint.
	if newEnemy(golib.Vector3{}, 0, 5).chaseSpeed <= newEnemy(golib.Vector3{}, 0, 1).chaseSpeed || enemyChaseMax >= sprintSpeed {
		t.Error("later enemies aren't faster, or the fastest outrun the player")
	}
}

func TestEveryoneStaysInsideTheWallsAndOutOfTheWell(t *testing.T) {
	for _, way := range []golib.Vector3{{X: 1}, {X: -1}, {Z: 1}, {Z: -1}, {X: 1, Z: 1}} {
		w := quietWorld()
		run(w, intent{move: way.Normalize()}, 8)
		at := w.player.position
		if abs(at.X) > arenaHalf-bodyRadius+0.001 || abs(at.Z) > arenaHalf-bodyRadius+0.001 {
			t.Errorf("running %v for 8 seconds took the player to %v, through the walls", way, at)
		}
	}
	w := quietWorld()
	w.player.position = golib.Vector3{Z: -3}
	run(w, forward(1), 2)
	if d := (golib.Vector3{X: w.player.position.X, Z: w.player.position.Z}).Length(); d < wellRadius+bodyRadius-0.001 {
		t.Errorf("running at the well, the player is %.2f m from its middle, inside it", d)
	}
}

func TestPushingRightRunsRightOnTheScreen(t *testing.T) {
	s := newPlayScene(record{})
	s.world.next.Stop()
	start, _ := s.camera.ToScreen(s.world.player.position)
	for range 30 { // the camera stays where it was, to see where the player went
		s.world.step(intent{move: s.camera.Direction(0, 1)}, dt)
	}
	now, _ := s.camera.ToScreen(s.world.player.position)
	if now.X <= start.X+20 {
		t.Errorf("pushing right moved the player from %v to %v on the screen, want it to the right", start, now)
	}
	if got := s.camera.Direction(1, 0).Length(); math.Abs(float64(got-1)) > 0.01 {
		t.Errorf("a full push forward is %.2f long, want 1", got)
	}
	if got := s.camera.Direction(0.3, 0).Length(); math.Abs(float64(got-0.3)) > 0.01 {
		t.Errorf("a gentle push is %.2f long, want 0.3: the walk needs to know how hard the stick is pushed", got)
	}
}

func TestTheCameraStaysInsideTheWalls(t *testing.T) {
	edge := float32(arenaHalf - bodyRadius)
	for _, heading := range []float32{0, 90, 180, 270, 45} {
		for _, at := range []golib.Vector3{{}, {Z: -edge}, {X: edge, Z: edge}, {X: -edge}, {X: 5, Z: edge}} {
			// The player stands there, and the camera has two seconds to
			// settle, as it would in the game.
			s := newPlayScene(record{})
			s.world.player.position = at
			s.camera.Heading = heading
			for range 120 {
				follow(s.camera, &s.world.player, dt)
				s.camera.Update(dt)
			}
			if eye := s.camera.Position; abs(eye.X) > arenaHalf || abs(eye.Z) > arenaHalf {
				t.Errorf("a camera facing %v behind a player at %v ends up at %v, outside the walls", heading, at, eye)
			}
		}
	}
	d, p, fits := cameraFit(golib.Vector3{Y: cameraLookUp}, 0)
	if d != cameraDistance || math.Abs(float64(p-cameraPitch)) > 0.01 || !fits {
		t.Errorf("in the middle of the courtyard the camera is %v m back at %v degrees, want %v and %v", d, p, cameraDistance, cameraPitch)
	}
}
