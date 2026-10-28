package main

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func builderAt(s *State, id int64, x, y float64) {
	r := s.Robots[id]
	r.X, r.Y = x, y
	s.Robots[id] = r
}

func TestBuildersReserveSeparateSitesAndRaiseThemInParallel(t *testing.T) {
	s := newGame()
	noRivals(s)
	col, row := groundNearCore()
	firstX, firstY := cellCenterUnits(col, row)
	secondX, secondY := cellCenterUnits(col+4, row)
	second := s.spawnRobot(RobotBuilder, secondX, secondY)
	builderAt(s, 1, firstX, firstY)
	s.Jobs = []Job{
		{Kind: BuildingSilo, Col: col, Row: row, Left: 3},
		{Kind: BuildingWarehouse, Col: col + 4, Row: row, Left: 3},
	}

	runTicks(s, 1)
	if s.Robots[1].BuildCol != col ||
		s.Robots[second].BuildCol != col+4 ||
		!s.Robots[1].BuildJob || !s.Robots[second].BuildJob {
		t.Fatalf("builders did not reserve separate nearby sites: %+v, %+v",
			s.Robots[1], s.Robots[second])
	}
	if s.Jobs[0].Left != 3 || s.Jobs[1].Left != 3 {
		t.Fatal("builders worked before reaching their sites")
	}
	if got := robotPanelActivity(s, s.Robots[second]); got != "building warehouse" {
		t.Errorf("the roster says %q for the second builder", got)
	}
	runTicks(s, 26)
	if len(s.Jobs) != 0 {
		t.Fatalf("sites did not finish in parallel: %+v", s.Jobs)
	}
	if _, ok := buildingAt(s, col, row); !ok {
		t.Error("the first builder did not finish its silo")
	}
	if _, ok := buildingAt(s, col+4, row); !ok {
		t.Error("the second builder did not finish its warehouse")
	}
}

func TestBuilderFinishesItsSiteBeforeTakingANewProtector(t *testing.T) {
	s := newGame()
	noRivals(s)
	col, row := groundNearCore()
	x, y := cellCenterUnits(col, row)
	builderAt(s, 1, x, y)
	s.Jobs = []Job{{Kind: BuildingSilo, Col: col, Row: row, Left: 2}}
	runTicks(s, 1)
	if !s.Robots[1].BuildJob {
		t.Fatal("the builder did not reserve the silo")
	}
	s.Jobs = append(s.Jobs, Job{
		Kind: BuildingProtector, Col: col + 8, Row: row, Left: 2,
	})
	runTicks(s, 28)
	if _, done := buildingAt(s, col, row); !done {
		t.Fatalf("a new protector interrupted the reserved silo: %+v, %+v",
			s.Robots[1], s.Jobs)
	}
	if !s.Robots[1].BuildJob || s.Robots[1].BuildCol != col+8 {
		t.Fatalf("the builder did not reserve the protector on finishing: %+v",
			s.Robots[1])
	}
	if s.Jobs[0].Left != 2 {
		t.Error("the builder worked on the protector before finishing the silo")
	}
}

func TestAnAvailableBuilderChoosesAProtectorBeforeNearerWork(t *testing.T) {
	s := newGame()
	noRivals(s)
	col, row := groundNearCore()
	x, y := cellCenterUnits(col, row)
	builderAt(s, 1, x, y)
	s.Jobs = []Job{
		{Kind: BuildingSilo, Col: col, Row: row, Left: 20},
		{Kind: BuildingProtector, Col: col + 8, Row: row, Left: 20},
	}
	runTicks(s, 1)
	if r := s.Robots[1]; !r.BuildJob || r.BuildCol != col+8 {
		t.Fatalf("the builder chose the nearer silo over a protector: %+v", r)
	}
}

func TestBuilderChoosesNearestAvailablePipeSectionOrSite(t *testing.T) {
	s := newGame()
	noRivals(s)
	seedStock(s)
	arriveAll(s)
	col, row := groundNearCore()
	silo := raised(t, s, BuildingSilo, col, row)
	Apply(s, LayPipe{From: coreTank, To: silo.ID})
	pipe := pipesOf(s, silo.ID)[0]
	path, _ := pipeSpine(s, pipe)
	spot := sectionSpot(path, 0)
	builderAt(s, 1, spot.X, spot.Y)
	s.Jobs = []Job{{
		Kind: BuildingWarehouse, Col: col + 12, Row: row, Left: 30,
	}}
	runTicks(s, 1)
	if r := s.Robots[1]; r.Pipe != pipe.ID || r.Section != 0 {
		t.Fatalf("the builder ignored the nearer pipe section: %+v", r)
	}
	if s.Jobs[0].Left != 30 {
		t.Error("the distant site took work from the claimed pipe section")
	}

	Apply(s, RemovePipe{Pipe: pipe.ID})
	runTicks(s, 1)
	if r := s.Robots[1]; !r.BuildJob || r.Pipe != 0 {
		t.Fatalf("the builder kept a removed pipe instead of the site: %+v", r)
	}
}

