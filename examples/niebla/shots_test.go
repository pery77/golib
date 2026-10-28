package main

import (
	"encoding/json"
	"math"
	"testing"
)

func TestShellLaunchPointStartsAtTheMuzzle(t *testing.T) {
	fromX, fromY := 80.0, 120.0
	toX, toY := 380.0, 520.0
	muzzleX, muzzleY := shellLaunchPoint(fromX, fromY, toX, toY)
	got := math.Hypot(muzzleX-fromX, muzzleY-fromY)
	if math.Abs(got-shellMuzzleOffsetUnits) > 0.0001 {
		t.Fatalf("the shell starts %v u ahead of the gun, want %v",
			got, shellMuzzleOffsetUnits)
	}
	remaining := math.Hypot(toX-muzzleX, toY-muzzleY)
	original := math.Hypot(toX-fromX, toY-fromY)
	if remaining >= original {
		t.Fatal("the muzzle point did not move toward the target")
	}
	closeX, closeY := shellLaunchPoint(fromX, fromY, fromX+10, fromY)
	got = math.Hypot(closeX-fromX, closeY-fromY)
	if math.Abs(got-5) > 0.0001 {
		t.Fatalf("a close shot starts %v u from the gun, want 5", got)
	}
}

func TestShellTrailGrowsWithItsFlight(t *testing.T) {
	shot := Shot{Kind: ShotShell, ToX: 500}
	if got := shellTrailSteps(shot); got != 0 {
		t.Fatalf("a shell at the muzzle has %d trail marks", got)
	}
	shot.X = shellTrailStepUnits - 0.1
	if got := shellTrailSteps(shot); got != 0 {
		t.Fatalf("a shell before its first trail step has %d marks", got)
	}
	shot.X = shellTrailStepUnits * 2
	if got, want := shellTrailSteps(shot), 2; got != want {
		t.Fatalf("a shell after two steps has %d trail marks, want %d", got, want)
	}
	shot.X = shellTrailLengthUnits * 2
	if got, want := shellTrailSteps(shot), 6; got != want {
		t.Fatalf("a long-flight shell has %d trail marks, want %d", got, want)
	}
}

func TestShellLeavesSubtleSmokeAlongItsFlight(t *testing.T) {
	f := newFxField()
	shot := Shot{
		ID: 1, Kind: ShotShell,
		FromX: 10, FromY: 20, ToX: 1010, ToY: 20,
	}
	shot.X, shot.Y = shot.FromX, shot.FromY
	s := &State{Shots: map[int64]Shot{shot.ID: shot}}
	f.update(s, 1.0/60, nil, nil)
	shot.X = shot.FromX + shellSmokeStepUnits*2 + shellSmokeStepUnits/2
	shot.Y = shot.FromY
	s.Shots[shot.ID] = shot
	f.update(s, 1.0/60, nil, nil)

	trailPuffs := 0
	for _, particle := range f.sparks {
		if particle.smoke && particle.opacity == shellSmokeOpacity {
			trailPuffs++
		}
	}
	if trailPuffs != 2 {
		t.Fatalf("the shell left %d trail puffs, want 2", trailPuffs)
	}
}

func TestTickReportsUnitDeathsAndKeepsThemOutOfSaves(t *testing.T) {
	s := newGame()
	noRivals(s)
	x, y := parkCenter()

	robotID := s.spawnRobot(RobotRepair, x+35, y+35)
	robot := s.Robots[robotID]
	robot.Health = 1
	s.Robots[robotID] = robot
	s.fire(Shot{
		Kind: ShotBullet, FromX: robot.X, FromY: robot.Y,
		ToX: robot.X, ToY: robot.Y,
		Robot: robotID, Damage: 2, Rival: true,
	})

	enemyIDs := []int64{}
	wantKinds := map[int64]EnemyKind{}
	for i, kind := range []EnemyKind{EnemyScout, EnemyArtillery} {
		id := s.NextID
		s.NextID++
		e := Enemy{
			ID: id, Kind: kind,
			X: x + float64(i*120), Y: y - 40,
			Health: 1,
		}
		s.Enemies[id] = e
		s.fire(Shot{
			Kind: ShotBullet, FromX: e.X, FromY: e.Y,
			ToX: e.X, ToY: e.Y, Enemy: id, Damage: 2,
		})
		enemyIDs = append(enemyIDs, id)
		wantKinds[id] = kind
	}

	Apply(s, Tick{})
	if got, want := len(s.Deaths), 3; got != want {
		t.Fatalf(
			"the tick reported %d deaths, want %d: %+v",
			got, want, s.Deaths,
		)
	}
	foundRobot := false
	foundEnemies := map[int64]bool{}
	for _, death := range s.Deaths {
		switch {
		case death.RobotKind != "":
			if death.ID != robotID || death.RobotKind != RobotRepair {
				t.Errorf("wrong robot death receipt: %+v", death)
			}
			if death.X != robot.X || death.Y != robot.Y {
				t.Errorf("robot death position is (%v, %v)", death.X, death.Y)
			}
			foundRobot = true
		case death.EnemyKind != "":
			if wantKinds[death.ID] != death.EnemyKind {
				t.Errorf("wrong enemy death receipt: %+v", death)
			}
			foundEnemies[death.ID] = true
		}
	}
	if !foundRobot {
		t.Error("the robot death was not reported")
	}
	for _, id := range enemyIDs {
		if !foundEnemies[id] {
			t.Errorf("enemy %d's death was not reported", id)
		}
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("the state did not marshal: %v", err)
	}
	var loaded State
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("the state did not unmarshal: %v", err)
	}
	if len(loaded.Deaths) != 0 {
		t.Fatalf("cosmetic death receipts survived a save: %+v", loaded.Deaths)
	}

	Apply(s, Tick{})
	if len(s.Deaths) != 0 {
		t.Fatalf("death receipts were not cleared by the next tick: %+v", s.Deaths)
	}
}

