package main

import (
	"testing"

	"golib"
)

const dt = 1.0 / 60 // the step golib.Run passes to Update

// emptyWorld returns a new world at the middle of the lagoon with no
// crystals, for tests of one thing.
func emptyWorld() *world {
	golib.SetRandomSeed(1)
	w := newWorld()
	w.crystals = nil
	return w
}

// run steps the world for seconds with the same controls, and gathers what
// happened.
func run(w *world, throttle, steer, seconds float32) (collected, faded int, bumped float32, ticks []int, timeUps int) {
	for range int(seconds*60 + 0.5) {
		e := w.step(throttle, steer, dt)
		collected += len(e.collected)
		faded += len(e.faded)
		bumped = max(bumped, e.bumped)
		if e.tick > 0 {
			ticks = append(ticks, e.tick)
		}
		if e.timeUp {
			timeUps++
		}
	}
	return
}

func TestRunStartsFacingTheSunset(t *testing.T) {
	w := emptyWorld()
	toward := golib.Vector3{X: -sunDirection.X, Z: -sunDirection.Z}.Normalize()
	if facing := w.craft.heading().Dot(toward); facing < 0.99 {
		t.Errorf("the hovercraft starts facing %v, want towards the sun, %v", w.craft.heading(), toward)
	}
}

func TestThrustMovesAlongTheNose(t *testing.T) {
	w := emptyWorld()
	w.craft.yaw = 0 // facing +Z
	run(w, 1, 0, 1)
	p := w.craft.position
	if p.Z < 4 || p.X > 0.01 || p.X < -0.01 {
		t.Errorf("after a second of full thrust facing +Z, the hovercraft is at %v, want well along +Z", p)
	}
}

func TestHasATopSpeedAndGlidesToAStop(t *testing.T) {
	w := emptyWorld()
	w.craft.yaw = 0
	for range 300 {
		w.step(1, 0, dt)
		if speed := w.craft.velocity.Length(); speed > maxSpeed+0.001 {
			t.Fatalf("the hovercraft goes %v m/s, over the top speed of %v", speed, maxSpeed)
		}
	}
	w.craft.position = golib.Vector3{}
	w.craft.velocity = golib.Vector3{Z: 10}
	run(w, 0, 0, 0.5)
	if speed := w.craft.velocity.Length(); speed >= 10 || speed < 3 {
		t.Errorf("half a second after letting go at 10 m/s, it goes %v m/s, want slower but still gliding", speed)
	}
	run(w, -1, 0, 1)
	if ahead := w.craft.velocity.Dot(w.craft.heading()); ahead > 0 {
		t.Errorf("after a second of braking it still goes %v m/s ahead, want stopped or reversing", ahead)
	}
}

func TestSteeringRightTurnsRightOnTheScreen(t *testing.T) {
	w := emptyWorld()
	w.craft.yaw = 30
	s := &playScene{world: w, camera: newCamera()}
	s.camera.Heading = w.craft.yaw
	s.placeCamera(dt)
	s.camera.Snap()
	for range 30 {
		w.step(1, 1, dt) // thrust and steer right, with the camera held still
	}
	at, ok := s.camera.ToScreen(w.craft.position)
	if !ok || at.X <= screenWidth/2+20 {
		t.Errorf("steering right moved the hovercraft to %v on the screen (in front: %v), want right of the middle", at, ok)
	}
}

func TestSideSlideDiesFasterThanSpeedAhead(t *testing.T) {
	w := emptyWorld()
	w.craft.yaw = 0
	w.craft.velocity = golib.Vector3{X: 8, Z: 8} // as much sideways as ahead
	run(w, 0, 0, 1)
	if side, ahead := w.craft.velocity.X, w.craft.velocity.Z; side >= ahead {
		t.Errorf("after a second the hovercraft slides %v m/s sideways and %v ahead, want the slide to die faster", side, ahead)
	}
}

func TestPolesAndIsletsAreSolid(t *testing.T) {
	for _, o := range obstacles {
		w := emptyWorld()
		// Start 10 meters to the side of it and drive straight at it.
		w.craft.position = o.position.Add(golib.Vector3{X: -(o.radius + 10)})
		w.craft.yaw = 90 // facing +X
		_, _, bumped, _, _ := run(w, 1, 0, 3)
		if bumped == 0 {
			t.Errorf("driving into the obstacle at %v made no bump", o.position)
		}
		if gap := flat(w.craft.position.Sub(o.position)).Length() - o.radius - craftRadius; gap < -0.01 {
			t.Errorf("driving into the obstacle at %v left the hovercraft %v meters inside it", o.position, -gap)
		}
	}
}

func TestTheLagoonKeepsTheHovercraftIn(t *testing.T) {
	w := emptyWorld()
	w.craft.position = golib.Vector3{X: 0, Z: -40} // open water towards -Z
	w.craft.yaw = 180                              // facing -Z, out of the lagoon
	run(w, 1, 0, 20)
	if out := flat(w.craft.position).Length(); out > lagoonRadius+lagoonEdge+0.01 {
		t.Errorf("after 20 seconds of thrust out of the lagoon, the hovercraft is %v meters from the middle, want at most %v", out, lagoonRadius+lagoonEdge)
	}
	if reach := lagoonRadius + lagoonEdge + craftRadius; reach >= buoyRing-0.5 {
		t.Errorf("the hovercraft reaches %v meters from the middle, into the buoys at %v", reach, buoyRing)
	}
}

