package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"testing"

	"golib"
)

// safePool returns the oil pool inside the core's bubble.
func safePool(t *testing.T) Deposit {
	t.Helper()
	col, row, ok := nearestTileOf(kindOil)
	if !ok {
		t.Fatal("the region has no oil pool")
	}
	d, _ := depositAt(col, row)
	return d
}

func outsideOilPool(t *testing.T, s *State) Deposit {
	t.Helper()
	for _, d := range land.deposits {
		if d.Kind != kindOil {
			continue
		}
		x, y := cellCenterUnits(d.HeartCol, d.HeartRow)
		if !inSafeZone(s, x, y) {
			return d
		}
	}
	t.Fatal("the region has no oil pool outside the core bubble")
	return Deposit{}
}

func protectorCellNearPool(t *testing.T, s *State, d Deposit) (int, int) {
	t.Helper()
	x, y := cellCenterUnits(d.HeartCol, d.HeartRow)
	for row := d.HeartRow - 16; row <= d.HeartRow+16; row++ {
		for col := d.HeartCol - 16; col <= d.HeartCol+16; col++ {
			px, py := cellCenterUnits(col, row)
			if math.Hypot(px-x, py-y) > protectorBubbleTiles*unitsPerTile ||
				!canPlace(s, BuildingProtector, col, row) {
				continue
			}
			return col, row
		}
	}
	t.Fatal("no clear buildable cell lies under a protector's bubble")
	return 0, 0
}

// pumpOn puts a finished pump on a pool's middle, skipping the robots'
// work.
func pumpOn(t *testing.T, s *State, d Deposit) Building {
	t.Helper()
	col, row := pumpCell(d)
	return raised(t, s, BuildingPump, col, row)
}

// pipeOut returns the first pipe that leaves an end.
func pipeOut(s *State, from int64) (Pipe, bool) {
	for _, p := range pipesOf(s, from) {
		if p.From == from {
			return p, true
		}
	}
	return Pipe{}, false
}

// laid runs the simulation until the robots have laid every pipe.
func laid(t *testing.T, s *State) {
	t.Helper()
	done := func() bool {
		_, owed := unlaidPipe(s)
		return !owed
	}
	if !tickUntil(s, 60*600, done) {
		t.Fatalf("the robots never laid the pipes: %v", s.Pipes)
	}
}

// layNow marks a pipe and lays it at once, skipping the robots' work.
func layNow(t *testing.T, s *State, from, to int64) Pipe {
	t.Helper()
	before := len(s.Pipes)
	Apply(s, LayPipe{From: from, To: to})
	if len(s.Pipes) != before+1 {
		t.Fatalf("no pipe was laid from %d to %d", from, to)
	}
	for _, p := range pipesOf(s, from) {
		if p.To == to {
			p.Left = 0
			s.Pipes[p.ID] = p
			return p
		}
	}
	t.Fatal("the pipe just laid is gone")
	return Pipe{}
}

func TestAPumpStandsOnAPoolAndAPoolTakesOne(t *testing.T) {
	s := newGame()
	seedStock(s)
	arriveAll(s)
	col, row := groundNearCore()
	if canPlace(s, BuildingPump, col, row) {
		t.Error("a pump was taken on plain ground")
	}
	d := safePool(t)
	pc, pr := pumpCell(d)
	if tcol, trow := cellTile(pc, pr); tileAt(tcol, trow) != kindOil {
		t.Fatalf("the pump's cell %d, %d stands off its pool", pc, pr)
	}
	if canPlace(s, BuildingSilo, pc, pr) {
		t.Error("a silo was taken on oil")
	}
	if !canPlace(s, BuildingPump, pc, pr) {
		t.Fatal("the safe pool refuses its pump")
	}
	Apply(s, MarkBuilding{Kind: BuildingPump, Col: pc, Row: pr})
	if len(s.Jobs) != 1 || s.Stock.Lilac != 1200-pumpCostLilac {
		t.Fatalf("marking a pump left %d jobs and %v kg", len(s.Jobs), s.Stock.Lilac)
	}
	if canPlace(s, BuildingPump, pc+1, pr) {
		t.Error("a pool with a pump rising took a second one")
	}
}

func TestFogBlocksAnOilPoolUntilAProtectorClearsIt(t *testing.T) {
	s := newGame()
	seedStock(s)
	arriveAll(s)
	noRivals(s)
	d := outsideOilPool(t, s)
	tcol, trow := cellTile(d.HeartCol, d.HeartRow)
	if !oilPoolInFog(s, tcol, trow) ||
		oilPoolAccess(s, tcol, trow) != "covered by fog" {
		t.Fatal("an outside oil pool is not reported as covered by fog")
	}

	r := s.Robots[1]
	Apply(s, AssignRobot{ID: r.ID, Col: tcol, Row: trow})
	r = s.Robots[r.ID]
	r.WorkTicks = 1
	r.X, r.Y = postSpot(tcol, trow, r.ID)
	s.Robots[r.ID] = r
	before := remainingAt(s, tcol, trow)
	stepRobot(s, &r)
	if r.WorkTicks != 0 || !r.hasPost() ||
		remainingAt(s, tcol, trow) != before {
		t.Fatalf("the covered pool loaded oil or lost its robot's post")
	}
	r.Carry = 0
	s.takeLoad(&r)
	if remainingAt(s, tcol, trow) != before || r.Carry != 0 {
		t.Fatal("a direct load took oil from a covered pool")
	}

	col, row := protectorCellNearPool(t, s, d)
	raised(t, s, BuildingProtector, col, row)
	if oilPoolInFog(s, tcol, trow) || oilPoolAccess(s, tcol, trow) != "clear" {
		t.Fatal("the protector did not clear the oil pool")
	}
	s.takeLoad(&r)
	if remainingAt(s, tcol, trow) >= before || r.Carry <= 0 {
		t.Fatal("the cleared pool did not resume extraction")
	}
}

func TestFogStopsAndRestartsAnExternalPump(t *testing.T) {
	s := newGame()
	seedStock(s)
	arriveAll(s)
	noRivals(s)
	d := outsideOilPool(t, s)
	pc, pr := pumpCell(d)
	pump := raised(t, s, BuildingPump, pc, pr)
	col, row := groundNearCore()
	silo := raised(t, s, BuildingSilo, col, row)
	pipe := layNow(t, s, pump.ID, silo.ID)
	tcol, trow := cellTile(pc, pr)
	before := remainingAt(s, tcol, trow)
	stepPipes(s)
	if got := s.Pipes[pipe.ID].Flow; got != 0 {
		t.Fatalf("a pump under fog sent %v L, want none", got)
	}
	if remainingAt(s, tcol, trow) != before ||
		pumpStatus(s, pump) != pumpFogged {
		t.Fatal("the covered pool lost oil or the pump missed its fog status")
	}

	col, row = protectorCellNearPool(t, s, d)
	raised(t, s, BuildingProtector, col, row)
	stepPipes(s)
	pipe = s.Pipes[pipe.ID]
	if math.Abs(pipe.Offered-pumpLitersPerSecond/60) > 1e-9 {
		t.Fatalf("a cleared pump offered %v L, want %v",
			pipe.Offered, pumpLitersPerSecond/60)
	}
	if math.Abs(pipe.Flow-tankFillPerSecond/60) > 1e-9 {
		t.Fatalf("a cleared pump moved %v L, want %v",
			pipe.Flow, tankFillPerSecond/60)
	}
	if remainingAt(s, tcol, trow) >= before || pumpStatus(s, pump) != pumpPumping {
		t.Fatal("the protected pool did not resume pumping")
	}
}

