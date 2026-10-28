package main

import (
	"encoding/json"
	"os"
	"testing"

	"golib"
)

func TestRepairProtocolRequiresBuildingDamageAndFirstRaidEnd(t *testing.T) {
	s := newGame()
	s.Raids.Visits = 1
	s.Raids.RivalBuildingHit = true
	s.Raids.PressureSortieStarted = true
	s.Raids.PressureSortieResolved = true
	if repairProtocolTrigger(s) {
		t.Fatal("the repair protocol arrived before the first raid ended")
	}

	s.Raids.Visits = 2
	s.Raids.RivalBuildingHit = false
	if repairProtocolTrigger(s) {
		t.Fatal("the repair protocol arrived without rival building damage")
	}

	s.Raids.RivalBuildingHit = true
	s.Raids.PressureSortieResolved = false
	if repairProtocolTrigger(s) {
		t.Fatal("the repair protocol arrived before the first city force's lull")
	}
}

func TestRepairProtocolWaitsUntilTheCityForceStopsAttacking(t *testing.T) {
	s := newGame()
	s.Raids.Visits = 2
	s.Raids.RivalBuildingHit = true
	s.Raids.PressureCity = 700
	s.Raids.PressureSortieStarted = true
	s.Raids.PressureSortieResolved = true

	for _, stage := range []PartyStage{
		StageApproach, StageCamp, StageRaid, StageLeave,
	} {
		s.Parties = map[int64]Party{
			701: {ID: 701, City: 700, Stage: stage},
		}
		if repairProtocolTrigger(s) {
			t.Errorf("the protocol arrived while the force was %s", stage)
		}
	}

	for _, stage := range []PartyStage{
		StageUnload, StageRebuild, StageRegroup,
	} {
		s.Parties = map[int64]Party{
			701: {ID: 701, City: 700, Stage: stage},
		}
		if !repairProtocolTrigger(s) {
			t.Errorf("the protocol waited during the %s lull", stage)
		}
	}
}

func TestRepairProtocolFallbackWaitsForTwelveMinutesAndNoSortie(t *testing.T) {
	s := newGame()
	s.Raids.Visits = 2
	s.Raids.RivalBuildingHit = true
	s.Ticks = techRepairFallbackTicks - 1
	if repairProtocolTrigger(s) {
		t.Fatal("the fallback arrived before minute twelve")
	}

	s.Ticks = techRepairFallbackTicks
	if !repairProtocolTrigger(s) {
		t.Fatal("the fallback did not arrive at minute twelve")
	}

	s.Parties[701] = Party{ID: 701, Stage: StageRaid}
	if repairProtocolTrigger(s) {
		t.Fatal("the fallback interrupted an active rival attack")
	}
}

func TestRepairProtocolFallbackDefersToAReadyFirstSortie(t *testing.T) {
	s := newGame()
	s.Raids.Visits = 2
	s.Raids.RivalBuildingHit = true
	s.Ticks = techRepairFallbackTicks
	cityID := s.foundCity(3000, 3000, 0)
	city := s.Cities[cityID]
	for range cityBuildOrder {
		s.finishCityBuilding(&city)
	}
	city.Oil, city.Lilac = citySortieOil, citySortieLilac
	s.Cities[cityID] = city
	s.Raids.PressureCity = cityID

	if !pressureCitySortieReady(s) {
		t.Fatal("the pressure city is ready to launch its first force")
	}
	if repairProtocolTrigger(s) {
		t.Fatal("the fallback unlocked immediately before the first sortie")
	}

	city.Oil, city.Lilac = 0, 0
	s.Cities[cityID] = city
	if !repairProtocolTrigger(s) {
		t.Fatal("the minute-twelve fallback waited on an unfunded sortie")
	}
}

func TestPressureSortieLullIsRemembered(t *testing.T) {
	s := newGame()
	s.Raids.PressureCity = 700
	city := City{ID: 700, Sorties: 1}
	s.Cities[city.ID] = city
	s.spawnCitySortie(city)
	if !s.Raids.PressureSortieStarted {
		t.Fatal("the pressure city's first sortie was not recorded")
	}
	partyID := sortedPartyIDs(s)[0]
	party := s.Parties[partyID]
	party.Stage = StageUnload
	s.Parties[party.ID] = party
	stepParty(s, party)
	if !s.Raids.PressureSortieResolved {
		t.Fatal("the pressure sortie's unload did not mark its lull")
	}

	s.endParty(Party{ID: 703, City: 700}, ReportDestroyed, 0, 0, 0)
	if !s.Raids.PressureSortieResolved {
		t.Fatal("a destroyed pressure sortie lost its resolved marker")
	}
}

func TestRivalBuildingHitSurvivesDestructionAndIgnoresFogDamage(t *testing.T) {
	s := newGame()
	col, row := groundNearCore()
	post := raised(t, s, BuildingGuard, col, row)
	x, y := cellCenterUnits(col, row)
	s.fire(Shot{
		Kind: ShotBullet, FromX: x - 1, FromY: y,
		ToX: x, ToY: y, Building: post.ID,
		Damage: buildingHealth(BuildingGuard), Rival: true,
	})
	runTicks(s, 1)
	if _, stands := s.Buildings[post.ID]; stands {
		t.Fatal("the lethal rival shot left the building standing")
	}
	if !s.Raids.RivalBuildingHit {
		t.Fatal("the building's destruction erased the rival-hit marker")
	}

	other := newGame()
	silo := raised(t, other, BuildingSilo, col, row)
	x, y = cellCenterUnits(col, row)
	other.fire(Shot{
		Kind: ShotShell, FromX: x, FromY: y, ToX: x, ToY: y,
		Damage: 1, Rival: true,
	})
	runTicks(other, 1)
	if !other.Raids.RivalBuildingHit || other.Buildings[silo.ID].Damage != 1 {
		t.Fatal("a rival shell did not record building damage")
	}

	fog := newGame()
	fogSilo := raised(t, fog, BuildingSilo, col, row)
	fog.hurtBuilding(fogSilo.ID, 10)
	if fog.Raids.RivalBuildingHit {
		t.Fatal("non-rival building damage counted as a rival hit")
	}
}