func TestCollectingACrystalScoresAndReplacesIt(t *testing.T) {
	golib.SetRandomSeed(2)
	w := newWorld()
	if len(w.crystals) != crystalCount {
		t.Fatalf("a new run has %d crystals, want %d", len(w.crystals), crystalCount)
	}
	target := w.crystals[0]
	target.golden = false
	w.crystals[0] = target
	w.craft.position = flat(target.position)
	e := w.step(0, 0, dt)
	if len(e.collected) != 1 || w.score != 1 {
		t.Fatalf("sitting on a crystal collected %d and scored %d, want 1 and 1", len(e.collected), w.score)
	}
	if len(w.crystals) != crystalCount {
		t.Errorf("after collecting one there are %d crystals, want %d", len(w.crystals), crystalCount)
	}
	for _, k := range w.crystals {
		if d := flat(k.position.Sub(w.craft.position)).Length(); d < crystalReach {
			t.Errorf("a crystal is %v meters from the hovercraft right after collecting, close enough to collect at once", d)
		}
	}
}

func TestNewCrystalsAppearInOpenWater(t *testing.T) {
	golib.SetRandomSeed(3)
	w := newWorld()
	for range 200 {
		w.crystals = w.crystals[1:]
		w.spawnCrystal()
		k := w.crystals[len(w.crystals)-1]
		if from := flat(k.position).Length(); from > crystalSpawnArea+0.01 {
			t.Fatalf("a crystal appeared %v meters from the middle, past %v", from, crystalSpawnArea)
		}
		for _, o := range obstacles {
			if d := flat(k.position.Sub(o.position)).Length(); d < o.radius+1 {
				t.Fatalf("a crystal appeared %v meters from an obstacle of radius %v at %v", d, o.radius, o.position)
			}
		}
		if k.position.Y != crystalHeight {
			t.Fatalf("a crystal floats %v meters up, want %v", k.position.Y, crystalHeight)
		}
	}
}

func TestEveryFifthCrystalIsGoldenAndFades(t *testing.T) {
	w := emptyWorld()
	w.spawned = 0
	var golden []int
	for i := range 10 {
		w.spawnCrystal()
		if w.crystals[i].golden {
			golden = append(golden, i+1)
		}
	}
	if len(golden) != 2 || golden[0] != goldenEvery || golden[1] != 2*goldenEvery {
		t.Fatalf("of the first 10 crystals, numbers %v were golden, want %d and %d", golden, goldenEvery, 2*goldenEvery)
	}
	if (crystal{golden: true}).points() != goldenPoints || (crystal{}).points() != 1 {
		t.Errorf("a golden crystal scores %d and a plain one %d, want %d and 1", (crystal{golden: true}).points(), (crystal{}).points(), goldenPoints)
	}
	w.craft.position = golib.Vector3{X: 500} // far from all of them
	_, faded, _, _, _ := run(w, 0, 0, goldenLife+0.1)
	if faded != 2 {
		t.Errorf("after %v seconds, %d golden crystals faded, want 2", goldenLife, faded)
	}
	if len(w.crystals) != 10 {
		t.Errorf("after golden crystals faded there are %d crystals, want them replaced, 10", len(w.crystals))
	}
}

func TestTheRunLastsNinetySecondsAndCountsDownTheLastTen(t *testing.T) {
	w := emptyWorld()
	_, _, _, ticks, timeUps := run(w, 0, 0, roundTime-0.5)
	if w.over || timeUps != 0 {
		t.Fatalf("half a second before the end, the run is over: %v", w.over)
	}
	_, _, _, more, timeUps := run(w, 0, 0, 1)
	ticks = append(ticks, more...)
	if !w.over || timeUps != 1 {
		t.Errorf("half a second after the end, over is %v with %d time-ups, want true and 1", w.over, timeUps)
	}
	want := []int{9, 8, 7, 6, 5, 4, 3, 2, 1}
	if len(ticks) != len(want) {
		t.Fatalf("the last seconds ticked %v, want %v", ticks, want)
	}
	for i := range want {
		if ticks[i] != want[i] {
			t.Fatalf("the last seconds ticked %v, want %v", ticks, want)
		}
	}
	score := w.score
	w.crystals = []crystal{{position: golib.Vector3{Y: crystalHeight}}}
	w.craft.position = golib.Vector3{}
	w.step(0, 0, dt)
	if w.score != score {
		t.Errorf("a crystal was collected after the time was up")
	}
}

func TestClockText(t *testing.T) {
	for seconds, want := range map[float32]string{90: "1:30", 59.2: "1:00", 9.01: "0:10", 0.5: "0:01", 0: "0:00"} {
		if got := clockText(seconds); got != want {
			t.Errorf("clockText(%v) = %q, want %q", seconds, got, want)
		}
	}
}

func TestTheSkyLoads(t *testing.T) {
	if err := sunsetSky.Err(); err != nil {
		t.Fatal(err)
	}
}
