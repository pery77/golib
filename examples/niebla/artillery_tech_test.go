package main

import (
	"encoding/json"
	"os"
	"testing"
)

func artilleryTechTestState() *State {
	s := newGame()
	s.Tech = map[string]bool{
		techIndustryID: true,
		techInfraID:    true,
		techGuardID:    true,
		techFrontierID: true,
		techMobileID:   true,
	}
	return s
}

func addArtilleryTestFactory(s *State, cityID int64) City {
	factoryID := s.NextID
	s.NextID++
	s.Enemies[factoryID] = Enemy{
		ID: factoryID, Kind: EnemyCityFactory, City: cityID,
		Health: enemySpecOf(EnemyCityFactory).health,
	}
	city := City{
		ID: cityID, Stage: len(cityBuildOrder),
		BuildingIDs: []int64{factoryID},
	}
	s.Cities[cityID] = city
	return city
}

func TestArtilleryArrivesAfterThreeNormalAttacksEnd(t *testing.T) {
	s := artilleryTechTestState()
	s.Raids.Visits = 1

	for attack := int64(1); attack <= 3; attack++ {
		party := Party{
			ID: 100 + attack, Stage: StageRaid,
			Siphon: raidSiphonTicks,
		}
		s.Parties[party.ID] = party
		s.endParty(party, ReportLeft, 0, 0, 0)

		if want := attack + 1; s.Raids.Visits != want {
			t.Fatalf("attack %d left %d visits, want %d",
				attack, s.Raids.Visits, want)
		}
		stepTech(s)

		if attack < 3 {
			if dropArrived(s, techArtilleryID) {
				t.Fatalf("artillery arrived after only %d attacks", attack)
			}
			continue
		}
		if !dropArrived(s, techArtilleryID) ||
			!kindUnlocked(s, BuildingArtillery) {
			t.Fatal("artillery did not arrive after the third attack ended")
		}
		if techPending(s) != techArtilleryID {
			t.Fatalf("pending schematic is %q, want artillery",
				techPending(s))
		}
		if opened, arrived := s.Tech[techArtilleryID]; !arrived || opened {
			t.Fatalf("artillery ledger entry is opened=%v, arrived=%v",
				opened, arrived)
		}
	}
}

func TestCityFactoryCompletionAndSortiesDoNotUnlockArtillery(t *testing.T) {
	s := artilleryTechTestState()
	noRivals(s)
	s.Raids.Visits = 3
	cityID := s.foundCity(3500, 3200, 0.4)
	city := s.Cities[cityID]
	city.Oil, city.Lilac = 1000, 2000
	for range len(cityBuildOrder) - 1 {
		s.finishCityBuilding(&city)
	}
	city.Work = 1
	s.Cities[city.ID] = city
	stepCity(s, &city)
	s.Cities[city.ID] = city
	if city.Stage != len(cityBuildOrder) ||
		!cityHasBuilding(s, city, EnemyCityFactory) {
		t.Fatal("the rival factory did not complete in the setup")
	}
	stepTech(s)
	if dropArrived(s, techArtilleryID) {
		t.Fatal("a new save unlocked artillery when the city factory completed")
	}

	for sortie := int64(1); sortie <= int64(raidMaxRaiders+1); sortie++ {
		city = s.Cities[city.ID]
		city.Sorties = sortie - 1
		city.NextSortie = s.Ticks
		s.Cities[city.ID] = city
		stepCity(s, &city)
		s.Cities[city.ID] = city

		partyID := sortedPartyIDs(s)[0]
		party := s.Parties[partyID]
		runTicks(s, party.Size*int(cityUnitBuildTicks))
		party = s.Parties[partyID]
		if party.City != city.ID ||
			party.Artillery != (sortie > int64(raidMaxRaiders)) {
			t.Fatalf("city sortie %d is %+v", sortie, party)
		}
		s.endParty(party, ReportLeft, 0, city.X, city.Y)
		if s.Raids.Visits != 3 {
			t.Fatalf("city sortie %d changed normal visits to %d",
				sortie, s.Raids.Visits)
		}
		stepTech(s)
		if dropArrived(s, techArtilleryID) {
			t.Fatalf("city sortie %d unlocked artillery", sortie)
		}
	}
}