func TestBuilderKeepsItsPipeSectionUntilDoneThenReservesTheSite(t *testing.T) {
	s := newGame()
	noRivals(s)
	seedStock(s)
	arriveAll(s)
	col, row := groundNearCore()
	silo := raised(t, s, BuildingSilo, col, row)
	Apply(s, LayPipe{From: coreTank, To: silo.ID})
	p := pipesOf(s, silo.ID)[0]
	p.Left = 2
	p.SectionLeft = make([]int64, p.Sections)
	p.SectionLeft[0] = 2
	s.Pipes[p.ID] = p
	path, _ := pipeSpine(s, p)
	spot := sectionSpot(path, 0)
	angle := float64(1) * goldenAngle
	builderAt(s, 1,
		spot.X+math.Cos(angle)*pipeLayStandoff,
		spot.Y+math.Sin(angle)*pipeLayStandoff)
	runTicks(s, 1)
	if s.Pipes[p.ID].Left != 1 {
		t.Fatal("the builder did not begin its pipe section")
	}
	s.Jobs = []Job{{
		Kind: BuildingProtector, Col: col + 8, Row: row, Left: 10,
	}}
	runTicks(s, 1)
	if s.Pipes[p.ID].Left != 0 {
		t.Fatal("the new protector interrupted pipe work")
	}
	if r := s.Robots[1]; !r.BuildJob || r.BuildCol != col+8 ||
		r.Pipe != 0 {
		t.Fatalf("the next site was not reserved on finishing: %+v", r)
	}
}

func TestRefuelingKeepsTheReservationForItsBuilder(t *testing.T) {
	s := newGame()
	noRivals(s)
	col, row := groundNearCore()
	x, y := cellCenterUnits(col, row)
	builderAt(s, 1, x, y)
	s.Jobs = []Job{{Kind: BuildingSilo, Col: col, Row: row, Left: 30}}
	runTicks(s, 1)
	r := s.Robots[1]
	r.Tank = robotTankLiters * robotLowTankAt / 2
	s.Robots[1] = r
	second := s.spawnRobot(RobotBuilder, x, y)
	s.Jobs = append(s.Jobs, Job{
		Kind: BuildingWarehouse, Col: col + 4, Row: row,
		Left: 30,
	})
	runTicks(s, 1)
	if r := s.Robots[1]; !r.BuildJob || r.BuildCol != col ||
		r.taskNow(s).name != taskRefuel {
		t.Fatalf("refueling released the reserved site: %+v", r)
	}
	if r := s.Robots[second]; !r.BuildJob || r.BuildCol != col+4 {
		t.Fatalf("the other builder took the reserved site: %+v", r)
	}
	r = s.Robots[1]
	r.Tank = robotTankLiters
	s.Robots[1] = r
	runTicks(s, 1)
	if r := s.Robots[1]; !r.BuildJob || r.BuildCol != col {
		t.Fatalf("the fueled builder abandoned its site: %+v", r)
	}
}

func TestDemolitionsJoinTheNearestUnreservedConstruction(t *testing.T) {
	s := newGame()
	noRivals(s)
	col, row := groundNearCore()
	near := raised(t, s, BuildingSilo, col, row)
	far := raised(t, s, BuildingWarehouse, col+5, row)
	firstX, firstY := cellCenterUnits(col, row)
	secondX, secondY := cellCenterUnits(col+5, row)
	builderAt(s, 1, firstX, firstY)
	second := s.spawnRobot(RobotBuilder, secondX, secondY)
	Apply(s, Demolish{Building: near.ID})
	Apply(s, Demolish{Building: far.ID})
	runTicks(s, 1)
	if s.Robots[1].Demolition != near.ID ||
		s.Robots[second].Demolition != far.ID {
		t.Fatalf("builders reserved the wrong demolition: %+v, %+v",
			s.Robots[1], s.Robots[second])
	}
	if got := robotCaption(s, s.Robots[second]); got != "taking down" {
		t.Errorf("the builder caption says %q", got)
	}
}

func TestReservedSiteIsFreedByCancellationOrRobotLoss(t *testing.T) {
	s := newGame()
	noRivals(s)
	col, row := groundNearCore()
	x, y := cellCenterUnits(col, row)
	second := s.spawnRobot(RobotBuilder, x, y)
	builderAt(s, 1, x, y)
	s.Jobs = []Job{{Kind: BuildingSilo, Col: col, Row: row, Left: 20}}
	runTicks(s, 1)
	if s.Robots[1].BuildJob == s.Robots[second].BuildJob {
		t.Fatal("two builders reserved the same site")
	}
	delete(s.Robots, 1)
	runTicks(s, 1)
	if !s.Robots[second].BuildJob {
		t.Fatal("the surviving builder did not take the vacant site")
	}
	Apply(s, CancelJob{Col: col, Row: row})
	runTicks(s, 1)
	if s.Robots[second].BuildJob {
		t.Fatal("the builder kept a cancelled site")
	}
}

func TestConstructionReservationsSurviveASave(t *testing.T) {
	s := newGame()
	noRivals(s)
	col, row := groundNearCore()
	x, y := cellCenterUnits(col, row)
	s.spawnRobot(RobotBuilder, x, y)
	builderAt(s, 1, x, y)
	s.Jobs = []Job{
		{Kind: BuildingSilo, Col: col, Row: row, Left: 30},
		{Kind: BuildingWarehouse, Col: col + 4, Row: row, Left: 30},
	}
	runTicks(s, 1)
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var back State
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	back.enterRegion()
	if !reflect.DeepEqual(s, &back) {
		t.Fatal("the build reservations changed across a save")
	}
	runTicks(s, 90)
	runTicks(&back, 90)
	if !reflect.DeepEqual(s, &back) {
		t.Fatal("the loaded builders did not continue the same work")
	}
}
