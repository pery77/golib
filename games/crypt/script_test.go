package main

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

// A script is a golib shot --input script, read the way golib shot reads
// it, so that a test can play the play scene exactly as a shot plays it,
// without a window, and say where everything is on any frame. The shots in
// DESIGN.md were planned with it.
type script struct {
	holds []hold
	moves []mouseMove
}

// hold is a key or button held from update from to update to.
type hold struct {
	name     string
	from, to int
}

// mouseMove puts the pointer at x, y from update at on.
type mouseMove struct {
	at   int
	x, y float32
}

// parseScript reads a script: Name@N, Name@A-B and Mouse@N:X,Y items.
func parseScript(t *testing.T, text string) script {
	t.Helper()
	var s script
	for _, item := range strings.Fields(text) {
		name, when, ok := strings.Cut(item, "@")
		if !ok {
			t.Fatalf("script item %q has no @", item)
		}
		name = strings.ToLower(name)
		if name == "mouse" {
			frame, at, _ := strings.Cut(when, ":")
			x, y, _ := strings.Cut(at, ",")
			s.moves = append(s.moves, mouseMove{at: atoi(t, frame), x: atof(t, x), y: atof(t, y)})
			continue
		}
		from, to, ranged := strings.Cut(when, "-")
		h := hold{name: name, from: atoi(t, from)}
		h.to = h.from
		if ranged {
			h.to = atoi(t, to)
		}
		s.holds = append(s.holds, h)
	}
	return s
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("script: %v", err)
	}
	return n
}

func atof(t *testing.T, s string) float32 {
	t.Helper()
	f, err := strconv.ParseFloat(s, 32)
	if err != nil {
		t.Fatalf("script: %v", err)
	}
	return float32(f)
}

// down says whether a key or button is held in update frame.
func (s script) down(frame int, names ...string) bool {
	for _, h := range s.holds {
		for _, name := range names {
			if h.name == name && frame >= h.from && frame <= h.to {
				return true
			}
		}
	}
	return false
}

// pressed says whether a key or button went down in update frame.
func (s script) pressed(frame int, names ...string) bool {
	for _, h := range s.holds {
		for _, name := range names {
			if h.name == name && frame == h.from {
				return true
			}
		}
	}
	return false
}

// controls is what readControls would read in update frame.
func (s script) controls(frame int) controls {
	var c controls
	if s.down(frame, "left", "a") {
		c.right = -1
	}
	if s.down(frame, "right", "d") {
		c.right = 1
	}
	if s.down(frame, "up", "w") {
		c.forward = 1
	}
	if s.down(frame, "down", "s") {
		c.forward = -1
	}
	if s.down(frame, "q") {
		c.turn = 1
	}
	if s.down(frame, "e") {
		c.turn = -1
	}
	c.click = s.pressed(frame, "mouseleft")
	c.smash = s.pressed(frame, "f", "j", "space")
	for _, m := range s.moves {
		if m.at <= frame {
			c.mouseX, c.mouseY = m.x, m.y
		}
	}
	return c
}

// play plays a play scene through a script from update first, the first
// update after the one that started it, to update last, and calls each with
// every update and what happened in it.
func (s script) play(scene *playScene, first, last int, each func(frame int, happened events)) {
	for frame := first; frame <= last; frame++ {
		happened := scene.advance(s.controls(frame), dt)
		if each != nil {
			each(frame, happened)
		}
	}
}

// dt is the step golib.Run passes to Update.
const dt = 1.0 / 60

