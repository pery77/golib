package main

// Actions in, state out: nothing but Apply's reducers touch the state.
// An action is a serializable struct, and Apply is total — any action on
// any state gives a well-defined result — and deterministic, so a seed
// plus a log of actions replays a game, which is also the shape of the
// later multiplayer server. Apply mutates the state it is given: the
// state has one owner (the scene, the tests, a later server), and
// copying it per action would buy nothing.

// Action is one order for the simulation.
type Action interface {
	apply(s *State)
}

// Apply applies an action to the state, in place.
func Apply(s *State, a Action) {
	useRegion(s.Seed)
	a.apply(s)
}

// Tick is the action the game loop sends 60 times per second of game
// time: one step of the simulation.
type Tick struct{}

func (Tick) apply(s *State) {
	s.Ticks++
	stepSim(s)
}

// SendRobot sends a free worker to a deposit tile: the tile becomes its
// post. It never retasks a worker that already has one. Builders can be
// assigned only through AssignRobot, by ID.
type SendRobot struct {
	Col int
	Row int
}

func (a SendRobot) apply(s *State) {
	kind := tileAt(a.Col, a.Row)
	if kind != kindOil && kind != kindLilac {
		return
	}
	if remainingAt(s, a.Col, a.Row) <= 0 {
		return
	}
	id := pickRobot(s, a.Col, a.Row)
	if id < 0 {
		return
	}
	r := s.Robots[id]
	r.PostCol, r.PostRow = a.Col, a.Row
	s.Robots[id] = r
}

// RecallRobot takes one robot's post away, which finishes any carry it
// holds and then returns to its role's ordinary work.
type RecallRobot struct {
	ID int64
}

func (a RecallRobot) apply(s *State) {
	r, ok := s.Robots[a.ID]
	if !ok || !r.hasPost() {
		return
	}
	r.clearPost()
	s.Robots[r.ID] = r
}

// AssignRobot sends one chosen builder or worker to a deposit tile. It
// leaves cargo in transit intact and cancels any unfinished loading.
type AssignRobot struct {
	ID       int64
	Col, Row int
}

func (a AssignRobot) apply(s *State) {
	r, ok := s.Robots[a.ID]
	if !ok || (r.Kind != RobotBuilder && r.Kind != RobotWorker) {
		return
	}
	kind := tileAt(a.Col, a.Row)
	if (kind != kindOil && kind != kindLilac) ||
		remainingAt(s, a.Col, a.Row) <= 0 {
		return
	}
	r.PostCol, r.PostRow = a.Col, a.Row
	r.WorkTicks, r.Pile = 0, 0
	r.clearConstruction()
	s.Robots[r.ID] = r
}

// MarkBuilding marks a cell for a building: the blueprint's cost is paid
// from the stores at once, and the job joins the queue for the robots to
// raise. It does nothing on a cell that can't take the kind (see
// canPlace), when the stores can't pay, or while the kind's schematics
// haven't arrived (see kindUnlocked).
type MarkBuilding struct {
	Kind     BuildingKind
	Col, Row int // the cell, the tile grid's last subdivision
}

func (a MarkBuilding) apply(s *State) {
	if !kindUnlocked(s, a.Kind) {
		return
	}
	if !canPlace(s, a.Kind, a.Col, a.Row) {
		return
	}
	lilac, oil := buildingCost(a.Kind)
	if !canAfford(s, a.Kind) {
		return
	}
	s.Stock.Lilac -= lilac
	s.payOil(oil)
	s.Jobs = append(s.Jobs, Job{
		Kind: a.Kind, Col: a.Col, Row: a.Row, Left: buildingWorkTicks,
	})
	x, y := cellCenterUnits(a.Col, a.Row)
	s.recordCost(costPlacement, 0, x, y, lilac, oil, false)
}

// QueueRobot puts a factory to work on one more robot, paying lilac and
// oil for it at once. A factory already building, or stores that can't
// pay, leave it as it is.
type QueueRobot struct {
	Building int64 // the factory's entity ID
	Kind     RobotKind
}

func (a QueueRobot) apply(s *State) {
	b, ok := s.Buildings[a.Building]
	if !ok {
		return
	}
	kind := a.Kind
	if kind == "" {
		kind, _, _, _, _ = robotWorks(b.Kind)
	}
	queueUnit(s, b.ID, kind)
}

// QueueMechanic puts a war factory to work on its one repair unit, paying
// its cost at once. A factory already building or with a mechanic leaves
// the action unanswered; the repair protocol must have arrived first.
type QueueMechanic struct {
	Building int64
}

func (a QueueMechanic) apply(s *State) {
	queueUnit(s, a.Building, RobotRepair)
}

func queueUnit(s *State, id int64, kind RobotKind) {
	b, ok := s.Buildings[id]
	if !ok || !canQueueUnit(s, b, kind) {
		return
	}
	lilac, oil, ticks, _ := robotProduction(kind)
	s.Stock.Lilac -= lilac
	s.payOil(oil)
	b.Work, b.WorkKind = ticks, kind
	s.Buildings[b.ID] = b
	x, y := cellCenterUnits(b.Col, b.Row)
	s.recordCost(costBuilding, b.ID, x, y, lilac, oil, false)
}

