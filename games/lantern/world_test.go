package main

import (
	"testing"

	"golib"
)

const dt = 1.0 / 60 // the step golib.Run passes to Update

func TestLevelLoadsWithItsMarkersAndLight(t *testing.T) {
	if err := level.Err(); err != nil {
		t.Fatal(err)
	}
	w := newWorld()
	if len(w.gems) != 6 {
		t.Errorf("the level has %d gems, want 6", len(w.gems))
	}
	if len(w.torches) != 8 {
		t.Errorf("the level has %d torches, want 8", len(w.torches))
	}
	if w.gate.size.X == 0 || w.exit == (golib.Vector3{}) {
		t.Errorf("the gate (%v) or the exit (%v) is missing", w.gate, w.exit)
	}
	if _, ok := level.Sun(); !ok {
		t.Error("the level has no moon (sun lamp)")
	}
	if n := len(level.Lights()); n != 8 {
		t.Errorf("the level has %d lamps, want 8", n)
	}
	if err := runeImage.Err(); err != nil {
		t.Error(err)
	}
}

func TestExplorerStandsOnTheFloor(t *testing.T) {
	w := newWorld()
	for range 60 {
		w.step(intent{}, dt)
	}
	if !w.player.body.OnGround || w.player.body.Position.Y < -0.05 || w.player.body.Position.Y > 0.05 {
		t.Errorf("after a second at the start, the explorer is at %v, on the ground %v", w.player.body.Position, w.player.body.OnGround)
	}
}

func TestWallsStopTheExplorer(t *testing.T) {
	w := newWorld()
	for range 600 { // ten seconds walking at the hall's side wall
		w.step(intent{move: golib.Vector3{X: 1}}, dt)
	}
	if x := w.player.body.Position.X; x > 5 {
		t.Errorf("walking into the wall, the explorer went through it, to x %v", x)
	}
}

func TestRunesLieOnTheFloorUnderTheGems(t *testing.T) {
	w := newWorld()
	for _, g := range w.gems {
		if g.floor.Y > 0.01 || g.floor.Y < -0.6 || g.position.Y-g.floor.Y < 0.5 {
			t.Errorf("the gem at %v has its rune at %v, not on the floor under it", g.position, g.floor)
		}
	}
}

// walkTo walks the explorer straight towards target until they are within
// reach of it, for at most limit seconds, and reports what happened.
func walkTo(w *world, target golib.Vector3, reach, limit float32) []event {
	var events []event
	for t := float32(0); t < limit; t += dt {
		way := target.Sub(w.player.body.Position)
		way.Y = 0
		if way.Length() < reach {
			break
		}
		events = append(events, w.step(intent{move: way.Normalize()}, dt))
	}
	return events
}

func TestSixGemsOpenTheGateAndTheExplorerEscapes(t *testing.T) {
	w := newWorld()
	// Through the halls by the middle, which every doorway is on, to each gem.
	route := []golib.Vector3{
		w.gems[0].position, w.gems[1].position, {Z: 11}, {Z: 13}, {X: -5.5, Z: 14}, {X: -5, Z: 20},
		w.gems[2].position, {X: 4.6, Z: 23}, {X: 4.6, Z: 26.6}, w.gems[3].position, {Z: 27}, {Z: 29.5},
		w.gems[4].position, {Z: 34}, w.gems[5].position, {Z: 38.5},
	}
	for _, at := range route {
		walkTo(w, at, 0.3, 20)
	}
	if n := w.gemsTaken(); n != 6 {
		for i, g := range w.gems {
			t.Logf("gem %d at %v taken %v", i, g.position, g.taken)
		}
		t.Fatalf("after walking to every gem, %d are taken, want 6; the explorer is at %v", n, w.player.body.Position)
	}
	if !w.gate.open {
		t.Fatal("the six gems didn't open the gate")
	}
	for range 200 { // let it sink
		w.step(intent{}, dt)
	}
	walkTo(w, w.exit, 0.3, 20)
	if !w.escaped {
		t.Errorf("walking to the exit through the open gate, the explorer stopped at %v, the exit is at %v", w.player.body.Position, w.exit)
	}
}

func TestGateIsSolidUntilOpened(t *testing.T) {
	w := newWorld()
	walkTo(w, golib.Vector3{Z: 38.5}, 0.3, 30)
	walkTo(w, w.exit, 0.3, 5)
	if z := w.player.body.Position.Z; z > w.gate.position.Z-w.gate.size.Z/2 {
		t.Errorf("the closed gate let the explorer through to z %v", z)
	}
}

func TestWadingIsSlower(t *testing.T) {
	w := newWorld()
	walkTo(w, golib.Vector3{Z: 13.5}, 0.3, 20)
	walkTo(w, golib.Vector3{Z: 20}, 0.3, 20)
	if !w.player.wading {
		t.Fatalf("in the middle of the pool, at %v, the explorer isn't wading", w.player.body.Position)
	}
	before := w.player.body.Position
	w.step(intent{move: golib.Vector3{X: 1}}, dt)
	if speed := w.player.body.Moved.Length() / dt; speed > wadeSpeed+0.1 {
		t.Errorf("wading from %v, the explorer moved at %v m/s, want at most %v", before, speed, wadeSpeed)
	}
}

func TestWalkingForwardMovesAwayFromTheCamera(t *testing.T) {
	w := newWorld()
	camera := newCamera(w)
	for range 30 {
		want := steer(controls{forward: 1}, camera, dt)
		w.step(want, dt)
		follow(camera, w, dt)
	}
	if w.player.body.Position.Z <= w.start.Z+0.5 {
		t.Errorf("walking forward from the start, which faces +Z, the explorer went to %v", w.player.body.Position)
	}
}
