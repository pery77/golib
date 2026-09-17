package main

import (
	"math"
	"testing"

	"golib"
)

const dt = 1.0 / 60 // the step golib.Run passes to Update

// quietWorld returns a world with the given rocks and a ship in the middle
// that can crash straight away. Random numbers start from a fixed seed, so
// tests repeat.
func quietWorld(rocks ...rock) world {
	golib.SetRandomSeed(1)
	w := world{lives: startLives, wave: 1, rocks: rocks}
	w.ship = ship{x: worldWidth / 2, y: worldHeight / 2, alive: true}
	return w
}

// stillRock returns a rock that doesn't move.
func stillRock(x, y float32, size int) rock {
	return rock{x: x, y: y, size: size}
}

// run steps the world for a number of updates with the same controls.
func run(w *world, updates int, c controls) {
	for range updates {
		w.step(c, dt)
	}
}

func TestNewWorld(t *testing.T) {
	golib.SetRandomSeed(1)
	w := newWorld()
	if w.lives != startLives || w.wave != 1 || !w.ship.alive {
		t.Fatalf("lives = %d, wave = %d, ship alive = %v; want %d, 1, true", w.lives, w.wave, w.ship.alive, startLives)
	}
	if len(w.rocks) != firstWaveRocks {
		t.Fatalf("the first wave has %d rocks, want %d", len(w.rocks), firstWaveRocks)
	}
	for _, r := range w.rocks {
		if r.size != bigRock {
			t.Errorf("the first wave has a rock of size %d, want only big rocks", r.size)
		}
		if distanceSquared(r.x, r.y, w.ship.x, w.ship.y) < 200*200 {
			t.Errorf("a rock starts at %v, %v, less than 200 pixels from the ship", r.x, r.y)
		}
	}
}

func TestWrap(t *testing.T) {
	for _, tt := range []struct{ value, want float32 }{{-1, 99}, {100, 0}, {250, 50}, {42, 42}} {
		if got := wrap(tt.value, 100); got != tt.want {
			t.Errorf("wrap(%v, 100) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestShipTurnsAndStopsAtTopSpeed(t *testing.T) {
	w := quietWorld(stillRock(100, 100, bigRock))
	run(&w, 60, controls{turn: 1})
	if math.Abs(float64(w.ship.angle-turnSpeed)) > 0.01 {
		t.Errorf("after turning right for a second, angle = %v, want about %v", w.ship.angle, turnSpeed)
	}

	w = quietWorld(stillRock(100, 100, bigRock))
	run(&w, 5*60, controls{thrust: true})
	if speed := length(w.ship.vx, w.ship.vy); speed > maxShipSpeed+0.01 || speed < maxShipSpeed-1 {
		t.Errorf("after five seconds of thrust, speed = %v, want %v", speed, maxShipSpeed)
	}
	if !thrustSound.Looping() {
		t.Error("the thrust rumble doesn't loop while the ship thrusts")
	}
	run(&w, 1, controls{})
	if thrustSound.Looping() {
		t.Error("the thrust rumble still loops after the thrust ends")
	}
}

func TestFireRateAndBulletLimit(t *testing.T) {
	// A far rock, so clearing the field doesn't start a new wave.
	w := quietWorld(stillRock(100, 100, bigRock))
	run(&w, 30, controls{fire: true})
	// Holding fire shoots in updates 1, 12 and 23: every fireCooldown seconds.
	if len(w.bullets) != 3 {
		t.Errorf("holding fire for half a second shot %d bullets, want 3", len(w.bullets))
	}

	w = quietWorld(stillRock(100, 100, bigRock))
	for range maxBullets {
		w.bullets = append(w.bullets, bullet{x: 640, y: 100, life: 1})
	}
	w.step(controls{fire: true}, dt)
	if len(w.bullets) != maxBullets {
		t.Errorf("with %d bullets in flight, firing left %d, want no more", maxBullets, len(w.bullets))
	}
}

func TestShootingSplitsARock(t *testing.T) {
	w := quietWorld(stillRock(900, 360, bigRock))
	w.bullets = []bullet{{x: 900, y: 360, life: 1}}
	w.step(controls{}, dt)
	if len(w.rocks) != 2 || w.rocks[0].size != mediumRock || w.rocks[1].size != mediumRock {
		t.Fatalf("after shooting a big rock there are %d rocks, want 2 medium ones", len(w.rocks))
	}
	if w.score != rockPoints[bigRock] || len(w.bullets) != 0 {
		t.Errorf("score = %d, bullets = %d; want %d and 0", w.score, len(w.bullets), rockPoints[bigRock])
	}
	if len(w.particles) == 0 {
		t.Error("breaking a rock made no sparks")
	}
}

func TestShootingASmallRockRemovesIt(t *testing.T) {
	w := quietWorld(stillRock(900, 360, smallRock), stillRock(100, 100, bigRock))
	w.bullets = []bullet{{x: 900, y: 360, life: 1}}
	w.step(controls{}, dt)
	if len(w.rocks) != 1 || w.rocks[0].size != bigRock {
		t.Errorf("after shooting a small rock, rocks = %+v, want only the big one", w.rocks)
	}
	if w.score != rockPoints[smallRock] {
		t.Errorf("score = %d, want %d", w.score, rockPoints[smallRock])
	}
}

func TestClearingAWaveStartsTheNext(t *testing.T) {
	w := quietWorld()
	w.step(controls{}, dt)
	if w.wave != 2 || len(w.rocks) != firstWaveRocks+1 {
		t.Errorf("after clearing wave 1: wave = %d with %d rocks, want wave 2 with %d", w.wave, len(w.rocks), firstWaveRocks+1)
	}
}

func TestCrashCostsALifeThenRespawns(t *testing.T) {
	w := quietWorld(stillRock(worldWidth/2, worldHeight/2, bigRock))
	w.step(controls{}, dt)
	if w.ship.alive || w.lives != startLives-1 || w.over {
		t.Fatalf("after hitting a rock: ship alive = %v, lives = %d, over = %v; want false, %d, false", w.ship.alive, w.lives, w.over, startLives-1)
	}
	run(&w, int(respawnDelay*60)+1, controls{})
	if !w.ship.alive || w.ship.invulnerable <= 0 {
		t.Errorf("after the respawn delay: ship alive = %v, invulnerable for %v s; want a new, invulnerable ship", w.ship.alive, w.ship.invulnerable)
	}
}

func TestLastCrashEndsTheGame(t *testing.T) {
	w := quietWorld(stillRock(worldWidth/2, worldHeight/2, bigRock))
	w.lives = 1
	w.step(controls{}, dt)
	if !w.over || w.lives != 0 {
		t.Fatalf("after losing the last ship: over = %v, lives = %d; want true, 0", w.over, w.lives)
	}
	run(&w, 5*60, controls{fire: true})
	if w.ship.alive || len(w.bullets) != 0 {
		t.Error("a ship came back, or fired, after the game ended")
	}
}
