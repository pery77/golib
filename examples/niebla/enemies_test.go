package main

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"

	"golib"
)

func TestWorldSpritesHaveEveryFacing(t *testing.T) {
	for name, model := range map[string]worldSprite{
		"core":         coreWorkerModel,
		"carrier":      carrierModel,
		"trooper":      trooperModel,
		"mechanic":     mechanicModel,
		"scout":        rivalScoutModel,
		"crawler":      rivalCrawlerModel,
		"city crawler": rivalCityCrawlerModel,
		"raider":       rivalRaiderModel,
		"artillery":    rivalArtilleryModel,
	} {
		if got := model.sprite.Frames(); got != 8 {
			t.Errorf("%s sprite has %d facings, want 8", name, got)
		}
		if got := model.shadow.Frames(); got != 8 {
			t.Errorf("%s shadow has %d facings, want 8", name, got)
		}
	}
}

func TestWorldSpritesMoveInScreenPixelsAtCloseZoom(t *testing.T) {
	camera := golib.NewCamera(float32(screenWidth), float32(screenHeight))
	camera.Zoom = 32
	camera.Target = golib.Vector2{X: 100, Y: 200}
	camera.Snap()
	before := modelScreenPosition(camera, golib.Vector2{X: 100.2, Y: 200})
	after := modelScreenPosition(camera, golib.Vector2{X: 100.4, Y: 200})
	if moved := after.X - before.X; math.Abs(float64(moved-6.4)) > 0.01 {
		t.Errorf("sprite moved %.2f screen pixels, want 6.4", moved)
	}
}

// noRivals keeps the rivals away for good, for the tests that are about
// something else.
func noRivals(s *State) {
	s.Raids.NextAt = math.MaxInt64
}

// visitNow brings the next visit in on the next tick.
func visitNow(s *State) {
	Apply(s, DevNextVisit{})
	runTicks(s, 1)
}

func lastReport(s *State) Report {
	if len(s.Reports) == 0 {
		return Report{}
	}
	return s.Reports[len(s.Reports)-1]
}

func TestTheFirstScoutSiphonsLeavesItsMarkAndGoes(t *testing.T) {
	s := newGame()
	runTicks(s, raidFirstScoutTicks-1)
	if len(s.Enemies) != 0 {
		t.Fatalf("%d vehicles before the scout's tick", len(s.Enemies))
	}
	runTicks(s, 1)
	if len(s.Enemies) != 1 || len(s.Parties) != 1 {
		t.Fatalf("the first visit is %d vehicles in %d parties, want a lone scout",
			len(s.Enemies), len(s.Parties))
	}
	oil := oilTotal(s)
	if !tickUntil(s, 60*600, func() bool { return len(s.Marks) == 1 }) {
		t.Fatalf("the scout never left its mark")
	}
	cap := enemySpecOf(EnemyScout).oilCap
	if got := oil - oilTotal(s); math.Abs(got-cap) > 0.001 {
		t.Errorf("the scout took %v L, want its tank's %v", got, cap)
	}
	if r := lastReport(s); r.Kind != ReportScout || math.Abs(r.Oil-cap) > 0.001 {
		t.Errorf("the last report is %+v, want the scout's with its %v L", r, cap)
	}
	if !tickUntil(s, 60*600, func() bool { return len(s.Parties) == 0 }) {
		t.Fatalf("the scout never left")
	}
	if len(s.Enemies) != 0 || s.Raids.Visits != 1 {
		t.Errorf("%d vehicles left and %d visits counted, want 0 and 1",
			len(s.Enemies), s.Raids.Visits)
	}
	if got, want := s.Raids.NextAt-s.Ticks,
		int64(raidFollowupTicks); got != want {
		t.Errorf("the next visit comes in %d ticks, want %d", got, want)
	}
}