func TestTickReportsFogDeathsButNotDepartingRivals(t *testing.T) {
	t.Run("fog", func(t *testing.T) {
		s := stationaryBuilderDeepFog(t)
		noRivals(s)
		robot := s.Robots[1]
		robot.Health = 0
		s.Robots[robot.ID] = robot

		id := s.NextID
		s.NextID++
		s.Enemies[id] = Enemy{
			ID: id, Kind: EnemyRaider,
			X: robot.X + 30, Y: robot.Y,
			Health: 0.01, StillTicks: fogStillGraceTicks,
		}
		Apply(s, Tick{})

		if len(s.Deaths) != 2 {
			t.Fatalf(
				"the fog reported %d deaths, want 2: %+v",
				len(s.Deaths), s.Deaths,
			)
		}
		foundRobot, foundEnemy := false, false
		for _, death := range s.Deaths {
			foundRobot = foundRobot ||
				(death.ID == robot.ID && death.RobotKind == RobotBuilder)
			foundEnemy = foundEnemy ||
				(death.ID == id && death.EnemyKind == EnemyRaider)
		}
		if !foundRobot || !foundEnemy {
			t.Fatalf(
				"the fog death receipts are incomplete: %+v",
				s.Deaths,
			)
		}
	})

	t.Run("departure", func(t *testing.T) {
		s := newGame()
		party := Party{
			ID: 900, Stage: StageLeave, EntryX: 120, EntryY: 160,
		}
		enemy := Enemy{
			ID: 901, Kind: EnemyRaider, Party: party.ID,
			X: party.EntryX, Y: party.EntryY, Health: 1,
		}
		s.Parties[party.ID] = party
		s.Enemies[enemy.ID] = enemy
		stepParty(s, party)
		if len(s.Deaths) != 0 {
			t.Fatalf(
				"a departed rival produced death effects: %+v",
				s.Deaths,
			)
		}
		if _, alive := s.Enemies[enemy.ID]; alive {
			t.Fatal("the departing rival remained in the region")
		}
	})
}