func TestExternalPumpIsEatenUnlessProtected(t *testing.T) {
	s := newGame()
	seedStock(s)
	arriveAll(s)
	noRivals(s)
	safe := safePool(t)
	var far Deposit
	for _, d := range land.deposits {
		if d.Kind != kindOil || d == safe {
			continue
		}
		x, y := cellCenterUnits(d.HeartCol, d.HeartRow)
		if !inSafeZone(s, x, y) {
			far = d
			break
		}
	}
	if far.Kind == 0 {
		t.Fatal("no oil pool outside the core's bubble")
	}
	col, row := pumpCell(far)
	panel := tooltipLayout(s, newPlayScene(s).camera,
		col, row, map[string]bool{})
	pumpButton := panel.findButton(buttonBuildPump)
	if pumpButton == nil || pumpButton.disabled {
		t.Fatal("the affordable outside pool doesn't offer an active pump")
	}
	s.Stock.Lilac = 0
	panel = tooltipLayout(s, newPlayScene(s).camera,
		col, row, map[string]bool{})
	pumpButton = panel.findButton(buttonBuildPump)
	if pumpButton == nil || !pumpButton.disabled ||
		len(pumpButton.costs) != 1 || !pumpButton.costs[0].missing {
		t.Fatal("the fog-covered pool hides its unaffordable pump")
	}
	seedStock(s)
	Apply(s, MarkBuilding{Kind: BuildingPump, Col: col, Row: row})
	if len(s.Jobs) != 1 {
		t.Fatal("the outside pool refused its pump site")
	}
	if !tickUntil(s, 60*180, func() bool {
		_, built := buildingAt(s, col, row)
		return built
	}) {
		t.Fatal("robots never raised the outside pump")
	}
	pump, _ := buildingAt(s, col, row)
	x, y := cellCenterUnits(pump.Col, pump.Row)
	exposure := miteExposureAt(s, x, y)
	if exposure <= 0 {
		t.Fatal("the external pump has no mite exposure")
	}
	runTicks(s, 60)
	if _, stands := s.Buildings[pump.ID]; !stands {
		t.Fatal("the pump vanished before mites could be seen eating it")
	}
	if s.Buildings[pump.ID].Damage <= 0 {
		t.Error("the unprotected pump has no damage")
	}
	mites := newMiteField()
	mites.update(s, 1.0/60)
	host := mites.hosts[fmt.Sprintf("building:%d", pump.ID)]
	if host == nil || host.Wanted == 0 {
		t.Error("no mites gather on the exposed pump")
	}
	ticks := int(math.Ceil(
		(buildingHealth(pump.Kind) - s.Buildings[pump.ID].Damage) * 60 /
			(miteDamagePerSecond * exposure),
	))
	runTicks(s, ticks+3)
	if _, stands := s.Buildings[pump.ID]; stands {
		t.Fatal("the unprotected pump survived the mites")
	}
	if lastReport(s).Kind != ReportMiteEaten {
		t.Errorf("no explanation for the lost pump: %+v", s.Reports)
	}
	if pile, ok := pileAt(s, col, row); !ok ||
		math.Abs(pile.Lilac-pumpCostLilac*wreckRefund) > 0.1 {
		t.Errorf("the pump left pile %v, found %v", pile, ok)
	}
	if canPlace(s, BuildingPump, col, row) {
		t.Error("the wreck does not hold the pump cell")
	}
}

func TestProtectorKeepsExternalPumpSafe(t *testing.T) {
	s := newGame()
	seedStock(s)
	arriveAll(s)
	noRivals(s)
	safe := safePool(t)
	var far Deposit
	for _, d := range land.deposits {
		if d.Kind == kindOil && d != safe {
			far = d
			break
		}
	}
	if far.Kind == 0 {
		t.Fatal("no second oil pool")
	}
	col, row := pumpCell(far)
	pump := pumpOn(t, s, far)
	runTicks(s, 60)
	if s.Buildings[pump.ID].Damage == 0 {
		t.Fatal("the external pump has not begun to be digested")
	}
	pc, pr, found := protectorSite(far)
	if !found {
		t.Fatal("no ground for a protector near the second pool")
	}
	protector := raised(t, s, BuildingProtector, pc, pr)
	if x, y := cellCenterUnits(col, row); !inSafeZone(s, x, y) {
		t.Fatalf("protector %d does not cover the pump", protector.ID)
	}
	mites := newMiteField()
	mites.update(s, 1.0/60)
	host := mites.hosts[fmt.Sprintf("building:%d", pump.ID)]
	if host != nil && host.Wanted != 0 {
		t.Error("mites still swarm a sheltered pump")
	}
	damage := s.Buildings[pump.ID].Damage
	runTicks(s, 60)
	if b, ok := s.Buildings[pump.ID]; !ok || b.Damage != damage {
		t.Errorf("the protected pump didn't survive unchanged: %+v, %v", b, ok)
	}
}

