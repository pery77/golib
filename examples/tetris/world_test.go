package main

import (
	"golib"
	"testing"
)

// dt is the step golib.Run passes to Update.
const dt = 1.0 / 60

// newTestWorld returns a world with a fixed piece sequence, so tests repeat.
func newTestWorld() *world {
	golib.SetRandomSeed(1)
	return newWorld()
}

// fillRow fills row y completely except for the columns in gap.
func fillRow(w *world, y int, gap ...int) {
	for x := 0; x < boardColumns; x++ {
		w.board[y][x] = int(pieceJ) + 1 // any non-zero value
	}
	for _, x := range gap {
		if x >= 0 && x < boardColumns {
			w.board[y][x] = 0
		}
	}
}

// TestBagGivesEveryPieceOnce checks that a bag holds each tetromino exactly
// once, so no piece waits too long.
func TestBagGivesEveryPieceOnce(t *testing.T) {
	golib.SetRandomSeed(7)
	w := &world{}
	seen := map[pieceKind]int{}
	for range pieceKinds {
		seen[w.draw()]++
	}
	for k := pieceKind(0); k < pieceKinds; k++ {
		if seen[k] != 1 {
			t.Fatalf("piece %d drawn %d times in one bag, want 1", k, seen[k])
		}
	}
}

// TestSpawnIsInsideTheBoard checks that a fresh piece sits at the top, whole
// and not overlapping anything.
func TestSpawnIsInsideTheBoard(t *testing.T) {
	w := newTestWorld()
	if w.phase != phasePlaying {
		t.Fatal("a new world is not playing")
	}
	for _, c := range w.piece.cells() {
		if c.x < 0 || c.x >= boardColumns || c.y < 0 || c.y >= boardRows {
			t.Fatalf("spawned cell %v is off the board", c)
		}
	}
}

// TestMoveStopsAtTheWall checks that a piece moved left over and over rests
// against the wall, not through it.
func TestMoveStopsAtTheWall(t *testing.T) {
	w := newTestWorld()
	for range boardColumns + 2 {
		w.step(actions{moveX: -1}, dt)
	}
	for _, c := range w.piece.cells() {
		if c.x < 0 {
			t.Fatalf("piece went through the left wall at %v", c)
		}
	}
}

// TestHardDropLocksAndSpawnsANewPiece checks that a hard drop fixes the piece
// and brings the next one.
func TestHardDropLocksAndSpawnsANewPiece(t *testing.T) {
	w := newTestWorld()
	w.step(actions{hardDrop: true}, dt)
	if w.phase != phasePlaying {
		t.Fatal("the game is not playing after a hard drop")
	}
	occupied := 0
	for y := 0; y < boardRows; y++ {
		for x := 0; x < boardColumns; x++ {
			if w.board[y][x] != 0 {
				occupied++
			}
		}
	}
	if occupied != 4 {
		t.Fatalf("the board holds %d blocks after one piece, want 4", occupied)
	}
}

// TestSingleClearScores checks the score and the events of clearing one line.
func TestSingleClearScores(t *testing.T) {
	w := newTestWorld()
	fillRow(w, boardRows-1, 4, 5)
	// An O piece dropped into the gap fills the last two cells.
	w.piece = activePiece{kind: pieceO, x: 4, y: boardRows - 2}
	w.hardDrop()

	if w.phase != phaseClearing {
		t.Fatalf("phase is %d after a clear, want clearing", w.phase)
	}
	if w.score != 100 {
		t.Fatalf("score is %d after one line, want 100", w.score)
	}

	events := w.TakeEvents()
	clearValue := -1
	for _, e := range events {
		if e.kind == eventClear {
			clearValue = e.value
		}
	}
	if clearValue != 1 {
		t.Fatalf("the clear event says %d lines, want 1", clearValue)
	}

	// Let the flash finish: the row disappears and a new piece arrives.
	for w.phase == phaseClearing {
		w.step(actions{}, dt)
	}
	if w.lines != 1 {
		t.Fatalf("lines is %d after one clear, want 1", w.lines)
	}
	if w.board[boardRows-1][0] != 0 {
		t.Fatal("the cleared row still holds a block")
	}
}

