package main

import (
	"testing"

	"golib"
)

const dt = 1.0 / 60 // the step golib.Run passes to Update

func mustLevels(t *testing.T) []*level {
	t.Helper()
	levels, err := loadLevels()
	if err != nil {
		t.Fatal(err)
	}
	return levels
}

func TestThreeLevelsLoad(t *testing.T) {
	levels := mustLevels(t)
	if len(levels) != 3 {
		t.Fatalf("%d levels, want 3", len(levels))
	}
	for i, l := range levels {
		if l.targets() == 0 || l.Par <= 0 || l.Name == "" {
			t.Errorf("level %d: %d targets, par %d, name %q", i+1, l.targets(), l.Par, l.Name)
		}
		if len(l.Stones) == 0 {
			t.Errorf("level %d has no stone block", i+1)
		}
	}
}

// overlaps says whether two boxes, by their middles and sizes, overlap.
func overlaps(a, as, b, bs golib.Vector3) bool {
	return abs(a.X-b.X) < (as.X+bs.X)/2-0.001 && abs(a.Y-b.Y) < (as.Y+bs.Y)/2-0.001 && abs(a.Z-b.Z) < (as.Z+bs.Z)/2-0.001
}

func TestPropsStartApart(t *testing.T) {
	for n, l := range mustLevels(t) {
		for i, a := range l.props {
			for j, b := range l.props[i+1:] {
				if overlaps(a.position, sizeOf(a.kind), b.position, sizeOf(b.kind)) {
					t.Errorf("level %d: props %d and %d start inside each other", n+1, i, i+1+j)
				}
			}
			for k, s := range l.Stones {
				at := golib.Vector3{X: s.X, Y: s.Height / 2, Z: s.Z}
				if overlaps(a.position, sizeOf(a.kind), at, golib.Vector3{X: s.Width, Y: s.Height, Z: s.Depth}) {
					t.Errorf("level %d: prop %d starts inside stone %d", n+1, i, k)
				}
			}
			if a.target {
				continue
			}
			for k, p := range l.Platforms {
				pl := platformPlace(p)
				if overlaps(a.position, sizeOf(a.kind), pl.Position, pl.Scale) {
					t.Errorf("level %d: loose prop %d starts inside platform %d", n+1, i, k)
				}
			}
		}
	}
}

func TestStacksStandOnTheirOwn(t *testing.T) {
	levels := mustLevels(t)
	for n := range levels {
		w := newWorld(levels, n)
		var start []golib.Vector3
		for _, p := range w.props {
			start = append(start, p.body.Position())
		}
		for range 300 {
			w.step(intent{}, dt)
		}
		if w.left() != w.targets() {
			t.Errorf("level %d: %d of %d targets fell with nothing thrown", n+1, w.targets()-w.left(), w.targets())
		}
		resting := 0
		for i, p := range w.props {
			if moved := p.body.Position().Distance(start[i]); moved > 0.05 {
				t.Errorf("level %d: prop %d moved %.2f m on its own", n+1, i, moved)
			}
			if p.body.Resting() {
				resting++
			}
		}
		if w.cleared {
			t.Errorf("level %d is cleared with nothing thrown", n+1)
		}
		t.Logf("level %d: %d of %d props resting after 5 s", n+1, resting, len(w.props))
	}
}

// throwAt throws a ball at a point, as if the mouse pointed there.
func throwAt(w *world, mark golib.Vector3) {
	from := w.hand(mark.Sub(w.player.body.Position))
	w.step(intent{throw: true, aimAt: from, aimBy: mark.Sub(from).Normalize()}, dt)
}

// seesFrom says whether a ball thrown from a player standing at spot flies
// straight at p, with nothing in the way.
func seesFrom(w *world, spot golib.Vector3, p *prop) bool {
	for _, s := range w.level.Stones { // not inside a stone or a platform
		if abs(spot.X-s.X) < s.Width/2+0.6 && abs(spot.Z-s.Z) < s.Depth/2+0.6 {
			return false
		}
	}
	for _, pl := range w.level.Platforms {
		if abs(spot.X-pl.X) < pl.Width/2+0.6 && abs(spot.Z-pl.Z) < pl.Depth/2+0.6 {
			return false
		}
	}
	at := p.body.Position()
	from := spot.Add(golib.Vector3{Y: throwHeight}).Add(golib.Vector3{X: at.X - spot.X, Z: at.Z - spot.Z}.Normalize().Scale(throwAhead))
	hit, ok := w.physics.SphereCast(from, at.Sub(from).Normalize(), ballRadius, 40)
	return ok && hit.Body == p.body.ID()
}

