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

// arriveAll grants every drop of the ladder, opened, so a test can
// build what it needs without waiting for the clock or the rivals.
func arriveAll(s *State) {
	s.Tech = map[string]bool{}
	for i := range techLadder {
		s.Tech[techLadder[i].id] = true
	}
}

func TestFactorySchematicsStartAtTheCoreAndDeliveryBringsInfrastructure(t *testing.T) {
	s := newGame()
	if techPending(s) != techIndustryID {
		t.Fatalf("the factory schematics are %q, want them at the core",
			techPending(s))
	}
	for _, kind := range (kinds{
		BuildingSilo, BuildingWarehouse, BuildingCharger, BuildingGuard,
		BuildingProtector, BuildingPump,
		BuildingWarFactory, BuildingArtillery,
	}) {
		if kindUnlocked(s, kind) {
			t.Fatalf("a new game can raise %s", kind)
		}
	}
	if !kindUnlocked(s, BuildingFactory) {
		t.Fatal("the factory is not available with the opening builder")
	}
	if repairProtocolUnlocked(s) {
		t.Fatal("a new colony received the repair protocol at the start")
	}
	Apply(s, AckTech{ID: techIndustryID})
	col, row, _ := nearestTileOf(kindOil)
	Apply(s, AssignRobot{ID: 1, Col: col, Row: row})
	if !tickUntil(s, 60*120, func() bool { return s.Deliveries > 0 }) {
		t.Fatal("the builder never delivered its first load")
	}
	if techPending(s) != techInfraID {
		t.Fatalf("the first delivery brought %q, want %q",
			techPending(s), techInfraID)
	}
	for _, kind := range (kinds{
		BuildingSilo, BuildingWarehouse, BuildingCharger,
	}) {
		if !kindUnlocked(s, kind) {
			t.Errorf("the first delivery did not unlock %s", kind)
		}
	}
	if hasBuiltWorker(s) {
		t.Fatal("a new colony already has a factory-built worker")
	}
	for _, kind := range (kinds{
		BuildingGuard, BuildingProtector, BuildingPump,
		BuildingWarFactory, BuildingArtillery,
	}) {
		if kindUnlocked(s, kind) {
			t.Errorf("the first delivery also unlocked %s", kind)
		}
	}
}

// kinds is a list of building kinds, so a test can loop over one.
type kinds []BuildingKind

func TestFirstDeliveryBringsInfrastructureBeforeTheScout(t *testing.T) {
	s := newGame()
	workerID := addWorker(s)
	oilCol, oilRow, ok := nearestTileOf(kindOil)
	if !ok {
		t.Fatal("the starting region has no oil pool")
	}
	lilacCol, lilacRow, ok := nearestTileOf(kindLilac)
	if !ok {
		t.Fatal("the starting region has no lilac vein")
	}
	Apply(s, AssignRobot{ID: 1, Col: oilCol, Row: oilRow})
	Apply(s, AssignRobot{ID: workerID, Col: lilacCol, Row: lilacRow})
	factoryCol, factoryRow := groundNearCore()
	Apply(s, AckTech{ID: techIndustryID})
	Apply(s, MarkBuilding{
		Kind: BuildingFactory, Col: factoryCol, Row: factoryRow,
	})

	var deliveryAt, factoryAt, workerAt, infrastructureAt int64
	factoryMarked, workerQueued := len(s.Jobs) > 0, false
	for s.Ticks < raidFirstScoutTicks {
		if s.Deliveries > 0 && deliveryAt == 0 {
			deliveryAt = s.Ticks
		}
		if factoryMarked && factoryAt == 0 {
			factoryAt = s.Ticks
		}
		if factoryMarked && !workerQueued {
			if factory, raised := buildingAt(s, factoryCol, factoryRow); raised {
				Apply(s, QueueRobot{Building: factory.ID})
				if s.Buildings[factory.ID].Work > 0 {
					workerQueued = true
				}
			}
		}

		Apply(s, Tick{})
		if hasBuiltWorker(s) && workerAt == 0 {
			workerAt = s.Ticks
		}
		if kindUnlocked(s, BuildingCharger) && infrastructureAt == 0 {
			infrastructureAt = s.Ticks
		}
		if len(s.Enemies) > 0 {
			break
		}
	}

	if deliveryAt == 0 {
		t.Fatal("the opening plan made no first delivery")
	}
	if factoryAt == 0 {
		t.Fatal("the opening plan did not mark the opening factory")
	}
	if !workerQueued {
		t.Fatal("the opening plan could not order a factory worker")
	}
	if workerAt == 0 {
		t.Fatal("the factory did not finish its first worker")
	}
	if infrastructureAt == 0 {
		t.Fatal("the first factory worker did not unlock infrastructure")
	}
	if infrastructureAt >= raidFirstScoutTicks {
		t.Fatalf("infrastructure arrived at tick %d, not before the scout at %d",
			infrastructureAt, raidFirstScoutTicks)
	}
	t.Logf(
		"opening milestones (seconds): delivery %.1f, factory %.1f, "+
			"worker %.1f, infrastructure %.1f, scout %.1f",
		float64(deliveryAt)/60,
		float64(factoryAt)/60,
		float64(workerAt)/60,
		float64(infrastructureAt)/60,
		float64(raidFirstScoutTicks)/60,
	)
}

