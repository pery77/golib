package main

import (
	"math"
	"testing"

	"golib"
)

const dt = 1.0 / 60 // the step golib.Run passes to Update

// near says whether two vectors are within a hair of each other.
func near(a, b golib.Vector3) bool {
	return a.Distance(b) < 0.01
}

// run steps the world for seconds, rolling the same way, and gathers what happened.
func run(w *world, move golib.Vector3, seconds float32) events {
	var all events
	for range int(seconds * 60) {
		e := w.step(move, dt)
		all.collected += e.collected
		all.levelUp = all.levelUp || e.levelUp
		all.fell = all.fell || e.fell
		all.hit = all.hit || e.hit
		all.lostLife = all.lostLife || e.lostLife
		all.gameOver = all.gameOver || e.gameOver
		if e.lostLife {
			break
		}
	}
	return all
}

// emptyWorld returns a new world with the gems and the cubes out of the way,
// for tests of one thing.
func emptyWorld() *world {
	golib.SetRandomSeed(1)
	w := newWorld()
	w.gems = nil
	w.cubes = nil
	return w
}

func TestMarbleRollsWhereAsked(t *testing.T) {
	w := emptyWorld()
	run(w, golib.Vector3{Z: 1}, 1)
	p := w.marble.position
	if p.Z < 3 || math.Abs(float64(p.X)) > 0.001 || p.Y != marbleRadius {
		t.Errorf("after a second rolling towards +Z, the marble is at %v, want well along +Z, on the platform", p)
	}
}

func TestMarbleHasATopSpeedAndSlowsDownByItself(t *testing.T) {
	w := emptyWorld()
	for range 600 {
		w.step(golib.Vector3{X: 1}, dt)
		if speed := w.marble.velocity.Length(); speed > marbleMaxSpeed+0.001 {
			t.Fatalf("the marble rolls at %v m/s, over the top speed of %v", speed, marbleMaxSpeed)
		}
		if w.marble.falling {
			w.resetMarble()
			w.marble.velocity = golib.Vector3{X: marbleMaxSpeed}
		}
	}
	w.resetMarble()
	w.marble.velocity = golib.Vector3{Z: 6}
	run(w, golib.Vector3{}, 0.5)
	if speed := w.marble.velocity.Length(); speed >= 6 || speed == 0 {
		t.Errorf("after half a second with no input, the marble rolls at %v m/s, want slower than 6 but still rolling", speed)
	}
}

func TestRollingOffTheEdgeCostsALife(t *testing.T) {
	w := emptyWorld()
	happened := run(w, golib.Vector3{X: 1}, 10)
	if !happened.fell || !happened.lostLife {
		t.Fatalf("rolling towards +X for ten seconds: fell %v, lost a life %v, want both", happened.fell, happened.lostLife)
	}
	if w.lives != startLives-1 {
		t.Errorf("lives = %d, want %d", w.lives, startLives-1)
	}
	if w.marble.falling || !near(w.marble.position, golib.Vector3{Y: marbleRadius}) || w.marble.velocity != (golib.Vector3{}) {
		t.Errorf("after the fall the marble is at %v, falling %v, rolling %v; want it at rest in the middle", w.marble.position, w.marble.falling, w.marble.velocity)
	}
}

func TestTheMarbleStaysOnUntilItsCenterIsOverTheEdge(t *testing.T) {
	w := emptyWorld()
	w.marble.position.X = half - 0.1
	w.step(golib.Vector3{}, dt)
	if w.marble.falling {
		t.Error("the marble fell with its center still over the platform")
	}
	w.marble.position.X = half + 0.01
	w.step(golib.Vector3{}, dt)
	if !w.marble.falling {
		t.Error("the marble didn't fall with its center over the edge")
	}
}

func TestTouchingACubeCostsALifeButNotDuringTheGrace(t *testing.T) {
	w := emptyWorld()
	home := golib.Vector3{X: 7, Y: cubeSize / 2, Z: 7}
	w.cubes = []cube{{position: golib.Vector3{X: 0.6, Y: cubeSize / 2}, home: home}}
	happened := w.step(golib.Vector3{}, dt)
	if !happened.hit || w.lives != startLives-1 {
		t.Fatalf("a cube next to the marble: hit %v, lives %d; want a hit and %d lives", happened.hit, w.lives, startLives-1)
	}
	if !near(w.cubes[0].position, home) {
		t.Errorf("after the hit the cube is at %v, want back home at %v", w.cubes[0].position, home)
	}
	if !w.grace.Running() {
		t.Fatal("after a hit the marble has no time without danger")
	}
	w.cubes[0].position = golib.Vector3{X: 0.6, Y: cubeSize / 2}
	if happened := w.step(golib.Vector3{}, dt); happened.hit {
		t.Error("a cube hurt the marble during the grace after a hit")
	}
	w.cubes = nil // out of the way while the grace runs out
	run(w, golib.Vector3{}, respawnGrace)
	w.cubes = []cube{{home: home}}
	w.cubes[0].position = golib.Vector3{X: 0.6, Y: cubeSize / 2}
	if happened := w.step(golib.Vector3{}, dt); !happened.hit {
		t.Error("a cube didn't hurt the marble once the grace was over")
	}
}

