package main

import (
	"fmt"
	"math"
)

const (
	cityIntervalCycles      = 30
	cityLimit               = 3
	cityRadiusTiles         = 10.0
	cityMinSeparation       = 900.0
	cityBuildTicks          = 45 * 60
	cityUnitBuildTicks      = 30 * 60
	cityAnnouncementTicks   = 60 * 60
	cityOilReserve          = 900.0
	cityLilacReserve        = 1800.0
	cityRebuildTicks        = 60 * 60
	cityRefoundDelayTicks   = 60 * 60
	citySortieCooldownTicks = 90 * 60
	cityUnloadPerSecond     = siphonLitersPerSecond
)

type City struct {
	ID            int64
	NexusID       int64 // Reserved at founding; built after the pylon
	X, Y          float64
	Angle         float64
	AnnounceUntil int64
	Stage         int     // founding-order stages completed; never moves backward
	Work          int64   // ticks left on the first missing building
	MiteDamage    float64 // damage to the city building site
	Oil           float64
	Lilac         float64
	OilDeposit    float64
	LilacDeposit  float64
	NextSortie    int64 // earliest tick a new city force can launch
	Sorties       int64 // complete city forces produced
	BuildingIDs   []int64
	Ruined        bool  // all city structures are gone; a crawler will return
	RefoundAt     int64 // earliest tick a replacement crawler may launch
}

const (
	CityCore     BuildingKind = "citycore"
	CityRepulsor BuildingKind = "cityrepulsor"
	CityOilworks BuildingKind = "cityoilworks"
	CityMine     BuildingKind = "citymine"
	CityFactory  BuildingKind = "cityfactory"
)

var cityBuildOrder = []BuildingKind{
	CityRepulsor, CityCore, CityOilworks, CityMine, CityFactory,
}

func sortedCityIDs(s *State) []int64 {
	ids := make([]int64, 0, len(s.Cities))
	for id := range s.Cities {
		ids = append(ids, id)
	}
	sortIDs(ids)
	return ids
}

func (s *State) migrateSettledCities() {
	if len(s.Cities) != 0 {
		return
	}
	for _, partyID := range sortedPartyIDs(s) {
		if s.Parties[partyID].Stage != StageSettled {
			continue
		}
		for _, enemyID := range sortedEnemyIDs(s) {
			e := s.Enemies[enemyID]
			if e.Party != partyID || e.Kind != EnemyBase {
				continue
			}
			cityID := s.NextID
			s.NextID++
			repulsorID := s.NextID
			s.NextID++
			nexusID := enemyID
			e.Health = clamp64(e.Health/900.0, 0, 1) * enemySpecOf(EnemyBase).health
			e.Reload, e.Aim = 0, 0
			s.Cities = map[int64]City{
				cityID: {
					ID: cityID, X: e.X, Y: e.Y, Stage: 2,
					NexusID: nexusID,
					Work:    cityBuildTicks, OilDeposit: cityOilReserve,
					LilacDeposit: cityLilacReserve,
					BuildingIDs:  []int64{enemyID, repulsorID},
				},
			}
			e.City = cityID
			s.Enemies[enemyID] = e
			s.Enemies[repulsorID] = Enemy{
				ID: repulsorID, Kind: EnemyCityRepulsor,
				X: e.X, Y: e.Y, Health: enemySpecOf(EnemyCityRepulsor).health,
				City: cityID,
			}
			if s.Raids.PressureCity == 0 {
				s.Raids.PressureCity = cityID
			}
			p := s.Parties[partyID]
			p.City = cityID
			s.Parties[partyID] = p
			return
		}
	}
}

func (s *State) migrateCityCrawlerConstruction() {
	for _, id := range sortedCityIDs(s) {
		city := s.Cities[id]
		if city.Ruined {
			continue
		}
		if crawlerID := cityCrawlerID(s, city); crawlerID != 0 {
			found := false
			for _, buildingID := range city.BuildingIDs {
				if buildingID == crawlerID {
					found = true
					break
				}
			}
			if !found {
				city.BuildingIDs = append(city.BuildingIDs, crawlerID)
			}
			s.Cities[id] = city
			continue
		}
		if cityHasStructures(s, city) {
			city.Work = cityBuildTicks
			city.MiteDamage = 0
			s.Cities[id] = city
			continue
		}
		city.Ruined = true
		city.RefoundAt = s.Ticks + cityRefoundDelayTicks
		city.Work = 0
		city.MiteDamage = 0
		city.Oil = 0
		city.Lilac = 0
		city.OilDeposit = 0
		city.LilacDeposit = 0
		city.BuildingIDs = nil
		for _, partyID := range sortedPartyIDs(s) {
			party := s.Parties[partyID]
			if party.City != city.ID || party.CityArrives {
				continue
			}
			party.Stage = StageLeave
			party.Wait = 0
			party.EntryX, party.EntryY = city.X, city.Y
			s.Parties[partyID] = party
		}
		s.Cities[id] = city
	}
}