func TestBuildingDeathsAreTransientAndNameTheirCause(t *testing.T) {
	s := newGame()
	noRivals(s)
	col, row := groundNearCore()
	destroyed := raised(t, s, BuildingGuard, col, row)
	demolished := raised(t, s, BuildingFactory, col+2, row)

	s.hurtBuilding(destroyed.ID, buildingHealth(destroyed.Kind))
	demolished.Demolish = 1
	s.Buildings[demolished.ID] = demolished
	s.workDemolish(demolished.ID)

	if len(s.BuildingDeaths) != 2 {
		t.Fatalf("the takedowns reported %d building deaths, want 2: %+v",
			len(s.BuildingDeaths), s.BuildingDeaths)
	}
	got := map[int64]BuildingDeath{}
	for _, death := range s.BuildingDeaths {
		got[death.ID] = death
	}
	if death := got[destroyed.ID]; death.Cause != BuildingDestroyed ||
		death.Kind != destroyed.Kind {
		t.Errorf("the destroyed building receipt is %+v", death)
	}
	if death := got[demolished.ID]; death.Cause != BuildingDemolished ||
		death.Kind != demolished.Kind {
		t.Errorf("the demolished building receipt is %+v", death)
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var loaded State
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	if len(loaded.BuildingDeaths) != 0 {
		t.Fatalf("building death receipts survived a save: %+v",
			loaded.BuildingDeaths)
	}
	Apply(s, Tick{})
	if len(s.BuildingDeaths) != 0 {
		t.Fatalf("building death receipts outlived their tick: %+v",
			s.BuildingDeaths)
	}
}

func TestDestroyedCityCrawlerEmitsAUnitDeath(t *testing.T) {
	s := newGame()
	noRivals(s)
	cityID := s.foundCity(3300, 3000, 0.4)
	rigID := s.Cities[cityID].BuildingIDs[0]
	s.killEnemy(rigID)
	if len(s.Deaths) != 1 {
		t.Fatalf("the destroyed city crawler produced %d unit deaths: %+v",
			len(s.Deaths), s.Deaths)
	}
	if s.Deaths[0].EnemyKind != EnemyCityCrawler ||
		len(s.BuildingDeaths) != 0 {
		t.Fatalf("the city crawler deaths are %+v and %+v",
			s.Deaths, s.BuildingDeaths)
	}
	city := s.Cities[cityID]
	if !city.Ruined || city.RefoundAt != s.Ticks+cityRefoundDelayTicks {
		t.Fatalf("a city with no remaining structures was not razed: %+v", city)
	}
}

func TestCityCoreCollapseRecordsItsDeath(t *testing.T) {
	s := newGame()
	noRivals(s)
	cityID := s.foundCity(3300, 3000, 0.4)
	city := s.Cities[cityID]
	repulsorID := s.NextID
	s.NextID++
	baseID := s.NextID
	s.NextID++
	s.Enemies[repulsorID] = Enemy{
		ID: repulsorID, Kind: EnemyCityRepulsor,
		X: city.X + 30, Y: city.Y + 30,
		Health: enemySpecOf(EnemyCityRepulsor).health, City: cityID,
	}
	s.Enemies[baseID] = Enemy{
		ID: baseID, Kind: EnemyBase,
		X: city.X, Y: city.Y,
		Health: enemySpecOf(EnemyBase).health, City: cityID,
	}
	city.NexusID = baseID
	city.BuildingIDs = append(city.BuildingIDs, repulsorID, baseID)
	s.Cities[cityID] = city

	s.killEnemy(baseID)
	if len(s.BuildingDeaths) != 1 {
		t.Fatalf("the falling Nexus made %d collapse events: %+v",
			len(s.BuildingDeaths), s.BuildingDeaths)
	}
	if s.BuildingDeaths[0].RivalKind != EnemyBase {
		t.Fatalf("the collapse event is %+v, want the Nexus",
			s.BuildingDeaths[0])
	}
	city = s.Cities[cityID]
	if city.Ruined || city.NexusID != 0 {
		t.Fatalf("the surviving city did not queue its Nexus for rebuilding: %+v",
			city)
	}
	if _, exists := s.Enemies[repulsorID]; !exists {
		t.Fatal("the surviving repulsor was destroyed with the Nexus")
	}
}

func TestUnitExplosionProfilesGiveArtilleryTheLargestBlast(t *testing.T) {
	if len(robotExplosions) != 4 {
		t.Fatalf(
			"there are %d robot blast profiles, want one per role",
			len(robotExplosions),
		)
	}
	if len(enemyExplosions) != 5 {
		t.Fatalf(
			"there are %d rival blast profiles, want one per vehicle kind",
			len(enemyExplosions),
		)
	}
	artillery := enemyExplosions[EnemyArtillery]
	crawler := enemyExplosions[EnemyCrawler]
	if artillery.lightReach <= crawler.lightReach ||
		artillery.sparkCount <= crawler.sparkCount ||
		artillery.smokeSize <= crawler.smokeSize {
		t.Fatal(
			"mobile artillery should have the largest light, sparks and smoke",
		)
	}
}

func TestUnitExplosionMergesWithItsImpact(t *testing.T) {
	f := newFxField()
	death := UnitDeath{ID: 7, RobotKind: RobotBuilder, X: 120, Y: 240}
	f.spawnUnitExplosion(death, nil)
	if len(f.flashes) != 1 {
		t.Fatalf("the standalone explosion made %d flashes, want 1", len(f.flashes))
	}
	fullSparks := len(f.sparks)
	if fullSparks <= robotExplosions[RobotBuilder].sparkCount {
		t.Fatalf("the explosion made only %d particles", fullSparks)
	}

	impact := Shot{
		Kind: ShotBullet, ToX: death.X, ToY: death.Y,
	}
	f.spawnUnitExplosion(death, []Shot{impact})
	if len(f.flashes) != 1 {
		t.Fatalf("the merged explosion added another flash: %d", len(f.flashes))
	}
	if len(f.sparks) >= fullSparks*2 {
		t.Fatalf("the merged explosion duplicated its full particle burst: %d", len(f.sparks))
	}
}

func TestBuildingCollapseThrowsMetalAndMergesWithItsImpact(t *testing.T) {
	destroyed := BuildingDeath{
		Kind: BuildingFactory, Cause: BuildingDestroyed,
		X: 120, Y: 240,
	}
	f := newFxField()
	f.spawnBuildingCollapse(destroyed, nil)
	fullShards := len(f.shards)
	fullSparks := len(f.sparks)
	if fullShards == 0 || fullSparks == 0 || len(f.flashes) != 1 {
		t.Fatalf("the collapse made %d shards, %d sparks and %d flashes",
			fullShards, fullSparks, len(f.flashes))
	}

	demolished := newFxField()
	destroyed.Cause = BuildingDemolished
	demolished.spawnBuildingCollapse(destroyed, nil)
	if len(demolished.shards) >= fullShards ||
		len(demolished.sparks) >= fullSparks {
		t.Fatalf("manual demolition was not quieter: %d shards, %d sparks",
			len(demolished.shards), len(demolished.sparks))
	}

	merged := newFxField()
	destroyed.Cause = BuildingDestroyed
	impact := Shot{Kind: ShotShell, ToX: destroyed.X, ToY: destroyed.Y}
	merged.spawnBuildingCollapse(destroyed, []Shot{impact})
	if len(merged.flashes) != 0 || len(merged.shards) >= fullShards ||
		len(merged.sparks) >= fullSparks {
		t.Fatalf("the shell impact was doubled by the collapse: %d flashes, %d shards, %d sparks",
			len(merged.flashes), len(merged.shards), len(merged.sparks))
	}
}

func TestSmallArmsShareAndRespectTheirRange(t *testing.T) {
	weapons := []struct {
		name string
		fire func(*testing.T, *State, float64)
	}{
		{
			name: "guard post",
			fire: func(t *testing.T, s *State, distance float64) {
				col, row := groundNearCore()
				post := raised(t, s, BuildingGuard, col, row)
				px, py := cellCenterUnits(post.Col, post.Row)
				s.Enemies[900] = Enemy{
					ID: 900, Kind: EnemyRaider,
					X: px + distance, Y: py,
					Health: enemySpecOf(EnemyRaider).health,
				}
				stepGuards(s)
			},
		},
		{
			name: "trooper",
			fire: func(_ *testing.T, s *State, distance float64) {
				r := Robot{
					ID: 900, Kind: RobotCombat,
					Tank: robotTankLiters,
				}
				r.X, r.Y = parkCenter()
				s.Enemies[901] = Enemy{
					ID: 901, Kind: EnemyRaider,
					X: r.X + distance, Y: r.Y,
					Health: enemySpecOf(EnemyRaider).health,
				}
				r.shoot(s)
			},
		},
		{
			name: "crawler",
			fire: func(t *testing.T, s *State, distance float64) {
				col, row := groundNearCore()
				post := raised(t, s, BuildingGuard, col, row)
				px, py := cellCenterUnits(post.Col, post.Row)
				s.Enemies[900] = Enemy{
					ID: 900, Kind: EnemyCrawler,
					X: px + distance, Y: py,
					Health: enemySpecOf(EnemyCrawler).health,
				}
				stepEnemyGuns(s)
			},
		},
		{
			name: "raider",
			fire: func(t *testing.T, s *State, distance float64) {
				col, row := groundNearCore()
				post := raised(t, s, BuildingGuard, col, row)
				px, py := cellCenterUnits(post.Col, post.Row)
				s.Enemies[900] = Enemy{
					ID: 900, Kind: EnemyRaider,
					X: px + distance, Y: py,
					Health: enemySpecOf(EnemyRaider).health,
				}
				stepEnemyGuns(s)
			},
		},
	}

	for _, weapon := range weapons {
		for _, test := range []struct {
			name     string
			distance float64
			wantShot bool
		}{
			{
				name: "at limit", distance: smallArmsRangeUnits,
				wantShot: true,
			},
			{
				name:     "beyond limit",
				distance: smallArmsRangeUnits + 0.01,
				wantShot: false,
			},
		} {
			t.Run(weapon.name+"/"+test.name, func(t *testing.T) {
				s := newGame()
				noRivals(s)
				seedStock(s)
				weapon.fire(t, s, test.distance)
				if got := len(s.Shots) > 0; got != test.wantShot {
					t.Errorf("at %.2f m, fired %v, want %v",
						test.distance, got, test.wantShot)
				}
			})
		}
	}
}

func TestOneGuardPostIsWornDownByTheFirstRaid(t *testing.T) {
	for _, seed := range []int64{0, 1, 2} {
		s := newGameOn(seed)
		s.Raids.Visits = 1
		noRivals(s)
		s.spawnVisit()
		partyID := sortedPartyIDs(s)[0]
		lead := partyMembers(s, partyID)[0]
		coreX, coreY := tileCenterUnits(coreCol, coreRow)
		angle := math.Atan2(lead.Y-coreY, lead.X-coreX)
		targetX := coreX + math.Cos(angle)*(siphonReachUnits-1)
		targetY := coreY + math.Sin(angle)*(siphonReachUnits-1)
		cellsPerTile := unitsPerTile / buildingCell
		firstRow := (coreRow - 1) * cellsPerTile
		lastRow := (coreRow + 2) * cellsPerTile
		firstCol := (coreCol - 1) * cellsPerTile
		lastCol := (coreCol + 2) * cellsPerTile
		// Place the single post on the approach side to the tank.
		postCol, postRow := 0, 0
		postGap := math.Inf(1)
		for row := firstRow; row < lastRow; row++ {
			for col := firstCol; col < lastCol; col++ {
				if !canPlace(s, BuildingGuard, col, row) {
					continue
				}
				x, y := cellCenterUnits(col, row)
				if !inSafeZone(s, x, y) {
					continue
				}
				if gap := math.Hypot(targetX-x, targetY-y); gap < postGap {
					postCol, postRow, postGap = col, row, gap
				}
			}
		}
		if math.IsInf(postGap, 1) {
			t.Fatalf("seed %d: no safe guard site faces the first raid", seed)
		}
		post := raised(t, s, BuildingGuard, postCol, postRow)
		s.Stock = Stock{
			Oil:   startingStockOil - guardCostOil,
			Lilac: startingStockLilac - guardCostLilac,
		}
		postX, postY := cellCenterUnits(post.Col, post.Row)
		nearestEnemyGap := math.Inf(1)
		if !tickUntil(s, 10*60*60, func() bool {
			for _, id := range sortedEnemyIDs(s) {
				enemy := s.Enemies[id]
				gap := math.Hypot(enemy.X-postX, enemy.Y-postY)
				nearestEnemyGap = math.Min(nearestEnemyGap, gap)
			}
			_, active := s.Parties[partyID]
			return !active
		}) {
			t.Fatalf("seed %d: the first raid did not finish", seed)
		}
		remaining := 0.0
		if building, stands := s.Buildings[post.ID]; stands {
			remaining = buildingHealth(BuildingGuard) - building.Damage
		}
		if remaining > buildingHealth(BuildingGuard)*0.2 {
			t.Errorf(
				"seed %d: one guard post retained %.1f / %.1f health; "+
					"nearest enemy was %.1f m away; reports: %+v",
				seed, remaining, buildingHealth(BuildingGuard),
				nearestEnemyGap, s.Reports,
			)
		}
	}
}

// settledCityNexus builds a city for shot tests and returns its Nexus.
func settledCityNexus(t *testing.T, s *State) Enemy {
	t.Helper()
	cityID := s.foundCity(3300, 3000, 0.4)
	city := s.Cities[cityID]
	for range cityBuildOrder {
		s.finishCityBuilding(&city)
	}
	s.Cities[cityID] = city
	partyID := s.NextID
	s.NextID++
	party := Party{ID: partyID, Stage: StageSettled, City: cityID}
	s.Parties[partyID] = party
	city = s.Cities[cityID]
	var core Enemy
	for _, id := range city.BuildingIDs {
		e := s.Enemies[id]
		if e.Kind != EnemyBase {
			continue
		}
		e.Party = partyID
		s.Enemies[id] = e
		core = e
	}
	if core.ID == 0 {
		t.Fatal("the city has no core")
	}
	return core
}

// cellNear returns the cell a spot of the region falls on.
func cellNear(x, y float64) (col, row int) {
	return int(x / buildingCell), int(y / buildingCell)
}

// towardCore returns the spot a way from a point toward the core.
func towardCore(x, y, way float64) (float64, float64) {
	cx, cy := tileCenterUnits(coreCol, coreRow)
	gap := math.Hypot(cx-x, cy-y)
	return x + (cx-x)/gap*way, y + (cy-y)/gap*way
}

func TestABulletFliesBeforeItHurts(t *testing.T) {
	s := newGame()
	seedStock(s)
	col, row := groundNearCore()
	raised(t, s, BuildingGuard, col, row)
	visitNow(s)
	if !tickUntil(s, 60*600, func() bool { return len(s.Shots) > 0 }) {
		t.Fatalf("the guard post never fired")
	}
	var shot Shot
	for _, sh := range s.Shots {
		shot = sh
	}
	scout := s.Enemies[shot.Enemy]
	if shot.Kind != ShotBullet || scout.Health != enemySpecOf(EnemyScout).health {
		t.Fatalf("the shot is %+v and the scout has %v health: a bullet hurts on arrival",
			shot, scout.Health)
	}
	gap := math.Hypot(scout.X-shot.X, scout.Y-shot.Y)
	runTicks(s, int(gap/(bulletSpeed/60))+2)
	if got := s.Enemies[scout.ID].Health; got != scout.Health-guardShotDamage {
		t.Errorf("after the bullet's flight the scout has %v health, want %v",
			got, scout.Health-guardShotDamage)
	}
}

func TestSettledCityDoesNotFireByItself(t *testing.T) {
	s := newGame()
	core := settledCityNexus(t, s)
	if core.Health != enemySpecOf(EnemyBase).health {
		t.Errorf("the city Nexus has %v health", core.Health)
	}
	city := s.Cities[s.Parties[core.Party].City]
	city.Stage = len(cityBuildOrder)
	city.BuildingIDs = []int64{core.ID}
	s.Cities[city.ID] = city
	x, y := towardCore(core.X, core.Y, 800)
	col, row := cellNear(x, y)
	silo := raised(t, s, BuildingSilo, col, row)
	runTicks(s, cityBuildTicks+60*60)
	if len(s.Shots) != 0 || s.Buildings[silo.ID].Damage != 0 {
		t.Fatalf("the city fired without producing a mobile artillery unit")
	}
}

func TestOnlyAMechanicRepairsAndSpendsOil(t *testing.T) {
	s := newGame()
	noRivals(s)
	s.Tech[techRepairID] = false
	col, row := groundNearCore()
	silo := raised(t, s, BuildingSilo, col, row)
	s.hurtBuilding(silo.ID, 120)
	runTicks(s, 60*30)
	if got := s.Buildings[silo.ID].Damage; got != 120 {
		t.Fatalf("ordinary workers repaired the silo to %v damage", got)
	}
	if got := robotCaption(s, s.Robots[1]); got == "repairing" {
		t.Errorf("an ordinary worker still says %q", got)
	}

	home := raised(t, s, BuildingWarFactory, col+2, row)
	s.Stock = Stock{Oil: 1000, Lilac: 2500}
	Apply(s, QueueMechanic{Building: home.ID})
	work := s.Buildings[home.ID]
	if work.Work != mechanicBuildTicks || work.WorkKind != RobotRepair {
		t.Fatalf("the war factory queued %+v, want a mechanic", work)
	}
	if s.Stock.Lilac != 2500-mechanicCostLilac ||
		s.Stock.Oil != 1000-mechanicCostOil {
		t.Fatalf("the mechanic cost left %+v in the stores", s.Stock)
	}
	runTicks(s, mechanicBuildTicks)
	mechanic, found := mechanicForFactory(s, home.ID)
	if !found || mechanic.Health != mechanicHealth || mechanic.Tank != robotTankLiters {
		t.Fatalf("the war factory produced mechanic %+v, found %v", mechanic, found)
	}
	if !tickUntil(s, 60*60, func() bool {
		return s.Buildings[silo.ID].Damage == 0
	}) {
		t.Fatalf("the mechanic left the silo at %v damage",
			s.Buildings[silo.ID].Damage)
	}
	repairLilac, repairOil := buildingRepairCost(silo.Kind, 120)
	if math.Abs(s.Stock.Lilac-(2500-mechanicCostLilac-repairLilac)) > 0.001 ||
		math.Abs(s.Stock.Oil-(1000-mechanicCostOil-repairOil)) > 0.001 {
		t.Errorf("the repair left stores at %+v, want its proportional cost paid",
			s.Stock)
	}
	mechanic = s.Robots[mechanic.ID]
	wantTank := robotTankLiters - 120*repairOilPerPoint
	if math.Abs(mechanic.Tank-wantTank) > 0.001 {
		t.Errorf("the mechanic has %v L left, want %v after repairs",
			mechanic.Tank, wantTank)
	}
}

func TestRepairCostIsHalfTheBuildingCostAtFullIntegrity(t *testing.T) {
	cases := []struct {
		kind   BuildingKind
		health float64
	}{
		{kind: BuildingSilo, health: buildingHealthPoints},
		{kind: BuildingGuard, health: buildingHealthPoints},
		{kind: BuildingProtector, health: protectorHealthPoints},
	}
	for _, test := range cases {
		constructionLilac, constructionOil := buildingCost(test.kind)
		gotLilac, gotOil := buildingRepairCost(test.kind, test.health)
		if gotLilac != constructionLilac*repairCostShare ||
			gotOil != constructionOil*repairCostShare {
			t.Errorf("a full %s repair costs %v kg and %v L; want %v kg and %v L",
				test.kind, gotLilac, gotOil,
				constructionLilac*repairCostShare,
				constructionOil*repairCostShare)
		}
	}
}

func TestRepairKeepsBuildingOilSeparateFromItsTwoCosts(t *testing.T) {
	s := newGame()
	noRivals(s)
	s.Stock = Stock{Oil: 500, Lilac: 1000}
	col, row := groundNearCore()
	protector := raised(t, s, BuildingProtector, col, row)
	protector.Damage = protectorHealthPoints / 2
	s.Buildings[protector.ID] = protector

	x, y := cellCenterUnits(col, row)
	mechanicID := s.spawnRobot(RobotRepair, x, y)
	mechanic := s.Robots[mechanicID]
	for i := 0; i < 60*60 && s.Buildings[protector.ID].Damage > 0; i++ {
		s.mend(protector.ID, &mechanic)
	}

	repairLilac, repairOil := buildingRepairCost(
		BuildingProtector, protectorHealthPoints/2,
	)
	if got := s.Buildings[protector.ID].Damage; got > 1e-9 {
		t.Errorf("the protector still has %v damage after repair", got)
	}
	if math.Abs(s.Stock.Lilac-(1000-repairLilac)) > 0.001 ||
		math.Abs(s.Stock.Oil-(500-repairOil)) > 0.001 {
		t.Errorf("repair left stores at %+v, want its construction share paid",
			s.Stock)
	}
	if got := s.Buildings[protector.ID].Oil; got != protectorCostOil {
		t.Errorf("repair changed the protector's own tank to %v L", got)
	}
	wantMechanicTank := robotTankLiters -
		protectorHealthPoints/2*repairOilPerPoint
	if math.Abs(mechanic.Tank-wantMechanicTank) > 0.001 {
		t.Errorf("the mechanic has %v L, want %v L after its own fuel use",
			mechanic.Tank, wantMechanicTank)
	}
}

func TestRepairWaitsForBothConstructionResources(t *testing.T) {
	cases := []struct {
		name          string
		kind          BuildingKind
		lilac, oil    float64
		wantRepair    float64
		wantLilacCost float64
		wantOilCost   float64
	}{
		{
			name:  "lilac",
			kind:  BuildingSilo,
			lilac: 0.01, oil: 500,
			wantRepair: 0.04, wantLilacCost: 0.01,
		},
		{
			name:  "oil",
			kind:  BuildingGuard,
			lilac: 500, oil: 0.005,
			wantRepair: 0.005 /
				(guardCostOil * repairCostShare / buildingHealthPoints),
			wantLilacCost: 0.005 /
				(guardCostOil * repairCostShare / buildingHealthPoints) *
				(guardCostLilac * repairCostShare / buildingHealthPoints),
			wantOilCost: 0.005,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			s := newGame()
			noRivals(s)
			s.Stock = Stock{Oil: test.oil, Lilac: test.lilac}
			col, row := groundNearCore()
			building := raised(t, s, test.kind, col, row)
			building.Damage = 100
			s.Buildings[building.ID] = building
			x, y := cellCenterUnits(col, row)
			mechanicID := s.spawnRobot(RobotRepair, x, y)
			mechanic := s.Robots[mechanicID]

			s.mend(building.ID, &mechanic)

			gotRepair := 100 - s.Buildings[building.ID].Damage
			if math.Abs(gotRepair-test.wantRepair) > 0.001 {
				t.Errorf("the repair advanced %.6f points, want %.6f",
					gotRepair, test.wantRepair)
			}
			if math.Abs(test.lilac-s.Stock.Lilac-test.wantLilacCost) > 0.001 ||
				math.Abs(test.oil-s.Stock.Oil-test.wantOilCost) > 0.001 {
				t.Errorf("repair spent %.6f kg and %.6f L, want %.6f kg and %.6f L",
					test.lilac-s.Stock.Lilac, test.oil-s.Stock.Oil,
					test.wantLilacCost, test.wantOilCost)
			}
			if s.Stock.Lilac < 0 || s.Stock.Oil < 0 {
				t.Errorf("repair made colony stores negative: %+v", s.Stock)
			}
		})
	}
}

