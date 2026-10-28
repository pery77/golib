package main

import (
	"math"
	"sort"
)

// The rivals' law, after DESIGN.md's "Enemies": other people live in the
// fog, and their repulsors drink oil like the colony's. They come for
// it. The introduction is a scout and one raiding party. After that,
// crawlers arrive to establish cities, which build extractors and a war
// factory that sends forces from the city (sim_cities.go).
//
// Mites wear exposed vehicles and city structures. Moving keeps a unit
// safe from hull damage; one that stands still loses hull after a grace.

// Tuning: the rivals' numbers, with units in the name.
const (
	raidFirstScoutTicks = 60 * 60 // ticks before the scout: 1 min
	raidFollowupTicks   = 60 * 60 // ticks between pressure parties: 1 min

	campPrepareTicks    = 3 * 60 * 60 // ticks a later raid camps before it moves: 3 min
	campPrepareShortens = 0.8         // each raid camps this much of the last one's time
	campPrepareMinTicks = 45 * 60     // ticks of camp at the least: 45 s
	campRadiusTiles     = 9.3         // tiles from the core to a camp: the clear ground's edge
	entryRadiusTiles    = 12.3        // tiles from the core to where a party comes in

	raidFirstRaiders = 1 // raiders of the first intro attack
	raidMaxRaiders   = 4 // raiders of a raid at the most

	siphonReachUnits      = 40.0    // u from a tank's middle to a party siphoning it
	siphonLitersPerSecond = 3.0     // L/s each vehicle draws
	raidSiphonTicks       = 60 * 60 // ticks a party siphons at the most: a minute
	smallArmsRangeUnits   = 130.0   // u; every light weapon's reach

	reportsKept = 12 // reports the state remembers

	cityArtilleryRangeUnits = 600.0 // u
	cityArtilleryReload     = 6 * 60
	cityArtilleryDamage     = 45.0
)

// EnemyKind names a rival vehicle or city structure.
type EnemyKind string

const (
	EnemyScout        EnemyKind = "scout"   // alone, under a small repulsor of its own
	EnemyCrawler      EnemyKind = "crawler" // carries the party's repulsor
	EnemyRaider       EnemyKind = "raider"  // a tanker: it lives under the crawler's bubble
	EnemyBase         EnemyKind = "base"
	EnemyArtillery    EnemyKind = "enemyartillery"
	EnemyCityCrawler  EnemyKind = "citycrawler"
	EnemyCityRepulsor EnemyKind = "cityrepulsor"
	EnemyCityOilworks EnemyKind = "cityoilworks"
	EnemyCityMine     EnemyKind = "citymine"
	EnemyCityFactory  EnemyKind = "cityfactory"
)

// enemySpec is a rival entity's physical and combat properties.
type enemySpec struct {
	speed     float64 // u/s
	health    float64
	bubble    float64 // u of repulsor radius; 0 carries none
	oilCap    float64 // L it can steal
	lootOil   float64 // L its wreck drops, besides what it stole
	lootLilac float64 // kg its wreck drops
	damage    float64 // a shot at defenders and guard posts; 0 means no gun
	reload    int64   // ticks between two shots
	gunRange  float64 // u
}

func enemySpecOf(kind EnemyKind) enemySpec {
	switch kind {
	case EnemyScout:
		return enemySpec{24, 60, 40, 25, 2, 5, 0, 0, 0}
	case EnemyCrawler:
		return enemySpec{
			12, 300, 120, 0, 10, 25, 8, 50, smallArmsRangeUnits,
		}
	case EnemyBase:
		return enemySpec{0, 1200, 0, 0, 60, 150, 0, 0, 0}
	case EnemyArtillery:
		return enemySpec{12, 240, 170, 0, 10, 35, 0, 0, 0}
	case EnemyCityCrawler:
		return enemySpec{0, 300, 0, 0, 10, 25, 0, 0, 0}
	case EnemyCityRepulsor:
		return enemySpec{0, 300, 170, 0, 0, 0, 0, 0, 0}
	case EnemyCityOilworks, EnemyCityMine:
		return enemySpec{0, 200, 0, 0, 0, 0, 0, 0, 0}
	case EnemyCityFactory:
		return enemySpec{0, 250, 0, 0, 0, 0, 0, 0, 0}
	}
	return enemySpec{
		20, 100, 0, 60, 4, 8, 5, 40, smallArmsRangeUnits,
	}
}

