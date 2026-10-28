package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"

	"golib"
)

func TestMitesDamageEveryStationaryUnitAtTheSameRate(t *testing.T) {
	s := newGame()
	noRivals(s)
	x, y := 500.0, 500.0
	if exposure := miteExposureAt(s, x, y); exposure != 1 {
		t.Fatalf("test units have mite exposure %v, want 1", exposure)
	}

	damages := make([]float64, 0, 5)
	for _, kind := range []RobotKind{
		RobotBuilder, RobotWorker, RobotCombat, RobotRepair,
	} {
		id := s.spawnRobot(kind, x, y)
		r := s.Robots[id]
		r.Health, r.StillTicks = 100, fogStillGraceTicks
		before := r.Health
		stepStationaryWear(s, &r, false)
		damages = append(damages, before-r.Health)
	}

	e := Enemy{
		ID: 900, Kind: EnemyRaider, X: x, Y: y,
		Health: 100, StillTicks: fogStillGraceTicks,
	}
	s.Enemies[e.ID] = e
	stepExposure(s, map[int64]PipePoint{e.ID: {X: x, Y: y}})
	damages = append(damages, e.Health-s.Enemies[e.ID].Health)

	want := miteDamagePerSecond / 60
	for i, damage := range damages {
		if math.Abs(damage-want) > 1e-9 {
			t.Errorf("unit %d lost %v health, want %v", i, damage, want)
		}
	}
}

func TestMovingUnitsTakeNoMiteDamageAndResetStillness(t *testing.T) {
	s := newGame()
	noRivals(s)
	r := Robot{
		ID: 800, Kind: RobotBuilder, X: 500, Y: 500,
		Tank: robotTankLiters, Health: 40, StillTicks: 900,
	}
	stepStationaryWear(s, &r, true)
	if r.Health != 40 || r.StillTicks != 0 {
		t.Fatalf("a moving robot has %v health and %d still ticks",
			r.Health, r.StillTicks)
	}

	e := Enemy{
		ID: 900, Kind: EnemyRaider, X: 500, Y: 500,
		Health: 40, StillTicks: 900,
	}
	s.Enemies[e.ID] = e
	stepExposure(s, map[int64]PipePoint{e.ID: {X: 499, Y: 500}})
	got := s.Enemies[e.ID]
	if got.Health != 40 || got.StillTicks != 0 {
		t.Fatalf("a moving rival has %v health and %d still ticks",
			got.Health, got.StillTicks)
	}
}

func TestAnActiveRepulsorBubbleSheltersEveryHostInside(t *testing.T) {
	s := newGame()
	noRivals(s)
	crawler := Enemy{
		ID: 900, Kind: EnemyCrawler, Party: 1,
		X: 500, Y: 500, Health: enemySpecOf(EnemyCrawler).health,
	}
	raider := Enemy{
		ID: 901, Kind: EnemyRaider, Party: 1,
		X: 520, Y: 500, Health: enemySpecOf(EnemyRaider).health,
		StillTicks: 900,
	}
	s.Enemies[crawler.ID] = crawler
	s.Enemies[raider.ID] = raider
	positions := map[int64]PipePoint{
		crawler.ID: {X: crawler.X, Y: crawler.Y},
		raider.ID:  {X: raider.X, Y: raider.Y},
	}
	stepExposure(s, positions)
	if miteExposureAt(s, raider.X, raider.Y) != 0 {
		t.Fatal("the crawler's active bubble left mite exposure on its raider")
	}
	if got := s.Enemies[raider.ID]; got.Health != raider.Health ||
		got.StillTicks != 0 {
		t.Fatalf("the sheltered raider has %v health and %d still ticks",
			got.Health, got.StillTicks)
	}
}

func TestAStationarySwellDoublesMiteDamage(t *testing.T) {
	damageAtPressure := func(pressure float64) float64 {
		s := newGame()
		noRivals(s)
		s.Fog.Pressure = pressure
		r := Robot{
			ID: 800, Kind: RobotBuilder, X: 500, Y: 500,
			Tank: robotTankLiters, Health: 100,
			StillTicks: fogStillGraceTicks,
		}
		stepStationaryWear(s, &r, false)
		return 100 - r.Health
	}

	calm := damageAtPressure(0)
	swell := damageAtPressure(1)
	if math.Abs(swell-calm*2) > 1e-9 {
		t.Fatalf("the swell dealt %v damage, want twice the calm %v",
			swell, calm)
	}
}