func TestOneMechanicCannotOutrepairContinuousArtillery(t *testing.T) {
	s := newGame()
	noRivals(s)
	col, row := groundNearCore()
	silo := raised(t, s, BuildingSilo, col, row)
	cx, cy := cellCenterUnits(col, row)
	id := s.spawnRobot(RobotRepair, cx, cy)
	r := s.Robots[id]
	r.Tank = robotTankLiters
	s.Robots[id] = r
	s.hurtBuilding(silo.ID, cityArtilleryDamage)
	for i := 0; i < 5; i++ {
		r = s.Robots[id]
		angle := float64(id) * goldenAngle
		r.X = cx + math.Cos(angle)*11
		r.Y = cy + math.Sin(angle)*11
		s.Robots[id] = r
		runTicks(s, cityArtilleryReload)
		s.hurtBuilding(silo.ID, cityArtilleryDamage)
	}
	if got := s.Buildings[silo.ID].Damage; got <= cityArtilleryDamage {
		t.Errorf("the mechanic held damage to %v under continuous fire", got)
	}
}

func TestRivalMobileArtilleryHasA600MeterRange(t *testing.T) {
	for _, test := range []struct {
		name     string
		col      int
		wantShot bool
	}{
		{name: "at limit", col: 24, wantShot: true},
		{name: "beyond limit", col: 25},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := newGame()
			const partyID, artilleryID, buildingID = 30, 31, 32
			s.Parties[partyID] = Party{ID: partyID, Stage: StageRaid}
			s.Enemies[artilleryID] = Enemy{
				ID: artilleryID, Kind: EnemyArtillery, Party: partyID,
				X: 12.5, Y: 12.5,
			}
			s.Buildings[buildingID] = Building{
				ID: buildingID, Kind: BuildingSilo,
				Col: test.col, Row: 0,
			}

			s.fireCityArtillery(s.Enemies[artilleryID])

			if got := len(s.Shots) > 0; got != test.wantShot {
				distance := float64(test.col * buildingCell)
				t.Fatalf("at %.0f m, fired %v, want %v",
					distance, got, test.wantShot)
			}
		})
	}
}