func TestOpeningCalloutsExplainTheNextStep(t *testing.T) {
	title, body := techWords(techIndustryID)
	if title != "robot factory" ||
		body != "The builder is a gift from the core. Build this factory to "+
			"choose builders or workers." {
		t.Fatalf("factory callout is %q, %q", title, body)
	}
	title, body = techWords(techInfraID)
	if title != "infrastructure" ||
		body != "Your first delivered load brings silos, warehouses and "+
			"chargers." {
		t.Fatalf("infrastructure callout is %q, %q", title, body)
	}
	title, body = techWords(techRepairID)
	wantRepairBody := "Rival fire damaged a building. Build a mechanic at a " +
		"war factory to repair it."
	if title != "repair protocol" ||
		body != wantRepairBody {
		t.Fatalf("repair callout is %q, %q", title, body)
	}
	title, body = techWords(techGuardID)
	wantGuardBody := "The scout has left the core's clear circle. Build a " +
		"guard post before its next visit."
	if title != "guard post" || body != wantGuardBody {
		t.Fatalf("guard callout is %q, %q", title, body)
	}
	title, body = techWords(techArtilleryID)
	wantArtilleryBody := "Three normal attacks have ended. Build artillery " +
		"to shell rival targets the colony sees."
	if title != "artillery" || body != wantArtilleryBody {
		t.Fatalf("artillery callout is %q, %q", title, body)
	}
}