func (s *State) foundCity(x, y, angle float64) int64 {
	if s.Cities == nil {
		s.Cities = map[int64]City{}
	}
	if s.Enemies == nil {
		s.Enemies = map[int64]Enemy{}
	}
	id := s.NextID
	s.NextID++
	rigID := s.NextID
	s.NextID++
	nexusID := s.NextID
	s.NextID++
	city := City{
		ID: id, NexusID: nexusID, X: x, Y: y, Angle: angle,
		AnnounceUntil: s.Ticks + cityAnnouncementTicks,
		Work:          cityBuildTicks, OilDeposit: cityOilReserve,
		LilacDeposit: cityLilacReserve,
		BuildingIDs:  []int64{rigID},
	}
	rigX, rigY := cityCrawlerPosition(city)
	s.Enemies[rigID] = Enemy{
		ID: rigID, Kind: EnemyCityCrawler, X: rigX, Y: rigY,
		Health: enemySpecOf(EnemyCityCrawler).health, City: id,
	}
	s.Cities[id] = city
	s.report(ReportSettled, 0, x, y)
	return id
}

func (s *State) foundCityOnBearing(angle float64) int64 {
	cx, cy := tileCenterUnits(coreCol, coreRow)
	x := clamp64(
		cx+math.Cos(angle)*cityRadiusTiles*unitsPerTile,
		1,
		float64(regionCols*unitsPerTile)-1,
	)
	y := clamp64(
		cy+math.Sin(angle)*cityRadiusTiles*unitsPerTile,
		1,
		float64(regionRows*unitsPerTile)-1,
	)
	return s.foundCity(x, y, angle)
}

func (s *State) nextCitySpot() (float64, float64, float64, bool) {
	cx, cy := tileCenterUnits(coreCol, coreRow)
	mostX := float64(regionCols*unitsPerTile) - 1
	mostY := float64(regionRows*unitsPerTile) - 1
	for range 32 {
		angle := s.roll() * 2 * math.Pi
		x := clamp64(cx+math.Cos(angle)*cityRadiusTiles*unitsPerTile, 1, mostX)
		y := clamp64(cy+math.Sin(angle)*cityRadiusTiles*unitsPerTile, 1, mostY)
		clear := true
		for _, id := range sortedCityIDs(s) {
			city := s.Cities[id]
			if math.Hypot(x-city.X, y-city.Y) < cityMinSeparation {
				clear = false
				break
			}
		}
		for _, partyID := range sortedPartyIDs(s) {
			party := s.Parties[partyID]
			if !party.CityArrives || party.Stage != StageApproach {
				continue
			}
			if math.Hypot(x-party.CampX, y-party.CampY) < cityMinSeparation {
				clear = false
				break
			}
		}
		if clear {
			return x, y, angle, true
		}
	}
	return 0, 0, 0, false
}

func (s *State) foundCityFromCrawler(
	crawler Enemy, angle float64, cityID int64,
) int64 {
	if s.Cities == nil {
		s.Cities = map[int64]City{}
	}
	if city, ok := s.Cities[cityID]; !ok || !city.Ruined {
		cityID = 0
	}
	if cityID == 0 {
		cityID = s.NextID
		s.NextID++
	}
	nexusID := s.NextID
	s.NextID++
	s.Cities[cityID] = City{
		ID: cityID, NexusID: nexusID,
		X: crawler.X, Y: crawler.Y, Angle: angle,
		AnnounceUntil: s.Ticks + cityAnnouncementTicks,
		Work:          cityBuildTicks,
		OilDeposit:    cityOilReserve, LilacDeposit: cityLilacReserve,
		BuildingIDs: []int64{crawler.ID},
	}
	return cityID
}

func (s *State) spawnCityVisit() bool {
	return s.spawnCityVisitFor(0)
}