func TestACubeAFewStepsAwayDoesntTouch(t *testing.T) {
	if touches(golib.Vector3{Y: marbleRadius}, golib.Vector3{X: 1.2, Y: cubeSize / 2}) {
		t.Error("a cube 1.2 m away touches the marble")
	}
	if !touches(golib.Vector3{Y: marbleRadius}, golib.Vector3{X: 0.9, Y: cubeSize / 2}) {
		t.Error("a cube 0.9 m away, its side 0.4 m from the marble's center, doesn't touch it")
	}
}

func TestTheLastLifeEndsTheGame(t *testing.T) {
	w := emptyWorld()
	for range startLives {
		run(w, golib.Vector3{Z: -1}, 10)
	}
	if !w.over || w.lives != 0 {
		t.Fatalf("after %d falls: over %v, lives %d; want the game over", startLives, w.over, w.lives)
	}
	before := w.marble.position
	w.step(golib.Vector3{X: 1}, dt)
	if w.marble.position != before {
		t.Error("the marble still moves after the game is over")
	}
}

func TestCollectingAGemScoresAndPutsItElsewhere(t *testing.T) {
	w := emptyWorld()
	w.gems = []gem{{position: golib.Vector3{X: 0.5, Y: gemHeight}}}
	happened := w.step(golib.Vector3{}, dt)
	if happened.collected != 1 || w.score != 1 {
		t.Fatalf("a gem by the marble: collected %d, score %d; want 1 and 1", happened.collected, w.score)
	}
	g := w.gems[0].position
	if flat(g).Distance(flat(w.marble.position)) < gemAwayFromBall || offPlatform(g) || g.Y != gemHeight {
		t.Errorf("the new gem is at %v: want it on the platform, %v m above it, %v m or more from the marble", g, gemHeight, gemAwayFromBall)
	}
}

func TestNewGemsAreOnThePlatformAndApart(t *testing.T) {
	for seed := uint64(1); seed <= 50; seed++ {
		golib.SetRandomSeed(seed)
		w := newWorld()
		if len(w.gems) != gemCount {
			t.Fatalf("a new world has %d gems, want %d", len(w.gems), gemCount)
		}
		for i, g := range w.gems {
			if math.Abs(float64(g.position.X)) > half-gemMargin || math.Abs(float64(g.position.Z)) > half-gemMargin {
				t.Errorf("seed %d: gem %d at %v is too near the edge", seed, i, g.position)
			}
			if flat(g.position).Distance(flat(w.marble.position)) < gemAwayFromBall {
				t.Errorf("seed %d: gem %d at %v is too near the marble", seed, i, g.position)
			}
			for j := i + 1; j < len(w.gems); j++ {
				if flat(g.position).Distance(flat(w.gems[j].position)) < gemAwayFromGem {
					t.Errorf("seed %d: gems %d and %d are too close", seed, i, j)
				}
			}
		}
	}
}

func TestCubesGetFasterEveryFiveGems(t *testing.T) {
	w := emptyWorld()
	w.score = gemsPerLevel - 1
	if w.cubeSpeed() != cubeBaseSpeed {
		t.Errorf("with %d gems the cubes' speed is %v, want %v", w.score, w.cubeSpeed(), cubeBaseSpeed)
	}
	w.gems = []gem{{position: golib.Vector3{Y: gemHeight}}}
	happened := w.step(golib.Vector3{}, dt)
	if !happened.levelUp {
		t.Error("the fifth gem didn't say the cubes got faster")
	}
	if want := float32(cubeBaseSpeed + cubeSpeedStep); w.cubeSpeed() != want {
		t.Errorf("with %d gems the cubes' speed is %v, want %v", w.score, w.cubeSpeed(), want)
	}
	w.score = 1000
	if w.cubeSpeed() != cubeMaxSpeed {
		t.Errorf("with %d gems the cubes' speed is %v, want the most, %v", w.score, w.cubeSpeed(), cubeMaxSpeed)
	}
	if cubeMaxSpeed >= marbleMaxSpeed {
		t.Error("the cubes' top speed is the marble's or more: the marble could never get away")
	}
}

func TestCubesChaseTheMarble(t *testing.T) {
	w := emptyWorld()
	start := golib.Vector3{X: 6, Y: cubeSize / 2, Z: 6}
	w.cubes = []cube{{position: start, home: start}}
	before := flat(start).Distance(flat(w.marble.position))
	run(w, golib.Vector3{}, 1)
	after := flat(w.cubes[0].position).Distance(flat(w.marble.position))
	if after >= before-1 {
		t.Errorf("in a second a cube went from %v to %v m from the marble, want it closer by a meter or more", before, after)
	}
	if w.cubes[0].position.Y != cubeSize/2 {
		t.Errorf("the cube left the platform's top: %v", w.cubes[0].position)
	}
}

