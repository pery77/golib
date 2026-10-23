package main

import (
	"math"
	"slices"
	"testing"

	"golib"
)

const dt = 1.0 / 60 // the step golib.Run passes to Update

// newTestWorld is a run on the real island, with the random numbers from a
// fixed seed.
func newTestWorld(t *testing.T) *world {
	t.Helper()
	golib.SetRandomSeed(1)
	l, err := loadIsland()
	if err != nil {
		t.Fatal(err)
	}
	return newWorld(l)
}

// run steps the world for seconds with the same intent in every update, and
// collects what happened.
func run(w *world, want intent, seconds float32) events {
	var all events
	for range int(seconds*60 + 0.5) {
		h := w.step(want, dt)
		all.jumped = all.jumped || h.jumped
		all.coins += h.coins
		all.punched = all.punched || h.punched
		all.rockHit = all.rockHit || h.rockHit
		all.rockBroke = all.rockBroke || h.rockBroke
		all.splashed = all.splashed || h.splashed
		all.won = all.won || h.won
		all.timeUp = all.timeUp || h.timeUp
		all.tick = all.tick || h.tick
	}
	return all
}

// towards is an intent to walk, or sprint, on the ground towards a point.
func towards(w *world, at golib.Vector3, sprint bool) intent {
	way := golib.Vector3{X: at.X - w.player.position.X, Z: at.Z - w.player.position.Z}
	return intent{move: way.Normalize(), sprint: sprint}
}

var north = intent{move: golib.Vector3{Z: 1}} // up the island, the way the astronaut starts facing

func TestTheModelsLoad(t *testing.T) {
	for _, m := range []*golib.Model{astronautModel, blockModel, coinModel, flagModel, rocksModel, treeModel, pineModel, flowersModel} {
		if err := m.Err(); err != nil {
			t.Error(err)
		}
	}
	// The astronaut stands on its feet and is under a meter tall; a grass
	// block is a meter high and about two across: the rules are tuned for that.
	lowest, upmost := astronautModel.Bounds()
	if abs(lowest.Y) > 0.01 || upmost.Y < 0.8 || upmost.Y > 1.0 {
		t.Errorf("the astronaut's bounds are %v to %v, want its feet at 0 and about 0.9 m tall", lowest, upmost)
	}
	lowest, upmost = blockModel.Bounds()
	if abs(upmost.Y-lowest.Y-1) > 0.01 || abs(upmost.X-lowest.X-2.08) > 0.05 {
		t.Errorf("a grass block's bounds are %v to %v, want 1 m high and about 2.08 m across", lowest, upmost)
	}
	have := astronautModel.Animations()
	for _, c := range []clip{clipIdle, clipWalk, clipSprint, clipJump, clipFall, clipPunch, clipDie, clipYes, clipPickUp} {
		if !slices.Contains(have, c.name) {
			t.Errorf("the astronaut has no animation %q; it has %v", c.name, have)
		}
	}
	if !slices.Contains(astronautModel.Bones(), boneTorso) {
		t.Errorf("the astronaut has no bone %q; it has %v", boneTorso, astronautModel.Bones())
	}
}

// posedAt is where the astronaut's torso and arms are, a little way along
// each, after an animation has played for seconds from its start.
func posedAt(name string, seconds float32) []golib.Vector3 {
	a := golib.NewAnimator(astronautModel)
	a.PlayWith(name, golib.AnimationOptions{Once: true, Fade: 0.0001})
	for range int(seconds*600 + 0.5) {
		a.Update(1.0 / 600)
	}
	var out []golib.Vector3
	for _, bone := range astronautModel.Bones() {
		out = append(out, a.Bone(bone).Child(golib.Transform{Position: golib.Vector3{X: 0.1, Y: 0.1, Z: 0.1}}).Position)
	}
	return out
}

func moved(a, b []golib.Vector3) float32 {
	var total float32
	for i := range a {
		total += a[i].Distance(b[i])
	}
	return total
}

// The pack's files hold their last pose until frame 250 (see clip in
// models.go): each clip's length, as golib gives it, must be where its moves
// end, or the game would loop a walk with a stop in it, or cut a punch short.
func TestTheClipsEndWhenTheySay(t *testing.T) {
	for _, c := range []clip{clipIdle, clipWalk, clipSprint, clipJump, clipFall, clipPunch, clipDie, clipYes, clipPickUp} {
		if golib := astronautModel.AnimationLength(c.name); golib < c.length() {
			t.Errorf("%s: golib says it lasts %.3f s, shorter than the %.3f s the game plays", c.name, golib, c.length())
		}
		end := posedAt(c.name, c.length())
		if d := moved(end, posedAt(c.name, c.length()+2)); d > 0.001 {
			t.Errorf("%s still moves after %.3f s (by %.4f m): its length is too short", c.name, c.length(), d)
		}
		if d := moved(posedAt(c.name, c.length()-1.0/24), end); d < 0.001 {
			t.Errorf("%s doesn't move in its last frame before %.3f s: its length is too long", c.name, c.length())
		}
	}
}