func (s *State) spawnCityVisitFor(cityID int64) bool {
	if s.Enemies == nil {
		s.Enemies = map[int64]Enemy{}
	}
	if s.Parties == nil {
		s.Parties = map[int64]Party{}
	}
	campX, campY, angle, ok := s.nextCitySpot()
	if !ok {
		return false
	}
	coreX, coreY := tileCenterUnits(coreCol, coreRow)
	point := func(radius float64) (float64, float64) {
		return clamp64(coreX+math.Cos(angle)*radius*unitsPerTile, 1,
				float64(regionCols*unitsPerTile)-1),
			clamp64(coreY+math.Sin(angle)*radius*unitsPerTile, 1,
				float64(regionRows*unitsPerTile)-1)
	}
	ex, ey := point(entryRadiusTiles)
	partyID := s.NextID
	s.NextID++
	party := Party{
		ID: partyID, City: cityID, Stage: StageApproach,
		EntryX: campX, EntryY: campY, CampX: campX, CampY: campY,
		CityArrives: true,
	}
	s.Parties[partyID] = party
	id := s.NextID
	s.NextID++
	s.Enemies[id] = Enemy{
		ID: id, Kind: EnemyCrawler, Party: partyID,
		X: ex, Y: ey, Health: enemySpecOf(EnemyCrawler).health,
	}
	s.report(ReportCityIncoming, 0, ex, ey)
	return true
}

func stepCities(s *State) {
	for _, id := range sortedCityIDs(s) {
		city := s.Cities[id]
		stepCity(s, &city)
		s.Cities[id] = city
	}
}

func stepCity(s *State, city *City) {
	if city.Ruined {
		if s.Ticks < city.RefoundAt || movingParty(s) {
			return
		}
		if !s.spawnCityVisitFor(city.ID) {
			city.RefoundAt = s.Ticks + fogCycleTicks
		}
		return
	}
	if cityNeedsCrawler(s, *city) {
		if city.Work <= 0 {
			city.Work = cityBuildTicks
		}
		city.Work--
		if city.Work <= 0 {
			s.finishCityCrawler(city)
		}
		return
	}
	if _, building := cityNextBuildingStage(s, *city); building {
		city.Work--
		if city.Work <= 0 {
			s.finishCityBuilding(city)
		}
		return
	}
	if cityHasBuilding(s, *city, EnemyCityOilworks) && city.OilDeposit > 0 {
		amount := math.Min(cityOilExtractPerSecond/60, city.OilDeposit)
		city.OilDeposit -= amount
		city.Oil += amount
	}
	if cityHasBuilding(s, *city, EnemyCityMine) && city.LilacDeposit > 0 {
		amount := math.Min(cityLilacExtractPerSecond/60, city.LilacDeposit)
		city.LilacDeposit -= amount
		city.Lilac += amount
	}
	if movingParty(s) {
		return
	}
	if !cityHasBuilding(s, *city, EnemyCityFactory) || city.NextSortie > s.Ticks ||
		city.Oil < citySortieOil || city.Lilac < citySortieLilac {
		return
	}
	s.startCitySortie(*city)
}

const (
	cityOilExtractPerSecond   = 2.0
	cityLilacExtractPerSecond = 4.0
	citySortieOil             = 120.0
	citySortieLilac           = 240.0
)

type cityBuildingSpecValue struct {
	kind   EnemyKind
	health float64
}

func cityBuildingSpec(kind BuildingKind) cityBuildingSpecValue {
	switch kind {
	case CityCore:
		return cityBuildingSpecValue{EnemyBase, 1200}
	case CityRepulsor:
		return cityBuildingSpecValue{EnemyCityRepulsor, 300}
	case CityOilworks:
		return cityBuildingSpecValue{EnemyCityOilworks, 200}
	case CityMine:
		return cityBuildingSpecValue{EnemyCityMine, 200}
	default:
		return cityBuildingSpecValue{EnemyCityFactory, 250}
	}
}