func TestWriteExternalPumpShotState(t *testing.T) {
	path := os.Getenv("NIEBLA_PUMP_SHOT_STATE")
	if path == "" {
		t.Skip("set NIEBLA_PUMP_SHOT_STATE to write the pump shot state")
	}
	s := newGame()
	seedStock(s)
	arriveAll(s)
	noRivals(s)
	safe := safePool(t)
	placed := false
	for _, d := range land.deposits {
		if d.Kind != kindOil || d == safe {
			continue
		}
		pump := pumpOn(t, s, d)
		pump.Damage = buildingHealth(pump.Kind) / 3
		s.Buildings[pump.ID] = pump
		placed = true
		break
	}
	if !placed {
		t.Fatal("no second oil pool for the shot")
	}
	data, err := json.MarshalIndent(map[string]any{"state": s}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAPipeIsPaidBySectionAndLaidByTheRobots(t *testing.T) {
	s := newGame()
	seedStock(s)
	arriveAll(s)
	pump := pumpOn(t, s, safePool(t))
	from, _ := pipeEndSpot(s, pump.ID)
	bends := []PipePoint{{from.X + 120, from.Y - 60}}
	sections, ok := canLayPipe(s, pump.ID, 0, bends)
	if !ok || sections < 2 {
		t.Fatalf("the pump can't take a pipe to the core: %d sections, %v", sections, ok)
	}
	Apply(s, LayPipe{From: pump.ID, To: 0, Bends: bends})
	p, piped := pipeOut(s, pump.ID)
	if !piped {
		t.Fatal("laying left no pipe")
	}
	if want := 1200 - float64(sections)*pipeSectionLilac; s.Stock.Lilac != want {
		t.Errorf("the stores hold %v kg, want %v: %d sections paid",
			s.Stock.Lilac, want, sections)
	}
	if p.Left != sections*pipeSectionWorkTicks {
		t.Errorf("the pipe asks %d ticks, want %d", p.Left, sections*pipeSectionWorkTicks)
	}
	oil := s.Stock.Oil
	runTicks(s, 120)
	if s.Stock.Oil != oil {
		t.Error("oil ran down a pipe nobody has laid yet")
	}
	if pumpStatus(s, pump) != pumpLaying {
		t.Errorf("the pump says %q while its pipe is laid", pumpStatus(s, pump))
	}
	laid(t, s)
	if _, again := canLayPipe(s, pump.ID, 0, nil); again {
		t.Error("two ends took a second pipe between them")
	}
}

func TestALaidPipeCarriesThePoolIntoItsTank(t *testing.T) {
	s := newGame()
	seedStock(s)
	arriveAll(s)
	d := safePool(t)
	pump := pumpOn(t, s, d)
	Apply(s, LayPipe{From: pump.ID, To: coreTank})
	laid(t, s)
	p, ok := pipeOut(s, pump.ID)
	if !ok {
		t.Fatal("the laid pipe disappeared")
	}
	oil, pool := s.Stock.Oil, s.Drain[depositKey(d)]
	runTicks(s, 600)
	gained := s.Stock.Oil - oil
	wantGained := tankFillPerSecond * 10
	if math.Abs(gained-wantGained) > 0.001 {
		t.Errorf("ten seconds of pumping brought %v L, want %v",
			gained, wantGained)
	}
	p = s.Pipes[p.ID]
	wantFlow := tankFillPerSecond / 60
	if math.Abs(p.Flow-wantFlow) > 1e-9 {
		t.Errorf("the pipe moved %v L this tick, want %v", p.Flow, wantFlow)
	}
	wantOffer := pumpLitersPerSecond / 60
	if math.Abs(p.Offered-wantOffer) > 1e-9 {
		t.Errorf("the pump offered %v L this tick, want %v",
			p.Offered, wantOffer)
	}
	if math.Abs(p.Moved-gained) > 0.001 {
		t.Errorf("the pipe records %v L moved, but the tank gained %v",
			p.Moved, gained)
	}
	if lost := pool - s.Drain[depositKey(d)]; math.Abs(lost-gained) > 0.001 {
		t.Errorf("the pool lost %v L and the core gained %v", lost, gained)
	}
	for _, r := range s.Robots {
		if r.Carry > 0 {
			t.Errorf("robot %d carries %v: the pipe's oil rides in nobody's arms",
				r.ID, r.Carry)
		}
	}
	// A full tank at the pipe's end stops the pump, and nothing is lost.
	moved := p.Moved
	s.Stock.Oil = coreOilCap - 1
	pool = s.Drain[depositKey(d)]
	runTicks(s, 600)
	p = s.Pipes[p.ID]
	if s.Stock.Oil != coreOilCap {
		t.Errorf("the core holds %v L in a tank of %v", s.Stock.Oil, coreOilCap)
	}
	if math.Abs(p.Moved-moved-1) > 0.001 || p.Flow != 0 {
		t.Errorf("a full tank left pipe movement at %v L total and %v L this tick",
			p.Moved-moved, p.Flow)
	}
	if lost := pool - s.Drain[depositKey(d)]; math.Abs(lost-1) > 0.001 {
		t.Errorf("the pool lost %v L for 1 L of room", lost)
	}
	if pumpStatus(s, pump) != pumpBlocked {
		t.Errorf("the pump says %q into a full tank", pumpStatus(s, pump))
	}
	// A dry pool stops it too.
	s.Stock.Oil = 0
	s.Drain[depositKey(d)] = 0.01
	moved = p.Moved
	runTicks(s, 60)
	p = s.Pipes[p.ID]
	if s.Drain[depositKey(d)] != 0 || pumpStatus(s, pump) != pumpDry {
		t.Errorf("the pool holds %v L and the pump says %q",
			s.Drain[depositKey(d)], pumpStatus(s, pump))
	}
	if math.Abs(p.Moved-moved-0.01) > 1e-9 || p.Flow != 0 {
		t.Errorf("a dry pool left pipe movement at %v L and %v L this tick",
			p.Moved-moved, p.Flow)
	}
}

func TestPipeFillsEachTankBeforePassingExcess(t *testing.T) {
	s := newGame()
	seedStock(s)
	arriveAll(s)
	noRivals(s)
	pump := pumpOn(t, s, safePool(t))
	col, row := groundNearCore()
	first := raised(t, s, BuildingSilo, col, row)
	second := raised(t, s, BuildingSilo, col+3, row)
	intoFirst := layNow(t, s, pump.ID, first.ID)
	between := layNow(t, s, first.ID, second.ID)
	intoCore := layNow(t, s, second.ID, coreTank)

	stepPipes(s)

	got, want := s.Pipes[intoFirst.ID].Flow*60, pumpLitersPerSecond
	if math.Abs(got-want) > 1e-8 {
		t.Errorf("the pump sent %v L/s, want %v", got, want)
	}
	got, want = s.Buildings[first.ID].Oil*60, tankFillPerSecond
	if math.Abs(got-want) > 1e-8 {
		t.Errorf("the first tank filled at %v L/s, want %v", got, want)
	}
	got, want = s.Pipes[between.ID].Flow*60,
		pumpLitersPerSecond-tankFillPerSecond
	if math.Abs(got-want) > 1e-8 {
		t.Errorf("the first tank passed %v L/s, want %v", got, want)
	}
	got, want = s.Buildings[second.ID].Oil*60,
		pumpLitersPerSecond-tankFillPerSecond
	if math.Abs(got-want) > 1e-8 {
		t.Errorf("the second tank filled at %v L/s, want %v", got, want)
	}
	if got := s.Pipes[intoCore.ID].Flow; got != 0 {
		t.Errorf("the core received %v L before the second tank filled", got)
	}
	firstTank := s.Buildings[first.ID]
	firstTank.Oil = siloOilCap
	s.Buildings[first.ID] = firstTank
	if got := pumpStatus(s, pump); got != pumpPumping {
		t.Errorf("a full tank with a free outlet reports %q", got)
	}
}

func TestTankFillRateIsSharedAcrossIncomingPipes(t *testing.T) {
	s := newGame()
	seedStock(s)
	arriveAll(s)
	noRivals(s)
	pump := pumpOn(t, s, safePool(t))
	col, row := groundNearCore()
	silo := raised(t, s, BuildingSilo, col, row)
	corePipe := layNow(t, s, coreTank, silo.ID)
	pumpPipe := layNow(t, s, pump.ID, silo.ID)

	runTicks(s, 60)
	totalFlow := s.Pipes[corePipe.ID].Moved + s.Pipes[pumpPipe.ID].Moved
	if math.Abs(totalFlow-tankFillPerSecond) > 1e-8 {
		t.Errorf("the tank received %v L in one second, want %v",
			totalFlow, tankFillPerSecond)
	}
	if math.Abs(s.Buildings[silo.ID].Oil-tankFillPerSecond) > 1e-8 {
		t.Errorf("the tank holds %v L after one second, want %v",
			s.Buildings[silo.ID].Oil, tankFillPerSecond)
	}
}

func TestPipeFlowAnimationFollowsDestinationDemand(t *testing.T) {
	t.Run("a silo receives at its fill rate", func(t *testing.T) {
		s := newGame()
		seedStock(s)
		arriveAll(s)
		d := safePool(t)
		pump := pumpOn(t, s, d)
		col, row := groundNearCore()
		silo := raised(t, s, BuildingSilo, col, row)
		p := layNow(t, s, pump.ID, silo.ID)

		runTicks(s, 600)
		p = s.Pipes[p.ID]
		wantMoved := tankFillPerSecond * 10
		if math.Abs(p.Moved-wantMoved) > 0.001 {
			t.Errorf("the pipe moved %v L in ten seconds, want %v",
				p.Moved, wantMoved)
		}
		wantFlow := tankFillPerSecond / 60
		if math.Abs(p.Flow-wantFlow) > 1e-9 {
			t.Errorf("the silo pipe moved %v L this tick, want %v",
				p.Flow, wantFlow)
		}
		wantOffer := pumpLitersPerSecond / 60
		if math.Abs(p.Offered-wantOffer) > 1e-9 {
			t.Errorf("the pump offered %v L this tick, want %v",
				p.Offered, wantOffer)
		}
		if got := pipeFlowPhase(p.Moved); math.Abs(got) > 1e-9 {
			t.Errorf("the pump-rate flow is at phase %v after 15 beats", got)
		}
	})

	t.Run("shared pump output slows both pipes", func(t *testing.T) {
		s := newGame()
		seedStock(s)
		arriveAll(s)
		d := safePool(t)
		pump := pumpOn(t, s, d)
		col, row := groundNearCore()
		first := raised(t, s, BuildingSilo, col, row)
		second := raised(t, s, BuildingSilo, col+2, row)
		firstPipe := layNow(t, s, pump.ID, first.ID)
		secondPipe := layNow(t, s, pump.ID, second.ID)

		runTicks(s, 600)
		for _, p := range []Pipe{
			s.Pipes[firstPipe.ID], s.Pipes[secondPipe.ID],
		} {
			wantMoved := pumpLitersPerSecond * 5
			if math.Abs(p.Moved-wantMoved) > 0.001 {
				t.Errorf("the shared pipe moved %v L in ten seconds, want %v",
					p.Moved, wantMoved)
			}
			wantFlow := pumpLitersPerSecond / 2 / 60
			if math.Abs(p.Flow-wantFlow) > 1e-9 {
				t.Errorf("the shared pipe moved %v L this tick, want %v",
					p.Flow, wantFlow)
			}
			if math.Abs(p.Offered-wantFlow) > 1e-9 {
				t.Errorf("the shared source offered %v L this tick, want %v",
					p.Offered, wantFlow)
			}
		}
	})

	t.Run("full protector draws only its upkeep", func(t *testing.T) {
		s := newGame()
		seedStock(s)
		arriveAll(s)
		d := safePool(t)
		pump := pumpOn(t, s, d)
		col, row := groundNearCore()
		protector := raised(t, s, BuildingProtector, col, row)
		p := layNow(t, s, pump.ID, protector.ID)

		if !tickUntil(s, 60*180, func() bool {
			return s.Buildings[protector.ID].Oil >= protectorOilCap-0.01
		}) {
			t.Fatal("the protector never filled without an outlet")
		}
		runTicks(s, 120)
		protector = s.Buildings[protector.ID]
		p = s.Pipes[p.ID]
		if protector.Oil < protectorOilCap-0.01 {
			t.Fatalf("the protector has %v L, not yet full", protector.Oil)
		}
		wantFlow := protectorOilPerSecond / 60
		if math.Abs(p.Flow-wantFlow) > 1e-9 {
			t.Fatalf("the full protector's pipe moved %v L this tick, want %v",
				p.Flow, wantFlow)
		}
		if math.Abs(p.Offered-pumpLitersPerSecond/60) > 1e-9 {
			t.Fatalf("the pump offered %v L this tick, want its full rate %v",
				p.Offered, pumpLitersPerSecond/60)
		}

		moved := p.Moved
		phase := pipeFlowPhase(moved)
		runTicks(s, 60)
		p = s.Pipes[p.ID]
		wantMoved := protectorOilPerSecond
		if math.Abs(p.Moved-moved-wantMoved) > 1e-9 {
			t.Errorf("the pipe moved %v L in one second, want upkeep %v",
				p.Moved-moved, wantMoved)
		}
		wantPhase := protectorOilPerSecond / flowLitersPerBeat
		phaseDelta := pipeFlowPhase(p.Moved) - phase
		if math.Abs(phaseDelta-wantPhase) > 1e-9 {
			t.Errorf("one second advanced the animation by %v, want %v",
				phaseDelta, wantPhase)
		}
	})
}

func TestPipeFlowBandsTrackTheSourceOffer(t *testing.T) {
	full := pipeFlowBandPart(pumpLitersPerSecond / 60)
	half := pipeFlowBandPart(pumpLitersPerSecond / 120)
	if math.Abs(full-0.9) > 1e-9 {
		t.Errorf("a full pump offer colors %v of the gap, want 0.9", full)
	}
	if math.Abs(half-0.45) > 1e-9 {
		t.Errorf("half a pump offer colors %v of the gap, want 0.45", half)
	}
	if got := pipeFlowBandPart(pumpLitersPerSecond); got != flowBandMaxPart {
		t.Errorf("an offer above pump capacity colors %v of the gap", got)
	}
	if got := pipeFlowBandPart(-pumpLitersPerSecond / 60); got != 0 {
		t.Errorf("a negative offer colors %v of the gap", got)
	}
}

func TestProtectorFillsAtItsRateAndPassesExcess(t *testing.T) {
	s := newGame()
	seedStock(s)
	arriveAll(s)
	noRivals(s)
	pump := pumpOn(t, s, safePool(t))
	col, row := groundNearCore()
	protector := raised(t, s, BuildingProtector, col, row)
	silo := raised(t, s, BuildingSilo, col+3, row)
	reserve := raised(t, s, BuildingSilo, col+6, row)
	inPipe := layNow(t, s, pump.ID, protector.ID)
	outPipe := layNow(t, s, protector.ID, silo.ID)
	tailPipe := layNow(t, s, silo.ID, reserve.ID)
	initialOil := s.Buildings[protector.ID].Oil

	runTicks(s, 60*60)
	wantOil := initialOil +
		(tankFillPerSecond-protectorOilPerSecond)*60
	if math.Abs(s.Buildings[protector.ID].Oil-wantOil) > 1e-6 {
		t.Errorf("the protector holds %v L, want %v after one minute",
			s.Buildings[protector.ID].Oil, wantOil)
	}
	got, want := s.Pipes[outPipe.ID].Flow*60,
		pumpLitersPerSecond-tankFillPerSecond
	if math.Abs(got-want) > 1e-8 {
		t.Fatalf("the filling protector passed %v L/s, want %v", got, want)
	}
	if s.Buildings[silo.ID].Oil <= 0 || s.Buildings[reserve.ID].Oil <= 0 {
		t.Fatal("the passing excess did not reach the second silo")
	}
	got, want = s.Pipes[tailPipe.ID].Flow*60,
		pumpLitersPerSecond-tankFillPerSecond
	if math.Abs(got-want) > 1e-8 {
		t.Errorf("the second silo received %v L/s, want %v", got, want)
	}

	if !tickUntil(s, 60*200, func() bool {
		return s.Buildings[protector.ID].Oil >= protectorOilCap-0.01
	}) {
		t.Fatal("the protector never filled its reserve")
	}
	runTicks(s, 120)
	if s.Buildings[protector.ID].Oil < protectorOilCap-0.01 {
		t.Fatalf(
			"the protector holds %v L after filling",
			s.Buildings[protector.ID].Oil,
		)
	}
	if s.Buildings[silo.ID].Oil <= 0 {
		t.Fatal("the full protector did not pass oil into the silo")
	}
	wantRate := pumpLitersPerSecond - protectorOilPerSecond
	gotRate := s.Pipes[outPipe.ID].Flow * 60
	if math.Abs(gotRate-wantRate) > 1e-8 {
		t.Errorf("the full protector passed %v L/s, want %v", gotRate, wantRate)
	}
	if s.Pipes[inPipe.ID].Flow <= s.Pipes[outPipe.ID].Flow {
		t.Error("the protector's upkeep did not reduce downstream flow")
	}
	got, want = s.Pipes[tailPipe.ID].Flow*60, tankFillPerSecond
	if math.Abs(got-want) > 1e-8 {
		t.Errorf("the reserve received %v L/s, want its fill limit %v",
			got, want)
	}
}

func TestProtectorChainConsumesUpkeepBeforeEachOutlet(t *testing.T) {
	s := newGame()
	seedStock(s)
	arriveAll(s)
	noRivals(s)
	d := safePool(t)
	pump := pumpOn(t, s, d)
	poolKey := depositKey(d)
	col, row := groundNearCore()
	protectors := make([]Building, 8)
	for i := range protectors {
		protectors[i] = raised(
			t, s, BuildingProtector, col+i*2, row,
		)
		protectors[i].Oil = protectorOilCap - protectorOilPerSecond/60
		s.Buildings[protectors[i].ID] = protectors[i]
	}
	silo := raised(t, s, BuildingSilo, col+len(protectors)*2, row)

	from := pump.ID
	pipes := make([]Pipe, 0, len(protectors)+1)
	for _, protector := range protectors {
		pipes = append(pipes, layNow(t, s, from, protector.ID))
		from = protector.ID
	}
	pipes = append(pipes, layNow(t, s, from, silo.ID))

	oilBefore := allOilTotal(s) + s.Drain[poolKey]
	runTicks(s, 120)
	wantOil := oilBefore -
		float64(len(protectors))*protectorOilPerSecond*2
	gotOil := allOilTotal(s) + s.Drain[poolKey]
	if math.Abs(gotOil-wantOil) > 1e-6 {
		t.Errorf("the chain keeps %v L including the pool, want %v",
			gotOil, wantOil)
	}
	for i, p := range pipes {
		wantRate := pumpLitersPerSecond -
			float64(i)*protectorOilPerSecond
		gotRate := s.Pipes[p.ID].Flow * 60
		if math.Abs(gotRate-wantRate) > 1e-8 {
			t.Errorf("pipe %d moves %v L/s, want %v", i+1, gotRate, wantRate)
		}
		wantOffer := wantRate
		if i == 0 {
			wantOffer = pumpLitersPerSecond
		}
		gotOffer := s.Pipes[p.ID].Offered * 60
		if math.Abs(gotOffer-wantOffer) > 1e-8 {
			t.Errorf("pipe %d is offered %v L/s, want %v",
				i+1, gotOffer, wantOffer)
		}
		if i > 0 && i < len(protectors) &&
			math.Abs(
				pipeFlowBandPart(s.Pipes[p.ID].Offered)-
					flowBandMaxPart*wantOffer/pumpLitersPerSecond,
			) > 1e-8 {
			t.Errorf("pipe %d's band does not follow its thinning offer", i+1)
		}
	}
}

func TestOilHasAPlaceAndPipesMoveItBetweenTanks(t *testing.T) {
	s := newGame()
	seedStock(s)
	arriveAll(s)
	delete(s.Robots, 1) // nobody hauls or refuels: the pipes alone move oil
	delete(s.Robots, 2)
	noRivals(s) // and nobody siphons
	col, row := groundNearCore()
	first := raised(t, s, BuildingSilo, col, row)
	second := raised(t, s, BuildingSilo, col+2, row)
	charger := raised(t, s, BuildingCharger, col+4, row)
	if got, want := oilCap(s), coreOilCap+2*siloOilCap+chargerOilCap; got != want {
		t.Fatalf("the colony's tanks hold %v L at most, want %v", got, want)
	}
	// The core feeds a silo, which feeds another silo and a charger.
	layNow(t, s, coreTank, first.ID)
	layNow(t, s, first.ID, second.ID)
	layNow(t, s, first.ID, charger.ID)
	total := oilTotal(s)
	runTicks(s, 60*400)
	if math.Abs(oilTotal(s)-total) > 0.001 {
		t.Errorf("the colony holds %v L after piping, had %v: pipes lose nothing",
			oilTotal(s), total)
	}
	if s.Stock.Oil > 0.001 || s.Buildings[first.ID].Oil > 0.001 {
		t.Errorf("the core holds %v L and the first silo %v: the oil ran on",
			s.Stock.Oil, s.Buildings[first.ID].Oil)
	}
	if got := s.Buildings[charger.ID].Oil; math.Abs(got-chargerOilCap) > 0.001 {
		t.Errorf("the charger holds %v L, want its tank full at %v", got, chargerOilCap)
	}
	if got := s.Buildings[second.ID].Oil; math.Abs(got-(600-chargerOilCap)) > 0.001 {
		t.Errorf("the second silo holds %v L, want the rest, %v", got, 600-chargerOilCap)
	}
	// What the colony pays comes out of any tank.
	Apply(s, MarkBuilding{Kind: BuildingProtector, Col: col, Row: row + 3})
	if got := total - oilTotal(s); math.Abs(got-protectorCostOil) > 0.001 {
		t.Errorf("a protector took %v L off the tanks, want %v", got, protectorCostOil)
	}
	// A building takes so many pipes and no more, and two ends take one.
	third := raised(t, s, BuildingSilo, col+6, row)
	if canJoin(s, first.ID, third.ID) {
		t.Errorf("a silo with %d pipes took one more", pipePorts)
	}
	if canJoin(s, second.ID, first.ID) {
		t.Error("two silos took a second pipe between them, the other way")
	}
	if canJoin(s, second.ID, second.ID) {
		t.Error("a silo took a pipe to itself")
	}
	// A demolished tank drops its own oil, and its pipes with it.
	held := s.Buildings[second.ID].Oil
	demolishNow(t, s, second.ID)
	pile, _ := pileAt(s, col+2, row)
	if math.Abs(pile.Oil-held) > 0.001 {
		t.Errorf("the pile holds %v L, want the silo's %v", pile.Oil, held)
	}
	if len(pipesOf(s, first.ID)) != 2 {
		t.Errorf("the first silo keeps %d pipes, want 2", len(pipesOf(s, first.ID)))
	}
}

func TestRobotsCarryOilToATankWithRoomAndRefillFromTheirPost(t *testing.T) {
	s := newGame()
	col, row := groundNearCore()
	silo := raised(t, s, BuildingSilo, col, row)
	s.Stock.Oil = coreOilCap
	x, y := parkSlot(0)
	r := s.Robots[1]
	r.X, r.Y, r.Carry, r.Cargo = x, y, 20, TypeOil
	s.Robots[1] = r
	if got := haulTank(s, r); got != silo.ID {
		t.Fatalf("a robot by a full core carries its oil to tank %d, want the silo", got)
	}
	if !tickUntil(s, 60*120, func() bool { return s.Robots[1].Carry == 0 }) {
		t.Fatal("the robot never unloaded")
	}
	if s.Buildings[silo.ID].Oil != 20 || s.Stock.Oil != coreOilCap {
		t.Errorf("the silo holds %v L and the core %v, want the load in the silo",
			s.Buildings[silo.ID].Oil, s.Stock.Oil)
	}
	// A dry charger sends a thirsty robot on to a post with oil.
	charger := raised(t, s, BuildingCharger, col+3, row)
	cx, cy := cellCenterUnits(charger.Col, charger.Row)
	thirsty := Robot{Kind: RobotWorker, X: cx + 5, Y: cy, Tank: 10}
	if got := refuelTank(s, thirsty); got != coreTank {
		t.Errorf("a robot by a dry charger refills at tank %d, want the core", got)
	}
	b := s.Buildings[charger.ID]
	b.Oil = 50
	s.Buildings[charger.ID] = b
	if got := refuelTank(s, thirsty); got != charger.ID {
		t.Errorf("a robot by a charger with oil refills at tank %d, want it", got)
	}
}

func TestLayPipeRefusesWhatItCantJoin(t *testing.T) {
	s := newGame()
	seedStock(s)
	arriveAll(s)
	col, row := groundNearCore()
	charger := raised(t, s, BuildingCharger, col, row)
	pump := pumpOn(t, s, safePool(t))
	refused := []LayPipe{
		{From: pump.ID, To: pump.ID},
		{From: charger.ID, To: pump.ID},
		{From: 998, To: coreTank},
		{From: pump.ID, To: 999},
		{From: pump.ID, To: 0, Bends: []PipePoint{{-5, 100}}},
		{From: pump.ID, To: 0, Bends: make([]PipePoint, pipeMaxBends+1)},
	}
	for _, a := range refused {
		Apply(s, a)
		if len(s.Pipes) != 0 || s.Stock.Lilac != 1200 {
			t.Fatalf("%+v was taken: %v, %v kg", a, s.Pipes, s.Stock.Lilac)
		}
	}
	s.Stock.Lilac = pipeSectionLilac - 1
	Apply(s, LayPipe{From: pump.ID, To: 0})
	if len(s.Pipes) != 0 {
		t.Error("a pipe was laid on stores that can't pay a section")
	}
}

func TestAPipeLeavesWithItsEndsAndItsCostFallsAsAPile(t *testing.T) {
	s := newGame()
	seedStock(s)
	arriveAll(s)
	pump := pumpOn(t, s, safePool(t))
	col, row := groundNearCore()
	silo := raised(t, s, BuildingSilo, col, row)
	Apply(s, LayPipe{From: pump.ID, To: silo.ID})
	p, _ := pipeOut(s, pump.ID)
	demolishNow(t, s, silo.ID)
	if len(s.Pipes) != 0 {
		t.Fatal("the pipe outlived the silo it ended at")
	}
	pile, _ := pileAt(s, col, row)
	if want := siloCostLilac + pipeCost(p.Sections); pile.Lilac != want {
		t.Errorf("the pile holds %v kg, want the silo's and the pipe's %v", pile.Lilac, want)
	}

	Apply(s, LayPipe{From: pump.ID, To: 0})
	p, _ = pipeOut(s, pump.ID)
	Apply(s, RemovePipe{Pipe: p.ID})
	if len(s.Pipes) != 0 {
		t.Fatal("the removed pipe is still there")
	}
	pile, _ = pileAt(s, pump.Col, pump.Row)
	if pile.Lilac != pipeCost(p.Sections) {
		t.Errorf("the pile by the pump holds %v kg, want %v", pile.Lilac, pipeCost(p.Sections))
	}

	Apply(s, LayPipe{From: pump.ID, To: 0})
	demolishNow(t, s, pump.ID)
	if len(s.Pipes) != 0 {
		t.Error("the pipe outlived its pump")
	}
}

func TestAPipesCurvePassesThroughItsBends(t *testing.T) {
	from, to := PipePoint{100, 100}, PipePoint{900, 300}
	bends := []PipePoint{{300, 400}, {320, 410}, {700, 50}}
	path := pipePath(from, bends, to)
	if path[0] != from || pointGap(path[len(path)-1], to) > 1e-9 {
		t.Fatalf("the curve runs from %v to %v", path[0], path[len(path)-1])
	}
	for _, bend := range bends {
		nearest := math.Inf(1)
		for _, p := range path {
			nearest = math.Min(nearest, pointGap(p, bend))
		}
		if nearest > 1e-6 {
			t.Errorf("the curve misses the bend %v by %v u", bend, nearest)
		}
	}
	for _, p := range path {
		if math.IsNaN(p.X) || math.IsNaN(p.Y) {
			t.Fatal("the curve holds a point that is no number")
		}
	}
	if !reflect.DeepEqual(path, pipePath(from, bends, to)) {
		t.Error("the same clicks drew two curves")
	}
	// No bends: a sag, not a ruler's line, and longer than one.
	sag := pipePath(from, nil, to)
	if pathLength(sag) <= pointGap(from, to) {
		t.Error("a pipe with no bends runs straight")
	}
	// Two clicks on one spot, or on an end, break nothing.
	twice := pipePath(from, []PipePoint{{300, 400}, {300, 400}, to}, to)
	for _, p := range twice {
		if math.IsNaN(p.X) || math.IsNaN(p.Y) {
			t.Fatal("a doubled click broke the curve")
		}
	}
	half := pathPointAt(path, pathLength(path)/2)
	if half == from || half == to {
		t.Errorf("the curve's middle is its end: %v", half)
	}
	if pathPointAt(path, -10) != from || pathPointAt(path, 1e9) != path[len(path)-1] {
		t.Error("a point past the curve's ends isn't held at them")
	}
}

func TestPipesSurviveASave(t *testing.T) {
	s := newGame()
	seedStock(s)
	arriveAll(s)
	pump := pumpOn(t, s, safePool(t))
	layNow(t, s, pump.ID, 0)
	runTicks(s, 300)
	p, ok := pipeOut(s, pump.ID)
	if !ok || p.Moved <= 0 || p.Flow <= 0 {
		t.Fatal("a flowing pipe didn't save its animation progress")
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("the state does not marshal: %v", err)
	}
	var back State
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("the state does not unmarshal: %v", err)
	}
	if !reflect.DeepEqual(&back, s) {
		t.Error("the pipes changed across a JSON round trip")
	}
	runTicks(s, 60)
	runTicks(&back, 60)
	if !reflect.DeepEqual(&back, s) {
		t.Error("a loaded pipe didn't continue its flow deterministically")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("the state fields don't unmarshal: %v", err)
	}
	var oldPipes map[string]map[string]json.RawMessage
	if err := json.Unmarshal(fields["Pipes"], &oldPipes); err != nil {
		t.Fatalf("the pipe fields don't unmarshal: %v", err)
	}
	for _, oldPipe := range oldPipes {
		delete(oldPipe, "Offered")
		delete(oldPipe, "Flow")
		delete(oldPipe, "Moved")
	}
	fields["Pipes"], err = json.Marshal(oldPipes)
	if err != nil {
		t.Fatalf("the old pipes don't marshal: %v", err)
	}
	oldData, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("the old state doesn't marshal: %v", err)
	}
	var legacy State
	if err := json.Unmarshal(oldData, &legacy); err != nil {
		t.Fatalf("the old state doesn't unmarshal: %v", err)
	}
	legacy.enterRegion()
	runTicks(&legacy, 1)
	if p := legacy.Pipes[p.ID]; math.Abs(p.Offered-pumpLitersPerSecond/60) > 1e-9 ||
		math.Abs(p.Flow-tankFillPerSecond/60) > 1e-9 ||
		math.Abs(p.Moved-p.Flow) > 1e-9 {
		t.Errorf("an old pipe didn't resume with fresh flow data: %+v", p)
	}
	// A save from before the pipes has no table for them, and takes one.
	old := newGame()
	seedStock(old)
	arriveAll(old)
	old.Pipes = nil
	oldPump := pumpOn(t, old, safePool(t))
	Apply(old, LayPipe{From: oldPump.ID, To: 0})
	if len(old.Pipes) != 1 {
		t.Error("a save with no pipe table refused a pipe")
	}
}