func TestGuardUnlockWaitsForTheFirstScoutToLeaveTheCoreBubble(t *testing.T) {
	s := newGame()
	Apply(s, AckTech{ID: techIndustryID})
	if kindUnlocked(s, BuildingGuard) {
		t.Fatal("the guard post stood ready before anyone came")
	}
	cx, cy := tileCenterUnits(coreCol, coreRow)
	radius := coreBubbleRadius * unitsPerTile
	partyID, scoutID := int64(10), int64(11)
	s.Parties[partyID] = Party{
		ID: partyID, Stage: StageRaid, Siphon: 0,
		EntryX: cx + radius + 200, EntryY: cy,
	}
	s.Enemies[scoutID] = Enemy{
		ID: scoutID, Party: partyID, Kind: EnemyScout,
		X: cx + radius - 300, Y: cy, Oil: 3,
	}
	Apply(s, Tick{})
	if s.Parties[partyID].Stage != StageLeave || len(s.Marks) == 0 {
		t.Fatal("the scout did not paint its mark and begin leaving")
	}
	if kindUnlocked(s, BuildingGuard) {
		t.Fatal("the scout's oil and mark unlocked the guard inside the bubble")
	}

	crossed := false
	for tick := 0; tick < 1800; tick++ {
		before := s.Enemies[scoutID]
		Apply(s, Tick{})
		after, alive := s.Enemies[scoutID]
		if !alive {
			t.Fatal("the scout reached its entry before crossing the bubble")
		}
		gap := math.Hypot(after.X-cx, after.Y-cy)
		if gap <= radius {
			if kindUnlocked(s, BuildingGuard) {
				t.Fatalf("the guard arrived while the scout was %.1f m inside", gap)
			}
			continue
		}
		if !crossedCoreBubble(before.X, before.Y, after.X, after.Y) {
			t.Fatalf("the scout moved outside without crossing from inside: %+v", after)
		}
		if !s.Raids.ScoutClearedCore || !kindUnlocked(s, BuildingGuard) {
			t.Fatal("the scout's outward crossing did not unlock the guard post")
		}
		if s.Raids.ScoutClearedCoreAt != s.Ticks {
			t.Fatalf("the crossing tick is %d, want %d",
				s.Raids.ScoutClearedCoreAt, s.Ticks)
		}
		if techPending(s) != techGuardID {
			t.Fatalf("the crossing's pending schematic is %q", techPending(s))
		}
		crossed = true
		break
	}
	if !crossed {
		t.Fatal("the first scout never crossed out of the core bubble")
	}
	if kindUnlocked(s, BuildingProtector) || kindUnlocked(s, BuildingWarFactory) {
		t.Error("the scout's crossing unlocked more than the guard post")
	}

	if opened, arrived := s.Tech[techGuardID]; !arrived || opened {
		t.Fatalf("guard schematic ledger entry is opened=%v arrived=%v",
			opened, arrived)
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var loaded State
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	loaded.enterRegion()
	if !loaded.Raids.ScoutClearedCore ||
		loaded.Raids.ScoutClearedCoreAt != s.Raids.ScoutClearedCoreAt ||
		!kindUnlocked(&loaded, BuildingGuard) ||
		techPending(&loaded) != techGuardID {
		t.Fatalf("the scout crossing or pending guard badge was lost: %+v",
			loaded.Raids)
	}
	if !reflect.DeepEqual(loaded.Tech, s.Tech) {
		t.Errorf("the tech ledger changed across save/load: %v vs %v",
			loaded.Tech, s.Tech)
	}
	noRivals(&loaded)
	runTicks(&loaded, techFrontierDelayTicks-1)
	if kindUnlocked(&loaded, BuildingProtector) ||
		kindUnlocked(&loaded, BuildingPump) {
		t.Fatal("the frontier kit arrived before one minute after the crossing")
	}
	runTicks(&loaded, 1)
	if !kindUnlocked(&loaded, BuildingProtector) ||
		!kindUnlocked(&loaded, BuildingPump) {
		t.Fatal("the saved crossing did not time the frontier kit")
	}
}

func TestOnlyTheFirstScoutCrossingClearsTheCore(t *testing.T) {
	s := newGame()
	cx, cy := tileCenterUnits(coreCol, coreRow)
	radius := coreBubbleRadius * unitsPerTile
	partyID, scoutID := int64(10), int64(11)
	s.Parties[partyID] = Party{
		ID: partyID, Stage: StageLeave,
		EntryX: cx + radius + 200, EntryY: cy,
	}
	s.Enemies[scoutID] = Enemy{
		ID: scoutID, Party: partyID, Kind: EnemyScout,
		X: cx + radius - 1, Y: cy,
	}
	s.Raids.Visits = 1

	for i := 0; i < 10; i++ {
		Apply(s, Tick{})
	}
	if s.Raids.ScoutClearedCore || kindUnlocked(s, BuildingGuard) {
		t.Fatal("a later scout cleared the first visit's guard trigger")
	}
}

func TestCoreBubbleCrossingRequiresMovingOutwardPastItsBoundary(t *testing.T) {
	cx, cy := tileCenterUnits(coreCol, coreRow)
	radius := coreBubbleRadius * unitsPerTile

	if crossedCoreBubble(cx+radius-1, cy, cx+radius, cy) {
		t.Fatal("reaching the boundary counted as leaving the bubble")
	}
	if crossedCoreBubble(cx+radius, cy, cx+radius-1, cy) {
		t.Fatal("moving inward from the boundary counted as leaving")
	}
	if !crossedCoreBubble(cx+radius, cy, cx+radius+1, cy) {
		t.Fatal("moving outward past the boundary was not a crossing")
	}
}

func TestFrontierKitArrivesOneMinuteAfterTheScoutCrosses(t *testing.T) {
	s := newGame()
	noRivals(s)
	s.Raids.ScoutClearedCore = true
	s.Raids.ScoutClearedCoreAt = 120
	s.Ticks = s.Raids.ScoutClearedCoreAt
	runTicks(s, techFrontierDelayTicks-1)
	if kindUnlocked(s, BuildingProtector) || kindUnlocked(s, BuildingPump) {
		t.Fatal("the frontier kit arrived before one minute had passed")
	}
	runTicks(s, 1)
	if !kindUnlocked(s, BuildingProtector) || !kindUnlocked(s, BuildingPump) {
		t.Fatal("the frontier kit did not arrive one minute after the crossing")
	}
}

func TestFrontierKitDoesNotArriveOnItsFormerClockInNewSaves(t *testing.T) {
	s := newGame()
	noRivals(s)
	runTicks(s, legacyTechFrontierTicks)
	if kindUnlocked(s, BuildingProtector) || kindUnlocked(s, BuildingPump) {
		t.Fatal("the new frontier kit still arrived on the 5:30 clock")
	}
}

func TestOldFrontierClockStaysOnMigratedSaves(t *testing.T) {
	s := newGame()
	s.Version = 8
	s.Tech = map[string]bool{techIndustryID: true}
	s.Ticks = legacyTechFrontierTicks - 2
	s.enterRegion()
	if !s.Raids.LegacyFrontierClock {
		t.Fatal("an old save lost its frontier clock")
	}
	runTicks(s, 1)
	if kindUnlocked(s, BuildingProtector) || kindUnlocked(s, BuildingPump) {
		t.Fatal("the migrated frontier kit arrived before 5:30")
	}
	runTicks(s, 1)
	if !kindUnlocked(s, BuildingProtector) || !kindUnlocked(s, BuildingPump) {
		t.Fatal("the migrated frontier kit did not arrive at 5:30")
	}
}

func TestMobileUnitsUnlockAtTheFirstCityAndKeepTheOldSaveTrigger(t *testing.T) {
	s := newGame()
	s.Raids.Visits = 1
	runTicks(s, 1)
	if kindUnlocked(s, BuildingWarFactory) {
		t.Fatal("the war factory arrived before the pressure city")
	}
	s.Cities[9] = City{ID: 9}
	s.Raids.PressureCity = 9
	runTicks(s, 1)
	if !kindUnlocked(s, BuildingWarFactory) {
		t.Fatal("the war factory did not arrive with the pressure city")
	}
	legacy := newGame()
	legacy.Raids.Visits = 2
	runTicks(legacy, 1)
	if !kindUnlocked(legacy, BuildingWarFactory) {
		t.Fatal("an old save did not keep its completed-visit trigger")
	}
}

func TestAMarkedBlueprintRefusesWhatIsLocked(t *testing.T) {
	s := newGame()
	seedStock(s)
	col, row := groundNearCore()
	Apply(s, MarkBuilding{Kind: BuildingSilo, Col: col, Row: row})
	if len(s.Jobs) != 0 {
		t.Fatalf("a locked silo was marked: %d jobs", len(s.Jobs))
	}
	if s.Stock.Lilac != 1200 {
		t.Fatalf("a locked silo was paid for: %v lilac left", s.Stock.Lilac)
	}
	arriveAll(s)
	Apply(s, MarkBuilding{Kind: BuildingSilo, Col: col, Row: row})
	if len(s.Jobs) != 1 || s.Jobs[0].Kind != BuildingSilo {
		t.Fatalf("an arrived silo was not marked: %d jobs", len(s.Jobs))
	}
}

func TestAPipeWaitsForTheFrontierKit(t *testing.T) {
	s := newGame()
	seedStock(s)
	Apply(s, LayPipe{From: coreTank, To: 0})
	if len(s.Pipes) != 0 {
		t.Fatalf("a pipe was marked with no frontier kit: %d pipes", len(s.Pipes))
	}
	arriveAll(s)
	col, row := groundNearCore()
	silo := raised(t, s, BuildingSilo, col, row)
	Apply(s, LayPipe{From: coreTank, To: silo.ID})
	if len(s.Pipes) != 1 {
		t.Fatalf("a pipe was refused once the frontier kit came: %d pipes",
			len(s.Pipes))
	}
}

func TestTheBadgeOpensAndAStaleAckDoesNothing(t *testing.T) {
	s := newGame()
	if techPending(s) != techIndustryID {
		t.Fatal("the opening factory badge is missing")
	}
	Apply(s, AckTech{ID: techIndustryID})
	Apply(s, DevNextTech{})
	runTicks(s, 1)
	if got := techPending(s); got != techInfraID {
		t.Fatalf("pending %q, want %q", got, techInfraID)
	}
	Apply(s, AckTech{ID: techArtilleryID})
	if _, ok := s.Tech[techArtilleryID]; ok {
		t.Fatal("an ack opened schematics that never arrived")
	}
	Apply(s, AckTech{ID: techInfraID})
	if opened := s.Tech[techInfraID]; !opened {
		t.Fatal("the ack did not open the drop")
	}
	if techPending(s) != "" {
		t.Fatalf("the badge still waits after its ack: %q", techPending(s))
	}
}

func TestDismissingTechCalloutPassesOutsideClicksThrough(t *testing.T) {
	s := newPlayScene(newGame())
	s.techCallout = techArtilleryID
	box := techCalloutBounds(s)
	inside := box.Center()
	if !s.dismissTechCallout(inside.X, inside.Y) {
		t.Fatal("a click on the callout was not consumed")
	}
	if s.techCallout != "" {
		t.Fatal("a click on the callout did not close it")
	}

	s.techCallout = techArtilleryID
	outsideX := box.X + box.Width + 1
	if s.dismissTechCallout(outsideX, inside.Y) {
		t.Fatal("a click outside the callout was consumed")
	}
	if s.techCallout != "" {
		t.Fatal("a click outside the callout did not close it")
	}
}

func TestTechBlueprintsCanBeSelectedOnceFromTheirCallout(t *testing.T) {
	s := newPlayScene(newGame())
	s.techCallout = techInfraID
	s.techUsed = map[BuildingKind]bool{}
	squares := techSquareLayout(s, techInfraID)
	if len(squares) != 3 {
		t.Fatalf("the infrastructure callout has %d squares, want 3",
			len(squares))
	}
	center := squares[0].area.Center()
	square, hit := techSquareAt(s, center.X, center.Y)
	if !hit || square.item.kind != BuildingSilo {
		t.Fatalf("the first square hit %v, want the silo", square)
	}
	if !s.selectTechBuilding(square.item.kind) ||
		s.techPlacing != BuildingSilo {
		t.Fatal("clicking the silo square did not arm its placement")
	}
	s.techPlacing = ""
	s.techUsed[BuildingSilo] = true
	square, hit = techSquareAt(s, center.X, center.Y)
	if !hit || !square.used {
		t.Fatal("the placed silo square does not read as used")
	}
	if s.selectTechBuilding(BuildingSilo) {
		t.Fatal("a used blueprint can be selected again")
	}
	if !techBuildingsRemain(techInfraID, s.techUsed) {
		t.Fatal("the callout ran out of buildings after one of three")
	}
	s.techUsed[BuildingWarehouse] = true
	s.techUsed[BuildingCharger] = true
	if techBuildingsRemain(techInfraID, s.techUsed) {
		t.Fatal("the callout still has a building after all three were used")
	}
	if techBuildingsRemain(techFrontierID, map[BuildingKind]bool{
		BuildingProtector: true,
		BuildingPump:      true,
	}) {
		t.Fatal("the informational pipe square kept the callout open")
	}
}

func TestTechPumpPlacementSnapsToItsPoolAndClosesAfterTheLastBuilding(t *testing.T) {
	s := newPlayScene(newGame())
	seedStock(s.state)
	arriveAll(s.state)
	s.techCallout = techFrontierID
	s.techUsed = map[BuildingKind]bool{}
	s.techPlacing = BuildingPump
	d := safePool(t)
	tcol, trow := cellTile(d.HeartCol, d.HeartRow)
	s.hoverCell = true
	s.hoverCellCol, s.hoverCellRow = tileCell(tcol, trow)
	col, row, inside := s.techPlacementCell()
	if !inside || col != d.HeartCol || row != d.HeartRow {
		t.Fatalf("pump preview targets %d,%d, want pool heart %d,%d",
			col, row, d.HeartCol, d.HeartRow)
	}
	if !techPlacementValid(s.state, BuildingPump, col, row) {
		t.Fatal("the safe oil pool cannot take its pump")
	}
	if !s.placeTechBuilding() {
		t.Fatal("the pump was not marked on its pool")
	}
	if len(s.state.Jobs) != 1 || s.state.Jobs[0].Kind != BuildingPump ||
		s.state.Jobs[0].Col != d.HeartCol ||
		s.state.Jobs[0].Row != d.HeartRow {
		t.Fatalf("pump job is %+v, want the pool heart", s.state.Jobs)
	}
	if !s.techUsed[BuildingPump] || s.techCallout != techFrontierID {
		t.Fatal("using the pump did not disable it and leave the protector offer")
	}
	if s.selectTechBuilding(BuildingPump) {
		t.Fatal("the used pump can be selected again")
	}
	if !s.selectTechBuilding(BuildingProtector) {
		t.Fatal("the unused protector could not be selected")
	}
	s.hoverCellCol, s.hoverCellRow = groundInTheFog()
	if !s.placeTechBuilding() {
		t.Fatal("the protector was not marked on valid ground")
	}
	if s.techCallout != "" || s.techPlacing != "" || s.techUsed != nil {
		t.Fatal("the callout stayed open after its last building was used")
	}
}

func TestInvalidTechPlacementKeepsTheBlueprintAndResources(t *testing.T) {
	s := newPlayScene(newGame())
	seedStock(s.state)
	arriveAll(s.state)
	s.techCallout = techIndustryID
	s.techUsed = map[BuildingKind]bool{}
	if !s.selectTechBuilding(BuildingFactory) {
		t.Fatal("the factory could not be selected")
	}
	s.hoverCell = true
	s.hoverCellCol, s.hoverCellRow = groundInTheFog()
	stock := s.state.Stock
	if s.placeTechBuilding() {
		t.Fatal("a factory was marked outside every bubble")
	}
	if len(s.state.Jobs) != 0 || s.state.Stock != stock {
		t.Fatal("an invalid placement changed jobs or stores")
	}
	if s.techPlacing != BuildingFactory || s.techUsed[BuildingFactory] {
		t.Fatal("an invalid placement consumed the factory offer")
	}
	s.state.Stock.Lilac = 0
	s.hoverCellCol, s.hoverCellRow = groundNearCore()
	col, row, _ := s.techPlacementCell()
	if techPlacementValid(s.state, BuildingFactory, col, row) {
		t.Fatal("an unaffordable factory preview reads as placeable")
	}
	if s.placeTechBuilding() || len(s.state.Jobs) != 0 {
		t.Fatal("an unaffordable factory was marked")
	}
	seedStock(s.state)
	x, y := cellCenterUnits(col, row)
	robot := s.state.Robots[1]
	robot.X, robot.Y = x, y
	s.state.Robots[robot.ID] = robot
	if techPlacementValid(s.state, BuildingFactory, col, row) {
		t.Fatal("a factory preview reads as placeable under a robot")
	}
	if s.placeTechBuilding() || len(s.state.Jobs) != 0 {
		t.Fatal("a factory was marked under a robot")
	}
}

func TestEveryDropSquaresWhatItBrings(t *testing.T) {
	want := map[string][]techItem{
		techInfraID: {
			{name: "Oil silo", kind: BuildingSilo},
			{name: "Mineral warehouse", kind: BuildingWarehouse},
			{name: "Robot charger", kind: BuildingCharger},
		},
		techGuardID: {
			{name: "Guard post", kind: BuildingGuard},
		},
		techFrontierID: {
			{name: "Shadow protector", kind: BuildingProtector},
			{name: "Oil pump", kind: BuildingPump},
			{name: "Pipes", pipes: true, informational: true},
		},
		techIndustryID: {
			{name: "Robot factory", kind: BuildingFactory},
		},
		techMobileID: {
			{name: "War factory", kind: BuildingWarFactory},
		},
		techArtilleryID: {
			{name: "Artillery", kind: BuildingArtillery},
		},
		techRepairID: {
			{
				name: "Mechanic", robot: RobotRepair,
				informational: true,
			},
		},
	}
	if len(want) != len(techLadder) {
		t.Fatalf("%d drops are squared about, the ladder has %d",
			len(want), len(techLadder))
	}
	for i := range techLadder {
		id := techLadder[i].id
		got := techBrings(id)
		if !reflect.DeepEqual(got, want[id]) {
			t.Errorf("drop %q brings %v, want %v", id, got, want[id])
		}
	}
	if items := techBrings("no such drop"); items != nil {
		t.Errorf("an unknown drop brings %v, want nothing", items)
	}
}

func TestTechCalloutHitboxCoversItsSquares(t *testing.T) {
	s := newPlayScene(newGame())
	s.techCallout = techInfraID
	box := techCalloutBounds(s)
	want := float32(2*techCalloutPad + techCalloutHead +
		techCalloutMaxBodyRows*techCalloutRow +
		techSquareGap + techSquareH)
	if box.Height != want {
		t.Fatalf("the hitbox is %v tall, want %v", box.Height, want)
	}
	bottomX, bottomY := box.X+4, box.Y+box.Height-1
	if !s.dismissTechCallout(bottomX, bottomY) {
		t.Fatal("a click on the squares' last row was not consumed")
	}
	if s.techCallout != "" {
		t.Fatal("a click on the squares did not close the callout")
	}
	s.techCallout = techInfraID
	if s.dismissTechCallout(bottomX, bottomY+1) {
		t.Fatal("a click under the callout was consumed")
	}
}

func TestRepairCalloutMechanicSquareIsInformational(t *testing.T) {
	s := newPlayScene(newGame())
	s.techCallout = techRepairID
	layout := techSquareLayout(s, techRepairID)
	if len(layout) != 1 {
		t.Fatalf("the repair callout has %d squares, want one", len(layout))
	}
	area := layout[0].area
	square, found := techSquareAt(s,
		area.X+area.Width/2, area.Y+area.Height/2)
	if !found || !square.item.informational ||
		square.item.robot != RobotRepair {
		t.Fatalf("the repair square is %+v, found %v", square.item, found)
	}
	if s.selectTechBuilding(square.item.kind) || s.techPlacing != "" {
		t.Fatal("the mechanic square armed building placement")
	}
	if techBuildingsRemain(techRepairID, map[BuildingKind]bool{}) {
		t.Fatal("the informational mechanic square counts as an unused building")
	}
}

func TestASaveFromBeforeTheSchematicsOpensWhatItEarned(t *testing.T) {
	s := newGame()
	s.Deliveries = 3
	s.Ticks = legacyTechIndustryTicks + 60
	s.Raids.Visits = 2
	s.Tech = nil
	Apply(s, Tick{})
	for _, id := range []string{
		techIndustryID, techInfraID, techFrontierID, techMobileID,
	} {
		if s.Tech[id] != true {
			t.Errorf("the old save's %s is %v, want opened", id, s.Tech[id])
		}
	}
	if _, ok := s.Tech[techGuardID]; ok {
		t.Error("the guard post came to an old save with nobody driving in")
	}
	if _, ok := s.Tech[techArtilleryID]; ok {
		t.Error("artillery came to an old save with no base")
	}
	if _, ok := s.Tech[techRepairID]; ok {
		t.Error("repair schematics came to an old save with no building hit")
	}
	// No badge waits: this legacy save has no earned guard trigger.
	if techPending(s) != "" {
		t.Errorf("an old save owes a click: pending %q", techPending(s))
	}
	s.Marks[1] = Mark{ID: 1, X: 100, Y: 100}
	Apply(s, Tick{})
	if kindUnlocked(s, BuildingGuard) || techPending(s) != "" {
		t.Error("a mark unlocked the guard post in a migrated current save")
	}
}

func TestOldGuardUnlocksMigrateWithoutNewBadges(t *testing.T) {
	for _, hasLedger := range []bool{false, true} {
		for _, trigger := range []string{"mark", "drinking"} {
			t.Run(fmt.Sprintf("ledger=%v/%s", hasLedger, trigger),
				func(t *testing.T) {
					s := newGame()
					s.Version = 5
					s.Ticks = 2
					if hasLedger {
						s.Tech = map[string]bool{techIndustryID: true}
					} else {
						s.Tech = nil
					}
					if trigger == "mark" {
						s.Marks[1] = Mark{ID: 1, X: 100, Y: 100}
					} else {
						s.Enemies[2] = Enemy{
							ID: 2, Party: 1, Kind: EnemyScout, Oil: 3,
						}
					}

					s.migrateState()
					stepTech(s)
					if opened, arrived := s.Tech[techGuardID]; !arrived || !opened {
						t.Fatalf(
							"legacy %s trigger migrated as opened=%v arrived=%v",
							trigger, opened, arrived,
						)
					}
					if techPending(s) == techGuardID {
						t.Fatal("migration created a new guard badge")
					}
				})
		}
	}
}

func TestOldPendingGuardBadgeStaysPendingAfterMigration(t *testing.T) {
	s := newGame()
	s.Version = 5
	s.Tech = map[string]bool{
		techIndustryID: true,
		techGuardID:    false,
	}
	s.Marks[1] = Mark{ID: 1, X: 100, Y: 100}

	s.migrateState()
	if opened, arrived := s.Tech[techGuardID]; !arrived || opened {
		t.Fatalf("the old guard badge migrated as opened=%v arrived=%v",
			opened, arrived)
	}
	if techPending(s) != techGuardID {
		t.Fatalf("the old guard badge is %q, want %q",
			techPending(s), techGuardID)
	}
}

func TestAnExistingInfrastructureUnlockIsNotRevoked(t *testing.T) {
	for _, opened := range []bool{false, true} {
		s := newGame()
		s.Tech = map[string]bool{
			techIndustryID: true,
			techInfraID:    opened,
		}
		Apply(s, Tick{})
		if !kindUnlocked(s, BuildingCharger) {
			t.Errorf(
				"opened=%v: an existing infrastructure unlock was revoked",
				opened,
			)
		}
		if s.Tech[techInfraID] != opened {
			t.Errorf("opened=%v: save changed infrastructure to %v", opened,
				s.Tech[techInfraID])
		}
		if !opened && techPending(s) != techInfraID {
			t.Errorf("the unacknowledged infrastructure badge became %q",
				techPending(s))
		}
	}
}

func TestDevNextTechBringsTheLadderInOrder(t *testing.T) {
	s := newGame()
	Apply(s, AckTech{ID: techIndustryID})
	for i := 1; i < len(techLadder); i++ {
		Apply(s, DevNextTech{})
		runTicks(s, 1)
		if got := techPending(s); got != techLadder[i].id {
			t.Fatalf("drop %d: pending %q, want %q", i, got, techLadder[i].id)
		}
		Apply(s, AckTech{ID: techLadder[i].id})
		if techPending(s) != "" {
			t.Fatalf("drop %d: the badge waits after its ack", i)
		}
	}
	Apply(s, DevNextTech{})
	for i := range techLadder {
		if _, ok := s.Tech[techLadder[i].id]; !ok {
			t.Errorf("drop %d never arrived through the dev tool", i)
		}
	}
}

func TestCardButtonsWaitForTheirSchematics(t *testing.T) {
	s := newGame()
	seedStock(s)
	camera := golib.NewCamera(float32(screenWidth), float32(screenHeight))
	col, row := groundNearCore()
	raised(t, s, BuildingSilo, col, row)
	// The silo's lay pipe waits for the frontier kit, the pool's build
	// pump for its own drop; neither is offered before theirs arrives.
	panel := tooltipLayout(s, camera, col, row, map[string]bool{})
	if panel.findButton(buttonLayPipe) != nil {
		t.Error("lay pipe is offered with no frontier kit")
	}
	d := safePool(t)
	dc, dr := tileCell(d.Col, d.Row)
	panel = tooltipLayout(s, camera, dc, dr, map[string]bool{})
	if panel.findButton(buttonBuildPump) != nil {
		t.Error("build pump is offered with no schematics")
	}
	arriveAll(s)
	panel = tooltipLayout(s, camera, col, row, map[string]bool{})
	if panel.findButton(buttonLayPipe) == nil {
		t.Error("lay pipe is still missing once the frontier kit arrived")
	}
	// An unlocked pump stays visible while its cost is out of reach.
	s.Stock.Lilac = 0
	panel = tooltipLayout(s, camera, dc, dr, map[string]bool{})
	pump := panel.findButton(buttonBuildPump)
	if pump == nil || !pump.disabled {
		t.Fatal("the unaffordable pump isn't visible and disabled")
	}
	if len(pump.costs) != 1 || !pump.costs[0].missing {
		t.Fatal("the pump cost doesn't mark its missing lilac")
	}
	if panel.buttonRowAt(
		pump.bx+pump.bw/2,
		pump.by+pump.bh/2,
	) != nil {
		t.Fatal("the unaffordable pump button still accepts clicks")
	}
	s.Stock.Lilac = pumpCostLilac
	panel = tooltipLayout(s, camera, dc, dr, map[string]bool{})
	pump = panel.findButton(buttonBuildPump)
	if pump == nil || pump.disabled || len(pump.costs) != 1 ||
		pump.costs[0].missing {
		t.Error("the pump didn't enable when its lilac cost became affordable")
	}
}

func TestTheTechSurvivesARoundTrip(t *testing.T) {
	s := newGame()
	Apply(s, DevNextTech{})
	runTicks(s, 1)
	Apply(s, AckTech{ID: techIndustryID})
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("the state does not marshal: %v", err)
	}
	var back State
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("the state does not unmarshal: %v", err)
	}
	if !reflect.DeepEqual(back.Tech, s.Tech) {
		t.Errorf("the tech ledger changed across a JSON round trip: %v vs %v",
			back.Tech, s.Tech)
	}
}