func TestChargedPylonsAreImmuneAndEmptyProtectorsWearDown(t *testing.T) {
	s := newGame()
	noRivals(s)
	col, row := groundInTheFog()
	s.raise(BuildingProtector, col, row)
	p, _ := buildingAt(s, col, row)
	x, y := cellCenterUnits(col, row)
	if miteExposureAt(s, x, y) != 0 {
		t.Fatal("a charged protector's bubble has mite exposure")
	}
	field := newMiteField()
	field.update(s, 1.0/60)
	if field.hosts["building:"+formatID(p.ID)] != nil {
		t.Fatal("a charged protector has a mite host")
	}

	p.Oil = 0
	s.Buildings[p.ID] = p
	if miteExposureAt(s, x, y) <= 0 {
		t.Fatal("an empty protector has no mite exposure")
	}
	stepMiteWear(s)
	if s.Buildings[p.ID].Damage <= 0 {
		t.Fatal("mites did not damage the empty protector")
	}
	field.update(s, 1.0/60)
	if host := field.hosts["building:"+formatID(p.ID)]; host == nil || host.Wanted == 0 {
		t.Fatal("mites did not gather on the empty protector")
	}
}

func TestExposedBuildingsAndSitesTakeTheSharedMiteDamage(t *testing.T) {
	s := newGame()
	noRivals(s)
	jobCol, jobRow := groundInTheFog()
	buildingCol, buildingRow := 40, 20
	s.Jobs = append(s.Jobs, Job{
		Kind: BuildingFactory, Col: jobCol, Row: jobRow,
		Left: buildingWorkTicks,
	})
	s.raise(BuildingFactory, buildingCol, buildingRow)
	building, _ := buildingAt(s, buildingCol, buildingRow)
	stepMiteWear(s)

	want := miteDamagePerSecond / 60
	if got := s.Jobs[0].Damage; math.Abs(got-want) > 1e-9 {
		t.Errorf("the exposed site took %v damage, want %v", got, want)
	}
	if got := s.Buildings[building.ID].Damage; math.Abs(got-want) > 1e-9 {
		t.Errorf("the exposed building took %v damage, want %v", got, want)
	}
}

func TestExposedRivalCityConstructionSiteWearsAndShowsMites(t *testing.T) {
	s := newGame()
	noRivals(s)
	id := s.foundCity(500, 500, 0)
	stepMiteCitySiteWear(s)
	city := s.Cities[id]
	want := miteDamagePerSecond / 60
	if math.Abs(city.MiteDamage-want) > 1e-9 {
		t.Fatalf("the exposed city site took %v damage, want %v",
			city.MiteDamage, want)
	}

	field := newMiteField()
	field.update(s, 1.0/60)
	if host := field.hosts[citySiteMiteKey(id)]; host == nil || host.Wanted == 0 {
		t.Fatal("the exposed city construction site has no visible mites")
	}
	stage, building := cityNextBuildingStage(s, city)
	if !building {
		t.Fatal("the city has no construction stage under way")
	}
	spec := cityBuildingSpec(cityBuildOrder[stage])
	city.MiteDamage = spec.health - want/2
	city.Work = 1
	s.Cities[id] = city
	stepMiteCitySiteWear(s)
	if got := s.Cities[id]; got.Work != cityBuildTicks || got.MiteDamage != 0 {
		t.Fatalf("consumed city work resumed as %+v", got)
	}
}

func TestRivalCitySwarmKeepsSixtyPercentParticles(t *testing.T) {
	s := newGame()
	noRivals(s)
	normal := miteHost{
		X: 500, Y: 500, Across: 24, Height: 18,
	}
	normal.want(s)
	city := normal
	city.RivalCity = true
	city.want(s)
	ratio := float64(city.Wanted) / float64(normal.Wanted)
	if ratio < 0.5 || ratio > 0.7 {
		t.Fatalf("a city host has %.2f of the normal swarm, want 0.50-0.70",
			ratio)
	}
}

func TestPilesLoseTheirMaterialOverThreeMinutesInFullFog(t *testing.T) {
	s := newGame()
	noRivals(s)
	col, row := groundInTheFog()
	x, y := cellCenterUnits(col, row)
	if exposure := miteExposureAt(s, x, y); exposure != 1 {
		t.Fatalf("the test pile has exposure %v, want 1", exposure)
	}
	s.dropPile(col, row, 100, 50)
	p, _ := pileAt(s, col, row)
	for tick := int64(0); tick < mitePileLifetimeTicks-1; tick++ {
		stepMitePileWear(s)
	}
	if got, exists := s.Piles[p.ID]; !exists || got.Oil <= pileDust {
		t.Fatalf("the pile was gone before three minutes: %+v, %v", got, exists)
	}
	stepMitePileWear(s)
	if _, exists := s.Piles[p.ID]; exists {
		t.Fatal("the pile survived three minutes in full fog")
	}
}