func TestAPumpAndItsSiteBelongOnlyToTheirCell(t *testing.T) {
	s := newGame()
	addWorker(s)
	seedStock(s)
	arriveAll(s)
	d := safePool(t)
	camera := newPlayScene(s).camera
	dc, dr := tileCell(d.Col, d.Row)
	panel := tooltipLayout(s, camera, dc, dr, map[string]bool{})
	if panel.findButton(buttonBuildPump) == nil {
		t.Fatal("a pool with no pump offers none")
	}
	s.Stock.Lilac = 0
	panel = tooltipLayout(s, camera, dc, dr, map[string]bool{})
	pumpButton := panel.findButton(buttonBuildPump)
	if pumpButton == nil || !pumpButton.disabled {
		t.Fatal("the unaffordable pump isn't visible and disabled")
	}
	if len(pumpButton.costs) != 1 || !pumpButton.costs[0].missing {
		t.Fatal("the pump cost doesn't mark its missing lilac")
	}
	seedStock(s)
	pump := pumpOn(t, s, d)
	pc, pr := pumpCell(d)
	if !patchPumped(s, d.Col, d.Row) {
		t.Fatal("the pump no longer belongs to its pool functionally")
	}
	for _, tile := range depositTiles(d) {
		col, row := tileCell(tile[0], tile[1])
		if col == pc && row == pr {
			col++
		}
		things := thingsAt(s, col, row)
		foundOil, foundPump := false, false
		for _, thing := range things {
			foundOil = foundOil || thing.Type == TypeOil
			foundPump = foundPump || thing.Type == TypePump
		}
		if !foundOil {
			t.Errorf("pool tile %d, %d lost its deposit card", tile[0], tile[1])
		}
		if foundPump {
			t.Errorf("pool tile %d, %d shows a pump from another cell",
				tile[0], tile[1])
		}
	}
	if things := thingsAt(s, pc, pr); len(things) != 1 ||
		things[0].Type != TypePump {
		t.Fatalf("the pump cell shows %v, want only the pump", things)
	}
	pumpPanel := tooltipLayout(s, camera, pc, pr, map[string]bool{})
	titles := 0
	for _, row := range pumpPanel.rows {
		if !row.title {
			continue
		}
		titles++
		if row.thing.Type != TypePump {
			t.Errorf("the pump cell also shows a %s card", row.thing.Type)
		}
	}
	if titles != 1 || pumpPanel.findButton(buttonLayPipe) == nil {
		t.Errorf("the pump cell has %d cards and pipe button %v",
			titles, pumpPanel.findButton(buttonLayPipe) != nil)
	}

	pcTileCol, pcTileRow := cellTile(pc, pr)
	pcTile := [2]int{pcTileCol, pcTileRow}
	var otherTile [2]int
	foundOtherTile := false
	for _, tile := range depositTiles(d) {
		if tile == pcTile {
			continue
		}
		otherTile = tile
		foundOtherTile = true
		break
	}
	if !foundOtherTile {
		t.Fatal("the pool has no tile away from its pump")
	}
	otherCol, otherRow := tileCell(otherTile[0], otherTile[1])
	otherPanel := tooltipLayout(s, camera, otherCol, otherRow, map[string]bool{})
	if otherPanel.findButton(buttonLayPipe) != nil {
		t.Error("another pool cell offers the pump's pipe")
	}
	if otherPanel.findButton(buttonSend) == nil {
		t.Error("another pool cell lost the deposit's send button")
	}

	site := newGame()
	site.Jobs = []Job{{Kind: BuildingPump, Col: pc, Row: pr}}
	if things := thingsAt(site, pc, pr); len(things) != 1 ||
		things[0].Type != TypeSite {
		t.Errorf("the pump site cell shows %v, want only the site", things)
	}
	if things := thingsAt(site, otherCol, otherRow); len(things) != 1 ||
		things[0].Type != TypeOil {
		t.Errorf("a different pool cell shows %v, want only its deposit", things)
	}

	worker := s.Robots[1]
	worker.X, worker.Y = cellCenterUnits(pc, pr)
	s.Robots[worker.ID] = worker
	scene := newPlayScene(s)
	gx, gy := projectBuilding(pump)
	across, height := buildingSize(BuildingPump)
	scale := buildingIcon(across, height, scene.zoom)
	point := scene.camera.ToScreen(golib.Vector2{
		X: gx,
		Y: gy - height*scale*unitH*0.5,
	})
	if !scene.pickPumpAt(point.X, point.Y) {
		t.Fatal("clicking the pump body didn't pick the pump")
	}
	if !scene.picked || scene.pickedCol != pc || scene.pickedRow != pr {
		t.Errorf("the pump body picked cell %d, %d, want %d, %d",
			scene.pickedCol, scene.pickedRow, pc, pr)
	}
	selected := tooltipLayoutForSelection(
		s, scene.camera, scene.pickedCol, scene.pickedRow,
		scene.expanded, scene.pickedThing,
	)
	selectedTitles := 0
	for _, row := range selected.rows {
		if !row.title {
			continue
		}
		selectedTitles++
		if row.thing.Type != TypePump {
			t.Errorf("clicking the raised pump shows a %s card", row.thing.Type)
		}
	}
	if selectedTitles != 1 {
		t.Errorf("clicking the raised pump shows %d cards, want only its card",
			selectedTitles)
	}

	Apply(s, LayPipe{From: pump.ID, To: 0})
	pumpPanel = tooltipLayout(s, camera, pc, pr, map[string]bool{})
	if pumpPanel.findButton(buttonRemovePipe) == nil {
		t.Error("a piped pump doesn't offer to remove its pipe")
	}
}