func (s *State) spawnCitySortie(city City) {
	if city.ID == s.Raids.PressureCity && city.Sorties == 1 {
		s.Raids.PressureSortieStarted = true
	}
	partyID := s.NextID
	s.NextID++
	count := raidersOf(city.Sorties)
	artillery := city.Sorties > int64(raidMaxRaiders)
	size := count + 1
	p := Party{
		ID: partyID, Stage: StageRaid, City: city.ID,
		Siphon: raidSiphonTicks, Artillery: artillery,
		CampX: city.X, CampY: city.Y,
		EntryX: city.X, EntryY: city.Y,
		Size: size,
	}
	if s.Parties == nil {
		s.Parties = map[int64]Party{}
	}
	s.Parties[partyID] = p
	formationStart := 0
	if !artillery {
		id := s.NextID
		s.NextID++
		s.Enemies[id] = Enemy{
			ID: id, Kind: EnemyCrawler, Party: partyID, City: city.ID,
			X: city.X, Y: city.Y,
			Health: enemySpecOf(EnemyCrawler).health,
		}
		formationStart = 1
	}
	for i := 0; i < count; i++ {
		dx, dy := formationOffset(i + formationStart)
		id := s.NextID
		s.NextID++
		s.Enemies[id] = Enemy{
			ID: id, Kind: EnemyRaider, Party: partyID,
			City: city.ID,
			X:    city.X + dx, Y: city.Y + dy,
			Health: enemySpecOf(EnemyRaider).health,
		}
	}
	if artillery {
		id := s.NextID
		s.NextID++
		s.Enemies[id] = Enemy{
			ID: id, Kind: EnemyArtillery, Party: partyID,
			City: city.ID,
			X:    city.X, Y: city.Y,
			Health: enemySpecOf(EnemyArtillery).health,
		}
	}
	s.report(ReportSortie, float64(count), city.X, city.Y)
}

func (s *State) startCitySortie(city City) {
	if s.Parties == nil {
		s.Parties = map[int64]Party{}
	}
	sortie := city.Sorties + 1
	count := raidersOf(sortie)
	artillery := sortie > int64(raidMaxRaiders)
	partyID := s.NextID
	s.NextID++
	s.Parties[partyID] = Party{
		ID: partyID, Stage: StageBuild, City: city.ID,
		CampX: city.X, CampY: city.Y,
		EntryX: city.X, EntryY: city.Y,
		Wait: cityUnitBuildTicks,
		Size: count + 1, Artillery: artillery,
	}
}

func cityPartyUnitNeeded(s *State, party Party) (EnemyKind, bool) {
	var raiders, crawlers, artillery int
	for _, member := range partyMembers(s, party.ID) {
		switch member.Kind {
		case EnemyRaider:
			raiders++
		case EnemyCrawler:
			crawlers++
		case EnemyArtillery:
			artillery++
		}
	}
	if !party.Artillery && crawlers == 0 {
		return EnemyCrawler, true
	}
	if raiders < party.Size-1 {
		return EnemyRaider, true
	}
	if party.Artillery && artillery == 0 {
		return EnemyArtillery, true
	}
	return "", false
}

func (s *State) buildCityPartyUnit(party *Party) bool {
	city, ok := s.Cities[party.City]
	if !ok || city.Ruined || party.Size <= 0 {
		return false
	}
	kind, needed := cityPartyUnitNeeded(s, *party)
	if !needed {
		return true
	}
	unitOil := citySortieOil / float64(party.Size)
	unitLilac := citySortieLilac / float64(party.Size)
	if city.Oil < unitOil || city.Lilac < unitLilac {
		return false
	}
	city.Oil -= unitOil
	city.Lilac -= unitLilac
	s.Cities[city.ID] = city

	x, y := city.X, city.Y
	if kind == EnemyRaider {
		dx, dy := formationOffset(len(partyMembers(s, party.ID)))
		x += dx
		y += dy
	}
	id := s.NextID
	s.NextID++
	s.Enemies[id] = Enemy{
		ID: id, Kind: kind, Party: party.ID, City: city.ID,
		X: x, Y: y, Health: enemySpecOf(kind).health,
	}
	return true
}

func (s *State) launchCitySortie(party *Party) {
	city := s.Cities[party.City]
	city.Sorties++
	city.NextSortie = s.Ticks
	s.Cities[city.ID] = city
	party.Stage = StageRaid
	party.Siphon = raidSiphonTicks
	count := raidersOf(city.Sorties)
	if city.ID == s.Raids.PressureCity && city.Sorties == 1 {
		s.Raids.PressureSortieStarted = true
	}
	s.report(ReportSortie, float64(count), city.X, city.Y)
}

func cityHasBuilding(s *State, city City, kind EnemyKind) bool {
	for _, id := range city.BuildingIDs {
		if e, ok := s.Enemies[id]; ok && e.Kind == kind {
			return true
		}
	}
	return false
}