// pushOff is the way that pushes p off its platform soonest: towards the
// edge of the platform nearest to it.
func pushOff(w *world, p *prop) golib.Vector3 {
	at := p.body.Position()
	way, best := golib.Vector3{Z: -1}, float32(1e9)
	for _, pl := range w.level.Platforms {
		if abs(at.X-pl.X) > pl.Width/2+0.1 || abs(at.Z-pl.Z) > pl.Depth/2+0.1 {
			continue
		}
		for _, edge := range []struct {
			gap float32
			way golib.Vector3
		}{
			{pl.X + pl.Width/2 - at.X, golib.Vector3{X: 1}},
			{at.X - (pl.X - pl.Width/2), golib.Vector3{X: -1}},
			{pl.Z + pl.Depth/2 - at.Z, golib.Vector3{Z: 1}},
			{at.Z - (pl.Z - pl.Depth/2), golib.Vector3{Z: -1}},
		} {
			if edge.gap < best {
				best, way = edge.gap, edge.way
			}
		}
	}
	return way
}

// moveToSee puts the player where it sees p, as a player would walk there:
// on the side that knocks it off its platform soonest if it can, else the
// nearest place it sees it from; false if there is none in the yard.
func moveToSee(w *world, p *prop) bool {
	here := w.player.body.Position
	at := p.body.Position()
	way := pushOff(w, p)
	best, found := golib.Vector3{}, false
	score := func(spot golib.Vector3) float32 {
		from := golib.Vector3{X: at.X - spot.X, Z: at.Z - spot.Z}.Normalize()
		return -from.Dot(way)*10 + spot.Distance(here)*0.1 // lower is better
	}
	for r := float32(3); r <= 8; r += 1 {
		for a := float32(0); a < 360; a += 15 {
			spot := golib.Vector3{X: at.X, Z: at.Z}.Add(golib.Rotation{Yaw: a}.Forward().Scale(r))
			if abs(spot.X) > yardHalf-1 || abs(spot.Z) > yardHalf-1 || !seesFrom(w, spot, p) {
				continue
			}
			if !found || score(spot) < score(best) {
				best, found = spot, true
			}
		}
	}
	if found {
		w.player.body.Position = best
	}
	return found
}

// playLevel throws at the lowest target still up, from where the player
// stands, until the level is cleared or limit balls are thrown. With walk,
// it first moves where it sees the target.
func playLevel(w *world, limit int, walk bool) {
	tries := map[*prop]int{}
	for w.thrown < limit && !w.cleared {
		var aim *prop
		for _, p := range w.props {
			if p.target && !p.down && (aim == nil || p.body.Position().Y+float32(tries[p]) < aim.body.Position().Y+float32(tries[aim])) {
				aim = p
			}
		}
		tries[aim]++
		if aim != nil {
			if walk {
				moveToSee(w, aim)
			}
			throwAt(w, aim.body.Position())
		}
		for range 120 {
			w.step(intent{}, dt)
			if w.cleared {
				return
			}
		}
	}
	for range 240 {
		w.step(intent{}, dt)
	}
}

func TestEveryLevelCanBeCleared(t *testing.T) {
	levels := mustLevels(t)
	for n, l := range levels {
		w := newWorld(levels, n)
		for range 60 {
			w.step(intent{}, dt)
		}
		playLevel(w, 40, true)
		if !w.cleared {
			t.Errorf("level %d: not cleared after %d balls thrown; %d still up", n+1, w.thrown, w.left())
			for i, p := range w.props {
				if p.target && !p.down {
					t.Logf("  prop %d up at %v, moving %v, player at %v", i, p.body.Position(), p.body.Velocity().Length(), w.player.body.Position)
				}
			}
			continue
		}
		t.Logf("level %d (%s): cleared with %d balls by a player who aims at the lowest one it sees (par %d)", n+1, l.Name, w.thrown, l.Par)
	}
}

func TestAThrowLandsWhereItIsAimed(t *testing.T) {
	from := golib.Vector3{Y: 1.2}
	for _, mark := range []golib.Vector3{{Z: -8, Y: 2}, {X: 3, Y: 0, Z: -2}, {X: -5, Y: 3, Z: -12}} {
		v := throwVelocity(from, mark)
		at, best := from, from.Distance(mark)
		for range 200 {
			v.Y -= 9.81 * dt
			at = at.Add(v.Scale(dt))
			best = min(best, at.Distance(mark))
		}
		if best > 0.25 {
			t.Errorf("a throw at %v passed %.2f m from it", mark, best)
		}
	}
}