// Enemy is one rival vehicle. Like a robot it carries no plan: its party's
// stage says what it is doing.
type Enemy struct {
	ID         int64
	Kind       EnemyKind
	Party      int64
	X, Y       float64 // units
	Facing     uint8   // screen-facing octant; zero points right
	Health     float64
	Oil        float64 // liters it stole
	StillTicks int64   // ticks spent standing in mite exposure
	Reload     int64   // ticks until its gun's next shot (sim_squads.go)
	Aim        int64   // the trooper its last shot went to
	City       int64   // owning city for its static structures
}

// PartyStage is where a visit has got to.
type PartyStage string

const (
	StageApproach PartyStage = "approach" // driving to its camp
	StageCamp     PartyStage = "camp"     // getting ready
	StageRaid     PartyStage = "raid"     // driving to a tank and siphoning it
	StageLeave    PartyStage = "leave"    // driving back out
	StageUnload   PartyStage = "unload"   // unloading at the city
	StageBuild    PartyStage = "build"    // assembling a city force
	StageRebuild  PartyStage = "rebuild"  // completing a damaged city force
	StageRegroup  PartyStage = "regroup"  // full force resting before its next raid
	StageSettled  PartyStage = "settled"
)

// Party is one visit: the vehicles that came together.
type Party struct {
	ID             int64
	Stage          PartyStage
	EntryX, EntryY float64 // where it came in, and where it leaves
	CampX, CampY   float64
	Wait           int64 // ticks of camp, rebuild or regroup left
	Siphon         int64 // ticks of siphoning left before it gives up
	City           int64 // city that produced this sortie; 0 for introduction visits
	Artillery      bool  // sortie includes mobile artillery
	CityArrives    bool  // this crawler is founding a city, not a raid
	Size           int   // city sortie's full vehicle count, before any losses
}

// Raids is the rivals' clock: ended visits, the next arrival, the saved
// bearing and the first city whose construction drives the pressure loop.
type Raids struct {
	Visits                 int64
	NextAt                 int64
	FirstBearing           float64
	BearingKnown           bool
	ScoutClearedCore       bool  // first scout crossed the core bubble outward
	ScoutClearedCoreAt     int64 // tick of that crossing; zero when unknown
	LegacyFrontierClock    bool  // old saves retain the 5:30 frontier unlock
	PressureCity           int64 // the first city, founded with the second visit
	RivalBuildingHit       bool  // any rival shot has damaged a colony building
	PressureSortieStarted  bool  // the pressure city's first force was produced
	PressureSortieResolved bool  // its first force reached a lull or was destroyed
	LegacyRepairUnlocked   bool  // an old save already had mechanic production
	LegacyArtillery        bool  // old saves use the rival-factory trigger
}

// Mark is what a scout paints on the ground before it leaves.
type Mark struct {
	ID   int64
	X, Y float64 // units
	Oil  float64 // liters the scout took
}

// ReportKind names something the rivals did that the player is told.
type ReportKind string

const (
	ReportScout        ReportKind = "scout"     // a scout siphoned and left its mark
	ReportCamp         ReportKind = "camp"      // a party camped
	ReportRaid         ReportKind = "raid"      // a camped party moves in
	ReportLeft         ReportKind = "left"      // a party got away
	ReportDestroyed    ReportKind = "destroyed" // a party was lost to the last vehicle
	ReportSettled      ReportKind = "settled"   // a party dug in: a base stands in the region
	ReportGun          ReportKind = "gun"
	ReportBaseDown     ReportKind = "basedown" // a base fell
	ReportRazed        ReportKind = "razed"    // a shell brought a building down
	ReportPumpEaten    ReportKind = "pumpeaten"
	ReportMiteEaten    ReportKind = "miteeaten"
	ReportSortie       ReportKind = "sortie"
	ReportCityIncoming ReportKind = "cityincoming"
	ReportCityBuilding ReportKind = "citybuilding"
	ReportCityCrawler  ReportKind = "citycrawler"
)

// Report is one line of news, written by the simulation and worded by
// the view.
type Report struct {
	Tick  int64
	Kind  ReportKind
	Oil   float64 // liters stolen, where the kind counts them
	Stage int64   // city building index, for ReportCityBuilding
	X, Y  float64 // where it happened, in units
}

// roll returns the state's next random number, from 0 to 1: gameplay
// randomness lives in the state, so a save reproduces its future.
func (s *State) roll() float64 {
	r := rng{s: uint64(s.Seed) ^ (s.Rolls+1)*0xd1342543de82ef95}
	s.Rolls++
	return r.float()
}

