package main

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"golib"
)

// raised puts a finished building on a cell, skipping the robots' work,
// and returns it.
func raised(t *testing.T, s *State, kind BuildingKind, col, row int) Building {
	t.Helper()
	s.raise(kind, col, row)
	b, ok := buildingAt(s, col, row)
	if !ok {
		t.Fatalf("no %s stands on cell %d, %d", kind, col, row)
	}
	return b
}

// tickUntil runs the simulation until done says so, or the ticks run out.
func tickUntil(s *State, ticks int, done func() bool) bool {
	for i := 0; i < ticks; i++ {
		if done() {
			return true
		}
		Apply(s, Tick{})
	}
	return done()
}

// workOffDemolition works a building's demolition order off the way a
// builder standing at its side would, without letting the world tick in
// between: the tests that use it pin what a takedown leaves, not the
// walk up to it.
func workOffDemolition(s *State, id int64) {
	for i := 0; i < demolishWorkTicks; i++ {
		s.workDemolish(id)
	}
}

// demolishNow orders a building down and works the order off.
func demolishNow(t *testing.T, s *State, id int64) {
	t.Helper()
	Apply(s, Demolish{Building: id})
	workOffDemolition(s, id)
	if _, stands := s.Buildings[id]; stands {
		t.Fatalf("building %d still stands after its demolition", id)
	}
}

func TestDemolishingLeavesTheCostAsAPileAndTheRobotsHaulItHome(t *testing.T) {
	s := newGame()
	noRivals(s)
	col, row := groundNearCore()
	b := raised(t, s, BuildingCharger, col, row)
	before := s.Stock
	Apply(s, Demolish{Building: b.ID})
	if _, stands := buildingAt(s, col, row); !stands {
		t.Fatal("the charger fell the instant it was ordered")
	}
	if _, littered := pileAt(s, col, row); littered {
		t.Fatal("the ordered charger already left its pile")
	}
	if !tickUntil(s, 60*600, func() bool {
		_, stands := buildingAt(s, col, row)
		return !stands
	}) {
		t.Fatal("no builder walked over to take the charger down")
	}
	if s.Stock != before {
		t.Errorf("the stores went from %v to %v: a refund is a haul, never a transfer",
			before, s.Stock)
	}
	p, ok := pileAt(s, col, row)
	if !ok {
		t.Fatal("the demolition left no pile on its cell")
	}
	if p.Lilac != chargerCostLilac || p.Oil != chargerCostOil {
		t.Errorf("the pile holds %v kg and %v L, want the charger's whole cost",
			p.Lilac, p.Oil)
	}
	if canPlace(s, BuildingSilo, col, row) {
		t.Error("a cell with a pile on it took a marking")
	}
	if !tickUntil(s, 60*600, func() bool { return len(s.Piles) == 0 }) {
		t.Fatalf("the robots never cleared the pile: %v", s.Piles)
	}
	tickUntil(s, 60*60, func() bool {
		for _, r := range s.Robots {
			if r.Carry > 0 {
				return false
			}
		}
		return true
	})
	if got := s.Stock.Lilac - before.Lilac; math.Abs(got-chargerCostLilac) > 0.001 {
		t.Errorf("the stores gained %v kg of lilac, want %v", got, chargerCostLilac)
	}
	if got := s.Stock.Oil - before.Oil; math.Abs(got-chargerCostOil) > 0.001 {
		t.Errorf("the stores gained %v L of oil, want %v", got, chargerCostOil)
	}
	if !canPlace(s, BuildingSilo, col, row) {
		t.Error("the cleared cell still refuses a marking")
	}
}

func TestDemolishingAFactoryRefundsItsMechanicInProgress(t *testing.T) {
	s := newGame()
	s.Tech[techRepairID] = false
	s.Stock = Stock{Oil: 1000, Lilac: 2500}
	col, row := groundNearCore()
	home := raised(t, s, BuildingWarFactory, col, row)
	Apply(s, QueueMechanic{Building: home.ID})
	demolishNow(t, s, home.ID)
	pile, found := pileAt(s, col, row)
	if !found {
		t.Fatal("the demolished factory left no pile")
	}
	if want := warFactoryCostLilac + mechanicCostLilac; pile.Lilac != want {
		t.Errorf("the pile holds %v kg, want %v", pile.Lilac, want)
	}
	if want := warFactoryCostOil + mechanicCostOil; pile.Oil != want {
		t.Errorf("the pile holds %v L, want %v", pile.Oil, want)
	}
}

