package main

import (
	"math"
	"testing"

	"golib"
)

const dt = 1.0 / 60 // the step golib.Run passes to Update

// floor is a test level: one large block, 10 meters a side, its top at Y = 0,
// with the start in its middle and the goal far away.
func floor() *level {
	return &level{
		blocks:    []box{{lowest: golib.Vector3{X: -5, Y: -1, Z: -5}, upmost: golib.Vector3{X: 5, Y: 0, Z: 5}}},
		goal:      golib.Vector3{Z: 100},
		killY:     -fallDepth,
		lowestTop: 0,
	}
}

// run steps the world for seconds with the same intent, and gathers what happened.
func run(w *world, want intent, seconds float32) events {
	var all events
	for range int(seconds * 60) {
		e := w.step(want, dt)
		all.jumped = all.jumped || e.jumped
		all.landed = max(all.landed, e.landed)
		all.coins += e.coins
		all.checkpoint = all.checkpoint || e.checkpoint
		all.fell = all.fell || e.fell
		all.lostLife = all.lostLife || e.lostLife
		all.gameOver = all.gameOver || e.gameOver
		all.won = all.won || e.won
		want.jump = false // pressed once, held after
		if e.lostLife {
			break
		}
	}
	return all
}

// peak jumps from the ground and returns how high the feet rise.
func peak(w *world, held bool) float32 {
	start := w.player.position.Y
	top := start
	w.step(intent{jump: true, jumpHeld: held}, dt)
	for range 120 {
		w.step(intent{jumpHeld: held}, dt)
		top = max(top, w.player.position.Y)
	}
	return top - start
}

func TestStandsStillOnTheGround(t *testing.T) {
	w := newWorld(floor())
	run(w, intent{}, 1)
	if !w.player.onGround || w.player.position != (golib.Vector3{}) {
		t.Errorf("after a second standing, the character is at %v, on the ground %v; want at rest at 0, 0, 0", w.player.position, w.player.onGround)
	}
}

func TestRunsWhereAskedAtRunSpeed(t *testing.T) {
	w := newWorld(floor())
	run(w, intent{move: golib.Vector3{Z: 1}}, 0.5)
	speed := w.player.velocity.Length()
	if math.Abs(float64(speed-runSpeed)) > 0.01 || w.player.velocity.Z <= 0 {
		t.Errorf("after half a second running towards +Z, the velocity is %v, want %v m/s along +Z", w.player.velocity, runSpeed)
	}
	if w.player.yaw != 0 {
		t.Errorf("running towards +Z, the character faces yaw %v, want 0", w.player.yaw)
	}
	run(w, intent{move: golib.Vector3{X: 1}}, 0.5)
	if math.Abs(float64(w.player.yaw-90)) > 0.01 {
		t.Errorf("running towards +X, the character faces yaw %v, want 90", w.player.yaw)
	}
}

func TestAFullJumpAndAShortHop(t *testing.T) {
	full := peak(newWorld(floor()), true)
	want := float32(jumpSpeed) * jumpSpeed / (2 * gravity)
	if math.Abs(float64(full-want)) > 0.15 {
		t.Errorf("a full jump rises %v m, want about %v", full, want)
	}
	hop := peak(newWorld(floor()), false)
	if hop >= full*0.6 || hop <= 0.2 {
		t.Errorf("a jump let go at once rises %v m, want well under the full jump's %v", hop, full)
	}
}

func TestNoJumpInTheAir(t *testing.T) {
	w := newWorld(floor())
	w.step(intent{jump: true, jumpHeld: true}, dt)
	for range 20 {
		w.step(intent{jumpHeld: true}, dt)
	}
	before := w.player.velocity.Y
	if e := w.step(intent{jump: true, jumpHeld: true}, dt); e.jumped || w.player.velocity.Y > before {
		t.Error("the character jumped again in the air")
	}
}

func TestCoyoteTimeAndJumpBuffer(t *testing.T) {
	// Run off the edge of the floor, then jump just after leaving it.
	w := newWorld(floor())
	w.player.position.Z = 4.9
	w.player.velocity.Z = runSpeed
	for w.player.onGround {
		w.step(intent{move: golib.Vector3{Z: 1}}, dt)
	}
	if e := w.step(intent{move: golib.Vector3{Z: 1}, jump: true, jumpHeld: true}, dt); !e.jumped {
		t.Error("a jump just after running off the edge didn't work")
	}

	// Press jump a moment before landing: it jumps on landing.
	w = newWorld(floor())
	w.player.position.Y = 0.1
	w.player.onGround = false
	w.player.airTime = 1
	jumped := false
	for i := range 30 {
		e := w.step(intent{jump: i == 0, jumpHeld: true}, dt)
		jumped = jumped || e.jumped
	}
	if !jumped {
		t.Error("a jump pressed just before landing didn't happen on landing")
	}
}

func TestBlocksStopTheCharacterAndCarryIt(t *testing.T) {
	l := floor()
	wall := box{lowest: golib.Vector3{X: -5, Y: 0, Z: 2}, upmost: golib.Vector3{X: 5, Y: 1, Z: 4.8}}
	l.blocks = append(l.blocks, wall)
	w := newWorld(l)
	run(w, intent{move: golib.Vector3{Z: 1}}, 2)
	if got, want := w.player.position.Z, wall.lowest.Z-playerRadius; math.Abs(float64(got-want)) > 0.001 {
		t.Errorf("running into a wall 1 m high, the character stopped at Z %v, want %v", got, want)
	}
	// Jump onto it, and stand on its top.
	w.step(intent{move: golib.Vector3{Z: 1}, jump: true, jumpHeld: true}, dt)
	for range 60 {
		if w.step(intent{move: golib.Vector3{Z: 1}, jumpHeld: true}, dt).landed > 0 {
			break
		}
	}
	run(w, intent{}, 0.5)
	if !w.player.onGround || w.player.position.Y != 1 {
		t.Errorf("after jumping onto the wall, the character is at %v, on the ground %v; want standing on its top, at Y 1", w.player.position, w.player.onGround)
	}
}