func TestRobotsLayAPipeASectionEachAndStandByIt(t *testing.T) {
	s := newGame()
	x, y := parkSlot(1)
	s.spawnRobot(RobotBuilder, x, y)
	workerID := addWorker(s)
	seedStock(s)
	arriveAll(s)
	pump := pumpOn(t, s, safePool(t))
	from, _ := pipeEndSpot(s, pump.ID)
	bends := []PipePoint{{from.X + 120, from.Y - 60}}
	Apply(s, LayPipe{From: pump.ID, To: coreTank, Bends: bends})
	p, _ := pipeOut(s, pump.ID)
	if p.Sections < 3 {
		t.Fatalf("the pipe has %d sections, want three or more to share", p.Sections)
	}
	// Each robot claims a section of its own, the free one nearest to it,
	// and the others know.
	runTicks(s, 1)
	first, second := s.Robots[1], s.Robots[2]
	if first.Pipe != p.ID || second.Pipe != p.ID {
		t.Fatalf("the robots claimed pipes %d and %d, want %d both",
			first.Pipe, second.Pipe, p.ID)
	}
	if first.Section == second.Section {
		t.Errorf("both robots claimed section %d, want one each", first.Section)
	}
	if worker := s.Robots[workerID]; worker.Pipe != 0 || worker.Section != 0 {
		t.Errorf("worker %d claimed pipe work: %+v", workerID, worker)
	}
	path, _ := pipeSpine(s, p)
	mine := pointGap(PipePoint{first.X, first.Y}, sectionSpot(path, first.Section))
	for i := int64(0); i < p.Sections; i++ {
		gap := pointGap(PipePoint{first.X, first.Y}, sectionSpot(path, i))
		if gap < mine-0.001 {
			t.Errorf("robot 1 claimed section %d, %v u away, with section %d at %v u",
				first.Section, mine, i, gap)
		}
	}
	// Whoever lays a section stands by it for as long as it takes.
	working := func() bool { return s.Pipes[p.ID].Left < p.Left }
	if !tickUntil(s, 60*120, working) {
		t.Fatal("nobody ever started laying")
	}
	var layer Robot
	for _, id := range sortedRobotIDs(s) {
		r := s.Robots[id]
		if sectionLeft(s.Pipes[p.ID], r.Section) < pipeSectionWorkTicks {
			layer = r
		}
	}
	runTicks(s, pipeSectionWorkTicks-2)
	if r := s.Robots[layer.ID]; r.X != layer.X || r.Y != layer.Y || r.Section != layer.Section {
		t.Errorf("robot %d moved on before its section was laid", layer.ID)
	}
	// A robot that goes takes its claim with it, and another lays that
	// section.
	delete(s.Robots, layer.ID)
	laid(t, s)
	for _, r := range s.Robots {
		if r.Pipe != 0 {
			t.Errorf("robot %d keeps a claim on a laid pipe", r.ID)
		}
	}
}

