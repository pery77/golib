package main

import (
	"fmt"
	"math"
)

// The simulation's state: one serializable value, the whole game. It
// holds no pointers, no channels, no functions, so it goes through
// golib.SaveData as it is, and loading it back loads the game, exactly.
// Everything the view adds for itself (the camera, the picked tile, the
// open cards) lives in the scenes, never here.

// State is the whole game as one value. Actions (actions.go) are the
// only thing that changes it.
type State struct {
	Version    int                // serialized state schema version
	Seed       int64              // the region's seed: its relief, cover and deposits (worldgen.go)
	Ticks      int64              // game ticks, 60 to the second
	NextID     int64              // the ID the next robot, building, pile or pipe gets
	Robots     map[int64]Robot    // the colony, by ID
	Buildings  map[int64]Building // the colony's structures, by ID
	Stock      Stock              // what the core's stores hold
	Drain      map[string]float64 // what remains in each deposit patch, by key
	Jobs       []Job              // build jobs, oldest first
	Piles      map[int64]Pile     // loose items on the ground, by ID
	Pipes      map[int64]Pipe     // oil pipes, laid or being laid, by ID
	Fog        Fog                // the region's weather: the cycles and the swell
	Enemies    map[int64]Enemy    // the rivals' vehicles, by ID (sim_enemies.go)
	Parties    map[int64]Party    // the rivals' visits under way, by ID
	Cities     map[int64]City     // settled rival cities and their production
	Raids      Raids              // the rivals' clock: the visits that were, the next one
	Marks      map[int64]Mark     // what the scouts painted on the ground, by ID
	Reports    []Report           // the news the rivals made, oldest first
	Rolls      uint64             // random numbers drawn so far: the state's own PRNG
	Deliveries int64              // loads the robots have brought home, the first of which brings the first schematics in
	Squads     map[int64]Squad    // the squads' orders, by their war factory's ID
	Shots      map[int64]Shot     // bullets and shells in the air, by ID (sim_shots.go)
	Tech       map[string]bool    // the schematics that arrived: drop ID -> opened (sim_tech.go)
	// The latest tick's death events, for view effects; never saved.
	Deaths         []UnitDeath     `json:"-"`
	BuildingDeaths []BuildingDeath `json:"-"`
	// Resource expenses for the view; the next tick replaces them.
	Costs []CostReceipt `json:"-"`
}

// UnitDeath is a one-tick simulation event for the view. It is not saved:
// effects are cosmetic and must not replay when a saved game is opened.
type UnitDeath struct {
	ID        int64
	RobotKind RobotKind
	EnemyKind EnemyKind
	X, Y      float64
}

type BuildingDeathCause string

const (
	BuildingDemolished BuildingDeathCause = "demolished"
	BuildingDestroyed  BuildingDeathCause = "destroyed"
)

type BuildingDeath struct {
	ID        int64
	Kind      BuildingKind
	RivalKind EnemyKind
	Cause     BuildingDeathCause
	X, Y      float64
}

func (s *State) recordRobotDeath(r Robot) {
	s.Deaths = append(s.Deaths, UnitDeath{
		ID: r.ID, RobotKind: r.Kind, X: r.X, Y: r.Y,
	})
}

func (s *State) recordEnemyDeath(e Enemy) {
	switch e.Kind {
	case EnemyScout, EnemyCrawler, EnemyRaider, EnemyArtillery,
		EnemyCityCrawler:
	default:
		return
	}
	s.Deaths = append(s.Deaths, UnitDeath{
		ID: e.ID, EnemyKind: e.Kind, X: e.X, Y: e.Y,
	})
}

func (s *State) recordBuildingDeath(
	b Building,
	cause BuildingDeathCause,
) {
	x, y := cellCenterUnits(b.Col, b.Row)
	s.BuildingDeaths = append(s.BuildingDeaths, BuildingDeath{
		ID: b.ID, Kind: b.Kind, Cause: cause, X: x, Y: y,
	})
}

func (s *State) recordRivalBuildingDeath(e Enemy) {
	s.BuildingDeaths = append(s.BuildingDeaths, BuildingDeath{
		ID: e.ID, RivalKind: e.Kind, Cause: BuildingDestroyed,
		X: e.X, Y: e.Y,
	})
}

const stateVersion = 12