func TestRivalShotsDamageMechanicsButNotWorkers(t *testing.T) {
	s := newGame()
	noRivals(s)
	worker := s.Robots[1]
	workerHealth := worker.Health
	col, row := groundNearCore()
	home := raised(t, s, BuildingWarFactory, col, row)
	cx, cy := cellCenterUnits(col, row)
	id := s.spawnRobot(RobotRepair, cx, cy)
	mechanic := s.Robots[id]
	mechanic.Factory = home.ID
	mechanic.Tank = 80
	s.Robots[id] = mechanic

	s.fire(Shot{
		Kind: ShotBullet, FromX: cx - 1, FromY: cy,
		ToX: cx, ToY: cy, Robot: id, Damage: 15, Rival: true,
	})
	runTicks(s, 2)
	if got := s.Robots[id].Health; got != mechanicHealth-15 {
		t.Fatalf("the rival bullet left the mechanic at %v health", got)
	}
	if got := s.Robots[worker.ID].Health; got != workerHealth {
		t.Errorf("a worker took rival-fire damage: %v to %v",
			workerHealth, got)
	}

	s.fire(Shot{
		Kind: ShotShell, FromX: cx, FromY: cy,
		ToX: cx, ToY: cy, Damage: 30, Rival: true,
	})
	runTicks(s, 1)
	if got := s.Robots[id].Health; got != mechanicHealth-45 {
		t.Fatalf("the rival shell left the mechanic at %v health", got)
	}
	wreckCol, wreckRow := robotCell(s.Robots[id])
	s.hurtColonyUnit(id, mechanicHealth-45)
	if _, alive := s.Robots[id]; alive {
		t.Fatal("the mechanic survived lethal damage")
	}
	pile, found := pileAt(s, wreckCol, wreckRow)
	if !found {
		t.Fatal("the fallen mechanic left no wreck")
	}
	if want := mechanicCostLilac * unitWreckRefund; pile.Lilac != want {
		t.Errorf("the mechanic wreck holds %v kg, want %v", pile.Lilac, want)
	}
	if want := (mechanicCostOil + 80) * unitWreckRefund; pile.Oil != want {
		t.Errorf("the mechanic wreck holds %v L, want %v", pile.Oil, want)
	}
}