func TestExposedPipesWearDownAndCarryVisibleMites(t *testing.T) {
	s := newGame()
	noRivals(s)
	s.raise(BuildingPump, 20, 20)
	s.raise(BuildingSilo, 40, 20)
	from, _ := buildingAt(s, 20, 20)
	to, _ := buildingAt(s, 40, 20)
	fromSpot, _ := pipeEndSpot(s, from.ID)
	toSpot, _ := pipeEndSpot(s, to.ID)
	path := pipePath(fromSpot, nil, toSpot)
	p := Pipe{
		ID: s.NextID, From: from.ID, To: to.ID,
		Sections: pipeSections(pathLength(path)),
	}
	s.NextID++
	s.Pipes[p.ID] = p

	golib.SetRandomSeed(1)
	field := newMiteField()
	field.update(s, 1.0/60)
	if host := field.hosts[pipeMiteKey(p.ID, 0)]; host == nil || host.Wanted == 0 {
		t.Fatal("the exposed pipe has no visible mites")
	}
	e := Enemy{
		ID: 900, Kind: EnemyRaider, X: 500, Y: 500,
		Health: 100,
	}
	s.Enemies[e.ID] = e
	field.update(s, 1.0/60)
	if host := field.hosts[enemyMiteKey(e.ID)]; host == nil || host.Wanted == 0 {
		t.Fatal("the exposed rival vehicle has no visible mites")
	}

	for range 30 {
		stepMitePipeWear(s)
	}
	if got := s.Pipes[p.ID].Damage; got <= 0 {
		t.Fatal("the exposed pipe has no mite damage")
	}
	p = s.Pipes[p.ID]
	p.Damage = buildingHealthPoints - 1
	s.Pipes[p.ID] = p
	for range 35 {
		stepMitePipeWear(s)
	}
	if _, exists := s.Pipes[p.ID]; exists {
		t.Fatal("the mites did not consume the damaged pipe")
	}
}

func TestMiteHalosTurnRedWhenHostsStop(t *testing.T) {
	moving := miteHaloColor(0)
	still := miteHaloColor(1)
	if moving.R != 0 || still.R != miteStationaryHaloRed ||
		still.G != 0 || still.B != 0 {
		t.Fatalf("mite halos are moving=%+v, still=%+v; want black and dark red",
			moving, still)
	}
}

func TestWriteMiteShotState(t *testing.T) {
	path := os.Getenv("NIEBLA_MITE_SHOT_STATE")
	if path == "" {
		t.Skip("set NIEBLA_MITE_SHOT_STATE to write the mite shot state")
	}
	s := newGame()
	noRivals(s)

	const robotCol, robotRow = 133, 100
	x, y := cellCenterUnits(robotCol, robotRow)
	s.dropPile(robotCol, robotRow, 60, 40)
	robot := s.Robots[sortedRobotIDs(s)[0]]
	robot.X, robot.Y = x, y
	robot.Pile = sortedPileIDs(s)[0]
	robot.WorkTicks = 10000
	s.Robots[robot.ID] = robot

	s.Enemies[900] = Enemy{
		ID: 900, Kind: EnemyRaider, X: x + 220, Y: y + 170,
		Health: enemySpecOf(EnemyRaider).health,
	}

	s.raise(BuildingProtector, 140, 100)
	empty, _ := buildingAt(s, 140, 100)
	empty.Oil = 0
	s.Buildings[empty.ID] = empty
	s.raise(BuildingProtector, 109, 135)

	s.raise(BuildingPump, 136, 109)
	s.raise(BuildingSilo, 142, 109)
	pump, _ := buildingAt(s, 136, 109)
	silo, _ := buildingAt(s, 142, 109)
	from, _ := pipeEndSpot(s, pump.ID)
	to, _ := pipeEndSpot(s, silo.ID)
	pathLine := pipePath(from, nil, to)
	p := Pipe{
		ID: s.NextID, From: pump.ID, To: silo.ID,
		Sections: pipeSections(pathLength(pathLine)),
	}
	s.NextID++
	s.Pipes[p.ID] = p

	data, err := json.MarshalIndent(map[string]any{"state": s}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func formatID(id int64) string {
	return fmt.Sprintf("%d", id)
}