// Fog is the region's weather, where the fog's breath has got to. The
// swell rises at a cycle's end and drains tick by tick; NextIn counts
// the cycles of calm left before the next one, and Swells remembers how
// many have passed, which is how the fog grows harder. SwellLeft > 0
// means a swell is up (sim_fog.go holds the law).
type Fog struct {
	Cycle     int64   // whole cycles since the region began
	CycleLeft int64   // ticks until the current cycle ends
	SwellLeft int64   // ticks left of the current swell; 0 is calm
	NextIn    float64 // cycles of calm left before the next swell
	Swells    int64   // swells that have passed: the difficulty's memory
	Held      bool    // the dev tools hold the swell up: it doesn't drain
	Pressure  float64 // 0 calm to 1 pressed in whole: how far the swell has come
}

// Stock is what the colony has stored. Lilac is one stock under every
// roof; oil has a place, and this is the core's own tank (sim_oil.go).
type Stock struct {
	Oil   float64 // liters, in the core's tank
	Lilac float64 // kilograms
}

// RobotKind says which role a colony unit has: builders construct, workers
// harvest, combat units fight in squads, and repair units mend buildings.
type RobotKind string

const (
	RobotBuilder RobotKind = "builder"
	RobotWorker  RobotKind = "worker"
	// RobotCombat is a war-factory trooper, which follows its squad.
	RobotCombat RobotKind = "combat"
	// RobotRepair is a mechanic that repairs damaged colony buildings.
	RobotRepair RobotKind = "repair"
)

// Robot is one colony unit. Builders raise buildings and lay pipes;
// workers mine deposits. Their current task is derived from state every
// tick, so a save reproduces its future. What a robot claims - a post or a
// construction task or a post - is state too, since the other robots read it.
type Robot struct {
	ID         int64
	Kind       RobotKind // builder, worker, combat or repair
	X, Y       float64   // position, in units (1 u = 1 m)
	Facing     uint8     // screen-facing octant; zero points right
	Tank       float64   // liters of oil left
	StillTicks int64     // ticks spent standing outside every bubble
	PostCol    int       // the tile of the patch it was sent to; -1 when free
	PostRow    int       //
	WorkTicks  int64     // ticks of loading left at its post
	Carry      float64   // what it carries, in the cargo's SI unit
	Cargo      ThingType // oil, lilac, or "" while empty
	Pile       int64     // the pile it is loading from; 0 while loading at its post
	BuildJob   bool      // a marked building site is reserved
	BuildCol   int       // the reserved site's cell
	BuildRow   int       //
	Demolition int64     // the building reserved for dismantling
	Pipe       int64     // the pipe whose section it claimed to lay; 0 with no claim
	Section    int64     // the claimed section, from the pipe's source out
	Squad      int64     // troopers: the war factory whose squad it is in
	Factory    int64     // mechanics: the war factory that built it
	Health     float64   // hull points left
	Reload     int64     // troopers: ticks until the next shot
	Aim        int64     // troopers: the vehicle the last shot went to
}

// tanked reports whether the robot runs on a tank of oil.
func (r Robot) tanked() bool {
	return r.Kind != ""
}

// BuildingKind names one of the structures the colony can raise. The
// blueprints' costs and the rules each kind obeys live in
// sim_buildings.go.
type BuildingKind string

const (
	BuildingFactory   BuildingKind = "factory"   // builds robots from lilac and oil
	BuildingCharger   BuildingKind = "charger"   // refills a built robot's tank
	BuildingSilo      BuildingKind = "silo"      // stores more oil
	BuildingWarehouse BuildingKind = "warehouse" // stores more lilac
	BuildingProtector BuildingKind = "protector" // a small bubble of safe ground
	BuildingPump      BuildingKind = "pump"      // draws a pool's oil into a pipe
	BuildingGuard     BuildingKind = "guard"     // shoots the rivals in its reach

	BuildingWarFactory BuildingKind = "warfactory" // builds troopers and unlocked mechanics
	BuildingArtillery  BuildingKind = "artillery"  // shells the rivals the colony sees
)

// Building is one raised structure. Its Col, Row are cell coordinates
// (the tile grid's last subdivision, a 40 u footprint — sim_buildings.go),
// so a tile may hold several buildings. Its Work counts down while a
// factory builds a unit; every other kind leaves it at zero. Its
// Demolish counts the robot work down to the building coming apart:
// zero when nothing is ordered down.
type Building struct {
	ID       int64
	Kind     BuildingKind
	Col, Row int       // the cell it stands on
	Work     int64     // ticks until the factory's unit is built
	WorkKind RobotKind // the unit being built; unset when idle or in old saves
	Demolish int64     // ticks of work left to take it down; 0 when not ordered
	Oil      float64   // liters in its tank: stores and protectors (sim_oil.go)
	Reload   int64     // guard posts: ticks until the next shot (sim_enemies.go)
	Aim      int64     // guard posts: the vehicle the last shot went to
	Damage   float64   // what it has taken; at its health it falls (sim_shots.go)
}