func TestDemolishingAFactoryCancelsItsRobot(t *testing.T) {
	s := newGame()
	col, row := groundNearCore()
	b := raised(t, s, BuildingFactory, col, row)
	Apply(s, QueueRobot{Building: b.ID})
	robots := len(s.Robots)
	demolishNow(t, s, b.ID)
	p, _ := pileAt(s, col, row)
	if p.Lilac != factoryCostLilac+robotCostLilac || p.Oil != robotCostOil {
		t.Errorf("the pile holds %v kg and %v L, want the factory's and its robot's cost",
			p.Lilac, p.Oil)
	}
	for i := 0; i < factoryRobotTicks+60; i++ {
		Apply(s, Tick{})
	}
	if len(s.Robots) != robots {
		t.Errorf("%d robots, want %d: the cancelled robot rolled out", len(s.Robots), robots)
	}
}

func TestDemolishingASiloSpillsWhatItsTankHeld(t *testing.T) {
	s := newGame()
	noRivals(s)
	col, row := groundNearCore()
	b := raised(t, s, BuildingSilo, col, row)
	s.Stock.Oil = coreOilCap
	b.Oil = 400
	s.Buildings[b.ID] = b
	demolishNow(t, s, b.ID)
	if s.Stock.Oil != coreOilCap {
		t.Errorf("the core holds %v L in a tank of %v", s.Stock.Oil, float64(coreOilCap))
	}
	p, _ := pileAt(s, col, row)
	if p.Oil != 400 || p.Lilac != siloCostLilac {
		t.Errorf("the pile holds %v L and %v kg, want the silo's oil and its cost",
			p.Oil, p.Lilac)
	}
	// The lilac goes home; the oil has no room and waits on the ground,
	// with nobody standing around with it in their arms.
	tickUntil(s, 60*600, func() bool {
		p, _ := pileAt(s, col, row)
		return p.Lilac == 0
	})
	for i := 0; i < 60*60; i++ {
		Apply(s, Tick{})
	}
	if p, _ := pileAt(s, col, row); p.Oil != 400 {
		t.Errorf("the pile holds %v L, want the 400 L the stores can't take", p.Oil)
	}
	for _, r := range s.Robots {
		if r.Cargo == TypeOil {
			t.Errorf("robot %d loaded oil the stores have no room for", r.ID)
		}
	}
	// Room made, the oil comes home too.
	s.Stock.Oil = 0
	if !tickUntil(s, 60*1200, func() bool { return len(s.Piles) == 0 }) {
		t.Errorf("the waiting oil never came home: %v", s.Piles)
	}
}

func TestCancellingASiteDropsItsCost(t *testing.T) {
	s := newGame()
	seedStock(s)
	arriveAll(s)
	col, row := groundNearCore()
	Apply(s, MarkBuilding{Kind: BuildingProtector, Col: col, Row: row})
	Apply(s, CancelJob{Col: col, Row: row})
	if len(s.Jobs) != 0 {
		t.Fatalf("%d jobs survive the cancelling", len(s.Jobs))
	}
	p, ok := pileAt(s, col, row)
	if !ok || p.Lilac != protectorCostLilac || p.Oil != protectorCostOil {
		t.Errorf("the cancelled site left %v, want the protector's cost", p)
	}
	Apply(s, CancelJob{Col: col + 1, Row: row})
	if len(s.Piles) != 1 {
		t.Errorf("cancelling a cell with no site changed the piles: %v", s.Piles)
	}
}

func TestAProtectorStaysWhileItAloneSheltersABuilding(t *testing.T) {
	s := newGame()
	col, row := groundInTheFog()
	protector := raised(t, s, BuildingProtector, col, row)
	charger := raised(t, s, BuildingCharger, col+2, row)
	Apply(s, Demolish{Building: protector.ID})
	if _, stands := s.Buildings[protector.ID]; !stands {
		t.Fatal("the protector took its order while it alone sheltered a charger")
	}
	demolishNow(t, s, charger.ID)
	demolishNow(t, s, protector.ID)
	if len(s.Buildings) != 0 {
		t.Errorf("%d buildings stand, want the outpost gone, protector last",
			len(s.Buildings))
	}
	// A second protector over the same charger frees the first.
	s = newGame()
	first := raised(t, s, BuildingProtector, col, row)
	raised(t, s, BuildingProtector, col+4, row)
	raised(t, s, BuildingCharger, col+2, row)
	demolishNow(t, s, first.ID)
	if _, stands := s.Buildings[first.ID]; stands {
		t.Error("a protector stayed though another bubble shelters the charger")
	}
}

