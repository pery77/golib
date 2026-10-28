package main

import (
	"math"
	"reflect"
	"testing"
)

func TestAHeldSwellStaysUpAndCountsForNothing(t *testing.T) {
	s := newGame()
	Apply(s, DevHoldSwell{On: true})
	runTicks(s, fogSwellTicksMax+fogCycleTicks)
	if s.Fog.SwellLeft <= 0 || fogLineNow(s) >= fogLineRadius {
		t.Fatal("the held swell drained, want it up until let go")
	}
	Apply(s, DevHoldSwell{On: false})
	if s.Fog.SwellLeft != 0 || s.Fog.Held {
		t.Fatalf("let go, the swell has %d ticks left, want calm", s.Fog.SwellLeft)
	}
	if s.Fog.Swells != 0 {
		t.Errorf("the held swell counted as %d, want the fog's own dials untouched",
			s.Fog.Swells)
	}
}

func TestDevSpawnRobotPutsAFreeBuiltRobotOnTheSpot(t *testing.T) {
	s := newGame()
	stock := s.Stock
	Apply(s, DevSpawnRobot{X: 500, Y: 700})
	if len(s.Robots) != startingBuilders+1 || s.Stock != stock {
		t.Fatalf("%d robots and stores %+v, want one more robot for nothing",
			len(s.Robots), s.Stock)
	}
	ids := sortedRobotIDs(s)
	r := s.Robots[ids[len(ids)-1]]
	if r.Kind != RobotWorker || r.Tank != robotTankLiters || r.X != 500 || r.Y != 700 {
		t.Errorf("the robot is %+v, want a built one, tank full, at 500, 700", r)
	}
	Apply(s, DevSpawnRobot{X: -1, Y: 700})
	if len(s.Robots) != startingBuilders+1 {
		t.Error("a spot outside the region took a robot")
	}
}

func TestDevResetWorldReplaysSeedAndResetsState(t *testing.T) {
	s := newGame()
	Apply(s, DevSpawnRobot{X: 500, Y: 700})
	runTicks(s, 120)
	Apply(s, DevResetWorld{Seed: s.Seed})
	if !reflect.DeepEqual(s, newGame()) {
		t.Error("a reset world differs from a new game")
	}
	Apply(s, DevResetWorld{Seed: 99})
	if s.Seed != 99 || land.Seed != 99 {
		t.Fatalf("the state's seed is %d and the ground's %d, want 99", s.Seed, land.Seed)
	}
	for _, d := range land.deposits {
		if s.Drain[depositKey(d)] != d.Full {
			t.Errorf("deposit %d of the new world holds %v, want %v",
				d.Index, s.Drain[depositKey(d)], d.Full)
		}
	}
	// Any action on a state brings its own ground back under it.
	Apply(newGame(), Tick{})
	if land.Seed != defaultSeed {
		t.Errorf("a tick on the default region ran on the ground of seed %d", land.Seed)
	}
}

func TestDistinctWorldSeedNeverRepeatsTheCurrentSeed(t *testing.T) {
	tests := []struct {
		previous, candidate, want int64
	}{
		{previous: 0, candidate: 7, want: 7},
		{previous: 7, candidate: 7, want: 8},
		{previous: math.MaxInt32 - 1, candidate: math.MaxInt32 - 1,
			want: math.MaxInt32},
		{previous: math.MaxInt32, candidate: math.MaxInt32, want: 1},
	}
	for _, test := range tests {
		got := distinctWorldSeed(test.previous, test.candidate)
		if got == test.previous || got != test.want {
			t.Errorf("distinctWorldSeed(%d, %d) = %d, want %d",
				test.previous, test.candidate, got, test.want)
		}
	}
}

func TestUnitsAtWorldUndoesProject(t *testing.T) {
	px, py := project(1234, 567)
	x, y := unitsAtWorld(float64(px), float64(py))
	if math.Abs(x-1234) > 0.5 || math.Abs(y-567) > 0.5 {
		t.Fatalf("1234, 567 came back as %v, %v", x, y)
	}
}

func TestFastForwardAndRivalToolsHaveDistinctButtons(t *testing.T) {
	was := devOpen
	defer func() { devOpen = was }()
	d := devTools{fast: true}
	devOpen = false
	if got := d.ticksPerUpdate(); got != 1 {
		t.Errorf("with the strip closed an update sends %d ticks, want 1", got)
	}
	devOpen = true
	if got := d.ticksPerUpdate(); got != devFastTicks {
		t.Errorf("fast forward sends %d ticks an update, want %d", got, devFastTicks)
	}
	visit, hurry := devButtonBounds(devVisitButton), devButtonBounds(devHurryButton)
	city, build := devButtonBounds(devCityButton), devButtonBounds(devBuildCityButton)
	if hurry.X <= visit.X || hurry.Y != visit.Y {
		t.Errorf("the intro hurry button %v should follow visit %v", hurry, visit)
	}
	if city.Y <= visit.Y || build.Y != city.Y {
		t.Errorf("city tools should have their own row: city %v, build %v", city, build)
	}
}

func TestDevHurryRivalsEndsACampsWait(t *testing.T) {
	s := newGame()
	s.Raids.Visits = 2
	s.Raids.PressureCity = s.foundCityOnBearing(0.7)
	visitNow(s)
	Apply(s, DevHurryRivals{}) // on its way in, a party has no wait to end
	tickUntil(s, 60*600, func() bool { return lastReport(s).Kind == ReportCamp })
	runTicks(s, 60)
	if lastReport(s).Kind != ReportCamp {
		t.Fatalf("the party moved in by itself a second into its camp")
	}
	Apply(s, DevHurryRivals{})
	runTicks(s, 1)
	if lastReport(s).Kind != ReportRaid {
		t.Errorf("hurried, the camped party reports %q, want it moving in", lastReport(s).Kind)
	}
}