func TestSecondVisitFoundsThePressureCityOnTheScoutsBearing(t *testing.T) {
	s := newGame()
	runTicks(s, raidFirstScoutTicks-1)
	if len(s.Enemies) != 0 {
		t.Fatal("the scout arrived before its one-minute mark")
	}
	runTicks(s, 1)
	if len(s.Enemies) != 1 {
		t.Fatalf("the first visit has %d vehicles, want the scout", len(s.Enemies))
	}
	if len(s.Cities) != 0 {
		t.Fatal("the city was founded before the scout left")
	}
	var scout int64
	for _, id := range sortedEnemyIDs(s) {
		if s.Enemies[id].Kind == EnemyScout {
			scout = id
		}
	}
	s.killEnemy(scout)
	runTicks(s, 1)
	if got := s.Raids.NextAt - s.Ticks; got != raidFollowupTicks {
		t.Fatalf("the second visit is in %d ticks, want %d", got, raidFollowupTicks)
	}
	runTicks(s, int(raidFollowupTicks)-1)
	if len(s.Cities) != 0 {
		t.Fatal("the city started before the one-minute follow-up")
	}
	runTicks(s, 1)
	if len(s.Cities) != 1 || len(s.Parties) != 1 {
		t.Fatalf("the follow-up created %d cities and %d parties, want one each",
			len(s.Cities), len(s.Parties))
	}
	city := s.Cities[s.Raids.PressureCity]
	party := s.Parties[sortedPartyIDs(s)[0]]
	if city.Stage != 0 || party.Stage != StageApproach {
		t.Fatalf("follow-up started city stage %d and party stage %q",
			city.Stage, party.Stage)
	}
	if math.Abs(city.Angle-s.Raids.FirstBearing) > 0.001 {
		t.Fatalf("city bearing %.3f differs from scout bearing %.3f",
			city.Angle, s.Raids.FirstBearing)
	}
	cx, cy := tileCenterUnits(coreCol, coreRow)
	distance := math.Hypot(city.X-cx, city.Y-cy) / unitsPerTile
	if math.Abs(distance-cityRadiusTiles) > 0.001 {
		t.Fatalf("the pressure city is %.3f tiles from the core, want %.1f",
			distance, cityRadiusTiles)
	}
	if !kindUnlocked(s, BuildingWarFactory) {
		t.Fatal("the mobile schematics did not arrive with the pressure city")
	}
}

func TestPressurePartiesRepeatOneMinuteAfterDisappearanceDuringConstruction(t *testing.T) {
	s := newGame()
	s.Raids.Visits = 1
	s.Raids.FirstBearing = 0.7
	s.Raids.BearingKnown = true
	s.Raids.NextAt = s.Ticks + 1
	runTicks(s, 1)
	first := s.Parties[sortedPartyIDs(s)[0]]
	if s.Raids.PressureCity == 0 || first.Stage != StageApproach {
		t.Fatal("the second visit did not start the city's construction")
	}
	for _, id := range sortedEnemyIDs(s) {
		delete(s.Enemies, id)
	}
	runTicks(s, 1)
	if got := s.Raids.NextAt - s.Ticks; got != raidFollowupTicks {
		t.Fatalf("the next party is in %d ticks, want %d", got,
			raidFollowupTicks)
	}
	runTicks(s, int(raidFollowupTicks)-1)
	if len(s.Parties) != 0 {
		t.Fatal("another party appeared before the one-minute wait ended")
	}
	runTicks(s, 1)
	second := s.Parties[sortedPartyIDs(s)[0]]
	if second.Stage != StageApproach ||
		math.Abs(second.EntryX-first.EntryX) > 0.001 ||
		math.Abs(second.EntryY-first.EntryY) > 0.001 {
		t.Fatalf("the repeated party entered at %.1f, %.1f, want %.1f, %.1f",
			second.EntryX, second.EntryY, first.EntryX, first.EntryY)
	}
}

func TestFirstActualRaidDoesNotCamp(t *testing.T) {
	s := newGame()
	s.Raids.Visits = 1
	visitNow(s)
	party := s.Parties[sortedPartyIDs(s)[0]]
	if party.Wait != 0 {
		t.Fatalf("the first raid waits %d ticks before attacking", party.Wait)
	}
	if !tickUntil(s, 60*300, func() bool {
		return s.Parties[party.ID].Stage == StageRaid
	}) {
		t.Fatal("the first raid never started")
	}
	if lastReport(s).Kind != ReportRaid {
		t.Fatalf("the first attack reported %q, want raid", lastReport(s).Kind)
	}
	for _, report := range s.Reports {
		if report.Kind == ReportCamp {
			t.Fatal("the first actual raid stopped to camp")
		}
	}
}