func TestAnOrderedBuildingSaysDemolishingOnItsCard(t *testing.T) {
	s := newGame()
	col, row := groundNearCore()
	b := raised(t, s, BuildingSilo, col, row)
	Apply(s, Demolish{Building: b.ID})
	thing := buildingThing(s.Buildings[b.ID])
	if thing.Caption != "demolishing, 5 s" {
		t.Errorf("the card reads %q, want %q", thing.Caption, "demolishing, 5 s")
	}
	if trash, blocked := trashFor(s, thing); trash || blocked {
		t.Error("a building ordered down still wears a trash can")
	}
}

func TestAnOrderedProtectorWaitsWhileItAloneSheltersABuilding(t *testing.T) {
	s := newGame()
	col, row := groundInTheFog()
	protector := raised(t, s, BuildingProtector, col, row)
	Apply(s, Demolish{Building: protector.ID})
	// A building raised under its bubble alone holds the work up: the
	// fog's law can't be broken by a takedown already ordered.
	charger := raised(t, s, BuildingCharger, col+2, row)
	workOffDemolition(s, protector.ID)
	if _, stands := s.Buildings[protector.ID]; !stands {
		t.Fatal("the protector fell while it alone sheltered a charger")
	}
	if s.Buildings[protector.ID].Demolish != demolishWorkTicks {
		t.Error("the protector's work went in though its order can't finish")
	}
	demolishNow(t, s, charger.ID)
	workOffDemolition(s, protector.ID)
	if _, stands := s.Buildings[protector.ID]; stands {
		t.Error("the protector stands though nothing leans on its bubble")
	}
}

func TestALoadGoesToTheNearestStoreOfItsKind(t *testing.T) {
	s := newGame()
	// A warehouse and a pile side by side, far from the core's pole.
	col, row := 104+20, 96
	raised(t, s, BuildingWarehouse, col, row)
	s.dropPile(col+2, row, 0, 20)
	var r Robot
	for _, id := range sortedRobotIDs(s) {
		r = s.Robots[id]
		break
	}
	r.Carry, r.Cargo = 20, TypeLilac
	r.X, r.Y = cellCenterUnits(col+2, row)
	wx, wy := cellCenterUnits(col, row)
	x, y := storeSpot(s, r)
	if d := math.Hypot(x-wx, y-wy); d > storeStandoff+0.001 {
		t.Errorf("lilac next to a warehouse unloads %v u from it, want at its side", d)
	}
	r.Cargo = TypeOil
	cx, cy := tileCenterUnits(coreCol, coreRow)
	x, y = storeSpot(s, r)
	if d := math.Hypot(x-cx, y-cy); math.Abs(d-storeStandoff) > 0.001 {
		t.Errorf("oil with no silo unloads %v u from the core, want at its side", d)
	}
}

func TestPilesSurviveASaveAndOldSavesTakeThem(t *testing.T) {
	s := newGame()
	col, row := groundNearCore()
	s.dropPile(col, row, 40, 120)
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var back State
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if p, ok := pileAt(&back, col, row); !ok || p.Oil != 40 || p.Lilac != 120 {
		t.Errorf("the pile came back as %v", p)
	}
	old := newGame()
	old.Piles = nil // a save from before the piles
	old.dropPile(col, row, 0, 10)
	if _, ok := pileAt(old, col, row); !ok {
		t.Error("a state with no pile table couldn't take a pile")
	}
}