func (s *State) report(kind ReportKind, oil, x, y float64) {
	s.Reports = append(s.Reports, Report{
		Tick: s.Ticks, Kind: kind, Oil: oil, X: x, Y: y,
	})
	if extra := len(s.Reports) - reportsKept; extra > 0 {
		s.Reports = append([]Report(nil), s.Reports[extra:]...)
	}
}

func sortIDs(ids []int64) {
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
}

func sortedEnemyIDs(s *State) []int64 {
	ids := make([]int64, 0, len(s.Enemies))
	for id := range s.Enemies {
		ids = append(ids, id)
	}
	sortIDs(ids)
	return ids
}

func sortedPartyIDs(s *State) []int64 {
	ids := make([]int64, 0, len(s.Parties))
	for id := range s.Parties {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func sortedMarkIDs(s *State) []int64 {
	ids := make([]int64, 0, len(s.Marks))
	for id := range s.Marks {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// partyMembers lists a party's living vehicles, the one that leads it
// first: the oldest that carries a repulsor, or the oldest of all when
// none does.
func partyMembers(s *State, party int64) []Enemy {
	var members []Enemy
	for _, id := range sortedEnemyIDs(s) {
		if e := s.Enemies[id]; e.Party == party {
			members = append(members, e)
		}
	}
	for i, e := range members {
		if enemySpecOf(e.Kind).bubble > 0 {
			lead := members[i]
			copy(members[1:i+1], members[:i])
			members[0] = lead
			break
		}
	}
	return members
}

// stepEnemies moves the rivals one tick forward: the clock that sends
// them, each party's stage, and the fog's due.
func stepEnemies(s *State) {
	positions := make(map[int64]PipePoint, len(s.Enemies))
	for _, id := range sortedEnemyIDs(s) {
		e := s.Enemies[id]
		positions[id] = PipePoint{X: e.X, Y: e.Y}
	}
	stepCities(s)
	stepRaids(s)
	for _, id := range sortedPartyIDs(s) {
		stepParty(s, s.Parties[id])
	}
	stepExposure(s, positions)
	stepEnemyGuns(s)
}

// stepRaids sends the next visit when its tick comes. One party on the
// move at a time: the clock for the next starts when this one ends. A
// settled one is no visit any more, and the raids go on around it.
func stepRaids(s *State) {
	for _, id := range sortedPartyIDs(s) {
		p := s.Parties[id]
		if p.Stage != StageSettled {
			return
		}
	}
	// A save from before the rivals wakes up with their first visit ahead.
	if s.Raids.NextAt == 0 {
		s.Raids.NextAt = s.Ticks + raidFirstScoutTicks
	}
	if s.Ticks < s.Raids.NextAt {
		return
	}
	if s.Raids.Visits == 0 ||
		(s.Raids.PressureCity == 0 && s.Raids.Visits == 1) {
		s.spawnVisit()
		return
	}
	if city, ok := s.Cities[s.Raids.PressureCity]; ok &&
		!city.Ruined && cityNeedsConstruction(s, city) {
		s.spawnVisit()
		return
	}
	if len(s.Cities) >= cityLimit {
		s.Raids.NextAt = s.Ticks +
			int64(cityIntervalCycles)*fogCycleTicks
		return
	}
	if s.spawnCityVisit() {
		s.Raids.NextAt = s.Ticks + int64(cityIntervalCycles)*fogCycleTicks
	} else {
		s.Raids.NextAt = s.Ticks + fogCycleTicks
	}
}

// settled reports whether a base stands in the region.
func settled(s *State) bool {
	for _, id := range sortedPartyIDs(s) {
		p := s.Parties[id]
		if p.Stage == StageSettled {
			return true
		}
	}
	return false
}

// raidersOf returns how many raiders a visit brings.
func raidersOf(visit int64) int {
	if visit < raidFirstRaiders {
		return raidFirstRaiders
	}
	if visit > raidMaxRaiders {
		return raidMaxRaiders
	}
	return int(visit)
}

// prepareTicks returns how long a visit camps before it moves.
func prepareTicks(visit int64) int64 {
	ticks := float64(campPrepareTicks) * math.Pow(campPrepareShortens, float64(visit-1))
	return int64(math.Max(campPrepareMinTicks, ticks))
}

// spawnVisit brings the next party in at a bearing the state rolls: the
// first visit is a scout that goes straight for the oil, the rest are a
// crawler and its raiders, which camp before later attacks.
func (s *State) spawnVisit() {
	// A save from before the rivals loads with no tables for them.
	if s.Enemies == nil {
		s.Enemies = map[int64]Enemy{}
	}
	if s.Parties == nil {
		s.Parties = map[int64]Party{}
	}
	angle := s.Raids.FirstBearing
	if s.Raids.Visits == 0 || !s.Raids.BearingKnown {
		angle = s.roll() * 2 * math.Pi
		if s.Raids.Visits == 0 {
			s.Raids.FirstBearing = angle
			s.Raids.BearingKnown = true
		}
	}
	cx, cy := tileCenterUnits(coreCol, coreRow)
	at := func(tiles float64) (x, y float64) {
		most := float64(regionCols*unitsPerTile) - 1
		return clamp64(cx+math.Cos(angle)*tiles*unitsPerTile, 1, most),
			clamp64(cy+math.Sin(angle)*tiles*unitsPerTile, 1, most)
	}
	p := Party{ID: s.NextID, Stage: StageRaid, Siphon: raidSiphonTicks}
	s.NextID++
	p.EntryX, p.EntryY = at(entryRadiusTiles)
	p.CampX, p.CampY = at(campRadiusTiles)
	kinds := []EnemyKind{EnemyScout}
	if visit := s.Raids.Visits; visit > 0 {
		p.Stage, p.Wait = StageApproach, prepareTicks(visit)
		if visit == 1 {
			p.Wait = 0
		}
		kinds = []EnemyKind{EnemyCrawler}
		for i := 0; i < raidersOf(visit); i++ {
			kinds = append(kinds, EnemyRaider)
		}
	}
	s.Parties[p.ID] = p
	for i, kind := range kinds {
		dx, dy := formationOffset(i)
		s.Enemies[s.NextID] = Enemy{
			ID: s.NextID, Kind: kind, Party: p.ID,
			X: p.EntryX + dx, Y: p.EntryY + dy,
			Health: enemySpecOf(kind).health,
		}
		s.NextID++
	}
	if s.Raids.Visits == 1 && s.Raids.PressureCity == 0 {
		s.Raids.PressureCity = s.foundCityOnBearing(angle)
	}
}

func clamp64(v, low, high float64) float64 {
	return math.Max(low, math.Min(high, v))
}

// formationOffset returns where a party's member rides, off its leader:
// the leader in the middle, the rest around it, inside its bubble.
func formationOffset(place int) (dx, dy float64) {
	if place == 0 {
		return 0, 0
	}
	angle := float64(place) * goldenAngle
	reach := 18 + 4*float64(place)
	return math.Cos(angle) * reach, math.Sin(angle) * reach
}

// stepParty gives a party its tick, by its stage.
func stepParty(s *State, p Party) {
	members := partyMembers(s, p.ID)
	if len(members) == 0 && p.Stage != StageBuild {
		s.endParty(p, ReportDestroyed, 0, p.CampX, p.CampY)
		return
	}
	lead := Enemy{}
	if len(members) > 0 {
		lead = members[0]
	}
	if city, ok := s.Cities[p.City]; p.City != 0 && ok &&
		city.Ruined && !p.CityArrives {
		p.Stage = StageLeave
		p.EntryX, p.EntryY = city.X, city.Y
	}
	if len(members) == 0 && p.Stage != StageBuild {
		s.endParty(p, ReportDestroyed, 0, p.CampX, p.CampY)
		return
	}
	// With its repulsor gone a party has nothing left to do but run.
	if len(members) > 0 && enemySpecOf(lead.Kind).bubble <= 0 &&
		(p.City == 0 ||
			(p.Stage == StageSettled && lead.Kind != EnemyBase)) {
		p.Stage = StageLeave
	}
	switch p.Stage {
	case StageApproach:
		if !s.driveParty(members, p.CampX, p.CampY) {
			break
		}
		if p.CityArrives {
			crawler := s.Enemies[lead.ID]
			coreX, coreY := tileCenterUnits(coreCol, coreRow)
			angle := math.Atan2(crawler.Y-coreY, crawler.X-coreX)
			cityID := s.foundCityFromCrawler(crawler, angle, p.City)
			city := s.Cities[cityID]
			crawler.Kind = EnemyCityCrawler
			crawler.Party, crawler.City = 0, cityID
			crawler.X, crawler.Y = cityCrawlerPosition(city)
			crawler.Health = enemySpecOf(EnemyCityCrawler).health
			s.Enemies[crawler.ID] = crawler
			delete(s.Parties, p.ID)
			s.report(ReportSettled, 0, city.X, city.Y)
			s.Raids.NextAt = s.Ticks + int64(cityIntervalCycles)*fogCycleTicks
			return
		}
		if p.Wait == 0 {
			p.Stage = StageRaid
			s.report(ReportRaid, 0, lead.X, lead.Y)
			break
		}
		p.Stage = StageCamp
		s.report(ReportCamp, 0, p.CampX, p.CampY)
	case StageSettled:
	case StageCamp:
		p.Wait--
		if p.Wait <= 0 {
			p.Stage = StageRaid
			s.report(ReportRaid, 0, lead.X, lead.Y)
		}
	case StageRaid:
		if !s.raid(&p, members) {
			if lead.Kind == EnemyScout {
				s.paintMark(s.Enemies[lead.ID])
			}
			p.Stage = StageLeave
		}
	case StageUnload:
		if _, ok := s.Cities[p.City]; !ok {
			for _, e := range members {
				delete(s.Enemies, e.ID)
			}
			s.endParty(p, ReportDestroyed, 0, p.CampX, p.CampY)
			return
		}
		s.unloadCityParty(&p, members)
	case StageBuild:
		city, exists := s.Cities[p.City]
		if !exists || city.Ruined {
			p.Stage = StageLeave
			p.EntryX, p.EntryY = p.CampX, p.CampY
			if len(members) == 0 {
				s.endParty(p, ReportDestroyed, 0, p.CampX, p.CampY)
				return
			}
			break
		}
		if !cityHasBuilding(s, city, EnemyCityFactory) ||
			cityNeedsConstruction(s, city) {
			break
		}
		p.Wait--
		if p.Wait <= 0 {
			if !s.buildCityPartyUnit(&p) {
				p.Wait = 60
			} else if _, needed := cityPartyUnitNeeded(s, p); !needed {
				s.launchCitySortie(&p)
			} else {
				p.Wait = cityUnitBuildTicks
			}
		}
	case StageRebuild:
		if _, ok := s.Cities[p.City]; !ok {
			for _, e := range members {
				delete(s.Enemies, e.ID)
			}
			s.endParty(p, ReportDestroyed, 0, p.CampX, p.CampY)
			return
		}
		city := s.Cities[p.City]
		if !cityHasBuilding(s, city, EnemyCityFactory) ||
			cityNeedsConstruction(s, city) {
			break
		}
		p.Wait--
		if p.Wait <= 0 {
			if !s.buildCityPartyUnit(&p) {
				p.Wait = 60
			} else if _, needed := cityPartyUnitNeeded(s, p); !needed {
				p.Stage = StageRaid
				p.Siphon = raidSiphonTicks
			} else {
				p.Wait = cityUnitBuildTicks
			}
		}
	case StageRegroup:
		if _, ok := s.Cities[p.City]; !ok {
			for _, e := range members {
				delete(s.Enemies, e.ID)
			}
			s.endParty(p, ReportDestroyed, 0, p.CampX, p.CampY)
			return
		}
		p.Wait--
		if p.Wait <= 0 {
			p.Stage = StageRaid
			s.report(ReportRaid, 0, lead.X, lead.Y)
		}
	case StageLeave:
		oldX, oldY := lead.X, lead.Y
		arrived := s.driveParty(members, p.EntryX, p.EntryY)
		if lead.Kind == EnemyScout && p.City == 0 && s.Raids.Visits == 0 {
			moved := s.Enemies[lead.ID]
			if crossedCoreBubble(oldX, oldY, moved.X, moved.Y) {
				s.Raids.ScoutClearedCore = true
				s.Raids.ScoutClearedCoreAt = s.Ticks
				// Input can acknowledge the badge before the next stepTech.
				if _, arrived := s.Tech[techGuardID]; !arrived {
					s.Tech[techGuardID] = false
				}
			}
		}
		if arrived {
			stolen := 0.0
			for _, e := range members {
				stolen += e.Oil
			}
			if city, ok := s.Cities[p.City]; p.City != 0 &&
				(!ok || city.Ruined) {
				for _, e := range members {
					delete(s.Enemies, e.ID)
				}
				s.endParty(p, ReportLeft, stolen, p.EntryX, p.EntryY)
				return
			}
			if p.City != 0 && stolen > 0 {
				city, ok := s.Cities[p.City]
				if ok {
					city.NextSortie = s.Ticks
					s.Cities[city.ID] = city
				}
				p.Stage = StageUnload
				s.Parties[p.ID] = p
				s.report(ReportLeft, stolen, p.EntryX, p.EntryY)
				return
			}
			for _, e := range members {
				delete(s.Enemies, e.ID)
			}
			s.endParty(p, ReportLeft, stolen, p.EntryX, p.EntryY)
			return
		}
	}
	if p.City == s.Raids.PressureCity &&
		(p.Stage == StageUnload || p.Stage == StageRebuild ||
			p.Stage == StageRegroup) {
		s.Raids.PressureSortieResolved = true
	}
	s.Parties[p.ID] = p
}

func crossedCoreBubble(oldX, oldY, newX, newY float64) bool {
	cx, cy := tileCenterUnits(coreCol, coreRow)
	radius := coreBubbleRadius * unitsPerTile
	wasInside := math.Hypot(oldX-cx, oldY-cy) <= radius
	isOutside := math.Hypot(newX-cx, newY-cy) > radius
	return wasInside && isOutside
}

// endParty removes a party, reports its end and schedules what follows.
func (s *State) endParty(p Party, kind ReportKind, oil, x, y float64) {
	delete(s.Parties, p.ID)
	if p.City == s.Raids.PressureCity && p.City != 0 &&
		s.Raids.PressureSortieStarted {
		s.Raids.PressureSortieResolved = true
	}
	if p.City != 0 || s.Raids.Visits > 0 || kind == ReportDestroyed {
		s.report(kind, oil, x, y)
	}
	if p.City != 0 {
		if city, ok := s.Cities[p.City]; ok {
			if city.Ruined {
				if p.CityArrives {
					city.RefoundAt = s.Ticks + cityRefoundDelayTicks
				}
			} else {
				city.NextSortie = s.Ticks + cityRebuildTicks
			}
			s.Cities[city.ID] = city
		}
		return
	}
	s.scheduleNextParty()
}

func (s *State) scheduleNextParty() {
	s.Raids.Visits++
	city, exists := s.Cities[s.Raids.PressureCity]
	if s.Raids.PressureCity == 0 && s.Raids.Visits == 1 {
		s.Raids.NextAt = s.Ticks + raidFollowupTicks
		return
	}
	if exists && !city.Ruined && cityNeedsConstruction(s, city) {
		s.Raids.NextAt = s.Ticks + raidFollowupTicks
		return
	}
	s.Raids.NextAt = s.Ticks + int64(cityIntervalCycles)*fogCycleTicks
}

func (s *State) fireCityArtillery(e Enemy) {
	if s.Parties[e.Party].Stage != StageRaid {
		return
	}
	if e.Reload > 0 {
		e.Reload--
		s.Enemies[e.ID] = e
		return
	}
	targetX, targetY, found, nearest := 0.0, 0.0, false, math.Inf(1)
	for _, id := range sortedBuildingIDs(s) {
		b := s.Buildings[id]
		x, y := cellCenterUnits(b.Col, b.Row)
		gap := math.Hypot(x-e.X, y-e.Y)
		if gap < 200 || gap > cityArtilleryRangeUnits || gap >= nearest {
			continue
		}
		targetX, targetY, nearest, found = x, y, gap, true
	}
	if !found {
		return
	}
	e.Reload = cityArtilleryReload
	s.Enemies[e.ID] = e
	muzzleX, muzzleY := shellLaunchPoint(e.X, e.Y, targetX, targetY)
	s.fire(Shot{
		Kind: ShotShell, FromX: muzzleX, FromY: muzzleY,
		ToX: targetX, ToY: targetY, Damage: cityArtilleryDamage,
		Rival: true,
	})
}

// driveParty moves a party a tick toward a spot, each member to its
// place around it, all at the pace of the slowest, and reports whether
// its leader arrived.
func (s *State) driveParty(members []Enemy, x, y float64) bool {
	step := math.Inf(1)
	for _, e := range members {
		step = math.Min(step, enemySpecOf(e.Kind).speed/60)
	}
	arrived := false
	for i, e := range members {
		oldX, oldY := e.X, e.Y
		dx, dy := formationOffset(i)
		tx, ty := x+dx-e.X, y+dy-e.Y
		if d := math.Hypot(tx, ty); d <= step {
			e.X, e.Y = x+dx, y+dy
			arrived = arrived || i == 0
		} else {
			e.X += tx / d * step
			e.Y += ty / d * step
		}
		if e.X != oldX || e.Y != oldY {
			e.Facing = facingFromMovement(e.X-oldX, e.Y-oldY)
		}
		s.Enemies[e.ID] = e
	}
	return arrived
}

// raidTarget returns the tank a party goes for: the nearest to its leader
// with oil worth the trip.
func raidTarget(s *State, lead Enemy) (tank int64, found bool) {
	bestGap := math.Inf(1)
	for _, id := range oilTanks(s) {
		if tankOil(s, id) < 1 {
			continue
		}
		spot, _ := tankSpot(s, id)
		if gap := math.Hypot(spot.X-lead.X, spot.Y-lead.Y); gap < bestGap {
			tank, found, bestGap = id, true, gap
		}
	}
	return tank, found
}

// raid drives a party to its tank and siphons it, and reports whether
// the raid goes on: it ends with every vehicle full, with no oil left
// anywhere to take, or when the party has siphoned long enough.
func (s *State) raid(p *Party, members []Enemy) bool {
	tank, found := raidTarget(s, members[0])
	if !found || p.Siphon <= 0 {
		return false
	}
	spot, _ := tankSpot(s, tank)
	lead := members[0]
	if math.Hypot(spot.X-lead.X, spot.Y-lead.Y) > siphonReachUnits {
		angle := math.Atan2(lead.Y-spot.Y, lead.X-spot.X)
		reach := siphonReachUnits - 1
		s.driveParty(members,
			spot.X+math.Cos(angle)*reach, spot.Y+math.Sin(angle)*reach)
		return true
	}
	p.Siphon--
	thirsty := false
	for _, e := range members {
		room := enemySpecOf(e.Kind).oilCap - e.Oil
		take := math.Min(math.Min(room, siphonLitersPerSecond/60), tankOil(s, tank))
		if take > 0 {
			s.addOil(tank, -take)
			e.Oil += take
			s.Enemies[e.ID] = e
		}
		thirsty = thirsty || e.Oil < enemySpecOf(e.Kind).oilCap
	}
	return thirsty
}

// paintMark leaves the scout's mark where it stood, and tells the
// player what the scout did.
func (s *State) paintMark(scout Enemy) {
	if s.Marks == nil {
		s.Marks = map[int64]Mark{}
	}
	s.Marks[s.NextID] = Mark{ID: s.NextID, X: scout.X, Y: scout.Y, Oil: scout.Oil}
	s.NextID++
	s.report(ReportScout, scout.Oil, scout.X, scout.Y)
}

func stepExposure(s *State, previous map[int64]PipePoint) {
	for _, id := range sortedEnemyIDs(s) {
		e := s.Enemies[id]
		if e.Kind == EnemyCityRepulsor {
			e.StillTicks = 0
			s.Enemies[id] = e
			continue
		}
		old, existed := previous[id]
		moved := existed && math.Hypot(e.X-old.X, e.Y-old.Y) > 1e-4
		exposure := miteExposureAt(s, e.X, e.Y)
		if exposure <= 0 || moved {
			e.StillTicks = 0
			s.Enemies[id] = e
			continue
		}
		e.StillTicks++
		if e.StillTicks <= fogStillGraceTicks {
			s.Enemies[id] = e
			continue
		}
		e.Health = math.Max(0, e.Health-
			miteDamagePerSecond*exposure*miteSwellFactor(s)/60)
		s.Enemies[id] = e
		if e.Health <= 0 {
			s.killEnemy(id)
		}
	}
}

// killEnemy takes a vehicle out of the state and leaves its wreck's loot
// on its cell as a pile: a little of its own, and all it had stolen.
func (s *State) killEnemy(id int64) {
	e, ok := s.Enemies[id]
	if !ok {
		return
	}
	city, belongs := s.Cities[e.City]
	oldStage, hadWork := cityNextBuildingStage(s, city)
	s.recordEnemyDeath(e)
	if isRivalBuilding(e) {
		s.recordRivalBuildingDeath(e)
	}
	delete(s.Enemies, id)
	stage := cityStageForEnemy(e.Kind)
	if e.Kind == EnemyCityCrawler && e.City != 0 && belongs {
		removeCityBuilding(&city, id)
		if cityHasStructures(s, city) {
			city.Work = cityBuildTicks
			city.MiteDamage = 0
			s.Cities[city.ID] = city
		} else {
			s.ruinCity(&city)
			s.report(ReportBaseDown, 0, city.X, city.Y)
		}
	} else if e.City != 0 && belongs && (e.Party == 0 || stage >= 0) {
		removeCityBuilding(&city, id)
		if e.Kind == EnemyBase {
			city.NexusID = 0
		}
		if stage >= 0 && city.Stage > 0 &&
			!cityHasStructures(s, city) {
			s.ruinCity(&city)
			s.report(ReportBaseDown, 0, city.X, city.Y)
		} else {
			newStage, needsWork := cityNextBuildingStage(s, city)
			if needsWork && (!hadWork || oldStage != newStage) {
				city.Work = cityBuildTicks
				city.MiteDamage = 0
			}
			s.Cities[city.ID] = city
		}
	}
	if e.Kind == EnemyBase {
		if e.City == 0 {
			s.report(ReportBaseDown, 0, e.X, e.Y)
		}
	}
	spec := enemySpecOf(e.Kind)
	col := int(clamp64(math.Floor(e.X/buildingCell), 0, regionCellCols-1))
	row := int(clamp64(math.Floor(e.Y/buildingCell), 0, regionCellRows-1))
	s.dropPile(col, row, spec.lootOil+e.Oil, spec.lootLilac)
}

func isRivalBuilding(e Enemy) bool {
	switch e.Kind {
	case EnemyBase, EnemyCityRepulsor, EnemyCityOilworks,
		EnemyCityMine, EnemyCityFactory:
		return true
	}
	return false
}

func (s *State) ruinCity(city *City) {
	for _, id := range city.BuildingIDs {
		if building, found := s.Enemies[id]; found {
			if isRivalBuilding(building) {
				s.recordRivalBuildingDeath(building)
			} else if building.Kind == EnemyCityCrawler {
				s.recordEnemyDeath(building)
			}
		}
		delete(s.Enemies, id)
	}
	city.BuildingIDs = nil
	city.Ruined = true
	city.RefoundAt = s.Ticks + cityRefoundDelayTicks
	city.Work = 0
	city.MiteDamage = 0
	city.Oil = 0
	city.Lilac = 0
	city.OilDeposit = 0
	city.LilacDeposit = 0
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
	s.Cities[city.ID] = *city
}

// Guard posts: the colony's first answer. A post shoots the nearest rival
// vehicle within its reach, and every shot is paid in oil.
const (
	guardCostLilac   = 150.0 // kg
	guardCostOil     = 30.0  // L
	guardReloadTicks = 40    // ticks between two shots
	guardShotDamage  = 12.0
	guardShotOil     = 0.5 // L a shot burns, out of any tank
)

// stepGuards reloads every guard post and fires the ones that are ready
// and have somebody in reach. A colony with no oil doesn't shoot.
func stepGuards(s *State) {
	for _, id := range sortedBuildingIDs(s) {
		b := s.Buildings[id]
		if b.Kind != BuildingGuard {
			continue
		}
		if b.Reload > 0 {
			b.Reload--
			s.Buildings[id] = b
			continue
		}
		x, y := cellCenterUnits(b.Col, b.Row)
		target, found := nearestEnemy(s, x, y, smallArmsRangeUnits)
		if !found || oilTotal(s) < guardShotOil {
			b.Aim = 0
			s.Buildings[id] = b
			continue
		}
		s.payOil(guardShotOil)
		s.recordCost(costBuilding, b.ID, x, y, 0, guardShotOil, true)
		b.Reload, b.Aim = guardReloadTicks, target.ID
		s.Buildings[id] = b
		s.fire(Shot{
			Kind: ShotBullet, FromX: x, FromY: y, ToX: target.X, ToY: target.Y,
			Enemy: target.ID, Damage: guardShotDamage,
		})
	}
}

// nearestEnemy returns the rival vehicle closest to a spot, within a
// reach; IDs break ties.
func nearestEnemy(s *State, x, y, reach float64) (Enemy, bool) {
	var best Enemy
	found, bestGap := false, reach
	for _, id := range sortedEnemyIDs(s) {
		e := s.Enemies[id]
		if gap := math.Hypot(e.X-x, e.Y-y); gap <= bestGap && (!found || gap < bestGap) {
			best, found, bestGap = e, true, gap
		}
	}
	return best, found
}