// playthrough plays the whole game, from the title to the way out, as
// golib shot plays it: DESIGN.md lists the shots taken from it, on the
// desktop and in a browser.
//
// The camera follows the astronaut, so it steers mostly by turning the
// camera, with Q and E, and running forward, with Up; a push to one side
// with Up makes it circle, as the camera swings round behind it.
const playthrough = "Enter@1" +
	// The entry hall: along its west wall, the astronaut sliding up it with
	// Up and Right, then east along the north wall, past the locked door.
	" E@10-58 Up@60-370 Q@100-148 Right@150-215 Q@216-264" +
	// A click on the crate in the north-east corner: its key falls out.
	" Mouse@378:556,344 MouseLeft@382 Up@410-430" +
	// Back to the door, which opens, and on into the great hall.
	" Q@440-538 Up@540-800 Q@577-625" +
	// The hall's guard sees the astronaut and chases it round a pillar,
	// and grabs it: back to the corridor, with a heart less.
	" E@948-996 Up@997-1300 E@1021-1250" +
	// Behind the guard's back to the stairs, up to the gallery and its key,
	// and down the ramp to the second door.
	" Up@1650-1700 E@1701-1749 Up@1750-1880 Q@1881-1929 Up@1930-2135 Q@2136-2184 Up@2185-2440 Q@2441-2489 Up@2490-2700" +
	// Through the corridor to the lift hall, round to the crate by the lift,
	// broken with Space for the last key.
	" E@2705-2753 Up@2754-2865 E@2876-2924 Up@2925-2985 Space@2992 Up@3033-3058" +
	// Onto the lift once it is down, up to the upper floor, through the
	// third door and on to the way out.
	" Q@3060-3108 Up@3109-3135 Up@3270-3300 E@3595-3643 Up@3644-4000"

func TestThePlaythroughWins(t *testing.T) {
	sc := parseScript(t, playthrough)
	scene := newPlayScene(newWorld(theLevel, nil), record{})
	w := scene.world
	type milestone struct {
		name  string
		frame int
	}
	var seen []milestone
	mark := func(name string, frame int) {
		seen = append(seen, milestone{name, frame})
	}
	var minWallX, slideFrom, slideTo float32 = 0, 100, 0
	var onLift, liftTop bool
	sc.play(scene, 2, 4100, func(frame int, happened events) {
		p := w.player.body.Position
		if frame >= 150 && frame <= 260 { // the slide along the west wall
			minWallX = min(minWallX, p.X)
			if p.X < -4.7 {
				slideFrom, slideTo = min(slideFrom, p.Z), max(slideTo, p.Z)
			}
		}
		if len(happened.broke) > 0 {
			mark("crate", frame)
		}
		if happened.keyTaken {
			mark("key", frame)
		}
		if len(happened.opened) > 0 {
			mark("door", frame)
		}
		if len(happened.spotted) > 0 {
			mark("spotted", frame)
		}
		if happened.caught {
			mark("grabbed", frame)
		}
		if p.Y > 2.4 && p.Z > 26 && p.X < 0 && !slices.ContainsFunc(seen, func(m milestone) bool { return m.name == "gallery" }) {
			mark("gallery", frame)
		}
		onLift = onLift || w.player.body.OnGround && w.player.body.GroundBody == w.lift.body && w.lift.height > 1
		liftTop = liftTop || onLift && w.lift.state == liftUp
		if happened.won {
			mark("won", frame)
		}
	})
	t.Logf("milestones: %v", seen)
	if minWallX < -5+playerRadius-0.02 {
		t.Errorf("the astronaut went into the west wall, to x %.2f", minWallX)
	}
	if slideTo-slideFrom < 2.5 {
		t.Errorf("the astronaut slid only from z %.2f to %.2f along the west wall", slideFrom, slideTo)
	}
	var names []string
	for _, m := range seen {
		names = append(names, m.name)
	}
	want := []string{"crate", "key", "door", "spotted", "grabbed", "gallery", "key", "door", "crate", "key", "door", "won"}
	if !slices.Equal(names, want) {
		t.Errorf("the playthrough went %v, want %v", names, want)
	}
	if !onLift || !liftTop {
		t.Errorf("the astronaut didn't ride the lift to the top: on it %v, at the top %v", onLift, liftTop)
	}
	if w.hearts != startHearts-1 {
		t.Errorf("the playthrough ends with %d hearts, want %d", w.hearts, startHearts-1)
	}
}