func TestOldWarFactoryWorkStillFinishesAsATrooper(t *testing.T) {
	s := newGame()
	col, row := groundNearCore()
	home := raised(t, s, BuildingWarFactory, col, row)
	home.Work = 5
	s.Buildings[home.ID] = home
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var save map[string]json.RawMessage
	if err := json.Unmarshal(data, &save); err != nil {
		t.Fatal(err)
	}
	var buildings map[string]json.RawMessage
	if err := json.Unmarshal(save["Buildings"], &buildings); err != nil {
		t.Fatal(err)
	}
	for id, encoded := range buildings {
		var building map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &building); err != nil {
			t.Fatal(err)
		}
		delete(building, "WorkKind")
		buildings[id], err = json.Marshal(building)
		if err != nil {
			t.Fatal(err)
		}
	}
	save["Buildings"], err = json.Marshal(buildings)
	if err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(save)
	if err != nil {
		t.Fatal(err)
	}
	var loaded State
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	runTicks(&loaded, 5)
	if got := len(squadMembers(&loaded, home.ID)); got != 1 {
		t.Fatalf("an old save's factory produced %d troopers, want one", got)
	}
	if got := mechanicCount(&loaded, home.ID); got != 0 {
		t.Errorf("an old save's factory produced %d mechanics", got)
	}
}