func TestABallKnocksACrateOff(t *testing.T) {
	levels := mustLevels(t)
	w := newWorld(levels, 0)
	for range 30 {
		w.step(intent{}, dt)
	}
	var top *prop
	for _, p := range w.props {
		if p.target && (top == nil || p.body.Position().Y > top.body.Position().Y) {
			top = p
		}
	}
	start := top.body.Position()
	throwAt(w, start)
	if w.thrown != 1 || len(w.balls) != 1 {
		t.Fatalf("after a throw: %d thrown, %d balls", w.thrown, len(w.balls))
	}
	for range 180 {
		w.step(intent{}, dt)
	}
	if !top.down {
		t.Errorf("the top crate, hit by a ball, is at %v, from %v: still up", top.body.Position(), start)
	}
}

func TestThrowsWaitForTheCooldown(t *testing.T) {
	w := newWorld(mustLevels(t), 0)
	for range 10 {
		throwAt(w, golib.Vector3{Z: -5})
	}
	if w.thrown != 1 {
		t.Errorf("ten throws in ten updates threw %d balls, want 1", w.thrown)
	}
}

func TestThePlayerPushesALooseCrateButNotTheStone(t *testing.T) {
	levels := mustLevels(t)
	w := newWorld(levels, 0)
	var loose *prop
	for _, p := range w.props {
		if !p.target && p.kind == crateKind {
			loose = p
		}
	}
	stone := w.stones[0]
	for _, c := range []struct {
		name  string
		body  *golib.RigidBody
		moves bool
	}{{"loose crate", loose.body, true}, {"stone block", stone, false}} {
		at := c.body.Position()
		// Stand 1.5 m in front of it, towards +Z, and walk into it for 2 s.
		w.player.body.Position = golib.Vector3{X: at.X, Z: at.Z + 1.5}
		for range 120 {
			w.step(intent{move: golib.Vector3{Z: -1}}, dt)
		}
		moved := c.body.Position().Distance(at)
		if c.moves && moved < 0.5 {
			t.Errorf("walking into the %s for 2 s moved it %.2f m", c.name, moved)
		}
		if !c.moves && moved > 0.02 {
			t.Errorf("walking into the %s for 2 s moved it %.2f m", c.name, moved)
		}
		t.Logf("%s moved %.2f m", c.name, moved)
	}
}

func TestWalkingForwardGoesAwayFromTheCamera(t *testing.T) {
	w := newWorld(mustLevels(t), 0)
	v := newView(w)
	before, _ := v.camera.ToScreen(w.player.body.Position)
	near := v.camera.Position.Distance(w.player.body.Position)
	c := controls{forward: 1}
	for range 30 {
		w.step(c.intent(v.camera, newAimer()), dt)
		v.follow(w, dt)
	}
	if far := v.camera.Position.Distance(w.player.body.Position); far < near-0.5 {
		t.Errorf("walking forward, the camera came %.2f m closer to the player", near-far)
	}
	_ = before
	start := golib.Vector3{X: w.level.Start.X, Z: w.level.Start.Z}
	if d := w.player.body.Position.Distance(w.focus()); d > start.Distance(w.focus())-1 {
		t.Errorf("walking forward for half a second, the player is %.2f m from the stacks, from %.2f", d, start.Distance(w.focus()))
	}
}

func TestStars(t *testing.T) {
	for _, c := range []struct{ balls, par, want int }{{2, 3, 3}, {3, 3, 3}, {5, 3, 2}, {6, 3, 1}} {
		if got := stars(c.balls, c.par); got != c.want {
			t.Errorf("stars(%d, %d) = %d, want %d", c.balls, c.par, got, c.want)
		}
	}
}

func TestTheRecordKeepsTheFewestBalls(t *testing.T) {
	golib.DeleteData(recordName)
	levels := mustLevels(t)
	r := &run{levels: levels}
	for n := range levels {
		r.finishLevel(n, 4)
	}
	if r.record.BestTotal != 12 {
		t.Fatalf("best total %d, want 12", r.record.BestTotal)
	}
	again := &run{levels: levels, record: loadRecord()}
	for n := range levels {
		again.finishLevel(n, 5)
	}
	if again.record.BestTotal != 12 || again.record.BestLevel[0] != 4 {
		t.Errorf("a worse game changed the record to %+v", again.record)
	}
}