// Job is one build job: what to raise, on which cell, and the ticks of
// work it still asks for.
type Job struct {
	Kind     BuildingKind
	Col, Row int // the cell it stands on
	Left     int64
	Damage   float64 // damage from mites
}

// Pile is what a demolished building leaves on its cell: a container
// that stands for every loose item lying there. It has no mass, no
// health and no capacity, and it leaves the state with its last item
// (sim_piles.go holds the law).
type Pile struct {
	ID        int64
	Col, Row  int     // the cell it lies on
	Oil       float64 // liters
	Lilac     float64 // kilograms
	MiteTicks float64 // exposure-weighted ticks before the pile is consumed
}

// hasPost reports whether the robot was sent to a deposit tile.
func (r Robot) hasPost() bool {
	return r.PostCol >= 0
}

func (r *Robot) clearPost() {
	r.PostCol, r.PostRow = -1, -1
}

// The core's gift: what its stores hold at the start, enough to mark
// the first buildings without a haul first.
const (
	startingStockOil   = 300.0 // L
	startingStockLilac = 600.0 // kg
)

// newGame deals the starting region on the default seed.
func newGame() *State {
	return newGameOn(defaultSeed)
}

// newGameOn deals the starting region a seed generates: every deposit is
// full, one fueled builder idles by the core, and the core gives its stores.
func newGameOn(seed int64) *State {
	useRegion(seed)
	s := &State{
		Version:   stateVersion,
		Seed:      seed,
		NextID:    1,
		Robots:    map[int64]Robot{},
		Buildings: map[int64]Building{},
		Piles:     map[int64]Pile{},
		Pipes:     map[int64]Pipe{},
		Enemies:   map[int64]Enemy{},
		Parties:   map[int64]Party{},
		Cities:    map[int64]City{},
		Marks:     map[int64]Mark{},
		Squads:    map[int64]Squad{},
		Shots:     map[int64]Shot{},
		Raids:     Raids{NextAt: raidFirstScoutTicks},
		Drain:     map[string]float64{},
		Stock:     Stock{Oil: startingStockOil, Lilac: startingStockLilac},
		Fog:       Fog{CycleLeft: fogCycleTicks, NextIn: fogSwellPeriod},
		Tech:      map[string]bool{techIndustryID: false},
	}
	for _, d := range land.deposits {
		s.Drain[depositKey(d)] = depositFull(d)
	}
	x, y := parkSlot(0)
	s.spawnRobot(RobotBuilder, x, y)
	return s
}

// enterRegion makes the ground the state's own, for a state that comes
// from a save. A save from before the generated regions names deposits
// that are gone: the ones it doesn't know wake up full.
func (s *State) enterRegion() {
	useRegion(s.Seed)
	s.migrateState()
	if s.Drain == nil {
		s.Drain = map[string]float64{}
	}
	for _, d := range land.deposits {
		if _, known := s.Drain[depositKey(d)]; !known {
			s.Drain[depositKey(d)] = depositFull(d)
		}
	}
	s.migrateSettledCities()
}