// OrderSquad tells a war factory's squad what to do: attack the party of
// a rival vehicle, that vehicle first, or guard a spot of the region. It
// does nothing for a building that is no war factory, and a vehicle that
// isn't there turns the order into guarding the spot.
type OrderSquad struct {
	Squad int64   // the war factory's entity ID
	Enemy int64   // the vehicle to go after; 0 guards the spot
	X, Y  float64 // the spot, in units
}

func (a OrderSquad) apply(s *State) {
	if b, ok := s.Buildings[a.Squad]; !ok || b.Kind != BuildingWarFactory {
		return
	}
	// A save from before the squads loads with no table for them.
	if s.Squads == nil {
		s.Squads = map[int64]Squad{}
	}
	sq := Squad{Home: a.Squad, Order: OrderGuard}
	most := float64(regionCols*unitsPerTile) - 1
	sq.X, sq.Y = clamp64(a.X, 1, most), clamp64(a.Y, 1, most)
	if e, ok := s.Enemies[a.Enemy]; ok {
		sq = Squad{
			Home: a.Squad, Order: OrderAttack,
			Party: e.Party, Focus: e.ID,
		}
	}
	s.Squads[a.Squad] = sq
}

// Demolish orders a building taken down: a builder walks to it and works
// it down (`demolishWorkTicks`), and only then does it go. Its tasks die
// with it, and its cost, the cost of a robot its factory was building,
// the pipes that started or ended at it and what the stores lose the
// roof for fall on its cell as one pile. It does nothing for a building
// that isn't there, one already ordered down, or a protector that alone
// shelters another building (see canDemolish).
type Demolish struct {
	Building int64 // the building's entity ID
}

func (a Demolish) apply(s *State) {
	b, ok := s.Buildings[a.Building]
	if !ok || b.Demolish > 0 || !canDemolish(s, b) {
		return
	}
	b.Demolish = demolishWorkTicks
	s.Buildings[a.Building] = b
}

// takeDown is a building leaving the state, demolished or destroyed: the
// given part of what it and its pipes cost, the robot it was building,
// the oil it held and what the stores lose the roof for fall on its cell
// as one pile.
func (s *State) takeDown(
	b Building,
	refund float64,
	cause BuildingDeathCause,
) {
	s.recordBuildingDeath(b, cause)
	delete(s.Buildings, b.ID)
	lilac, oil := buildingCost(b.Kind)
	lilac *= refund
	oil -= initialBuildingOil(b.Kind)
	oil *= refund
	if b.Work > 0 {
		if robotLilac, robotOil, _, builds := robotProduction(robotWorkKind(b)); builds {
			lilac += robotLilac * refund
			oil += robotOil * refund
		}
	}
	lilac += s.takePipesOf(b.ID) * refund
	s.dropPile(b.Col, b.Row, oil+b.Oil, lilac)
	s.spillOverflow(b.Col, b.Row)
}

// CancelJob takes a site out of the queue: the work put into it is gone
// and its cost falls on its cell as a pile. It does nothing on a cell
// with no site.
type CancelJob struct {
	Col, Row int // the site's cell
}

func (a CancelJob) apply(s *State) {
	for i, job := range s.Jobs {
		if job.Col != a.Col || job.Row != a.Row {
			continue
		}
		s.Jobs = append(s.Jobs[:i:i], s.Jobs[i+1:]...)
		lilac, oil := buildingCost(job.Kind)
		s.dropPile(job.Col, job.Row, oil*demolishRefund, lilac*demolishRefund)
		return
	}
}

// LayPipe marks a pipe from a pump or a tank to a tank, through the
// bends the player clicked: its sections are paid in lilac at once, and
// the robots lay it a section each. It does nothing when the ends
// can't take a pipe (see canLayPipe), when the stores can't pay, or
// before the frontier kit has arrived with the pipes (see dropArrived).
type LayPipe struct {
	From  int64       // the pump's or the tank's entity ID; 0 is the core
	To    int64       // the tank's entity ID; 0 is the core
	Bends []PipePoint // the clicks between the ends, in units
}

func (a LayPipe) apply(s *State) {
	if !dropArrived(s, techFrontierID) {
		return
	}
	sections, ok := canLayPipe(s, a.From, a.To, a.Bends)
	if !ok || s.Stock.Lilac < pipeCost(sections) {
		return
	}
	s.Stock.Lilac -= pipeCost(sections)
	// A save from before the pipes loads with no table for them.
	if s.Pipes == nil {
		s.Pipes = map[int64]Pipe{}
	}
	id := s.NextID
	s.NextID++
	s.Pipes[id] = Pipe{
		ID: id, From: a.From, To: a.To,
		Bends:    append([]PipePoint(nil), a.Bends...),
		Sections: sections,
		Left:     sections * pipeSectionWorkTicks,
	}
	start, _ := pipeEndSpot(s, a.From)
	s.recordCost(costPipe, id, start.X, start.Y, pipeCost(sections), 0, false)
}

