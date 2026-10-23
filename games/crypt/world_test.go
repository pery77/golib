package main

import (
	"fmt"
	"os"
	"slices"
	"testing"

	"golib"
)

// TestMain reads the level, as main does, before any test runs.
func TestMain(m *testing.M) {
	var err error
	if theLevel, err = loadLevel(levelModel); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// The rules, tested by calling them directly, on the real level, without a
// window or a keyboard.

// run steps the world for seconds with the same intent in every update, and
// returns everything that happened, in order.
func run(w *world, want intent, seconds float32) []events {
	var all []events
	for range int(seconds/dt + 0.5) {
		all = append(all, w.step(want, dt))
	}
	return all
}

// walk is an intent to run along direction, on the ground.
func walk(x, z float32) intent {
	return intent{move: golib.Vector3{X: x, Z: z}.Normalize(), smash: -1}
}

// freshWorld is the crypt at the start, with the guards taken out, so a test
// of the player isn't grabbed by one wandering by.
func freshWorld(t *testing.T) *world {
	t.Helper()
	w := newWorld(theLevel, nil)
	w.guards = nil
	return w
}

func TestTheLevelHasWhatTheGameNeeds(t *testing.T) {
	l := theLevel
	if got := l.keysInLevel(); got != 3 {
		t.Errorf("the crypt holds %d keys, want 3", got)
	}
	if len(l.doors) != 3 {
		t.Errorf("the crypt has %d locked doors, want 3", len(l.doors))
	}
	if len(l.guards) != 3 {
		t.Errorf("the crypt has %d guards, want 3", len(l.guards))
	}
	for _, g := range l.guards {
		if len(g.route) < 2 {
			t.Errorf("%s patrols %d points, want at least 2", g.name, len(g.route))
		}
	}
	if len(l.torches) < 10 {
		t.Errorf("the crypt has %d torches, want 10 or more", len(l.torches))
	}
	if l.lift.rise < 3 {
		t.Errorf("the lift rises %.1f m, want more than 3", l.lift.rise)
	}
	if len(l.checkpoints) != 4 {
		t.Errorf("the crypt has %d checkpoints, want 4", len(l.checkpoints))
	}
	for _, d := range l.doors {
		if d.size.X < 0.2 || d.size.Y < 2 || d.size.Z < 0.2 {
			t.Errorf("%s is %v: the marker's scale should be the door's size", d.name, d.size)
		}
	}
}

func TestTheCharacterHasItsAnimations(t *testing.T) {
	have := characterModel.Animations()
	for _, name := range []string{animIdle, animWalk, animSprint, animFall, animDie, animCheer, animPunch, animPickUp, animInteract, animReach, animGrab} {
		if !slices.Contains(have, name) {
			t.Errorf("the astronaut has no animation %q; it has %v", name, have)
		}
	}
	if !slices.Contains(characterModel.Bones(), boneTorso) {
		t.Errorf("the astronaut has no bone %q; it has %v", boneTorso, characterModel.Bones())
	}
}

func TestWalkingIntoAWallSlidesAlongIt(t *testing.T) {
	w := freshWorld(t)
	w.placePlayer(golib.Vector3{X: -4.3, Z: 2.0}, 0)
	run(w, walk(-1, 1), 1.2) // into the entry hall's west wall, and along it
	p := w.player.body.Position
	if p.X < -5+playerRadius-0.02 {
		t.Errorf("the astronaut went into the wall: x = %.2f, the wall's face is at -5", p.X)
	}
	if p.Z < 2.0+2.0 {
		t.Errorf("the astronaut moved from z 2.0 to %.2f along the wall, want at least 2 m", p.Z)
	}
	if !w.player.body.OnWall {
		t.Error("the astronaut doesn't touch the wall it slides along")
	}
}

func TestPillarsAndCratesAreInTheWay(t *testing.T) {
	w := freshWorld(t)
	w.placePlayer(golib.Vector3{X: -2.5, Z: 3.0}, 0)
	run(w, walk(0, 1), 1.5) // straight at the pillar at -2.5, 4.5
	if z := w.player.body.Position.Z; z > 4.5-0.5-playerRadius+0.02 {
		t.Errorf("the astronaut walked into the pillar: z = %.2f", z)
	}
	if y := w.player.body.Position.Y; y > 0.01 {
		t.Errorf("the astronaut climbed the pillar's foot to %.2f m", y)
	}
	w.placePlayer(golib.Vector3{X: 3.9, Z: 6.5}, 0)
	run(w, walk(0, 1), 1) // at the crate at 3.9, 7.9
	if z := w.player.body.Position.Z; z > 7.9-0.4-playerRadius+0.02 {
		t.Errorf("the astronaut walked into the crate: z = %.2f", z)
	}
}

func TestTheStairsLeadUpToTheGallery(t *testing.T) {
	w := freshWorld(t)
	w.placePlayer(golib.Vector3{X: -7.2, Z: 16.0}, 0)
	run(w, walk(0, 1), 4)
	p := w.player.body.Position
	if p.Z < 26.5 || p.Y < 2.45 || !w.player.body.OnGround {
		t.Errorf("after 4 s up the stairs the astronaut is at %v, on the ground %v; want on the gallery, 2.5 m up", p, w.player.body.OnGround)
	}
}

func TestTheRampLeadsDownFromTheGallery(t *testing.T) {
	w := freshWorld(t)
	w.placePlayer(golib.Vector3{X: 7.2, Y: 2.5, Z: 27.5}, 180)
	run(w, walk(0, -1), 4)
	p := w.player.body.Position
	if p.Z > 17.4 || p.Y > 0.01 {
		t.Errorf("after 4 s down the ramp the astronaut is at %v; want at its foot, on the floor", p)
	}
}

func TestTheLiftCarriesThePlayerUpAndDown(t *testing.T) {
	w := freshWorld(t)
	lift := w.lift.spec
	w.placePlayer(golib.Vector3{X: lift.center.X, Y: lift.center.Y + lift.size.Y/2, Z: lift.center.Z}, 0)
	highest := float32(0)
	for range int(12 / dt) {
		w.step(noIntent, dt)
		highest = max(highest, w.player.body.Position.Y)
		if w.lift.state == liftUp {
			break
		}
	}
	if w.lift.state != liftUp {
		t.Fatalf("the lift didn't reach the top in 12 s: %.2f m up", w.lift.height)
	}
	if y := w.player.body.Position.Y; abs(y-w.lift.top()) > 0.05 || !w.player.body.OnGround {
		t.Errorf("at the top the astronaut is %.2f m up, on the ground %v; the lift's floor is at %.2f", y, w.player.body.OnGround, w.lift.top())
	}
	if highest > w.lift.top()+0.1 {
		t.Errorf("the astronaut bounced up to %.2f m on a lift that rose to %.2f", highest, w.lift.top())
	}
	run(w, walk(0, 1), 1) // off, onto the upper floor
	if p := w.player.body.Position; p.Z < 20.3 || abs(p.Y-upperFloor) > 0.02 {
		t.Errorf("walking off the lift at the top, the astronaut is at %v; want on the upper floor", p)
	}
	// And down again: back on, and wait.
	w.placePlayer(golib.Vector3{X: lift.center.X, Y: w.lift.top(), Z: lift.center.Z}, 0)
	for range int(12 / dt) {
		w.step(noIntent, dt)
		if w.lift.state == liftDown {
			break
		}
	}
	if y := w.player.body.Position.Y; abs(y-w.lift.top()) > 0.05 || w.lift.height != 0 {
		t.Errorf("at the bottom the astronaut is %.2f m up and the lift %.2f; want it on the lift's floor at %.2f", y, w.lift.height, w.lift.top())
	}
}

// upperFloor is the height of the lift hall's upper floor and the exit chamber.
const upperFloor = 4.0

func TestTheLiftWaitsForSomebodyUnderIt(t *testing.T) {
	w := freshWorld(t)
	w.lift.height, w.lift.state = w.lift.spec.rise, liftUp
	w.lift.wait.Start(0.1)
	w.physics.SetTransform(w.lift.body, w.lift.transform())
	w.placePlayer(golib.Vector3{X: w.lift.spec.center.X, Z: w.lift.spec.center.Z}, 0)
	run(w, noIntent, 6)
	if w.lift.bottom() < playerHeight {
		t.Errorf("the lift came down to %.2f m onto the astronaut standing under it", w.lift.bottom())
	}
	w.placePlayer(golib.Vector3{X: w.lift.spec.center.X, Z: w.lift.spec.center.Z - 3}, 0)
	lowest := w.lift.height
	for range int(3 / dt) {
		w.step(noIntent, dt)
		lowest = min(lowest, w.lift.height)
	}
	if lowest > 0 {
		t.Errorf("once the astronaut stepped away, the lift came down only to %.2f m", lowest)
	}
}

// crateNamed is the crate with this marker name, by its number.
func crateNamed(t *testing.T, w *world, name string) int {
	t.Helper()
	for i, c := range w.crates {
		if c.spec.name == name {
			return i
		}
	}
	t.Fatalf("no crate %q", name)
	return -1
}

func TestBreakingACrateLetsOutItsKey(t *testing.T) {
	w := freshWorld(t)
	i := crateNamed(t, w, "Crate.A2")
	c := w.crates[i]
	w.placePlayer(golib.Vector3{X: c.spec.center.X, Z: c.spec.center.Z - 1.0}, 0)
	run(w, intent{smash: -1, smashNear: true}, dt)
	run(w, noIntent, punchTime)
	if !c.broken {
		t.Fatal("a punch at the crate in reach didn't break it")
	}
	if _, ok := w.physics.Raycast(c.spec.center.Add(golib.Vector3{Z: -1}), golib.Vector3{Z: 1}, 1); ok {
		t.Error("the broken crate is still solid")
	}
	if len(w.keys) != 2 {
		t.Fatalf("there are %d keys after breaking the crate with one, want 2", len(w.keys))
	}
	run(w, noIntent, 0.6) // the key hops out
	run(w, walk(0, 1), 0.5)
	if w.keysFound != 1 || w.keysHeld != 1 {
		t.Errorf("after walking over the key: %d found, %d held; want 1 and 1", w.keysFound, w.keysHeld)
	}
}

func TestACrateOutOfReachDoesntBreak(t *testing.T) {
	w := freshWorld(t)
	i := crateNamed(t, w, "Crate.A1")
	c := w.crates[i]
	w.placePlayer(golib.Vector3{X: c.spec.center.X + 3, Z: c.spec.center.Z}, 0)
	run(w, intent{smash: i}, dt)
	run(w, noIntent, punchTime)
	if c.broken {
		t.Error("a crate 3 m away broke")
	}
	if w.message == "" {
		t.Error("clicking a crate out of reach says nothing")
	}
}

func TestClickingACrateBreaksIt(t *testing.T) {
	w := freshWorld(t)
	i := crateNamed(t, w, "Crate.A2")
	c := w.crates[i]
	w.placePlayer(golib.Vector3{X: c.spec.center.X - 0.5, Z: c.spec.center.Z - 1.2}, 0)
	scene := newPlayScene(w, record{})
	at, ok := scene.camera.ToScreen(c.spec.center)
	if !ok {
		t.Fatal("the crate is behind the camera")
	}
	scene.advance(controls{click: true, mouseX: at.X, mouseY: at.Y}, dt)
	for range 27 { // a punch
		scene.advance(controls{mouseX: at.X, mouseY: at.Y}, dt)
	}
	if !c.broken {
		t.Errorf("clicking the crate at %v on the screen didn't break it", at)
	}
}

func TestALockedDoorNeedsAKey(t *testing.T) {
	w := freshWorld(t)
	d := w.doors[0]
	w.placePlayer(golib.Vector3{X: d.spec.center.X, Z: d.spec.center.Z - 1.5}, 0)
	all := run(w, walk(0, 1), 1)
	if d.open {
		t.Fatal("the door opened without a key")
	}
	if !slices.ContainsFunc(all, func(e events) bool { return e.locked }) {
		t.Error("walking into a locked door doesn't say it is locked")
	}
	if z := w.player.body.Position.Z; z > d.spec.center.Z-d.spec.size.Z/2 {
		t.Errorf("the astronaut walked through the locked door to z %.2f", z)
	}
	w.keysFound, w.keysHeld = 1, 1
	run(w, walk(0, 1), 0.2)
	if !d.open || w.keysHeld != 0 {
		t.Fatalf("with a key the door is open %v, keys held %d", d.open, w.keysHeld)
	}
	run(w, walk(0, 1), 2)
	if z := w.player.body.Position.Z; z < d.spec.center.Z+1 {
		t.Errorf("the open door still stops the astronaut at z %.2f", z)
	}
}

func TestTheWayOutNeedsEveryKey(t *testing.T) {
	w := freshWorld(t)
	w.placePlayer(w.level.exit, 0)
	w.keysFound = 2
	run(w, noIntent, 0.1)
	if w.won {
		t.Fatal("won with 2 keys of 3")
	}
	w.keysFound = 3
	run(w, noIntent, 0.1)
	if !w.won {
		t.Fatal("didn't win at the way out with every key")
	}
}

func TestCheckpointsAreReachedAndSaved(t *testing.T) {
	w := freshWorld(t)
	cp := w.level.checkpoints[1]
	w.placePlayer(cp.position.Add(golib.Vector3{Z: -1}), 0)
	all := run(w, noIntent, 0.1)
	if w.checkpoint != 1 || !slices.ContainsFunc(all, func(e events) bool { return e.checkpoint }) {
		t.Fatalf("standing by the great hall's checkpoint, the checkpoint is %d", w.checkpoint)
	}
	w.doors[0].open = true
	w.crates[crateNamed(t, w, "Crate.A2")].broken = true
	w.keysFound = 1
	saved := w.progress()
	again := newWorld(theLevel, &saved)
	if again.checkpoint != 1 || !again.doors[0].open || again.keysFound != 1 {
		t.Errorf("from saved progress: checkpoint %d, door open %v, keys %d", again.checkpoint, again.doors[0].open, again.keysFound)
	}
	if again.player.body.Position.Distance(cp.position) > 0.01 {
		t.Errorf("carrying on, the astronaut starts at %v, not at the checkpoint %v", again.player.body.Position, cp.position)
	}
	// The broken crate's key, not picked up, lies where the crate was.
	if !slices.ContainsFunc(again.keys, func(k *key) bool { return k.name == "Crate.A2" && !k.taken }) {
		t.Error("the key of the broken crate is gone after carrying on")
	}
}

func TestTheCameraKeepsLeftOnTheLeft(t *testing.T) {
	w := freshWorld(t)
	camera := newFollowCamera(w)
	before, _ := camera.ToScreen(w.player.body.Position)
	run(w, intent{move: camera.Direction(0, -1), smash: -1}, 0.3)
	after, _ := camera.ToScreen(w.player.body.Position)
	if after.X >= before.X {
		t.Errorf("pushing left moved the astronaut from x %.0f to %.0f on the screen", before.X, after.X)
	}
}

func TestTheCameraStaysInFrontOfWalls(t *testing.T) {
	w := freshWorld(t)
	// Backed against the entry hall's south wall, facing north: the camera
	// would be behind the wall.
	w.placePlayer(golib.Vector3{X: 0, Z: 0.6}, 0)
	camera := newFollowCamera(w)
	for range 30 {
		follow(camera, w, dt)
	}
	const wallTop = 3.2 // the entry hall's walls
	if camera.Position.Z < 0.05 && camera.Position.Y < wallTop+0.1 {
		t.Errorf("the camera is at %v, inside or behind the south wall, whose face is at z 0", camera.Position)
	}
	target := w.player.body.Position.Add(golib.Vector3{Y: cameraLookUp})
	way := target.Sub(camera.Position)
	if hit, ok := w.physics.Raycast(camera.Position, way.Normalize(), way.Length()); ok {
		t.Errorf("something solid at %v is between the camera and the astronaut", hit.Position)
	}
}