func TestLegacyArtilleryFactoryTriggerMigratesAndPersists(t *testing.T) {
	old := artilleryTechTestState()
	old.Version = 6
	old.Raids.Visits = 2
	addArtilleryTestFactory(old, 90)
	old.migrateState()
	if old.Version != stateVersion ||
		!old.Raids.LegacyArtillery {
		t.Fatalf("old artillery trigger did not migrate: %+v", old.Raids)
	}
	if !dropArrived(old, techArtilleryID) {
		t.Fatal("an old save lost artillery access from its city factory")
	}

	legacyWithoutLedger := newGame()
	legacyWithoutLedger.Version = 6
	legacyWithoutLedger.Ticks = 2
	legacyWithoutLedger.Tech = nil
	addArtilleryTestFactory(legacyWithoutLedger, 92)
	legacyWithoutLedger.migrateState()
	stepTech(legacyWithoutLedger)
	if opened, arrived := legacyWithoutLedger.Tech[techArtilleryID]; !arrived || !opened {
		t.Fatalf("old factory access migrated as opened=%v arrived=%v",
			opened, arrived)
	}
	if techPending(legacyWithoutLedger) == techArtilleryID {
		t.Fatal("migration created an artillery badge for an old factory")
	}

	future := artilleryTechTestState()
	future.Version = 6
	future.Raids.Visits = 2
	future.migrateState()
	data, err := json.Marshal(future)
	if err != nil {
		t.Fatal(err)
	}
	var loaded State
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	loaded.enterRegion()
	addArtilleryTestFactory(&loaded, 91)
	stepTech(&loaded)
	if !dropArrived(&loaded, techArtilleryID) ||
		techPending(&loaded) != techArtilleryID {
		t.Fatal("a migrated save lost its future factory trigger or badge")
	}
}

func TestLegacyArtilleryMigrationDoesNotGrantNewVisitUnlocks(t *testing.T) {
	s := newGame()
	s.Version = 6
	s.Ticks = 2
	s.Raids.Visits = 4
	s.Tech = nil
	s.migrateState()
	stepTech(s)
	if _, arrived := s.Tech[techArtilleryID]; arrived {
		t.Fatal("migration unlocked artillery from the new visit-count rule")
	}
}

func TestLegacyPendingArtilleryBadgeStaysPending(t *testing.T) {
	s := artilleryTechTestState()
	s.Version = 6
	s.Tech[techArtilleryID] = false
	s.migrateState()
	stepTech(s)
	if opened, arrived := s.Tech[techArtilleryID]; !arrived || opened {
		t.Fatalf("the pending old artillery badge became opened=%v arrived=%v",
			opened, arrived)
	}
	if techPending(s) != techArtilleryID {
		t.Fatalf("pending schematic is %q, want artillery", techPending(s))
	}
}

func TestWriteArtilleryTechShotStates(t *testing.T) {
	lockedPath := os.Getenv("NIEBLA_ARTILLERY_LOCKED_SHOT_STATE")
	unlockedPath := os.Getenv("NIEBLA_ARTILLERY_UNLOCKED_SHOT_STATE")
	if lockedPath == "" && unlockedPath == "" {
		t.Skip("set an artillery schematic shot-state path")
	}
	write := func(path string, unlocked bool) {
		if path == "" {
			return
		}
		s := artilleryTechTestState()
		if unlocked {
			s.Raids.Visits = 4
		} else {
			s.Raids.Visits = 3
			coreX, coreY := tileCenterUnits(coreCol, coreRow)
			partyID := int64(100)
			s.Parties[partyID] = Party{
				ID: partyID, Stage: StageRaid,
				Siphon: raidSiphonTicks,
				EntryX: coreX + 1800, EntryY: coreY,
				CampX: coreX + 1500, CampY: coreY,
			}
			for index, kind := range []EnemyKind{
				EnemyCrawler, EnemyRaider,
			} {
				id := int64(101 + index)
				s.Enemies[id] = Enemy{
					ID: id, Kind: kind, Party: partyID,
					X: coreX + 900, Y: coreY + float64(index*30),
					Health: enemySpecOf(kind).health,
				}
			}
		}
		play := newPlayScene(s)
		x, y := techBadgeAt(play)
		t.Logf("artillery badge click: %.0f,%.0f", x, y)
		data, err := json.MarshalIndent(
			map[string]any{"state": s}, "", "  ",
		)
		if err != nil {
			t.Fatalf("the artillery shot state doesn't marshal: %v", err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}
	write(lockedPath, false)
	write(unlockedPath, true)
}