// TestWriteTechShotState writes the states the schematics callout's
// shots start from: NIEBLA_TECH_SHOT_STATE with the infrastructure,
// NIEBLA_TECH_PIPES_SHOT_STATE with the frontier kit, or
// NIEBLA_TECH_REPAIR_SHOT_STATE with the informational mechanic item.
// Skipped otherwise, the way tests write nothing. Each run prints where
// the badge and first square stand on the current layout.
func TestWriteTechShotState(t *testing.T) {
	infra := os.Getenv("NIEBLA_TECH_SHOT_STATE")
	frontier := os.Getenv("NIEBLA_TECH_PIPES_SHOT_STATE")
	repair := os.Getenv("NIEBLA_TECH_REPAIR_SHOT_STATE")
	if infra == "" && frontier == "" && repair == "" {
		t.Skip("set a NIEBLA_TECH_*_SHOT_STATE path" +
			" to write the schematics callout's shot states")
	}
	write := func(path string, tech map[string]bool) {
		s := newGame()
		noRivals(s)
		s.Deliveries = 1
		s.Tech = tech
		play := newPlayScene(s)
		id := techPending(s)
		play.techCallout = id
		x, y := techBadgeAt(play)
		t.Logf("schematics badge click: %0.0f,%0.0f", x, y)
		if squares := techSquareLayout(play, id); len(squares) > 0 {
			center := squares[0].area.Center()
			t.Logf("first %s square: %0.0f,%0.0f",
				squares[0].item.name, center.X, center.Y)
		}
		col, row := groundNearCore()
		gx, gy := projectBuilding(Building{Col: col, Row: row})
		ground := play.camera.ToScreen(golib.Vector2{X: gx, Y: gy})
		t.Logf("valid ground cell %d,%d: %0.0f,%0.0f",
			col, row, ground.X, ground.Y)
		data, err := json.MarshalIndent(map[string]any{"state": s}, "", "  ")
		if err != nil {
			t.Fatalf("the state doesn't marshal: %v", err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}
	if infra != "" {
		write(infra, map[string]bool{
			techIndustryID: true, techInfraID: false,
		})
	}
	if frontier != "" {
		write(frontier, map[string]bool{
			techIndustryID: true, techInfraID: true,
			techFrontierID: false,
		})
	}
	if repair != "" {
		write(repair, map[string]bool{
			techIndustryID: true, techInfraID: true, techGuardID: true,
			techFrontierID: true, techMobileID: true,
			techArtilleryID: true, techRepairID: false,
		})
	}
}

func TestWriteGuardTimingShotState(t *testing.T) {
	path := os.Getenv("NIEBLA_GUARD_SHOT_STATE")
	if path == "" {
		t.Skip("set NIEBLA_GUARD_SHOT_STATE to write a guard timing state")
	}
	s := newGame()
	s.Tech = map[string]bool{techIndustryID: true}
	cx, cy := tileCenterUnits(coreCol, coreRow)
	radius := coreBubbleRadius * unitsPerTile
	s.Parties[100] = Party{
		ID: 100, Stage: StageLeave,
		EntryX: cx + radius + 2500, EntryY: cy,
	}
	s.Enemies[101] = Enemy{
		ID: 101, Party: 100, Kind: EnemyScout,
		X: cx + radius - 9, Y: cy, Oil: 3,
	}
	s.Marks[102] = Mark{ID: 102, X: cx, Y: cy}
	play := newPlayScene(s)
	badgeX, badgeY := techBadgeAt(play)
	t.Logf("guard badge click: %.0f,%.0f", badgeX, badgeY)
	data, err := json.MarshalIndent(map[string]any{"state": s}, "", "  ")
	if err != nil {
		t.Fatalf("the guard timing state doesn't marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