func TestCubesStayOnThePlatformAndApart(t *testing.T) {
	w := emptyWorld()
	for i := range 3 {
		at := golib.Vector3{X: 8 + float32(i)*0.1, Y: cubeSize / 2, Z: 8}
		w.cubes = append(w.cubes, cube{position: at, home: at, velocity: golib.Vector3{X: 20, Z: 20}})
	}
	w.marble.position = golib.Vector3{X: half - 0.6, Y: marbleRadius, Z: half - 0.6}
	w.grace.Start(100) // the cubes can't hurt it: this test watches them only
	run(w, golib.Vector3{}, 3)
	limit := float32(half - cubeSize/2)
	for i, c := range w.cubes {
		if math.Abs(float64(c.position.X)) > float64(limit)+0.001 || math.Abs(float64(c.position.Z)) > float64(limit)+0.001 {
			t.Errorf("cube %d at %v is over the edge", i, c.position)
		}
		for j := i + 1; j < len(w.cubes); j++ {
			if d := flat(c.position).Distance(flat(w.cubes[j].position)); d < cubeSpacing*0.7 {
				t.Errorf("cubes %d and %d are %v m apart, inside each other", i, j, d)
			}
		}
	}
}

func TestTheMarksRollWithTheMarble(t *testing.T) {
	w := emptyWorld()
	// A quarter turn: rolling a quarter of the way round, towards +Z.
	speed := float32(math.Pi / 2 * marbleRadius)
	w.marble.velocity = golib.Vector3{Z: speed}
	w.marble.roll(1)
	if top := w.marble.axes[0]; !near(top, golib.Vector3{Z: 1}) {
		t.Errorf("after a quarter turn towards +Z, the mark that was on top points %v, want +Z", top)
	}
	if front := w.marble.axes[1]; !near(front, golib.Vector3{Y: -1}) {
		t.Errorf("after a quarter turn towards +Z, the mark that was in front points %v, want down", front)
	}
}

func TestRollDirectionFollowsTheCamera(t *testing.T) {
	for _, yaw := range []float32{0, 90, -135, 200} {
		camera := golib.NewCamera3D(screenWidth, screenHeight)
		camera.Orbit(golib.Vector3{}, yaw, cameraPitch, cameraDistance)
		forward := camera.Forward()
		forward.Y = 0
		if got := rollDirection(yaw, 1, 0); !near(got, forward.Normalize()) {
			t.Errorf("yaw %v: forward rolls %v, want where the camera looks, %v", yaw, got, forward.Normalize())
		}
		if got := rollDirection(yaw, 0, 1); !near(got, camera.Right()) {
			t.Errorf("yaw %v: right rolls %v, want the screen's right, %v", yaw, got, camera.Right())
		}
		// And the screen agrees: rolling right moves the marble right on the screen.
		from, _ := camera.ToScreen(golib.Vector3{})
		to, _ := camera.ToScreen(rollDirection(yaw, 0, 1))
		if to.X <= from.X {
			t.Errorf("yaw %v: rolling right moves the marble from x %v to %v on the screen", yaw, from.X, to.X)
		}
	}
	if got := rollDirection(0, 1, 1).Length(); got > 1.0001 {
		t.Errorf("rolling diagonally is %v strong, want at most 1", got)
	}
}

func TestTheCameraSwingsBehindTheMarble(t *testing.T) {
	yaw := float32(0)
	for range 300 {
		yaw = followYaw(yaw, golib.Vector3{X: 5, Z: 5}, dt) // rolling away, to the left
	}
	if math.Abs(float64(yaw-45)) > 1 {
		t.Errorf("after five seconds rolling towards yaw 45, the camera's yaw is %v, want about 45", yaw)
	}
	if got := followYaw(0, golib.Vector3{Z: -5}, dt); got != 0 {
		t.Errorf("rolling back towards the camera turned it to %v", got)
	}
	if got := followYaw(0, golib.Vector3{X: 0.5, Z: 0.5}, dt); got != 0 {
		t.Errorf("rolling slowly turned the camera to %v", got)
	}
	if got := angleBetween(170, -170); got != 20 {
		t.Errorf("angleBetween(170, -170) = %v, want 20", got)
	}
}

func TestTheCameraSeesTheMarbleFromBehind(t *testing.T) {
	golib.SetRandomSeed(1)
	s := newPlayScene(record{})
	at, inFront := s.camera.ToScreen(s.world.marble.position)
	if !inFront || at.X < screenWidth*0.4 || at.X > screenWidth*0.6 || at.Y < screenHeight*0.5 || at.Y > screenHeight*0.9 {
		t.Errorf("the marble is at %v on the screen (in front %v), want it in the middle of the lower half", at, inFront)
	}
	eye := s.camera.Position
	if eye.Z >= s.world.marble.position.Z || eye.Y <= s.world.marble.position.Y {
		t.Errorf("the camera is at %v, want it behind the marble (towards -Z) and above it", eye)
	}
}
