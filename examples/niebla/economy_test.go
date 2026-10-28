package main

import (
	"encoding/csv"
	"math"
	"os"
	"sort"
	"strconv"
	"testing"
)

const economySampleTicks = 60 * 60 // one minute of game time

type economyPlan struct {
	name           string
	workerGoal     int
	robotsPerPatch int
	expand         bool
	defend         bool
	trooperGoal    int
	repair         bool
}

var economyPlans = []economyPlan{
	{name: "safe-harvest", workerGoal: 1, robotsPerPatch: 1},
	{name: "growth", workerGoal: 3, robotsPerPatch: 2},
	{name: "outpost", workerGoal: 3, robotsPerPatch: 1, expand: true},
	{name: "defense", workerGoal: 3, robotsPerPatch: 2,
		defend: true, trooperGoal: 2, repair: true},
}

type economyMilestones struct {
	scoutClearedCoreAt      int64
	firstBuildingHitAt      int64
	firstIntroAttackEndedAt int64
	firstPressureSortieAt   int64
	firstPressureLullAt     int64
	artillerySchematicsAt   int64
	repairProtocolArrivedAt int64
}

func newEconomyMilestones() economyMilestones {
	return economyMilestones{
		scoutClearedCoreAt:      -1,
		firstBuildingHitAt:      -1,
		firstIntroAttackEndedAt: -1,
		firstPressureSortieAt:   -1,
		firstPressureLullAt:     -1,
		artillerySchematicsAt:   -1,
		repairProtocolArrivedAt: -1,
	}
}

func (m *economyMilestones) observe(s *State) {
	if m.scoutClearedCoreAt < 0 && s.Raids.ScoutClearedCore {
		m.scoutClearedCoreAt = s.Ticks
	}
	if m.firstBuildingHitAt < 0 && s.Raids.RivalBuildingHit {
		m.firstBuildingHitAt = s.Ticks
	}
	if m.firstIntroAttackEndedAt < 0 && s.Raids.Visits >= 2 {
		m.firstIntroAttackEndedAt = s.Ticks
	}
	if m.firstPressureSortieAt < 0 && s.Raids.PressureSortieStarted {
		m.firstPressureSortieAt = s.Ticks
	}
	if m.firstPressureLullAt < 0 && s.Raids.PressureSortieResolved {
		m.firstPressureLullAt = s.Ticks
	}
	if m.artillerySchematicsAt < 0 && dropArrived(s, techArtilleryID) {
		m.artillerySchematicsAt = s.Ticks
	}
	if m.repairProtocolArrivedAt < 0 && repairProtocolUnlocked(s) {
		m.repairProtocolArrivedAt = s.Ticks
	}
}

type economyPlanner struct {
	plan        economyPlan
	factoryCol  int
	factoryRow  int
	guardCol    int
	guardRow    int
	warCol      int
	warRow      int
	farOil      Deposit
	hasFarOil   bool
	farLilac    Deposit
	hasFarLilac bool
}