func TestThePunchLandsWhenTheFistIsOut(t *testing.T) {
	// The fist is furthest in front, +Z, at the moment the rules land the
	// punch, played at the speed the rules play it.
	speed := clipPunch.length() / punchTime
	best, bestAt := float32(-1), float32(0)
	for at := float32(0); at < punchTime; at += 1.0 / 120 {
		a := golib.NewAnimator(astronautModel)
		a.PlayWith(clipPunch.name, golib.AnimationOptions{Once: true, Fade: 0.0001, Speed: speed})
		for range int(at * 120) {
			a.Update(1.0 / 120)
		}
		fist := a.Bone("arm-right").Child(golib.Transform{Position: golib.Vector3{X: -0.25}}).Position
		if fist.Z > best {
			best, bestAt = fist.Z, at
		}
	}
	if abs(bestAt-punchHitAt) > 0.04 {
		t.Errorf("the fist is furthest out %.3f s into a punch, but the rules land it at %.3f s", bestAt, punchHitAt)
	}
}

func TestTheAstronautWalksAndSprints(t *testing.T) {
	w := newTestWorld(t)
	start := w.player.position
	run(w, north, 1)
	if got := w.player.animator.Playing(); got != clipWalk.name {
		t.Errorf("walking, the astronaut plays %q, want %q", got, clipWalk.name)
	}
	if speed := w.player.speed(); abs(speed-walkSpeed) > 0.05 {
		t.Errorf("walking, the astronaut goes %.2f m/s, want %v", speed, walkSpeed)
	}
	if w.player.position.Z <= start.Z+1.5 || abs(w.player.position.X-start.X) > 0.01 {
		t.Errorf("walking north for a second moved the astronaut from %v to %v", start, w.player.position)
	}
	if w.player.yaw != 0 {
		t.Errorf("walking north, the astronaut faces %v, want 0 (+Z)", w.player.yaw)
	}
	// The walk loops at its real end, not at golib's 10.375 seconds.
	for range 120 {
		w.step(north, dt)
		if time := w.player.animator.Time(); time > clipWalk.length()+0.05 {
			t.Fatalf("the walk has played %.2f s without starting again: it should loop every %.3f s", time, clipWalk.length())
		}
	}

	w = newTestWorld(t)
	run(w, intent{move: north.move, sprint: true}, 1)
	if got := w.player.animator.Playing(); got != clipSprint.name {
		t.Errorf("sprinting, the astronaut plays %q, want %q", got, clipSprint.name)
	}
	if speed := w.player.speed(); abs(speed-sprintSpeed) > 0.05 {
		t.Errorf("sprinting, the astronaut goes %.2f m/s, want %v", speed, sprintSpeed)
	}

	w = newTestWorld(t)
	run(w, intent{}, 2)
	if got := w.player.animator.Playing(); got != clipIdle.name {
		t.Errorf("standing, the astronaut plays %q, want %q", got, clipIdle.name)
	}
}

func TestItJumpsOntoABlockItCantWalkOnto(t *testing.T) {
	w := newTestWorld(t)
	// Up the middle of the island, the third cell ahead of the start is two
	// blocks high: walking stops at it, jumping gets onto it.
	high := w.island.center(10, 6)
	if top := w.island.top(10, 6); top != 2*w.island.blockHeight {
		t.Fatalf("the island changed: the cell at row 10, column 6 is %v m high, want two blocks", top)
	}
	run(w, north, 4)
	if w.player.position.Y != w.island.blockHeight || w.player.position.Z > high.Z {
		t.Fatalf("walking into a block two high, the astronaut is at %v, want it stopped on the ground below", w.player.position)
	}
	happened := w.step(intent{move: north.move, jump: true}, dt)
	if !happened.jumped {
		t.Fatal("jump pressed on the ground didn't jump")
	}
	run(w, north, 0.1)
	if got := w.player.animator.Playing(); got != clipJump.name {
		t.Errorf("rising from a jump, the astronaut plays %q, want %q", got, clipJump.name)
	}
	run(w, north, 1)
	if w.player.position.Y != 2*w.island.blockHeight || !w.player.onGround {
		t.Errorf("after jumping at the high block, the astronaut is at %v, want on top of it", w.player.position)
	}
}

func TestItPicksUpCoins(t *testing.T) {
	w := newTestWorld(t)
	c := w.coins[0]
	w.player.position = golib.Vector3{X: c.position.X, Y: c.position.Y - coinHover, Z: c.position.Z - 1.2}
	clock := w.clock
	happened := run(w, towards(w, c.position, false), 1)
	if happened.coins != 1 || !c.taken || w.taken != 1 {
		t.Fatalf("walking through a coin picked up %d coins", happened.coins)
	}
	if want := clock - 1 + coinTime; abs(w.clock-want) > 0.02 {
		t.Errorf("a coin left %.2f s on the clock, want %.2f", w.clock, want)
	}
	// The astronaut reaches down for it, on the torso, and stops at the
	// pick-up's real end.
	w = newTestWorld(t)
	c = w.coins[0]
	w.player.position = golib.Vector3{X: c.position.X, Y: c.position.Y - coinHover, Z: c.position.Z - 0.6}
	w.step(intent{}, dt)
	if got := w.player.animator.PlayingOn(boneTorso); got != clipPickUp.name {
		t.Errorf("picking a coin up, the torso plays %q, want %q", got, clipPickUp.name)
	}
	run(w, intent{}, clipPickUp.length()+0.4)
	if got := w.player.animator.PlayingOn(boneTorso); got != "" {
		t.Errorf("%.2f s after picking a coin up, the torso still plays %q", clipPickUp.length()+0.4, got)
	}
}