func cityHasCrawler(s *State, city City) bool {
	return cityCrawlerID(s, city) != 0
}

func cityCrawlerID(s *State, city City) int64 {
	for _, id := range sortedEnemyIDs(s) {
		e := s.Enemies[id]
		if e.City == city.ID && e.Party == 0 &&
			e.Kind == EnemyCityCrawler {
			return id
		}
	}
	return 0
}

func cityNeedsCrawler(s *State, city City) bool {
	return !city.Ruined && !cityHasCrawler(s, city)
}

func cityConstructionSite(
	s *State,
	city City,
) (x, y float64, kind EnemyKind, health float64, building bool) {
	if cityNeedsCrawler(s, city) {
		x, y = cityCrawlerPosition(city)
		return x, y, EnemyCityCrawler,
			enemySpecOf(EnemyCityCrawler).health, true
	}
	stage, needsBuilding := cityNextBuildingStage(s, city)
	if !needsBuilding {
		return 0, 0, "", 0, false
	}
	buildingKind := cityBuildOrder[stage]
	x, y = cityBuildingPosition(city, stage)
	spec := cityBuildingSpec(buildingKind)
	return x, y, spec.kind, spec.health, true
}

func cityConstructionName(s *State, city City) string {
	if cityNeedsCrawler(s, city) {
		return "crawler"
	}
	stage, building := cityNextBuildingStage(s, city)
	if !building {
		return ""
	}
	return cityBuildingName(cityBuildOrder[stage])
}

func cityNextBuildingStage(s *State, city City) (int, bool) {
	if city.Ruined {
		return 0, false
	}
	for stage, kind := range cityBuildOrder {
		if !cityHasBuilding(s, city, cityBuildingSpec(kind).kind) {
			return stage, true
		}
	}
	return 0, false
}

func cityNeedsConstruction(s *State, city City) bool {
	if cityNeedsCrawler(s, city) {
		return true
	}
	_, building := cityNextBuildingStage(s, city)
	return building
}

func cityHasStructures(s *State, city City) bool {
	for _, kind := range cityBuildOrder {
		if cityHasBuilding(s, city, cityBuildingSpec(kind).kind) {
			return true
		}
	}
	return false
}

func cityStageForEnemy(kind EnemyKind) int {
	for stage, building := range cityBuildOrder {
		if cityBuildingSpec(building).kind == kind {
			return stage
		}
	}
	return -1
}

func removeCityBuilding(city *City, id int64) {
	for i, buildingID := range city.BuildingIDs {
		if buildingID != id {
			continue
		}
		city.BuildingIDs = append(city.BuildingIDs[:i],
			city.BuildingIDs[i+1:]...)
		return
	}
}

func (s *State) finishCityBuilding(city *City) {
	if cityNeedsCrawler(s, *city) {
		s.finishCityCrawler(city)
		return
	}
	stage, building := cityNextBuildingStage(s, *city)
	if !building {
		return
	}
	kind := cityBuildOrder[stage]
	eid := int64(0)
	if kind == CityCore {
		if city.NexusID != 0 {
			eid = city.NexusID
		} else {
			eid = s.NextID
			s.NextID++
			city.NexusID = eid
		}
	} else {
		eid = s.NextID
		s.NextID++
	}
	spec := cityBuildingSpec(kind)
	x, y := cityBuildingPosition(*city, stage)
	s.Enemies[eid] = Enemy{
		ID: eid, Kind: spec.kind, X: x, Y: y,
		Health: spec.health, City: city.ID,
	}
	city.BuildingIDs = append(city.BuildingIDs, eid)
	if stage == city.Stage {
		city.Stage++
	}
	city.Work = cityBuildTicks
	city.MiteDamage = 0
	s.report(ReportCityBuilding, 0, city.X, city.Y)
	s.Reports[len(s.Reports)-1].Stage = int64(stage)
	if city.Stage == len(cityBuildOrder) &&
		!cityNeedsConstruction(s, *city) {
		city.NextSortie = s.Ticks
		if s.Raids.PressureCity == city.ID {
			s.Raids.NextAt = s.Ticks +
				int64(cityIntervalCycles)*fogCycleTicks
		}
	}
}

