package main

import (
	"testing"

	"golib"
)

// guardAt puts a guard of the great hall's at feet, facing yaw, into a world
// without the others.
func guardAt(w *world, feet golib.Vector3, yaw float32) *guard {
	g := newGuard(w.level.guards[0])
	g.body.Position = feet
	g.yaw = yaw
	w.guards = []*guard{g}
	return g
}

func TestAGuardSeesWhatIsInFrontOfIt(t *testing.T) {
	w := freshWorld(t)
	g := guardAt(w, golib.Vector3{X: 0, Z: 17}, 0) // in the great hall, facing north
	for _, c := range []struct {
		player golib.Vector3
		sees   bool
		why    string
	}{
		{golib.Vector3{X: 0.5, Z: 21}, true, "4 m in front"},
		{golib.Vector3{X: 0, Z: 25.5}, true, "8.5 m in front"},
		{golib.Vector3{X: 0, Z: 27.5}, false, "10.5 m in front, too far"},
		{golib.Vector3{X: 0, Z: 15}, false, "2 m behind"},
		{golib.Vector3{X: -0.5, Z: 16.2}, true, "right behind, close enough to sense"},
		{golib.Vector3{X: 6, Z: 18}, false, "to its side"},
		{golib.Vector3{X: -2.6, Z: 20.5}, true, "in front, with the pillar to one side"},
	} {
		w.placePlayer(c.player, 0)
		if got := w.sees(g); got != c.sees {
			t.Errorf("player %s at %v: sees %v, want %v", c.why, c.player, got, c.sees)
		}
	}
}

func TestWallsAndPillarsHideThePlayer(t *testing.T) {
	w := freshWorld(t)
	g := guardAt(w, golib.Vector3{X: -2.6, Z: 16.0}, 0) // facing the pillar at -2.6, 19
	w.placePlayer(golib.Vector3{X: -2.6, Z: 21}, 0)
	if w.sees(g) {
		t.Error("the guard sees the player through a pillar")
	}
	g.body.Position = golib.Vector3{X: 0, Z: 7} // in the entry hall, facing the locked door
	w.placePlayer(golib.Vector3{X: 0, Z: 11}, 0)
	if w.sees(g) {
		t.Error("the guard sees the player through the locked door")
	}
}

func TestAGuardChasesAroundAPillarAndGrabs(t *testing.T) {
	w := freshWorld(t)
	// The pillar at 2.6, 19 stands between the guard and the player.
	pillar := golib.Vector3{X: 2.6, Z: 19}
	g := guardAt(w, golib.Vector3{X: 2.6, Z: 16.8}, 0)
	w.placePlayer(golib.Vector3{X: 2.6, Z: 21.4}, 0)
	g.startChase(w.player.body.Position)
	closest := float32(100)
	for i := range int(6 / dt) {
		w.step(noIntent, dt)
		closest = min(closest, flat(g.body.Position.Sub(pillar)).Length())
		if w.hearts < startHearts {
			t.Logf("grabbed after %.2f s", float32(i)*dt)
			break
		}
	}
	if w.hearts != startHearts-1 {
		t.Fatalf("the guard didn't get round the pillar to the player in 6 s: it is at %v", g.body.Position)
	}
	if closest < 0.45+guardRadius-0.05 {
		t.Errorf("the guard went %.2f m from the pillar's middle, into it", closest)
	}
}

func TestAGuardGivesUpWhenThePlayerGetsAway(t *testing.T) {
	w := freshWorld(t)
	g := guardAt(w, golib.Vector3{X: 0, Z: 17}, 0)
	w.placePlayer(golib.Vector3{X: 0, Z: 22}, 0)
	run(w, noIntent, 0.1)
	if g.state != chasing {
		t.Fatal("the guard didn't start chasing the player in front of it")
	}
	// The player gets away, out of sight on the other side of the locked door.
	w.placePlayer(golib.Vector3{X: 0, Z: 6}, 0)
	run(w, noIntent, searchTime+0.5)
	if g.state != patrolling {
		t.Errorf("%.1f s after losing the player the guard still chases", searchTime+0.5)
	}
}

