package main

import (
	"testing"

	"golib"
)

const dt = 1.0 / 60 // the step golib.Run passes to Update

// play steps the world for a number of seconds, with the same input every step.
func play(w *world, seconds float32, move float32, jump, slash bool) {
	for i := 0; i < int(seconds*60); i++ {
		w.step(move, jump, slash, dt)
	}
}

// findTile returns the first tile of the level's ground layer, row by row,
// for which match is true.
func findTile(t *testing.T, what string, match func(golib.MapTile) bool) golib.MapTile {
	t.Helper()
	for _, tile := range level.TilesIn(groundLayer, golib.Rectangle{Width: level.Width(), Height: level.Height()}) {
		if match(tile) {
			return tile
		}
	}
	t.Fatalf("the level has no %s", what)
	return golib.MapTile{}
}

func TestLevelHasWhatTheGameNeeds(t *testing.T) {
	w := newWorld()
	if w.startX <= 0 || w.startX >= w.width || w.startY <= 0 || w.startY > w.height {
		t.Errorf("the start, %v, %v, isn't inside the %v by %v level: add a point named start to the things layer", w.startX, w.startY, w.width, w.height)
	}
	if len(w.chests) != 5 {
		t.Errorf("the level has %d chests, want 5", len(w.chests))
	}
	if len(w.snakes) == 0 {
		t.Error("the level has no snakes")
	}
	for i, s := range w.snakes {
		if s.right <= s.left {
			t.Errorf("snake %d crawls from %v to %v: give it a positive distance", i, s.left, s.right)
		}
	}
	play(&w, 1, 0, false, false)
	if !w.player.onGround {
		t.Error("the player isn't standing a second after the start")
	}
	if feet := w.player.y + playerHeight; int(feet)%16 != 0 || feet != w.startY {
		t.Errorf("the player's feet are at y = %v, want the start, %v, on top of a tile", feet, w.startY)
	}
}

func TestJumpOnlyFromTheGround(t *testing.T) {
	w := newWorld()
	play(&w, 1, 0, false, false)
	w.step(0, true, false, dt)
	if w.player.velocityY >= 0 {
		t.Fatal("jumping from the ground didn't move the player up")
	}
	before := w.player.velocityY
	w.step(0, true, false, dt)
	if w.player.velocityY <= before {
		t.Error("the player jumped again in mid-air")
	}
}

func TestJumpClearsThreeTiles(t *testing.T) {
	w := newWorld()
	w.snakes = nil
	play(&w, 1, 0, false, false)
	ground := w.player.y
	w.step(0, true, false, dt)
	highest := w.player.y
	for range 60 {
		w.step(0, false, false, dt)
		highest = min(highest, w.player.y)
	}
	// The level's platforms are three tiles, 48 pixels, above what the player
	// jumps from.
	if rise := ground - highest; rise < 48 || rise > 56 {
		t.Errorf("a jump rises %v pixels, want 48 to 56", rise)
	}
	if !w.player.onGround || w.player.y != ground {
		t.Errorf("after the jump the player is at y = %v, on the ground: %v; want back at %v", w.player.y, w.player.onGround, ground)
	}
}

func TestCratesBlockWalking(t *testing.T) {
	w := newWorld()
	w.snakes = nil
	crate := findTile(t, "crate", func(tile golib.MapTile) bool { return tile.Class == "crate" })
	// Stand on the ground left of the crate, and walk into it.
	w.player = player{x: crate.X - 40, y: crate.Y + crate.Height - playerHeight}
	play(&w, 2, 1, false, false)
	if got := w.player.x + playerWidth; got != crate.X {
		t.Errorf("player's right side at x = %v, want %v (the crate)", got, crate.X)
	}
	if !w.player.onGround {
		t.Error("the player isn't on the ground next to the crate")
	}
}

func TestOpeningEveryChestWins(t *testing.T) {
	w := newWorld()
	w.snakes = nil
	for i := range w.chests {
		// Put the player right on the chest.
		c := w.chests[i].bounds
		w.player = player{x: c.X + 2, y: c.Y + c.Height - playerHeight}
		w.step(0, false, false, dt)
	}
	if got := w.chestsOpened(); got != len(w.chests) {
		t.Fatalf("opened %d chests, want %d", got, len(w.chests))
	}
	if !w.won {
		t.Error("opening every chest didn't win the game")
	}
}

func TestWaterStartsOver(t *testing.T) {
	w := newWorld()
	water := findTile(t, "water", func(tile golib.MapTile) bool { return tile.Properties.Bool("water") })
	w.player = player{x: water.X, y: water.Y - playerHeight + 2} // feet in the water
	w.step(0, false, false, dt)
	if w.player != w.startingPlayer() {
		t.Errorf("player is %+v after touching water, want back at the start", w.player)
	}
}

func TestFallingOffStartsOver(t *testing.T) {
	w := newWorld()
	w.player = player{x: 100, y: w.height + fallMargin}
	w.step(0, false, false, dt)
	if w.player != w.startingPlayer() {
		t.Errorf("player is %+v after falling off, want back at the start", w.player)
	}
}

func TestSnakesPatrol(t *testing.T) {
	w := newWorld()
	s := w.snakes[0]
	leftmost, rightmost := s.x, s.x
	for range 10 * 60 {
		w.step(0, false, false, dt)
		leftmost, rightmost = min(leftmost, w.snakes[0].x), max(rightmost, w.snakes[0].x)
	}
	if leftmost != s.left || rightmost != s.right {
		t.Errorf("the snake crawled from %v to %v, want from %v to %v", leftmost, rightmost, s.left, s.right)
	}
}

func TestSnakeBiteStartsOver(t *testing.T) {
	w := newWorld()
	s := w.snakes[0]
	w.player = player{x: s.x, y: s.y + snakeHeight - playerHeight}
	w.step(0, false, false, dt)
	if w.player != w.startingPlayer() {
		t.Errorf("player is %+v after touching a snake, want back at the start", w.player)
	}
}

func TestSlashDefeatsSnakes(t *testing.T) {
	for _, facingLeft := range []bool{false, true} {
		w := newWorld()
		w.snakes = w.snakes[:1]
		s := w.snakes[0]
		// Stand just out of the snake's reach, left of it.
		w.player = player{x: s.x - playerWidth - 10, y: s.y + snakeHeight - playerHeight, facingLeft: facingLeft}
		w.step(0, false, true, dt)
		if defeated := w.snakes[0].defeated; defeated == facingLeft {
			t.Errorf("facing left: %v, the snake is defeated: %v", facingLeft, defeated)
		}
		if !w.player.slash.Running() {
			t.Error("the slash didn't start")
		}
	}
}

func TestSlashLastsBeforeTheNext(t *testing.T) {
	w := newWorld()
	w.step(0, false, true, dt)
	w.step(0, false, false, dt)
	left := w.player.slash.Left()
	w.step(0, false, true, dt)
	if w.player.slash.Left() >= left {
		t.Errorf("a second slash started while the first lasted: %v seconds left, were %v", w.player.slash.Left(), left)
	}
	play(&w, slashTime, 0, false, false)
	if w.player.slash.Running() {
		t.Errorf("the slash still has %v seconds left after %v seconds", w.player.slash.Left(), slashTime)
	}
}