// TestWriteEconomyReport runs deterministic colonies over four legal
// opening policies and three seeds. Set NIEBLA_ECONOMY_REPORT to write a
// CSV report; ordinary tests leave no diagnostics behind.
func TestWriteEconomyReport(t *testing.T) {
	path := os.Getenv("NIEBLA_ECONOMY_REPORT")
	if path == "" {
		t.Skip("set NIEBLA_ECONOMY_REPORT to write the economy report")
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating %s: %v", path, err)
	}
	defer file.Close()

	w := csv.NewWriter(file)
	if err := w.Write([]string{
		"plan", "seed", "minute", "oil", "protector_oil", "lilac", "oil_mined",
		"lilac_mined", "workers", "troopers", "buildings", "protectors",
		"pumps", "pipes", "guards", "swells", "visits", "enemies",
		"party_stage", "party_size", "party_raiders", "party_artillery",
		"city_stage", "city_work_ticks", "city_sorties", "city_oil",
		"city_lilac", "scout_cleared_core_tick", "first_building_hit_tick",
		"first_intro_attack_end_tick", "first_pressure_sortie_tick",
		"first_pressure_sortie_lull_tick", "artillery_schematics_tick",
		"repair_protocol_tick",
		"mechanics", "building_damage",
	}); err != nil {
		t.Fatalf("writing the report header: %v", err)
	}
	for _, plan := range economyPlans {
		for _, seed := range []int64{0, 1, 2} {
			writeEconomyRun(t, w, plan, seed, 60)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func writeEconomyRun(
	t *testing.T,
	w *csv.Writer,
	plan economyPlan,
	seed int64,
	minutes int,
) {
	t.Helper()
	s := newGameOn(seed)
	p := newEconomyPlanner(plan)
	milestones := newEconomyMilestones()
	for minute := 0; minute <= minutes; minute++ {
		if err := w.Write(economyRow(
			s, plan.name, seed, minute, milestones,
		)); err != nil {
			t.Fatalf("writing %s, seed %d, minute %d: %v", plan.name, seed, minute, err)
		}
		if minute == minutes {
			break
		}
		for tick := 0; tick < economySampleTicks; tick++ {
			if s.Ticks%60 == 0 {
				p.decide(s)
			}
			Apply(s, Tick{})
			milestones.observe(s)
		}
	}
}

func newEconomyPlanner(plan economyPlan) economyPlanner {
	factoryCol, factoryRow := groundNearCore()
	p := economyPlanner{
		plan:       plan,
		factoryCol: factoryCol,
		factoryRow: factoryRow,
		guardCol:   factoryCol + 2,
		guardRow:   factoryRow,
		warCol:     factoryCol + 4,
		warRow:     factoryRow,
	}
	for _, deposit := range depositsByDistance() {
		if depositDistance(deposit) <= coreBubbleRadius {
			continue
		}
		if deposit.Kind == kindOil && !p.hasFarOil {
			p.farOil, p.hasFarOil = deposit, true
		}
		if deposit.Kind == kindLilac && !p.hasFarLilac {
			p.farLilac, p.hasFarLilac = deposit, true
		}
	}
	return p
}

func (p economyPlanner) decide(s *State) {
	p.openSchematics(s)
	p.assignPosts(s)
	p.growWorkers(s)
	if p.plan.expand {
		p.expandOil(s)
	}
	if p.plan.defend {
		p.buildDefense(s)
	}
}

func (p economyPlanner) openSchematics(s *State) {
	for _, drop := range techLadder {
		opened, arrived := s.Tech[drop.id]
		if arrived && !opened {
			Apply(s, AckTech{ID: drop.id})
		}
	}
}

func (p economyPlanner) assignPosts(s *State) {
	for _, deposit := range depositsByDistance() {
		tileCol, tileRow := cellTile(deposit.HeartCol, deposit.HeartRow)
		far := depositDistance(deposit) > coreBubbleRadius
		if !p.plan.expand && far {
			continue
		}
		if p.plan.expand && far && !depositSafe(s, deposit) {
			continue
		}
		if remainingAt(s, tileCol, tileRow) <= 0 {
			continue
		}
		for len(postRobots(s, tileCol, tileRow)) < p.plan.robotsPerPatch {
			if worker := pickRobot(s, tileCol, tileRow); worker >= 0 {
				Apply(s, SendRobot{Col: tileCol, Row: tileRow})
				continue
			}
			assignedBuilder := false
			for _, id := range sortedRobotIDs(s) {
				r := s.Robots[id]
				if r.Kind != RobotBuilder || r.hasPost() {
					continue
				}
				Apply(s, AssignRobot{
					ID: id, Col: tileCol, Row: tileRow,
				})
				assignedBuilder = true
				break
			}
			if !assignedBuilder {
				break
			}
		}
	}
}

func (p economyPlanner) growWorkers(s *State) {
	if !p.mark(s, BuildingFactory, p.factoryCol, p.factoryRow) {
		return
	}
	if workerCount(s) >= p.plan.workerGoal {
		return
	}
	if factory, raised := buildingAt(s, p.factoryCol, p.factoryRow); raised {
		Apply(s, QueueRobot{
			Building: factory.ID, Kind: RobotWorker,
		})
	}
}

func (p economyPlanner) buildDefense(s *State) {
	if kindUnlocked(s, BuildingGuard) &&
		!p.mark(s, BuildingGuard, p.guardCol, p.guardRow) {
		return
	}
	if !kindUnlocked(s, BuildingWarFactory) ||
		!p.mark(s, BuildingWarFactory, p.warCol, p.warRow) {
		return
	}
	factory, raised := buildingAt(s, p.warCol, p.warRow)
	if !raised || factory.Work > 0 {
		return
	}
	if len(squadMembers(s, factory.ID)) < p.plan.trooperGoal {
		Apply(s, QueueRobot{Building: factory.ID, Kind: RobotCombat})
		return
	}
	if p.plan.repair && repairProtocolUnlocked(s) {
		Apply(s, QueueMechanic{Building: factory.ID})
	}
}

func (p economyPlanner) expandOil(s *State) {
	if !p.hasFarOil {
		return
	}
	col, row, found := protectorSite(p.farOil)
	if !found || !p.mark(s, BuildingProtector, col, row) {
		return
	}
	pumpCol, pumpRow := pumpCell(p.farOil)
	if !p.mark(s, BuildingPump, pumpCol, pumpRow) {
		return
	}
	protector, raised := buildingAt(s, col, row)
	if !raised {
		return
	}
	pump, raised := buildingAt(s, pumpCol, pumpRow)
	if !raised {
		return
	}
	if canJoin(s, pump.ID, protector.ID) {
		Apply(s, LayPipe{From: pump.ID, To: protector.ID})
		return
	}
	if canJoin(s, protector.ID, coreTank) {
		Apply(s, LayPipe{From: protector.ID, To: coreTank})
	}
}

// mark asks for a building until it is raised. Its true answer means the
// site or building now stands, so later parts of a plan may depend on it.
func (p economyPlanner) mark(s *State, kind BuildingKind, col, row int) bool {
	if building, raised := buildingAt(s, col, row); raised {
		return building.Kind == kind
	}
	for _, job := range s.Jobs {
		if job.Col == col && job.Row == row {
			return job.Kind == kind
		}
	}
	Apply(s, MarkBuilding{Kind: kind, Col: col, Row: row})
	return false
}

// protectorSite finds flat ground close enough to cover a far deposit's
// pump. The generator guarantees the heart is flat; its body may not leave
// a free cell for the protector itself.
func protectorSite(d Deposit) (col, row int, found bool) {
	for radius := 1; radius <= 16; radius++ {
		for rowOffset := -radius; rowOffset <= radius; rowOffset++ {
			for colOffset := -radius; colOffset <= radius; colOffset++ {
				if colOffset != -radius && colOffset != radius &&
					rowOffset != -radius && rowOffset != radius {
					continue
				}
				col, row = d.HeartCol+colOffset, d.HeartRow+rowOffset
				if col < 0 || row < 0 || col >= regionCellCols || row >= regionCellRows ||
					tileAt(cellTile(col, row)) != kindGround || !land.flatCell(col, row) {
					continue
				}
				x, y := cellCenterUnits(col, row)
				px, py := cellCenterUnits(d.HeartCol, d.HeartRow)
				if math.Hypot(x-px, y-py) <= protectorBubbleTiles*unitsPerTile-buildingCell {
					return col, row, true
				}
			}
		}
	}
	return 0, 0, false
}

func depositsByDistance() []Deposit {
	deposits := append([]Deposit(nil), land.deposits...)
	sort.Slice(deposits, func(i, j int) bool {
		return depositDistance(deposits[i]) < depositDistance(deposits[j])
	})
	return deposits
}

func depositDistance(d Deposit) float32 {
	col, row := cellTile(d.HeartCol, d.HeartRow)
	return tileDistance(col, row)
}

func depositSafe(s *State, d Deposit) bool {
	x, y := cellCenterUnits(d.HeartCol, d.HeartRow)
	return inSafeZone(s, x, y)
}

func economyRow(
	s *State,
	plan string,
	seed int64,
	minute int,
	milestones economyMilestones,
) []string {
	oilMined, lilacMined := mined(s)
	workers, troopers := robotCounts(s)
	buildings, protectors, pumps, guards := buildingCounts(s)
	partyStage, partySize, partyRaiders, partyArtillery := partyCounts(s)
	cityStage, cityWork, citySorties, cityOil, cityLilac := cityEconomy(s)
	mechanics, damage := repairEconomy(s)
	return []string{
		plan,
		strconv.FormatInt(seed, 10),
		strconv.Itoa(minute),
		quantity(oilTotal(s)),
		quantity(protectorOilTotal(s)),
		quantity(s.Stock.Lilac),
		quantity(oilMined),
		quantity(lilacMined),
		strconv.Itoa(workers),
		strconv.Itoa(troopers),
		strconv.Itoa(buildings),
		strconv.Itoa(protectors),
		strconv.Itoa(pumps),
		strconv.Itoa(len(s.Pipes)),
		strconv.Itoa(guards),
		strconv.FormatInt(s.Fog.Swells, 10),
		strconv.FormatInt(s.Raids.Visits, 10),
		strconv.Itoa(len(s.Enemies)),
		partyStage,
		strconv.Itoa(partySize),
		strconv.Itoa(partyRaiders),
		strconv.Itoa(partyArtillery),
		cityStage,
		cityWork,
		citySorties,
		cityOil,
		cityLilac,
		economyTick(milestones.scoutClearedCoreAt),
		economyTick(milestones.firstBuildingHitAt),
		economyTick(milestones.firstIntroAttackEndedAt),
		economyTick(milestones.firstPressureSortieAt),
		economyTick(milestones.firstPressureLullAt),
		economyTick(milestones.artillerySchematicsAt),
		economyTick(milestones.repairProtocolArrivedAt),
		strconv.Itoa(mechanics),
		quantity(damage),
	}
}

func repairEconomy(s *State) (mechanics int, damage float64) {
	for _, id := range sortedRobotIDs(s) {
		if s.Robots[id].Kind == RobotRepair {
			mechanics++
		}
	}
	for _, id := range sortedBuildingIDs(s) {
		damage += s.Buildings[id].Damage
	}
	return mechanics, damage
}

func economyTick(tick int64) string {
	if tick < 0 {
		return ""
	}
	return strconv.FormatInt(tick, 10)
}

func TestEconomyMilestonesRememberTheirTicks(t *testing.T) {
	s := newGame()
	milestones := newEconomyMilestones()
	s.Ticks = 100
	s.Raids.ScoutClearedCore = true
	s.Raids.RivalBuildingHit = true
	milestones.observe(s)
	s.Ticks = 200
	s.Raids.Visits = 2
	milestones.observe(s)
	s.Ticks = 300
	s.Raids.PressureSortieStarted = true
	milestones.observe(s)
	s.Ticks = 400
	s.Raids.PressureSortieResolved = true
	s.Parties[701] = Party{ID: 701, Stage: StageRaid}
	milestones.observe(s)
	if milestones.artillerySchematicsAt != -1 {
		t.Fatalf("artillery arrived after only %d normal visits",
			milestones.artillerySchematicsAt)
	}
	s.Ticks = 500
	s.Raids.Visits = 4
	s.Parties[701] = Party{ID: 701, Stage: StageUnload}
	s.Tech[techRepairID] = false
	milestones.observe(s)

	if milestones.scoutClearedCoreAt != 100 ||
		milestones.firstBuildingHitAt != 100 ||
		milestones.firstIntroAttackEndedAt != 200 ||
		milestones.firstPressureSortieAt != 300 ||
		milestones.firstPressureLullAt != 400 ||
		milestones.artillerySchematicsAt != 500 ||
		milestones.repairProtocolArrivedAt != 500 {
		t.Fatalf("the economy probe recorded %+v", milestones)
	}
}

func partyCounts(s *State) (stage string, size, raiders, artillery int) {
	for _, id := range sortedPartyIDs(s) {
		party := s.Parties[id]
		if party.Stage == StageSettled {
			continue
		}
		stage = string(party.Stage)
		for _, enemy := range partyMembers(s, id) {
			size++
			if enemy.Kind == EnemyRaider {
				raiders++
			}
			if enemy.Kind == EnemyArtillery {
				artillery++
			}
		}
		return stage, size, raiders, artillery
	}
	return "", 0, 0, 0
}

func cityEconomy(s *State) (stage, work, sorties, oil, lilac string) {
	ids := sortedCityIDs(s)
	if len(ids) == 0 {
		return "", "", "", "", ""
	}
	city := s.Cities[ids[0]]
	return strconv.Itoa(city.Stage), strconv.FormatInt(city.Work, 10),
		strconv.FormatInt(city.Sorties, 10), quantity(city.Oil),
		quantity(city.Lilac)
}

func mined(s *State) (oil, lilac float64) {
	for _, deposit := range land.deposits {
		mined := deposit.Full - s.Drain[depositKey(deposit)]
		if deposit.Kind == kindOil {
			oil += mined
		} else {
			lilac += mined
		}
	}
	return oil, lilac
}

func robotCounts(s *State) (workers, troopers int) {
	for _, robot := range s.Robots {
		switch robot.Kind {
		case RobotWorker:
			workers++
		case RobotCombat:
			troopers++
		}
	}
	return workers, troopers
}

func workerCount(s *State) int {
	workers, _ := robotCounts(s)
	return workers
}

func buildingCounts(s *State) (all, protectors, pumps, guards int) {
	for _, building := range s.Buildings {
		all++
		switch building.Kind {
		case BuildingProtector:
			protectors++
		case BuildingPump:
			pumps++
		case BuildingGuard:
			guards++
		}
	}
	return all, protectors, pumps, guards
}

func quantity(value float64) string {
	return strconv.FormatFloat(value, 'f', 1, 64)
}