func TestCardsCarryTheirTrashCan(t *testing.T) {
	s := newGame()
	seedStock(s)
	arriveAll(s)
	camera := golib.NewCamera(float32(screenWidth), float32(screenHeight))
	col, row := groundNearCore()
	b := raised(t, s, BuildingSilo, col, row)
	Apply(s, MarkBuilding{Kind: BuildingCharger, Col: col + 1, Row: row})
	// The cell is the unit: the silo's panel is the silo's alone, and the
	// site beside it has its own.
	panel := tooltipLayout(s, camera, col, row, map[string]bool{})
	site := tooltipLayout(s, camera, col+1, row, map[string]bool{})
	trashOf := func(p *tooltip, kind ThingType) *tooltipRow {
		for i := range p.rows {
			if r := &p.rows[i]; r.trash && r.thing.Type == kind {
				return r
			}
		}
		return nil
	}
	can, siteCan := trashOf(&panel, TypeSilo), trashOf(&site, TypeSite)
	if can == nil || siteCan == nil {
		t.Fatalf("the silo's and the site's cards want a trash can each, got %v and %v",
			can, siteCan)
	}
	if trashOf(&panel, TypeSite) != nil {
		t.Error("the silo's cell shows the site on the cell beside it")
	}
	thing, blocked, ok := panel.trashAt(can.bx+can.bw/2, can.by+can.bh/2)
	if !ok || blocked || thing.Ref != b.ID {
		t.Errorf("the silo's trash can answered %v, %v, %v", thing, blocked, ok)
	}
	panel.arm(thing.ID)
	site.arm(thing.ID)
	if !can.armed || siteCan.armed {
		t.Error("arming the silo's card should arm it alone")
	}
	coreCellCol, coreCellRow := tileCell(coreCol, coreRow)
	core := tooltipLayout(s, camera, coreCellCol, coreCellRow, map[string]bool{})
	for _, r := range core.rows {
		if r.trash {
			t.Error("the core's card carries a trash can: it is indestructible both ways")
		}
	}
}

func TestALoadEndsItsWalkAtTheNearestStore(t *testing.T) {
	s := newGame()
	col, row := groundNearCore()
	warehouse := raised(t, s, BuildingWarehouse, col+8, row)
	silo := raised(t, s, BuildingSilo, col+8, row+2)
	cx, cy := tileCenterUnits(coreCol, coreRow)
	near := func(x, y, wantX, wantY float64) bool {
		return math.Abs(math.Hypot(x-wantX, y-wantY)-storeStandoff) < 0.001
	}
	for _, cargo := range []ThingType{TypeLilac, TypeOil} {
		store := warehouse
		if cargo == TypeOil {
			store = silo
		}
		sx, sy := cellCenterUnits(store.Col, store.Row)
		r := Robot{ID: 7, Carry: 10, Cargo: cargo, X: cx + 5, Y: cy}
		if x, y := storeSpot(s, r); !near(x, y, cx, cy) {
			t.Errorf("%s by the core unloads at %v, %v, want the core's side", cargo, x, y)
		}
		r.X, r.Y = sx+5, sy
		if x, y := storeSpot(s, r); !near(x, y, sx, sy) {
			t.Errorf("%s by its %s unloads at %v, %v, want its side", cargo, store.Kind, x, y)
		}
	}
}

// TestWriteDemolishShotState writes the region the demolition shot
// starts from: NIEBLA_DEMOLISH_SHOT_STATE for a silo standing near the
// core with one builder at the ranks. Skipped otherwise, the way tests
// write nothing.
func TestWriteDemolishShotState(t *testing.T) {
	path := os.Getenv("NIEBLA_DEMOLISH_SHOT_STATE")
	if path == "" {
		t.Skip("set NIEBLA_DEMOLISH_SHOT_STATE to write a demolition shot state")
	}
	s := newGame()
	noRivals(s)
	seedStock(s)
	col, row := groundNearCore()
	raised(t, s, BuildingSilo, col, row)

	camera := golib.NewCamera(float32(screenWidth), float32(screenHeight))
	camera.Bounds = regionOnScreen()
	camera.Zoom = zoomOfStop(zoomOut)
	camera.Snap()
	x, y := cellCenterUnits(col, row)
	px, py := project(float32(x), float32(y))
	point := camera.ToScreen(golib.Vector2{X: px, Y: py})
	t.Logf("silo cell click: %0.0f,%0.0f", point.X, point.Y)
	world := camera.ToWorld(point.X, point.Y)
	cellCol, cellRow, _ := cellAtWorld(float64(world.X), float64(world.Y))
	panel := tooltipLayout(s, camera, cellCol, cellRow, map[string]bool{})
	for _, r := range panel.rows {
		if r.trash {
			t.Logf("trash can click: %0.0f,%0.0f",
				r.bx+r.bw/2, r.by+r.bh/2)
		}
	}

	data, err := json.MarshalIndent(map[string]any{"state": s}, "", "  ")
	if err != nil {
		t.Fatalf("the state doesn't marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