func TestPunchesBreakRocksOpenForCoins(t *testing.T) {
	w := newTestWorld(t)
	r := w.rocks[0]
	// Stand south of the rock, facing a little away from it: the punch turns
	// the astronaut to it.
	w.player.position = r.position.Add(golib.Vector3{Z: -0.8})
	w.player.yaw = 40
	before := len(w.coins)
	for i := range rockHits {
		happened := run(w, intent{punch: true}, dt)
		if !happened.punched {
			t.Fatalf("punch %d didn't start", i+1)
		}
		happened = run(w, intent{}, punchTime)
		if !happened.rockHit {
			t.Fatalf("punch %d didn't hit the rock in front", i+1)
		}
		if got := w.player.animator.PlayingOn(boneTorso); got != "" && i == 0 {
			t.Errorf("after a punch the torso still plays %q", got)
		}
	}
	if !r.broken || len(w.coins) != before+rockCoins {
		t.Fatalf("after %d punches, the rock is broken: %v, with %d new coins", rockHits, r.broken, len(w.coins)-before)
	}
	run(w, intent{}, 2)
	for _, c := range w.coins[before:] {
		if c.flying || c.position.Y < w.island.blockHeight {
			t.Errorf("a coin from the rock is at %v, flying %v: want it landed on the grass", c.position, c.flying)
		}
	}
	// Walk over the coins: all three can be picked up.
	taken := w.taken
	for _, c := range w.coins[before:] {
		run(w, towards(w, c.position, false), 1.5)
	}
	if w.taken != taken+rockCoins {
		t.Errorf("walking to each coin from the rock picked up %d of %d", w.taken-taken, rockCoins)
	}
}

func TestFallingInTheSeaCostsTime(t *testing.T) {
	w := newTestWorld(t)
	clock := w.clock
	happened := run(w, intent{move: golib.Vector3{Z: -1}}, 3) // south, off the end of the island
	safe := w.player.safe
	if !happened.splashed {
		t.Fatal("walking south off the island didn't fall in the sea")
	}
	if want := clock - 3 - splashCost; abs(w.clock-want) > 0.05 {
		t.Errorf("after the splash the clock shows %.2f, want %.2f", w.clock, want)
	}
	run(w, intent{}, splashTime)
	if w.player.position != safe || !w.player.onGround {
		t.Errorf("after the splash, the astronaut is at %v, want back on the ground at %v", w.player.position, safe)
	}
	if ground := w.island.highestAround(safe.X, safe.Z, 0.5); ground != safe.Y {
		t.Errorf("the astronaut came back at %v, too close to an edge", safe)
	}
}

func TestTheClockRunsOut(t *testing.T) {
	w := newTestWorld(t)
	w.clock = 10.5
	happened := run(w, intent{}, 5)
	if !happened.tick {
		t.Error("the last ten seconds didn't tick")
	}
	happened = run(w, intent{}, 6)
	if !happened.timeUp || w.outcome != timeUp || w.clock != 0 {
		t.Fatalf("after the clock ran out the outcome is %v, the clock %.2f", w.outcome, w.clock)
	}
	if got := w.player.animator.Playing(); got != clipDie.name {
		t.Errorf("when time is up the astronaut plays %q, want %q", got, clipDie.name)
	}
	run(w, north, 1)
	if w.player.speed() > 0 {
		t.Error("the astronaut still moves after time is up")
	}
}

func TestReachingTheFlagWins(t *testing.T) {
	w := newTestWorld(t)
	flag := w.island.flag
	w.player.position = flag.Add(golib.Vector3{Z: -1.6})
	w.taken = 3
	happened := run(w, towards(w, flag, false), 1)
	if !happened.won || w.outcome != won {
		t.Fatalf("walking to the flag didn't win: outcome %v", w.outcome)
	}
	clock := w.clock
	if want := 3*coinScore + int(math.Ceil(float64(clock)))*secondScore; w.score() != want {
		t.Errorf("the score is %d, want %d", w.score(), want)
	}
	// It cheers, over and over: the emote loops at its real end.
	for range 120 {
		w.step(intent{}, dt)
	}
	if got := w.player.animator.Playing(); got != clipYes.name || w.player.animator.Time() > clipYes.length()+0.05 {
		t.Errorf("at the flag the astronaut plays %q at %.2f s, want %q looping", got, w.player.animator.Time(), clipYes.name)
	}
	if w.clock != clock {
		t.Error("the clock went on after the flag")
	}
}
