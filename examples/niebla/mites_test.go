package main

import (
	"math"
	"testing"

	"golib"
)

// foggedRobot deals a game whose first robot stands deep in the fog, and
// returns the robot's ID. The mites only read the state, so the tests
// move the robot by hand instead of ticking the simulation.
func foggedRobot(t *testing.T) (*State, int64) {
	t.Helper()
	golib.SetRandomSeed(1)
	s := newGame()
	id := sortedRobotIDs(s)[0]
	r := s.Robots[id]
	r.X, r.Y = 500, 500
	s.Robots[id] = r
	if fogAt(s, r.X, r.Y) < 1 {
		t.Fatalf("the test's robot stands in %v of fog, want 1",
			fogAt(s, r.X, r.Y))
	}
	return s, id
}

func runMites(f *miteField, s *State, seconds float32) {
	for i := 0; i < int(seconds*60); i++ {
		f.update(s, 1.0/60)
	}
}

// swarm returns how far the mites stand from their host on average, and
// where their middle is, off the host.
func swarm(h *miteHost) (spread, offX, offY float32) {
	for _, m := range h.Mites {
		dx, dy := m.X-h.X, m.Y-h.Y
		spread += float32(math.Hypot(float64(dx), float64(dy)))
		offX += dx
		offY += dy
	}
	n := float32(len(h.Mites))
	return spread / n, offX / n, offY / n
}

func TestMitesGatherByVolumeAndOnlyInExposedAir(t *testing.T) {
	s, id := foggedRobot(t)
	col, row := groundInTheFog()
	s.Jobs = append(s.Jobs, Job{
		Kind: BuildingProtector, Col: col, Row: row, Left: buildingWorkTicks,
	})
	f := newMiteField()
	runMites(f, s, 3)

	robot := f.hosts[robotMiteKey(id)]
	volume := robotBodyAcross * robotBodyAcross * robotBodyHeight
	want := int(math.Floor(volume * mitesPerCubicUnit * 2))
	if robot == nil || len(robot.Mites) != want {
		t.Fatalf("the fogged robot has %v mites, want %d", robot, want)
	}
	site := f.hosts[siteMiteKey(col, row)]
	if site == nil || len(site.Mites) <= len(robot.Mites) {
		t.Fatalf("the site, the bigger body, has no more mites than the robot")
	}
	if len(f.hosts) != 2 {
		t.Errorf("%d hosts, want two: the robot in the bubble has none",
			len(f.hosts))
	}
}

func TestMitesAlsoShowInTheClearHazeAtHalfStrength(t *testing.T) {
	golib.SetRandomSeed(1)
	s := newGame()
	id := sortedRobotIDs(s)[0]
	r := s.Robots[id]
	r.X, r.Y = pointAtTiles(5)
	s.Robots[id] = r
	if got := fogExposureAt(s, r.X, r.Y); got != fogHazeExposure {
		t.Fatalf("the robot stands in exposure %v, want %v",
			got, fogHazeExposure)
	}
	f := newMiteField()
	runMites(f, s, 3)
	want := int(math.Floor(robotBodyAcross * robotBodyAcross *
		robotBodyHeight * mitesPerCubicUnit * fogHazeExposure * 2))
	if got := len(f.hosts[robotMiteKey(id)].Mites); got != want {
		t.Fatalf("the hazy robot has %d mites, want %d", got, want)
	}
}

func TestMitesCloseInOnWhatStandsStillAndTrailAWalker(t *testing.T) {
	s, id := foggedRobot(t)
	f := newMiteField()
	runMites(f, s, 6)
	h := f.hosts[robotMiteKey(id)]
	still, _, _ := swarm(h)
	if still > robotBodyAcross/2*1.2 {
		t.Fatalf("around a still robot the mites stand %v u out, want them on it",
			still)
	}

	// The fogged pace, along x.
	for i := 0; i < 180; i++ {
		r := s.Robots[id]
		r.X += 15.0 / 60
		s.Robots[id] = r
		f.update(s, 1.0/60)
	}
	walking, offX, _ := swarm(h)
	if walking < still*2 {
		t.Errorf("around a walker the mites stand %v u out, %v u when still: "+
			"want them let go", walking, still)
	}
	if offX > -robotBodyAcross/2 {
		t.Errorf("the swarm's middle is %v u off the walker along its way, "+
			"want it trailing behind the body", offX)
	}
}

func TestStationaryMitesDoubleAndDoubleAgainInASwell(t *testing.T) {
	s, _ := foggedRobot(t)
	h := miteHost{
		X: 500, Y: 500, Across: robotBodyAcross,
		Height: robotBodyHeight,
	}
	h.want(s)
	moving := h.Wanted
	h.Grip = 1
	h.want(s)
	still := h.Wanted
	s.Fog.Pressure = 1
	h.want(s)
	swell := h.Wanted

	if still != moving*2 {
		t.Fatalf("a still host has %d mites, want twice the moving %d",
			still, moving)
	}
	if swell != still*2 {
		t.Errorf("a still host in a swell has %d mites, want twice %d",
			swell, still)
	}
}

func TestMitesFadeOverWhatIsGone(t *testing.T) {
	s, id := foggedRobot(t)
	f := newMiteField()
	runMites(f, s, 3)
	delete(s.Robots, id)
	f.update(s, 1.0/60)
	if h := f.hosts[robotMiteKey(id)]; h == nil || len(h.Mites) == 0 {
		t.Fatal("the swarm went with its robot, want it to fade over the spot")
	}
	runMites(f, s, miteFadeSeconds+0.5)
	if len(f.hosts) != 0 {
		t.Fatalf("%d hosts left after the fade, want none", len(f.hosts))
	}
}

func TestMiteFalloffIsBlackAtTheHeartAndFadesToNothing(t *testing.T) {
	falloff := miteFalloff()
	if falloff[0] != 1 || falloff[1] != miteHaloOpacity {
		t.Fatalf("the falloff starts %v, want 1 then %v", falloff[:2],
			miteHaloOpacity)
	}
	for ring := 1; ring < len(falloff); ring++ {
		if falloff[ring] >= falloff[ring-1] || falloff[ring] <= 0 {
			t.Errorf("ring %d darkens by %v after %v, want it ever fainter",
				ring, falloff[ring], falloff[ring-1])
		}
	}
	if rim := falloff[len(falloff)-1]; rim > 0.02 {
		t.Errorf("the rim darkens by %v, want it next to nothing", rim)
	}
}

func TestMiteLayersStackIntoTheFalloff(t *testing.T) {
	falloff := miteFalloff()
	layers := miteLayers(falloff)
	light := float32(1)
	for ring := len(layers) - 1; ring >= 0; ring-- {
		light *= 1 - layers[ring]
		if got := 1 - light; math.Abs(float64(got-falloff[ring])) > 1e-5 {
			t.Errorf("ring %d darkens by %v, want %v", ring, got, falloff[ring])
		}
	}
}