func TestTheGuardsWalkTheirRounds(t *testing.T) {
	w := newWorld(theLevel, nil)
	w.placePlayer(golib.Vector3{X: 0, Z: 3}, 0) // out of everyone's sight
	reached := make([]int, len(w.guards))
	last := make([]int, len(w.guards))
	for i, g := range w.guards {
		last[i] = g.next
	}
	for range int(40 / dt) {
		w.step(noIntent, dt)
		for i, g := range w.guards {
			if g.next != last[i] {
				reached[i]++
				last[i] = g.next
			}
		}
	}
	for i, g := range w.guards {
		if reached[i] < len(g.spec.route) {
			t.Errorf("%s reached %d points of its round of %d in 40 s; it is at %v", g.spec.name, reached[i], len(g.spec.route), g.body.Position)
		}
	}
}

func TestAGrabSendsThePlayerBackToTheCheckpoint(t *testing.T) {
	w := freshWorld(t)
	g := guardAt(w, golib.Vector3{X: 0, Z: 20}, 180)
	w.checkpoint = 1
	w.placePlayer(golib.Vector3{X: 0, Z: 19.5}, 0)
	run(w, noIntent, 0.2)
	if w.hearts != startHearts-1 || !w.player.caught.Running() {
		t.Fatalf("a guard right by the player didn't grab them: hearts %d", w.hearts)
	}
	run(w, noIntent, caughtTime)
	if d := w.player.body.Position.Distance(w.level.checkpoints[1].position); d > 0.1 {
		t.Errorf("after the grab the player is %.2f m from the checkpoint", d)
	}
	if again := w.guards[0]; again == g || again.state != patrolling || again.body.Position.Distance(w.level.guards[0].start) > 0.5 {
		t.Errorf("the guard isn't back on its round: at %v, chasing %v", again.body.Position, again.state == chasing)
	}
}

func TestTheLastGrabEndsTheGame(t *testing.T) {
	w := freshWorld(t)
	w.hearts = 1
	guardAt(w, golib.Vector3{X: 0, Z: 20}, 180)
	w.placePlayer(golib.Vector3{X: 0, Z: 19.5}, 0)
	all := run(w, noIntent, caughtTime+0.3)
	over := false
	for _, e := range all {
		over = over || e.gameOver
	}
	if !over || !w.over {
		t.Errorf("the last heart lost: game over said %v, world over %v", over, w.over)
	}
}

// playerNav is where the astronaut can walk, as the guards' NavMesh is made.
func playerNav(w *world) *golib.NavMesh {
	return golib.NewNavMesh(w.physics, golib.CharacterController{Height: playerHeight, Radius: playerRadius, StepHeight: stepHeight})
}

func TestEveryKeyAndTheWayOutCanBeReached(t *testing.T) {
	w := freshWorld(t)
	start := w.level.start
	front := func(name string) golib.Vector3 { // where to stand to punch a crate
		c := w.crates[crateNamed(t, w, name)].spec
		return golib.Vector3{X: c.center.X, Y: c.center.Y - c.size.Y/2, Z: c.center.Z - c.size.Z/2 - 0.5}
	}
	reach := func(what string, from, to golib.Vector3, want bool) {
		t.Helper()
		_, ok := playerNav(w).Path(from, to)
		if ok != want {
			t.Errorf("%s: a way from %v to %v is %v, want %v", what, from, to, ok, want)
		}
	}
	open := func(i int) {
		w.doors[i].open = true
		w.physics.Remove(w.doors[i].body)
	}
	liftTop := func() golib.Vector3 {
		return golib.Vector3{X: w.lift.spec.center.X, Y: w.lift.top(), Z: w.lift.spec.center.Z}
	}

	reach("the first key's crate, in the entry hall", start, front("Crate.A2"), true)
	reach("the great hall, through the locked door", start, golib.Vector3{Z: 18}, false)
	open(0)
	reach("the gallery's key, up the stairs", start, theLevel.keys[0].center, true)
	reach("the lift hall, through the second locked door", start, golib.Vector3{X: 16, Z: 15.7}, false)
	open(1)
	reach("the third key's crate, in the lift hall", start, front("Crate.C4"), true)
	reach("the lift", start, liftTop(), true)
	reach("the upper floor, without the lift", start, golib.Vector3{X: 15, Y: upperFloor, Z: 23}, false)
	w.lift.height = w.lift.spec.rise
	w.physics.SetTransform(w.lift.body, w.lift.transform())
	reach("the upper floor, from the lift at the top", liftTop(), golib.Vector3{X: 15, Y: upperFloor, Z: 23}, true)
	reach("the exit chamber, through the third locked door", liftTop(), w.level.exit, false)
	open(2)
	reach("the way out", liftTop(), w.level.exit, true)
}