func TestWriteFirstRaidShotState(t *testing.T) {
	path := os.Getenv("NIEBLA_FIRST_RAID_SHOT_STATE")
	if path == "" {
		t.Skip("set NIEBLA_FIRST_RAID_SHOT_STATE to write the first-raid state")
	}
	s := newGame()
	s.Raids.Visits = 1
	visitNow(s)
	partyID := sortedPartyIDs(s)[0]
	if !tickUntil(s, 60*300, func() bool {
		return s.Parties[partyID].Stage == StageRaid
	}) {
		t.Fatal("the first attack never moved in without camping")
	}
	data, err := json.MarshalIndent(map[string]any{"state": s}, "", "  ")
	if err != nil {
		t.Fatalf("the first-raid state doesn't marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func TestRaiderBattalionsGrowByVisitAndStopAtFour(t *testing.T) {
	for _, test := range []struct {
		visit int64
		want  int
	}{{1, 1}, {2, 2}, {3, 3}, {4, 4}, {5, 4}, {10, 4}} {
		if got := raidersOf(test.visit); got != test.want {
			t.Errorf("visit %d brings %d raiders, want %d",
				test.visit, got, test.want)
		}
	}
}

func TestARaidCampsGetsReadyStealsAndLeaves(t *testing.T) {
	s := newGame()
	seedStock(s)
	s.Raids.Visits = 2
	cityID := s.foundCityOnBearing(0.7)
	s.Raids.PressureCity = cityID
	visitNow(s)
	party := s.Parties[sortedPartyIDs(s)[0]]
	if got, want := len(partyMembers(s, party.ID)),
		1+raidersOf(2); got != want {
		t.Fatalf("the first raid brings %d vehicles, want %d", got, want)
	}
	if party.Artillery || len(partyMembers(s, party.ID)) == 0 ||
		enemySpecOf(partyMembers(s, party.ID)[0].Kind).bubble == 0 {
		t.Fatal("the first raid has no antimist vehicle")
	}
	if party.Stage != StageApproach {
		t.Fatalf("the raid comes in at stage %q, want approach", party.Stage)
	}
	staged := func(stage PartyStage) func() bool {
		return func() bool { return s.Parties[party.ID].Stage == stage }
	}
	if !tickUntil(s, 60*300, staged(StageCamp)) {
		t.Fatalf("the raid never camped")
	}
	if lastReport(s).Kind != ReportCamp {
		t.Errorf("camping reported %q", lastReport(s).Kind)
	}
	lead := partyMembers(s, party.ID)[0]
	cx, cy := tileCenterUnits(coreCol, coreRow)
	gap := math.Hypot(lead.X-cx, lead.Y-cy) / unitsPerTile
	if math.Abs(gap-campRadiusTiles) > 0.01 {
		t.Errorf("the camp is %v tiles out, want %v", gap, campRadiusTiles)
	}
	oil := oilTotal(s)
	runTicks(s, int(prepareTicks(2))-1)
	if !staged(StageCamp)() || oilTotal(s) != oil {
		t.Errorf("the raid moved before its time")
	}
	runTicks(s, 1)
	if !staged(StageRaid)() || lastReport(s).Kind != ReportRaid {
		t.Errorf("after getting ready the stage is %q and the report %q",
			s.Parties[party.ID].Stage, lastReport(s).Kind)
	}
	if !tickUntil(s, 60*600, func() bool { return len(s.Parties) == 0 }) {
		t.Fatalf("the raid never left")
	}
	want := float64(raidersOf(2)) * enemySpecOf(EnemyRaider).oilCap
	if got := oil - oilTotal(s); math.Abs(got-want) > 0.001 {
		t.Errorf("the raid took %v L, want every raider full: %v", got, want)
	}
	if r := lastReport(s); r.Kind != ReportLeft || math.Abs(r.Oil-want) > 0.001 {
		t.Errorf("the last report is %+v, want them gone with %v L", r, want)
	}
	if prepareTicks(3) >= prepareTicks(2) {
		t.Errorf("later raids should spend less time getting ready")
	}
}

func TestAGuardPostShootsForOilAndTheWrecksDropWhatTheyStole(t *testing.T) {
	s := newGame()
	seedStock(s)
	col, row := groundNearCore()
	raised(t, s, BuildingGuard, col, row)
	visitNow(s)
	oil := oilTotal(s)
	if !tickUntil(s, 60*600, func() bool { return len(s.Parties) == 0 }) {
		t.Fatalf("the scout's visit never ended")
	}
	if lastReport(s).Kind != ReportDestroyed || len(s.Marks) != 0 {
		t.Errorf("the last report is %q with %d marks, want the scout shot down first",
			lastReport(s).Kind, len(s.Marks))
	}
	spec := enemySpecOf(EnemyScout)
	shots := math.Ceil(spec.health / guardShotDamage)
	if got, want := oil-oilTotal(s), shots*guardShotOil; math.Abs(got-want) > 0.001 {
		t.Errorf("the colony burned %v L, want %v shots' worth: %v", got, shots, want)
	}
	if len(s.Piles) != 1 {
		t.Fatalf("%d piles after the wreck, want 1", len(s.Piles))
	}
	for _, p := range s.Piles {
		if math.Abs(p.Oil-spec.lootOil) > 0.001 || math.Abs(p.Lilac-spec.lootLilac) > 0.001 {
			t.Errorf("the wreck dropped %v L and %v kg, want %v and %v",
				p.Oil, p.Lilac, spec.lootOil, spec.lootLilac)
		}
	}
	// A dry colony doesn't shoot.
	s.Stock.Oil = 0
	s.Raids.Visits = 0
	visitNow(s)
	runTicks(s, 60*60)
	for _, e := range s.Enemies {
		if e.Health != spec.health {
			t.Errorf("a guard with no oil hurt the scout: %v health", e.Health)
		}
	}
}

func TestAStationaryRivalTakesMiteDamageAfterGrace(t *testing.T) {
	s := newGame()
	noRivals(s)
	e := Enemy{
		ID: 900, Kind: EnemyRaider, X: 500, Y: 500,
		Health: 100, StillTicks: fogStillGraceTicks,
	}
	s.Enemies[e.ID] = e
	positions := map[int64]PipePoint{
		e.ID: {X: e.X, Y: e.Y},
	}
	stepExposure(s, positions)

	got := s.Enemies[e.ID]
	wantDamage := miteDamagePerSecond * miteExposureAt(s, e.X, e.Y) / 60
	if math.Abs((e.Health-got.Health)-wantDamage) > 1e-9 {
		t.Fatalf("a stationary raider lost %v health, want %v",
			e.Health-got.Health, wantDamage)
	}
	if got.StillTicks != fogStillGraceTicks+1 {
		t.Errorf("the raider has %d still ticks, want %d",
			got.StillTicks, fogStillGraceTicks+1)
	}

	got.Health = 0.01
	got.StillTicks = fogStillGraceTicks + 1
	s.Enemies[e.ID] = got
	stepExposure(s, positions)
	if _, alive := s.Enemies[e.ID]; alive {
		t.Fatal("mites did not digest a rival whose hull ran out")
	}
	if len(s.Piles) == 0 {
		t.Fatal("the digested raider left no wreck")
	}
}

func TestRivalsReplayAndSurviveASave(t *testing.T) {
	play := func() *State {
		s := newGame()
		s.Raids.Visits = 2
		visitNow(s)
		runTicks(s, 60*200)
		return s
	}
	s := play()
	if !reflect.DeepEqual(s, play()) {
		t.Errorf("two runs of the same raid ended in different states")
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
		t.Errorf("the state changed across a JSON round trip")
	}
	// A save from before the rivals has no tables and no clock for them.
	old := newGame()
	old.Enemies, old.Parties, old.Marks, old.Raids = nil, nil, nil, Raids{}
	runTicks(old, raidFirstScoutTicks+60*600)
	if old.Raids.Visits < 1 || len(old.Marks) != 1 {
		t.Errorf("an old save saw %d visits and %d marks, want the scout's mark",
			old.Raids.Visits, len(old.Marks))
	}
}