// TestTetrisScores checks that clearing four lines at once scores 800.
func TestTetrisScores(t *testing.T) {
	w := newTestWorld()
	for y := boardRows - 4; y < boardRows; y++ {
		fillRow(w, y, 0)
	}
	// A vertical I dropped into column 0 fills all four rows.
	w.piece = activePiece{kind: pieceI, rotation: 1, x: -2, y: boardRows - 4}
	w.hardDrop()

	if w.phase != phaseClearing {
		t.Fatalf("phase is %d after a tetris, want clearing", w.phase)
	}
	if w.score != 800 {
		t.Fatalf("score is %d after a tetris, want 800", w.score)
	}
}

// TestLevelRisesEveryTenLines checks that the level grows with the lines, and
// that the drop gets faster.
func TestLevelRisesEveryTenLines(t *testing.T) {
	w := newTestWorld()
	before := w.dropInterval()
	w.lines = linesPerLevel - 1
	fillRow(w, boardRows-1, 4, 5)
	w.piece = activePiece{kind: pieceO, x: 4, y: boardRows - 2}
	w.hardDrop()
	for w.phase == phaseClearing {
		w.step(actions{}, dt)
	}
	if w.level != 2 {
		t.Fatalf("level is %d after %d lines, want 2", w.level, w.lines)
	}
	if w.dropInterval() >= before {
		t.Fatalf("the drop did not speed up: %v then %v", before, w.dropInterval())
	}
}

// TestComboBonus checks that clearing on two pieces in a row pays a bonus.
func TestComboBonus(t *testing.T) {
	w := newTestWorld()
	clearOne := func() {
		fillRow(w, boardRows-1, 4, 5)
		w.piece = activePiece{kind: pieceO, x: 4, y: boardRows - 2}
		w.hardDrop()
		for w.phase == phaseClearing {
			w.step(actions{}, dt)
		}
	}
	clearOne()            // first clear: 100, combo 1
	afterFirst := w.score // 100
	clearOne()            // second clear: 100 + combo bonus 50
	if w.score != afterFirst+100+50 {
		t.Fatalf("score is %d after a combo, want %d", w.score, afterFirst+100+50)
	}
}

// TestGameOverWhenTheStackReachesTheTop checks that a piece that can't spawn
// ends the game.
func TestGameOverWhenTheStackReachesTheTop(t *testing.T) {
	w := newTestWorld()
	for y := 0; y < boardRows; y++ {
		for x := 0; x < boardColumns; x++ {
			w.board[y][x] = int(pieceJ) + 1
		}
	}
	w.spawnNext()
	if w.phase != phaseGameOver {
		t.Fatal("the game did not end when the stack reached the top")
	}
}

// TestHoldSwapsOncePerPiece checks that hold brings a piece back, and can't be
// used twice on the same piece.
func TestHoldSwapsOncePerPiece(t *testing.T) {
	w := newTestWorld()
	first := w.piece.kind
	w.step(actions{hold: true}, dt)
	if !w.hasHold || w.hold != first {
		t.Fatal("hold did not take the first piece")
	}
	held := w.hold
	before := w.piece.kind
	w.step(actions{hold: true}, dt)
	if w.piece.kind != before {
		t.Fatal("hold worked twice for one piece")
	}
	w.hardDrop()
	w.step(actions{hold: true}, dt)
	if w.piece.kind != held {
		t.Fatalf("hold brought %d, want the held piece %d", w.piece.kind, held)
	}
}

// TestGhostSitsBelowThePiece checks that the ghost row is at or under the piece
// and that the piece could rest there.
func TestGhostSitsBelowThePiece(t *testing.T) {
	w := newTestWorld()
	ghost := w.ghostY()
	if ghost < w.piece.y {
		t.Fatalf("ghost row %d is above the piece at %d", ghost, w.piece.y)
	}
	rest := activePiece{kind: w.piece.kind, rotation: w.piece.rotation, x: w.piece.x, y: ghost}
	if w.collides(rest) {
		t.Fatal("the ghost row overlaps something")
	}
	if !w.collides(rest.moved(0, 1)) {
		t.Fatal("the ghost is not resting on anything")
	}
}