func (s *State) migrateState() {
	if s.Version >= stateVersion {
		return
	}
	if s.Version < 4 && len(s.Cities) > 0 && s.Raids.PressureCity == 0 {
		ids := sortedCityIDs(s)
		s.Raids.PressureCity = ids[0]
	}
	if s.Version < 4 {
		for _, id := range sortedPartyIDs(s) {
			p := s.Parties[id]
			if p.City == 0 || p.Size > 0 {
				continue
			}
			members := partyMembers(s, id)
			p.Size = len(members)
			for _, e := range members {
				if e.Kind == EnemyArtillery {
					p.Artillery = true
				}
			}
			s.Parties[id] = p
		}
	}
	if s.Version < 1 {
		for _, id := range sortedBuildingIDs(s) {
			b := s.Buildings[id]
			if b.Kind != BuildingProtector || b.Oil > 0 {
				continue
			}
			b.Oil = protectorCostOil
			s.Buildings[id] = b
		}
	}
	for _, id := range sortedRobotIDs(s) {
		r := s.Robots[id]
		switch string(r.Kind) {
		case "core":
			r.Kind = RobotBuilder
			r.Tank = robotTankLiters
		case "built":
			r.Kind = RobotWorker
		}
		if health := robotMaxHealth(r.Kind); r.Health <= 0 && health > 0 {
			r.Health = health
		}
		s.Robots[id] = r
	}
	for _, id := range sortedBuildingIDs(s) {
		b := s.Buildings[id]
		if b.WorkKind == "built" {
			b.WorkKind = RobotWorker
			s.Buildings[id] = b
		}
	}
	if s.Version < 5 {
		if s.legacyRepairWasAvailable() {
			s.Raids.LegacyRepairUnlocked = true
		}
		if city, ok := s.Cities[s.Raids.PressureCity]; ok &&
			city.Sorties > 0 {
			s.Raids.PressureSortieStarted = true
		}
		for _, id := range sortedPartyIDs(s) {
			party := s.Parties[id]
			if party.City != s.Raids.PressureCity || party.City == 0 {
				continue
			}
			s.Raids.PressureSortieStarted = true
			if party.Stage == StageUnload || party.Stage == StageRebuild ||
				party.Stage == StageRegroup {
				s.Raids.PressureSortieResolved = true
			}
		}
	}
	if s.Version < 6 && s.Tech != nil {
		if _, arrived := s.Tech[techGuardID]; !arrived &&
			legacyGuardTechArrived(s) {
			s.Tech[techGuardID] = true
		}
	}
	if s.Version < 7 {
		s.Raids.LegacyArtillery = true
	}
	if s.Version < 9 {
		// Saves without a scout-crossing tick keep the former 5:30 drop.
		s.Raids.LegacyFrontierClock = true
	}
	if s.Version < 10 {
		for _, id := range sortedCityIDs(s) {
			city := s.Cities[id]
			if city.Ruined || !cityNeedsConstruction(s, city) ||
				city.Work > 0 {
				continue
			}
			city.Work = cityBuildTicks
			city.MiteDamage = 0
			s.Cities[id] = city
		}
	}
	if s.Version < 11 {
		s.migrateCityAntimist()
	}
	if s.Version < 12 {
		s.migrateCityCrawlerConstruction()
	}
	s.Version = stateVersion
}

func (s *State) migrateCityAntimist() {
	for _, id := range sortedPartyIDs(s) {
		party := s.Parties[id]
		if party.City == 0 || party.CityArrives {
			continue
		}
		members := partyMembers(s, party.ID)
		if len(members) == 0 {
			continue
		}
		protected := false
		for _, member := range members {
			if enemySpecOf(member.Kind).bubble > 0 {
				protected = true
				break
			}
		}
		if protected {
			continue
		}
		id := s.NextID
		s.NextID++
		leader := members[0]
		s.Enemies[id] = Enemy{
			ID: id, Kind: EnemyCrawler, Party: party.ID, City: party.City,
			X: leader.X, Y: leader.Y,
			Health: enemySpecOf(EnemyCrawler).health,
		}
		if party.Size > 0 {
			party.Size++
		} else {
			party.Size = len(members) + 1
		}
		s.Parties[party.ID] = party
	}
}

func (s *State) legacyRepairWasAvailable() bool {
	if dropArrived(s, techMobileID) {
		return true
	}
	for _, id := range sortedBuildingIDs(s) {
		if s.Buildings[id].Kind == BuildingWarFactory {
			return true
		}
	}
	for _, id := range sortedRobotIDs(s) {
		if s.Robots[id].Kind == RobotRepair {
			return true
		}
	}
	return false
}

// spawnRobot adds one robot to the colony at a spot, with the tank full
// when it has one, and returns its ID.
func (s *State) spawnRobot(kind RobotKind, x, y float64) int64 {
	id := s.NextID
	s.NextID++
	r := Robot{ID: id, Kind: kind, X: x, Y: y, PostCol: -1, PostRow: -1}
	if r.tanked() {
		r.Tank = robotTankLiters
	}
	if health := robotMaxHealth(kind); health > 0 {
		r.Health = health
	}
	s.Robots[id] = r
	return id
}

// raise turns a finished job into the building it asked for.
func (s *State) raise(kind BuildingKind, col, row int) {
	id := s.NextID
	s.NextID++
	s.Buildings[id] = Building{
		ID: id, Kind: kind, Col: col, Row: row,
		Oil: initialBuildingOil(kind),
	}
}

// drainKey names a deposit tile inside State.Drain.
func drainKey(col, row int) string {
	return fmt.Sprintf("%d,%d", col, row)
}

// tileCenterUnits returns the middle of a tile, in world units.
func tileCenterUnits(col, row int) (x, y float64) {
	return float64(col)*unitsPerTile + unitsPerTile*0.5,
		float64(row)*unitsPerTile + unitsPerTile*0.5
}

// robotCell returns the cell a robot stands on, from its position.
func robotCell(r Robot) (col, row int) {
	return int(math.Floor(r.X / buildingCell)),
		int(math.Floor(r.Y / buildingCell))
}
