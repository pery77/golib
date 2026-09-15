package main

import (
	"testing"

	"golib"
)

const dt = 1.0 / 60 // the step golib.Run passes to Update

// play steps the world for a number of seconds, with the same input every step.
func play(w *world, seconds float32, move float32, jump bool) {
	for i := 0; i < int(seconds*60); i++ {
		w.step(move, jump, dt)
	}
}

func TestPlayerLandsOnTheGround(t *testing.T) {
	w := newWorld()
	play(&w, 1, 0, false)
	if !w.player.onGround {
		t.Fatal("the player isn't on the ground after falling for a second")
	}
	if got, want := w.player.y+playerHeight, float32(660); got != want {
		t.Errorf("player's feet at y = %v, want %v (the top of the ground)", got, want)
	}
}

func TestJumpOnlyFromTheGround(t *testing.T) {
	w := newWorld()
	play(&w, 1, 0, false)
	w.step(0, true, dt)
	if w.player.velocityY >= 0 {
		t.Fatal("jumping from the ground didn't move the player up")
	}
	before := w.player.velocityY
	w.step(0, true, dt)
	if w.player.velocityY <= before {
		t.Error("the player jumped again in mid-air")
	}
}

func TestPlatformsBlockWalking(t *testing.T) {
	w := newWorld()
	w.platforms = []golib.Rectangle{
		{X: 0, Y: 660, Width: 1280, Height: 60},  // ground
		{X: 200, Y: 500, Width: 40, Height: 160}, // a wall standing on it
	}
	play(&w, 1, 0, false)
	play(&w, 2, 1, false) // walk right, into the wall
	if got, want := w.player.x+playerWidth, float32(200); got != want {
		t.Errorf("player's right side at x = %v, want %v (the wall)", got, want)
	}
}

func TestCollectingEveryCoinWins(t *testing.T) {
	w := newWorld()
	for i := range w.coins {
		// Put the player right on the coin.
		w.player = player{x: w.coins[i].x - playerWidth/2, y: w.coins[i].y - playerHeight/2}
		w.step(0, false, dt)
	}
	if got := w.coinsCollected(); got != len(w.coins) {
		t.Fatalf("collected %d coins, want %d", got, len(w.coins))
	}
	if !w.won {
		t.Error("collecting every coin didn't win the game")
	}
}

func TestFallingOffStartsOver(t *testing.T) {
	w := newWorld()
	w.player = player{x: 540, y: 700} // in the gap, below the ground
	play(&w, 1, 0, false)
	if w.player.x != spawnX {
		t.Errorf("player at x = %v after falling off, want the spawn point %v", w.player.x, spawnX)
	}
}