func TestWarFactoryRejectsMechanicOrdersUntilTheProtocolArrives(t *testing.T) {
	s := newGame()
	s.Stock = Stock{Oil: 1000, Lilac: 2500}
	col, row := groundNearCore()
	home := raised(t, s, BuildingWarFactory, col, row)
	stock := s.Stock
	Apply(s, QueueMechanic{Building: home.ID})
	Apply(s, QueueRobot{Building: home.ID, Kind: RobotRepair})
	if s.Stock != stock || s.Buildings[home.ID].Work != 0 {
		t.Fatal("a locked mechanic order charged stores or started production")
	}

	s.Tech[techRepairID] = false
	Apply(s, QueueRobot{Building: home.ID, Kind: RobotRepair})
	if s.Buildings[home.ID].WorkKind != RobotRepair ||
		s.Buildings[home.ID].Work != mechanicBuildTicks {
		t.Fatal("an arrived repair protocol did not allow mechanic production")
	}
	if s.Stock.Lilac != stock.Lilac-mechanicCostLilac ||
		s.Stock.Oil != stock.Oil-mechanicCostOil {
		t.Fatal("the mechanic order did not charge its normal cost")
	}
}

func TestOldWarFactorySavesKeepTheirRepairCapability(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup func(*State)
	}{
		{
			name: "mobile schematics",
			setup: func(s *State) {
				s.Tech = map[string]bool{techMobileID: false}
			},
		},
		{
			name: "factory before schematic ledger",
			setup: func(s *State) {
				s.Tech = nil
				col, row := groundNearCore()
				s.raise(BuildingWarFactory, col, row)
			},
		},
		{
			name: "built factory",
			setup: func(s *State) {
				col, row := groundNearCore()
				s.raise(BuildingWarFactory, col, row)
			},
		},
		{
			name: "mechanic in production",
			setup: func(s *State) {
				col, row := groundNearCore()
				s.raise(BuildingWarFactory, col, row)
				factory, _ := buildingAt(s, col, row)
				factory.WorkKind = RobotRepair
				factory.Work = 10
				s.Buildings[factory.ID] = factory
			},
		},
		{
			name: "built mechanic",
			setup: func(s *State) {
				s.spawnRobot(RobotRepair, 10, 10)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := newGame()
			s.Version = 4
			test.setup(s)
			s.migrateState()
			if !repairProtocolUnlocked(s) {
				t.Fatal("the repair capability was revoked by migration")
			}
		})
	}

	s := newGame()
	s.Version = 4
	s.Tech = map[string]bool{techIndustryID: true}
	s.migrateState()
	if repairProtocolUnlocked(s) {
		t.Fatal("migration granted mechanic access before the war factory")
	}
	if _, arrived := s.Tech[techRepairID]; arrived {
		t.Fatal("migration queued new repair schematics for an old save")
	}
}

func TestRepairBattleMarkersSurviveSaveRoundTrip(t *testing.T) {
	s := newGame()
	s.Raids.RivalBuildingHit = true
	s.Raids.PressureSortieStarted = true
	s.Raids.PressureSortieResolved = true
	s.Raids.LegacyRepairUnlocked = true

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var loaded State
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	if !loaded.Raids.RivalBuildingHit ||
		!loaded.Raids.PressureSortieStarted ||
		!loaded.Raids.PressureSortieResolved ||
		!loaded.Raids.LegacyRepairUnlocked {
		t.Fatalf("repair markers changed across a save round trip: %+v",
			loaded.Raids)
	}
}

func TestWriteRepairCardShotStates(t *testing.T) {
	lockedPath := os.Getenv("NIEBLA_REPAIR_LOCKED_SHOT_STATE")
	unlockedPath := os.Getenv("NIEBLA_REPAIR_UNLOCKED_SHOT_STATE")
	shortPath := os.Getenv("NIEBLA_REPAIR_SHORT_SHOT_STATE")
	if lockedPath == "" && unlockedPath == "" && shortPath == "" {
		t.Skip("set a repair card shot-state path to write a fixture")
	}
	write := func(path string, unlocked, short bool) {
		if path == "" {
			return
		}
		s := newGame()
		s.Tech = map[string]bool{
			techIndustryID: true, techInfraID: true,
			techGuardID: true, techFrontierID: true,
			techMobileID: true, techArtilleryID: true,
		}
		if unlocked {
			s.Tech[techRepairID] = true
		}
		s.Stock = Stock{Oil: 1000, Lilac: 2500}
		if short {
			s.Stock = Stock{
				Oil:   mechanicCostOil - 1,
				Lilac: mechanicCostLilac - 1,
			}
		}
		col, row := groundNearCore()
		factory := raised(t, s, BuildingWarFactory, col, row)
		scene := newPlayScene(s)
		gx, gy := projectBuilding(factory)
		click := scene.camera.ToScreen(golib.Vector2{X: gx, Y: gy})
		t.Logf("war factory card click: %.0f,%.0f", click.X, click.Y)
		data, err := json.MarshalIndent(map[string]any{"state": s}, "", "  ")
		if err != nil {
			t.Fatalf("the repair card state doesn't marshal: %v", err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}
	write(lockedPath, false, false)
	write(unlockedPath, true, false)
	write(shortPath, true, true)
}