func TestBumpsItsHeadOnABlockAbove(t *testing.T) {
	l := floor()
	l.blocks = append(l.blocks, box{lowest: golib.Vector3{X: -1, Y: 1.2, Z: -1}, upmost: golib.Vector3{X: 1, Y: 2, Z: 1}})
	w := newWorld(l)
	top := peak(w, true)
	if top > 1.2-playerHeight+0.001 {
		t.Errorf("jumping under a block 1.2 m up, the feet rose %v m, want no more than %v", top, 1.2-playerHeight)
	}
}

func TestFallingOffCostsALifeAndTheLastOneEndsTheGame(t *testing.T) {
	w := newWorld(floor())
	for life := startLives; life > 0; life-- {
		happened := run(w, intent{move: golib.Vector3{X: 1}}, 5)
		if !happened.fell || !happened.lostLife {
			t.Fatalf("running off the floor: fell %v, lost a life %v, want both", happened.fell, happened.lostLife)
		}
		if w.lives != life-1 {
			t.Fatalf("lives = %d, want %d", w.lives, life-1)
		}
		if life > 1 && (w.player.position != w.level.start || !w.player.onGround) {
			t.Fatalf("after a fall the character is at %v, want back at the start", w.player.position)
		}
	}
	if !w.over {
		t.Fatal("the game isn't over after the last life")
	}
	timeAtTheEnd := w.time
	run(w, intent{move: golib.Vector3{X: 1}}, 1)
	if w.time != timeAtTheEnd {
		t.Error("the clock kept running after the game ended")
	}
}

func TestCoinsCheckpointsAndTheGoal(t *testing.T) {
	l := floor()
	l.coins = []golib.Vector3{{Z: 2, Y: 0.5}, {Z: 2.3, Y: 0.5}, {X: 3, Y: 0.5}}
	l.checkpoints = []golib.Vector3{{Z: 3.5}}
	l.goal = golib.Vector3{Z: 4.5}
	w := newWorld(l)

	happened := run(w, intent{move: golib.Vector3{Z: 1}}, 0.6)
	if happened.coins != 2 || w.collected != 2 || !w.taken[0] || !w.taken[1] || w.taken[2] {
		t.Errorf("running through two coins picked up %d, taken %v; want the two in the way", happened.coins, w.taken)
	}
	if !happened.checkpoint || w.respawnAt != l.checkpoints[0] {
		t.Errorf("running over the checkpoint: reached %v, respawn at %v; want it reached", happened.checkpoint, w.respawnAt)
	}
	happened = run(w, intent{move: golib.Vector3{Z: 1}}, 1)
	if !happened.won || !w.won {
		t.Fatal("running into the flag didn't win")
	}
	if w.collected != 2 {
		t.Errorf("collected = %d after the win, want the same 2", w.collected)
	}

	// A fall after the checkpoint starts again there.
	l.goal = golib.Vector3{Z: 100}
	w = newWorld(l)
	run(w, intent{move: golib.Vector3{Z: 1}}, 0.7)
	run(w, intent{move: golib.Vector3{X: 1}}, 5)
	if w.player.position != l.checkpoints[0] {
		t.Errorf("after a fall past the checkpoint, the character starts at %v, want %v", w.player.position, l.checkpoints[0])
	}
}

func TestRunningRightMovesRightOnTheScreen(t *testing.T) {
	camera := golib.NewCamera3D(screenWidth, screenHeight)
	for _, yaw := range []float32{0, 90, -135} {
		camera.Orbit(golib.Vector3{}, yaw, cameraPitch, cameraDistance)
		center, _ := camera.ToScreen(golib.Vector3{})
		right, _ := camera.ToScreen(runDirection(yaw, 0, 1))
		up, _ := camera.ToScreen(runDirection(yaw, 1, 0))
		if right.X <= center.X+1 {
			t.Errorf("camera yaw %v: running right goes to %v on the screen, from %v; want to the right", yaw, right, center)
		}
		if up.Y >= center.Y-1 {
			t.Errorf("camera yaw %v: running forward goes to %v on the screen, from %v; want up the screen", yaw, up, center)
		}
	}
}

func TestQTurnsTheCameraSoTheViewGoesLeft(t *testing.T) {
	// A larger yaw turns left: what was ahead slides to the right of the screen.
	camera := golib.NewCamera3D(screenWidth, screenHeight)
	ahead := golib.Vector3{Z: 10}
	camera.Orbit(golib.Vector3{}, 0, cameraPitch, cameraDistance)
	before, _ := camera.ToScreen(ahead)
	camera.Orbit(golib.Vector3{}, 20, cameraPitch, cameraDistance)
	after, _ := camera.ToScreen(ahead)
	if after.X <= before.X {
		t.Errorf("turning the camera's yaw up moved what is ahead from %v to %v on the screen, want to the right", before.X, after.X)
	}
}