func mechanicForFactory(s *State, factory int64) (Robot, bool) {
	for _, id := range sortedRobotIDs(s) {
		r := s.Robots[id]
		if r.Kind == RobotRepair && r.Factory == factory {
			return r, true
		}
	}
	return Robot{}, false
}

func TestArtilleryShellsWhatTheColonySeesAndCityNexusRefounds(t *testing.T) {
	s := newGame()
	s.Stock = Stock{Oil: 1000, Lilac: 2500}
	delete(s.Robots, 1)
	delete(s.Robots, 2)
	core := settledCityNexus(t, s)
	city := s.Cities[s.Parties[core.Party].City]
	for _, id := range city.BuildingIDs {
		if id != core.ID {
			delete(s.Enemies, id)
		}
	}
	city.BuildingIDs = []int64{core.ID}
	city.Work = cityBuildTicks * 1000
	s.Cities[city.ID] = city
	x, y := towardCore(core.X, core.Y, 1000)
	col, row := cellNear(x, y)
	raised(t, s, BuildingArtillery, col, row)
	runTicks(s, 60*20)
	if len(s.Shots) != 0 || s.Stock.Lilac != 2500 {
		t.Fatalf("the piece fired at a Nexus nobody sees")
	}
	// A spotter within sight of the Nexus, and the shells fly.
	sx, sy := towardCore(core.X, core.Y, sightUnits*0.8)
	s.spawnRobot(RobotBuilder, sx, sy)
	spotter := s.NextID - 1
	keep := func() {
		r := s.Robots[spotter]
		r.X, r.Y = sx, sy
		s.Robots[spotter] = r
	}
	fired := false
	for i := 0; i < 60*60 && !fired; i++ {
		keep()
		runTicks(s, 1)
		fired = len(s.Shots) > 0
	}
	if !fired || s.Stock.Lilac != 2500-artilleryShellLilac {
		t.Fatalf("with a spotter the piece fired %v and the lilac is %v", fired, s.Stock.Lilac)
	}
	health := s.Enemies[core.ID].Health
	flight := int(1000/(shellSpeed/60)) + 5
	for i := 0; i < flight; i++ {
		keep()
		runTicks(s, 1)
	}
	if got := s.Enemies[core.ID].Health; got > health-artilleryShellDamage {
		t.Errorf("after a shell's flight the Nexus has %v health, had %v", got, health)
	}
	for i := 0; i < 60*400; i++ {
		if _, stands := s.Enemies[core.ID]; !stands {
			break
		}
		keep()
		runTicks(s, 1)
	}
	if _, stands := s.Enemies[core.ID]; stands {
		t.Fatalf("the artillery never brought the Nexus down")
	}
	if len(s.BuildingDeaths) != 1 ||
		s.BuildingDeaths[0].RivalKind != EnemyBase {
		t.Errorf("the fallen Nexus has no building collapse event: %+v",
			s.BuildingDeaths)
	}
	found := false
	for _, r := range s.Reports {
		found = found || r.Kind == ReportBaseDown
	}
	if !found {
		t.Errorf("nobody reported the city's fall: %+v", s.Reports)
	}
	cityID := city.ID
	if !s.Cities[cityID].Ruined {
		t.Fatal("the city did not enter its refounding state")
	}
	if !tickUntil(s, int(cityRefoundDelayTicks)+1, func() bool {
		for _, id := range sortedPartyIDs(s) {
			party := s.Parties[id]
			if party.City == cityID && party.CityArrives {
				return true
			}
		}
		return false
	}) {
		t.Fatal("the city did not send a replacement crawler")
	}
	party := s.Parties[sortedPartyIDs(s)[0]]
	if math.Hypot(party.CampX-city.X, party.CampY-city.Y) <
		cityMinSeparation {
		t.Fatal("the replacement city is too close to the Nexus that fell")
	}
	if !tickUntil(s, 60*300, func() bool {
		return !s.Cities[cityID].Ruined
	}) {
		t.Fatal("the replacement crawler did not establish a city")
	}
}