func TestAHalfLaidPipeFromAnOldSaveKeepsItsWork(t *testing.T) {
	p := Pipe{Sections: 4, Left: 4*pipeSectionWorkTicks - 150}
	want := []int64{0, pipeSectionWorkTicks - 30, pipeSectionWorkTicks, pipeSectionWorkTicks}
	for i, left := range want {
		if got := sectionLeft(p, int64(i)); got != left {
			t.Errorf("section %d asks %d ticks, want %d", i, got, left)
		}
	}
	p.Left = 0
	if sectionLeft(p, 2) != 0 {
		t.Error("a laid pipe still asks for work")
	}
}

func TestWritePipeFlowShotStates(t *testing.T) {
	path := os.Getenv("NIEBLA_PIPE_FLOW_SHOT_STATE")
	if path == "" {
		t.Skip("set NIEBLA_PIPE_FLOW_SHOT_STATE to write a shot state")
	}

	s := newGame()
	noRivals(s)
	seedStock(s)
	arriveAll(s)
	pump := pumpOn(t, s, safePool(t))
	silo := raised(t, s, BuildingSilo, pump.Col+10, pump.Row+4)
	coreCellCol, coreCellRow := tileCell(coreCol, coreRow)
	protector := raised(
		t, s, BuildingProtector, coreCellCol+20, coreCellRow,
	)
	pumpPipe := layNow(t, s, pump.ID, silo.ID)
	protectorPipe := layNow(t, s, coreTank, protector.ID)
	runTicks(s, 60*100)
	if s.Pipes[pumpPipe.ID].Flow <= 0 ||
		s.Pipes[protectorPipe.ID].Flow <= 0 {
		t.Fatal("a pipe in the shot state has no flow")
	}
	data, err := json.MarshalIndent(map[string]any{"state": s}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
