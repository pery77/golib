package main

import "testing"

const dt = 1.0 / 60 // the step golib.Run passes to Update

// room is a small level for the rule tests: the player between two crates,
// with a goal beyond each.
var room = []string{
	"#########",
	"#.$ @$ .#",
	"#       #",
	"#########",
}

func TestWalking(t *testing.T) {
	w := newWorld(parseLayout(t, room...))
	result := w.step(down)
	if !result.moved || result.pushed != -1 || w.player != (cell{4, 2}) {
		t.Fatalf("walking down: %+v, player at %v; want moved to 4, 2", result, w.player)
	}
	if w.moves != 1 || w.pushes != 0 {
		t.Errorf("moves %d, pushes %d; want 1 and 0", w.moves, w.pushes)
	}
}

func TestWallsBlock(t *testing.T) {
	w := newWorld(parseLayout(t, room...))
	result := w.step(up)
	if result.moved || w.player != (cell{4, 1}) || w.moves != 0 {
		t.Errorf("walking into a wall: %+v, player at %v, %d moves; want no move", result, w.player, w.moves)
	}
	if w.facing != up {
		t.Errorf("the player faces %v after bumping the wall above, want up", w.facing)
	}
}

func TestPushing(t *testing.T) {
	w := newWorld(parseLayout(t, room...))
	result := w.step(right) // the crate at 5, 1 goes to 6, 1
	if !result.moved || result.pushed != 1 || result.landed {
		t.Fatalf("pushing right: %+v; want a push of crate 1 that doesn't land", result)
	}
	if w.crates[1] != (cell{6, 1}) || w.player != (cell{5, 1}) || w.pushes != 1 {
		t.Fatalf("after the push: crate at %v, player at %v, %d pushes", w.crates[1], w.player, w.pushes)
	}
	result = w.step(right) // onto the goal at 7, 1
	if !result.landed || w.cratesOnGoals() != 1 {
		t.Errorf("pushing onto the goal: %+v, %d crates on goals", result, w.cratesOnGoals())
	}
	result = w.step(right) // the wall stops the crate
	if result.moved || w.crates[1] != (cell{7, 1}) {
		t.Errorf("pushing a crate into a wall: %+v, crate at %v; want no move", result, w.crates[1])
	}
}

func TestCratesBlockCrates(t *testing.T) {
	w := newWorld(parseLayout(t,
		"#######",
		"#@$$ .#",
		"#    .#",
		"#######",
	))
	if result := w.step(right); result.moved {
		t.Errorf("pushed two crates at once: %+v", result)
	}
}

func TestFinishing(t *testing.T) {
	w := newWorld(parseLayout(t, room...))
	for _, d := range []direction{right, right, left, left, left} {
		w.step(d)
	}
	if w.solved() {
		t.Fatal("solved before the second crate is on its goal")
	}
	w.step(left)
	if !w.solved() {
		t.Fatalf("not solved with crates at %v", w.crates)
	}
	if result := w.step(right); result.moved {
		t.Error("the player moved after finishing")
	}
}

func TestUndo(t *testing.T) {
	w := newWorld(parseLayout(t, room...))
	start := newWorld(w.layout)
	for _, d := range []direction{right, down, left, up} {
		w.step(d)
	}
	for w.undo() {
	}
	if w.player != start.player || w.crates[0] != start.crates[0] || w.crates[1] != start.crates[1] {
		t.Errorf("after undoing everything: player %v, crates %v; want %v, %v", w.player, w.crates, start.player, start.crates)
	}
	if w.moves != 0 || w.pushes != 0 || w.facing != down {
		t.Errorf("after undoing everything: %d moves, %d pushes, facing %v; want 0, 0, down", w.moves, w.pushes, w.facing)
	}
	if w.undo() {
		t.Error("undo reported a step with none left")
	}
}

func TestUndoTakesBackOnePush(t *testing.T) {
	w := newWorld(parseLayout(t, room...))
	w.step(right)
	w.step(right)
	w.undo()
	if w.crates[1] != (cell{6, 1}) || w.player != (cell{5, 1}) || w.moves != 1 || w.pushes != 1 {
		t.Errorf("after two pushes and an undo: crate at %v, player at %v, %d moves, %d pushes", w.crates[1], w.player, w.moves, w.pushes)
	}
}

func TestRestartKeepsTheLayout(t *testing.T) {
	l := parseLayout(t, room...)
	w := newWorld(l)
	w.step(right)
	if l.crates[1] != (cell{5, 1}) {
		t.Errorf("playing moved the layout's crate to %v", l.crates[1])
	}
}