func TestAShellMissesWhoMovedOn(t *testing.T) {
	s := newGame()
	noRivals(s)
	s.Enemies[900] = Enemy{ID: 900, Kind: EnemyRaider, X: 3000, Y: 3000, Health: 100}
	s.fire(Shot{Kind: ShotShell, FromX: 2500, FromY: 2500, ToX: 3000, ToY: 3000, Damage: 60})
	e := s.Enemies[900]
	e.X += shellBlastUnits * 2
	s.Enemies[900] = e
	s.Parties[901] = Party{ID: 901, Stage: StageCamp, Wait: 1 << 40}
	e.Party = 901
	s.Enemies[900] = e
	tickUntil(s, 60*60, func() bool { return len(s.Shots) == 0 })
	if got := s.Enemies[900].Health; got != 100 {
		t.Errorf("a shell hurt a vehicle two blasts away: %v health", got)
	}
}

func TestARaiderShootsAGuardPostAndThePostCanFall(t *testing.T) {
	s := newGame()
	seedStock(s)
	noRivals(s)
	col, row := groundNearCore()
	post := raised(t, s, BuildingGuard, col, row)
	// A raider parked inside its own reach, inside the bubble, with
	// nobody of the colony to shoot at but the post.
	px, py := cellCenterUnits(col, row)
	s.Enemies[900] = Enemy{
		ID: 900, Kind: EnemyRaider, X: px + 80, Y: py, Health: 100,
	}
	var shot Shot
	if !tickUntil(s, 60*60, func() bool {
		for _, sh := range s.Shots {
			if sh.Building == post.ID {
				shot = sh
				return true
			}
		}
		return false
	}) {
		t.Fatalf("the raider never shot the post")
	}
	if shot.Kind != ShotBullet || !shot.Rival || shot.Robot != 0 {
		t.Fatalf("the shot at the post is %+v", shot)
	}
	// Leave the post two hits short: the next two bring it down, and a
	// building's wreck is a pile of half its cost.
	b := s.Buildings[post.ID]
	b.Damage = buildingHealth(BuildingGuard) -
		2*enemySpecOf(EnemyRaider).damage
	s.Buildings[post.ID] = b
	if !tickUntil(s, 60*60, func() bool {
		_, stands := s.Buildings[post.ID]
		return !stands
	}) {
		t.Fatalf("the post never fell under the raider's fire")
	}
	if lastReport(s).Kind != ReportRazed {
		t.Errorf("the report is %q, want the post razed", lastReport(s).Kind)
	}
	if p, littered := pileAt(s, col, row); !littered ||
		p.Lilac != guardCostLilac*wreckRefund {
		t.Errorf("the wreck left %+v, want %v kg", p, guardCostLilac*wreckRefund)
	}
}