func (s *State) finishCityCrawler(city *City) {
	if !cityNeedsCrawler(s, *city) {
		return
	}
	id := s.NextID
	s.NextID++
	x, y := cityCrawlerPosition(*city)
	s.Enemies[id] = Enemy{
		ID: id, Kind: EnemyCityCrawler, X: x, Y: y,
		Health: enemySpecOf(EnemyCityCrawler).health, City: city.ID,
	}
	city.BuildingIDs = append(city.BuildingIDs, id)
	city.Work = cityBuildTicks
	city.MiteDamage = 0
	s.Cities[city.ID] = *city
	s.report(ReportCityCrawler, 0, city.X, city.Y)
}

func (s *State) finishCitySortie(city *City) {
	if city.Ruined || cityNeedsConstruction(s, *city) || movingParty(s) {
		return
	}
	if !cityHasBuilding(s, *city, EnemyCityFactory) {
		return
	}
	city.Sorties++
	s.spawnCitySortie(*city)
	city.NextSortie = s.Ticks
}

func (s *State) unloadCityParty(p *Party, members []Enemy) {
	city, ok := s.Cities[p.City]
	if !ok {
		return
	}
	carrying := false
	for _, member := range members {
		e := s.Enemies[member.ID]
		amount := math.Min(e.Oil, cityUnloadPerSecond/60)
		e.Oil -= amount
		city.Oil += amount
		carrying = carrying || e.Oil > 0
		s.Enemies[e.ID] = e
	}
	if carrying {
		s.Cities[city.ID] = city
		return
	}
	for _, member := range members {
		e := s.Enemies[member.ID]
		e.Oil = 0
		s.Enemies[e.ID] = e
	}
	s.Cities[city.ID] = city
	if p.Size > 0 && len(members) < p.Size {
		p.Stage = StageRebuild
		p.Wait = cityUnitBuildTicks
		return
	}
	p.Stage = StageRegroup
	p.Wait = citySortieCooldownTicks
	p.Siphon = raidSiphonTicks
	city.NextSortie = s.Ticks + citySortieCooldownTicks
	s.Cities[city.ID] = city
}

func movingParty(s *State) bool {
	for _, id := range sortedPartyIDs(s) {
		if s.Parties[id].Stage != StageSettled {
			return true
		}
	}
	return false
}

func cityHasRepulsor(s *State, city City) bool {
	for _, id := range city.BuildingIDs {
		if e, ok := s.Enemies[id]; ok && e.Kind == EnemyCityRepulsor {
			return true
		}
	}
	return false
}

func cityPartyStatus(s *State, city int64) string {
	for _, id := range sortedPartyIDs(s) {
		p := s.Parties[id]
		if p.City != city {
			continue
		}
		switch p.Stage {
		case StageBuild:
			return "assembling a force"
		case StageRaid:
			return "attacking"
		case StageLeave:
			return "returning"
		case StageUnload:
			return "unloading stolen oil"
		case StageRebuild:
			return "completing the squad"
		case StageRegroup:
			return "regrouping before the next attack"
		}
	}
	return ""
}

func cityBuildingPosition(city City, stage int) (float64, float64) {
	spots := [][2]float64{{0, 0}, {0, -80}, {-75, 0}, {75, 0}, {0, 80}}
	offset := spots[stage]
	return cityOffset(city, offset[0], offset[1])
}

func cityCrawlerPosition(city City) (float64, float64) {
	return cityOffset(city, 0, 150)
}

func cityOffset(city City, dx, dy float64) (float64, float64) {
	cos, sin := math.Cos(city.Angle), math.Sin(city.Angle)
	return city.X + dx*cos - dy*sin,
		city.Y + dx*sin + dy*cos
}

func cityBuildingName(kind BuildingKind) string {
	switch kind {
	case CityRepulsor:
		return "antimist post"
	case CityCore:
		return "Nexus"
	case CityOilworks:
		return "oil extractor"
	case CityMine:
		return "lilac mine"
	case CityFactory:
		return "military factory"
	}
	return string(kind)
}

func cityBuildingCaption(s *State, e Enemy) string {
	switch e.Kind {
	case EnemyCityCrawler:
		return "city construction crawler"
	case EnemyBase:
		return "Nexus"
	case EnemyCityRepulsor:
		return "repelling the fog"
	case EnemyCityOilworks:
		return "extracting oil"
	case EnemyCityMine:
		return "mining lilac"
	case EnemyCityFactory:
		return "building the next force"
	}
	if city, ok := s.Cities[e.City]; ok {
		if name := cityConstructionName(s, city); name != "" {
			return fmt.Sprintf("city construction: %s",
				name)
		}
	}
	return "city structure"
}