// RemovePipe takes a pipe up at once, laid or not: what it was made of
// falls as a pile by the building it started at, or by the one it ended
// at when it started at the core. It does nothing for a pipe that isn't
// there.
type RemovePipe struct {
	Pipe int64 // the pipe's entity ID
}

// AckTech opens the schematics a drop brought in: the badge over the
// core goes away and the next in line, if any, takes its place. It does
// nothing for schematics that never arrived.
type AckTech struct {
	ID string // the drop's name (sim_tech.go)
}

func (a AckTech) apply(s *State) {
	if _, arrived := s.Tech[a.ID]; !arrived {
		return
	}
	s.Tech[a.ID] = true
}

func (a RemovePipe) apply(s *State) {
	p, ok := s.Pipes[a.Pipe]
	if !ok {
		return
	}
	delete(s.Pipes, p.ID)
	at, ok := s.Buildings[p.From]
	if !ok {
		at = s.Buildings[p.To]
	}
	s.dropPile(at.Col, at.Row, 0, pipeCost(p.Sections)*demolishRefund)
}

// The developer's actions, sent by the dev tools (dev.go) and by nobody
// else. They are actions like any other, so a log with them in it still
// replays.

// DevHoldSwell raises a swell and holds it up, or lets go of the held one
// and ends it. A held swell is no swell of the fog's own: it doesn't
// count, so the next ones come no sooner, longer or deeper for it.
type DevHoldSwell struct {
	On bool
}

func (a DevHoldSwell) apply(s *State) {
	s.Fog.Held = a.On
	if !a.On {
		s.Fog.SwellLeft = 0
		return
	}
	if s.Fog.SwellLeft <= 0 {
		s.Fog.SwellLeft = swellTicks(s)
	}
}

// DevResetWorld deals the region again, from nothing: the same ground for
// the seed the state already has, new ground for another.
type DevResetWorld struct {
	Seed int64
}

func (a DevResetWorld) apply(s *State) {
	*s = *newGameOn(a.Seed)
}

// DevNextVisit brings the next scheduled arrival in at once. It does
// nothing while a party is moving through the region.
type DevNextVisit struct{}

func (DevNextVisit) apply(s *State) {
	for _, id := range sortedPartyIDs(s) {
		p := s.Parties[id]
		if p.Stage != StageSettled {
			return
		}
	}
	s.Raids.NextAt = s.Ticks + 1
}

// DevHurryRivals ends rival camp, regroup and rebuild waits.
type DevHurryRivals struct{}

func (DevHurryRivals) apply(s *State) {
	for _, id := range sortedPartyIDs(s) {
		p := s.Parties[id]
		switch p.Stage {
		case StageBuild, StageCamp, StageRebuild, StageRegroup:
			p.Wait = 0
		}
		s.Parties[id] = p
	}
}

type DevNewCity struct{}

func (DevNewCity) apply(s *State) {
	if len(s.Cities) >= cityLimit {
		return
	}
	x, y, angle, ok := s.nextCitySpot()
	if !ok {
		return
	}
	s.foundCity(x, y, angle)
}

type DevFinishCityBuilding struct{}

func (DevFinishCityBuilding) apply(s *State) {
	for _, id := range sortedCityIDs(s) {
		city := s.Cities[id]
		if cityNeedsConstruction(s, city) {
			s.finishCityBuilding(&city)
			s.Cities[id] = city
			return
		}
	}
}

type DevFinishCityBattalion struct{}

func (DevFinishCityBattalion) apply(s *State) {
	for _, id := range sortedCityIDs(s) {
		city := s.Cities[id]
		before := len(s.Parties)
		s.finishCitySortie(&city)
		s.Cities[id] = city
		if len(s.Parties) > before {
			return
		}
	}
}

type DevSendCityBattalion struct{}

func (DevSendCityBattalion) apply(s *State) {
	for _, cityID := range sortedCityIDs(s) {
		for _, partyID := range sortedPartyIDs(s) {
			p := s.Parties[partyID]
			if p.City == cityID &&
				(p.Stage == StageCamp || p.Stage == StageRebuild ||
					p.Stage == StageRegroup) {
				p.Wait = 0
				s.Parties[partyID] = p
				return
			}
		}
	}
}

// DevSpawnRobot puts a built robot, its tank full, on a spot of the
// region, free of charge. It does nothing for a spot outside the region.
type DevSpawnRobot struct {
	X, Y float64 // units
}

func (a DevSpawnRobot) apply(s *State) {
	if a.X < 0 || a.Y < 0 ||
		a.X >= regionCols*unitsPerTile || a.Y >= regionRows*unitsPerTile {
		return
	}
	s.spawnRobot(RobotWorker, a.X, a.Y)
}

// DevNextTech makes the next drop of the ladder arrive at once, whether
// or not its clock or its visit has come. It does nothing once every
// drop is in.
type DevNextTech struct{}

func (DevNextTech) apply(s *State) {
	if s.Tech == nil {
		s.Tech = map[string]bool{}
	}
	for i := range techLadder {
		id := techLadder[i].id
		if _, ok := s.Tech[id]; !ok {
			s.Tech[id] = false
			return
		}
	}
}
