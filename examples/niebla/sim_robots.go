package main

import (
	"math"
	"sort"
)

// Tuning: the robots' numbers, with units in the name. The colony starts
// with one free builder; factory-built builders and workers cost oil and
// lilac. Every role has a tank and answers to the fog. A robot is a small,
// fast rover, 6 u across: it shuttles like an ant, carrying a little and
// coming back for more, so a worked deposit shows a constant coming and
// going.
const (
	startingBuilders   = 1    // builder the core gives the colony
	robotBuilderHealth = 60.0 // hull points for builders
	robotWorkerHealth  = 60.0 // hull points for workers

	robotSpeed       = 30.0      // units (m) per second
	robotLoadTicks   = 150       // ticks of loading at a deposit: 2.5 s
	robotCarryOil    = 30.0      // liters per worker trip
	robotCarryLilac  = 20.0      // kilograms per worker trip
	builderCarryPart = 1.0 / 3.0 // fraction of a worker's load

	// Idle robots rest by the core, lined up in ranks before its broad
	// face. Past parkSlots of them the ranks are full: the rest stand
	// inside the first ones, and the view writes how many there are.
	parkSlots    = 10   // robots the ranks show
	parkRankSize = 5    // robots to a rank
	parkSpacing  = 7.0  // u between two parked robots: one, and a bit
	parkFromCore = 10.0 // u from the core's middle to the first rank

	postSpreadRadius = 9.0 // u around the shared loading spot

	tankFullSlack = 0.001 // L short of the brim that still count as a full tank

	goldenAngle = 2.399963229728653 // spreads robots around what they work at
)

func robotMaxHealth(kind RobotKind) float64 {
	switch kind {
	case RobotBuilder:
		return robotBuilderHealth
	case RobotWorker:
		return robotWorkerHealth
	case RobotCombat:
		return trooperHealth
	case RobotRepair:
		return mechanicHealth
	}
	return 0
}

// stepSim moves the world one tick forward: the weather, the factories,
// pipes, protector upkeep, rivals and guard posts, then robots in ID order,
// so the outcome never depends on map iteration. A tanked robot whose oil
// or hull runs out outside every bubble is digested by the fog and leaves
// a quarter of its cost and onboard resources in a wreck.
func stepSim(s *State) {
	s.Deaths = nil
	s.BuildingDeaths = nil
	s.Costs = nil
	stepTech(s)
	stepFog(s)
	stepFactories(s)
	stepPipes(s)
	stepProtectors(s)
	stepSquads(s)
	stepEnemies(s)
	stepGuards(s)
	stepArtillery(s)
	stepShots(s)
	for _, id := range sortedRobotIDs(s) {
		r := s.Robots[id]
		// A defender the rivals shot this tick is gone already.
		if _, alive := s.Robots[id]; !alive {
			continue
		}
		stepRobot(s, &r)
		if r.tanked() && (r.Tank <= 0 || r.Health <= 0) &&
			miteExposureAt(s, r.X, r.Y) > 0 {
			s.recordRobotDeath(r)
			delete(s.Robots, id)
			s.dropRobotWreck(r)
			continue
		}
		s.Robots[id] = r
	}
	stepMiteWear(s)
}

func sortedRobotIDs(s *State) []int64 {
	ids := make([]int64, 0, len(s.Robots))
	for id := range s.Robots {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// robotTask is one line of a robot's day: its name, whether it claims
// the robot this tick, and the tick of it.
type robotTask struct {
	name   string
	claims func(r *Robot, s *State) bool
	step   func(r *Robot, s *State)
}

// The lines of a robot's day, by name.
const (
	taskHaul    = "haul"    // bring home what it carries
	taskRefuel  = "refuel"  // mind its tank
	taskRepair  = "repair"  // mechanics mend damaged buildings
	taskLoad    = "load"    // finish loading, at its post or at a pile
	taskBuild   = "build"   // build sites and lay pipe
	taskCollect = "collect" // pick up loose items
	taskPost    = "post"    // work its own post
	taskSquad   = "squad"   // troopers: follow the squad's order
	taskIdle    = "idle"    // stand by the core
)

// robotDay is the general task order. Builders alone can claim the build
// line; workers skip it, so marking construction never pulls them from
// their posts. The first line that claims a robot owns its tick.
var robotDay []robotTask

// Idling asks what the other robots are at, which reads the day back:
// filling it here keeps Go from seeing an initialization cycle.
func init() {
	robotDay = []robotTask{
		{taskHaul, (*Robot).hauling, (*Robot).stepHaul},
		{taskRefuel, (*Robot).refueling, (*Robot).stepRefuel},
		{taskRepair, (*Robot).repairing, (*Robot).stepRepair},
		{taskLoad, (*Robot).loading, (*Robot).stepLoad},
		{taskBuild, (*Robot).building, (*Robot).stepBuild},
		{taskCollect, (*Robot).collecting, (*Robot).stepCollect},
		{taskPost, (*Robot).posted, (*Robot).stepPost},
		{taskSquad, (*Robot).squadded, (*Robot).stepSquad},
		{taskIdle, (*Robot).idling, (*Robot).stepIdle},
	}
}

// taskNow returns the line of the day that owns the robot this tick.
func (r *Robot) taskNow(s *State) robotTask {
	for _, task := range robotDay {
		if task.claims(r, s) {
			return task
		}
	}
	return robotDay[len(robotDay)-1]
}

// stepRobot burns the tank and gives the tick to the robot's task. The
// robot carries no plan beyond its reserved construction task.
func stepRobot(s *State, r *Robot) {
	// A post with nothing left to give holds nobody: it ran dry under
	// another robot's hands, or an old save names ground the region has
	// since lost.
	if r.hasPost() && remainingAt(s, r.PostCol, r.PostRow) <= 0 {
		r.clearPost()
	}
	if r.WorkTicks > 0 && r.Pile == 0 && r.hasPost() &&
		oilPoolInFog(s, r.PostCol, r.PostRow) {
		r.WorkTicks = 0
	}
	if r.Kind == RobotBuilder {
		if _, valid := claimedConstruction(s, *r); !valid {
			r.clearConstruction()
		}
	}
	if r.Kind == RobotCombat {
		r.shoot(s)
	}
	// The tank pays for carrying: an empty-handed robot burns nothing.
	if r.tanked() && r.Tank > 0 && r.Carry > 0 {
		burn := robotBurnPerSecond / 60
		if !inSafeZone(s, r.X, r.Y) {
			// The swell's mist is meaner in the open, by as much as it presses.
			burn *= 1 + (fogSwellBurn-1)*s.Fog.Pressure
		}
		r.Tank = math.Max(0, r.Tank-burn)
	}
	task := r.taskNow(s)
	startX, startY := r.X, r.Y
	task.step(r, s)
	moved := math.Hypot(r.X-startX, r.Y-startY) > 1e-4
	stepStationaryWear(s, r, moved)
}

func stepStationaryWear(s *State, r *Robot, moved bool) {
	exposure := miteExposureAt(s, r.X, r.Y)
	if !r.tanked() || moved || exposure <= 0 {
		r.StillTicks = 0
		return
	}
	r.StillTicks++
	if r.StillTicks <= fogStillGraceTicks {
		return
	}
	fuelWear := fogStillBurnPerSecond * exposure / 60
	hullDamage := miteDamagePerSecond * exposure * miteSwellFactor(s) / 60
	r.Tank = math.Max(0, r.Tank-fuelWear)
	r.Health = math.Max(0, r.Health-hullDamage)
}

func (r *Robot) hauling(s *State) bool {
	return r.Carry > 0
}

func (r *Robot) stepHaul(s *State) {
	x, y := storeSpot(s, *r)
	if r.walkTowards(s, x, y) {
		s.deposit(r)
	}
}

func (r *Robot) stepRefuel(s *State) {
	x, y := refuelSpot(s, *r)
	if r.walkTowards(s, x, y) {
		s.refill(r)
	}
}

func (r *Robot) repairing(s *State) bool {
	return r.Kind == RobotRepair
}

func (r *Robot) stepRepair(s *State) {
	if building, damaged := damagedBuilding(s); damaged {
		cx, cy := cellCenterUnits(building.Col, building.Row)
		angle := float64(r.ID) * goldenAngle
		x := cx + math.Cos(angle)*11
		y := cy + math.Sin(angle)*11
		if r.walkTowards(s, x, y) {
			s.mend(building.ID, r)
		}
		return
	}
	x, y := repairStandbySpot(s, *r)
	r.walkTowards(s, x, y)
}

func repairStandbySpot(s *State, r Robot) (x, y float64) {
	if factory, ok := s.Buildings[r.Factory]; ok &&
		factory.Kind == BuildingWarFactory {
		x, y = cellCenterUnits(factory.Col, factory.Row)
		angle := float64(r.ID) * goldenAngle
		return x + math.Cos(angle)*18, y + math.Sin(angle)*18
	}
	return parkSlot(parkSlots - 1)
}

func (r *Robot) loading(s *State) bool {
	return r.WorkTicks > 0
}

func (r *Robot) stepLoad(s *State) {
	r.WorkTicks--
	if r.WorkTicks > 0 {
		return
	}
	if r.Pile != 0 {
		s.takeFromPile(r)
		return
	}
	s.takeLoad(r)
}

func (r *Robot) building(s *State) bool {
	if r.Kind != RobotBuilder {
		return false
	}
	if _, ok := claimedConstruction(s, *r); ok {
		return true
	}
	_, ok := freeConstruction(s, *r)
	return ok
}

func (r *Robot) stepBuild(s *State) {
	task, ok := claimedConstruction(s, *r)
	if !ok {
		task, ok = freeConstruction(s, *r)
		if !ok {
			r.clearConstruction()
			return
		}
		r.reserveConstruction(task)
	}
	angle := float64(r.ID) * goldenAngle
	standoff := 11.0
	if task.kind == constructionPipe {
		standoff = pipeLayStandoff
	}
	x := task.spot.X + math.Cos(angle)*standoff
	y := task.spot.Y + math.Sin(angle)*standoff
	if r.walkTowards(s, x, y) {
		switch task.kind {
		case constructionSite:
			s.workJob(task.index)
		case constructionDemolition:
			s.workDemolish(task.id)
		case constructionPipe:
			s.workSection(task.id, task.section)
		}
	}
	if _, valid := claimedConstruction(s, *r); !valid {
		r.clearConstruction()
		if next, free := freeConstruction(s, *r); free {
			r.reserveConstruction(next)
		}
	}
}

func (r *Robot) collecting(s *State) bool {
	if r.Kind == RobotWorker {
		if r.hasPost() {
			return false
		}
	} else if r.Kind != RobotBuilder {
		return false
	}
	_, found := nearestPile(s, *r)
	return found
}

func (r *Robot) stepCollect(s *State) {
	p, _ := nearestPile(s, *r)
	x, y := cellCenterUnits(p.Col, p.Row)
	if r.walkTowards(s, x, y) {
		r.WorkTicks = robotLoadTicks
		r.Pile = p.ID
	}
}

func (r *Robot) posted(s *State) bool {
	return (r.Kind == RobotBuilder || r.Kind == RobotWorker) &&
		r.hasPost() && remainingAt(s, r.PostCol, r.PostRow) > 0
}

func (r *Robot) stepPost(s *State) {
	if oilPoolInFog(s, r.PostCol, r.PostRow) {
		x, y := parkSlot(int(r.ID) % parkSlots)
		r.walkTowards(s, x, y)
		return
	}
	cx, cy := postSpot(r.PostCol, r.PostRow, r.ID)
	if r.walkTowards(s, cx, cy) {
		r.WorkTicks = robotLoadTicks
		r.Pile = 0
	}
}

func robotCarryCapacity(r Robot, cargo ThingType) float64 {
	capacity := 0.0
	switch cargo {
	case TypeOil:
		capacity = robotCarryOil
	case TypeLilac:
		capacity = robotCarryLilac
	}
	if r.Kind == RobotBuilder {
		capacity *= builderCarryPart
	}
	return capacity
}

// postSpot returns a worker's loading place beside the deposit's richest
// cell, on the side the pump leaves free. Workers spread around that spot
// so several sent to the same vein are easier to see.
func postSpot(col, row int, robotID int64) (x, y float64) {
	d, ok := depositAt(col, row)
	if !ok {
		return tileCenterUnits(col, row)
	}
	x, y = cellCenterUnits(d.HeartCol+1, d.HeartRow)
	angle := float64(robotID) * goldenAngle
	return x + math.Cos(angle)*postSpreadRadius,
		y + math.Sin(angle)*postSpreadRadius
}

func (r *Robot) idling(s *State) bool {
	return true
}

func (r *Robot) stepIdle(s *State) {
	px, py := parkSlot(idleRank(s, *r))
	r.walkTowards(s, px, py)
}

// idleRank returns how many idle robots come before this one, by ID:
// its place in the ranks. When one leaves, the ranks close up.
func idleRank(s *State, r Robot) int {
	rank := 0
	for _, id := range sortedRobotIDs(s) {
		if id >= r.ID {
			break
		}
		other := s.Robots[id]
		if other.taskNow(s).name == taskIdle {
			rank++
		}
	}
	return rank
}

// idleRobots returns how many robots have nothing to do.
func idleRobots(s *State) int {
	idle := 0
	for _, id := range sortedRobotIDs(s) {
		r := s.Robots[id]
		if r.taskNow(s).name == taskIdle {
			idle++
		}
	}
	return idle
}

// refueling reports whether the tank owns the robot's day: low, it
// claims it and walks to the nearest refill post; there it stands until
// the tank is full, even past the low line, so it doesn't dance between
// post and work. A dry post holds nobody with oil left in its tank: the
// way to fill the posts again is to go and fetch oil.
func (r *Robot) refueling(s *State) bool {
	if !r.tanked() || r.Tank >= robotTankLiters-tankFullSlack {
		return false
	}
	if r.Tank > 0 && tankOil(s, refuelTank(s, *r)) <= 0 {
		return false
	}
	if r.Tank < robotTankLiters*robotLowTankAt {
		return true
	}
	x, y := refuelSpot(s, *r)
	return math.Hypot(r.X-x, r.Y-y) < 0.5
}

// chargeStatus names what a built robot's tank has it doing, when it has
// any claim: "" when the tank is fine, else low oil on the walk over,
// refueling while it fills, or out of oil waiting for the stores.
func chargeStatus(s *State, r Robot) string {
	if !r.tanked() {
		return ""
	}
	x, y := refuelSpot(s, r)
	at := math.Hypot(r.X-x, r.Y-y) < 0.5
	switch {
	case at && r.Tank < robotTankLiters:
		if tankOil(s, refuelTank(s, r)) <= 0 {
			return "out of oil"
		}
		return "refueling"
	case !at && r.Tank < robotTankLiters*robotLowTankAt:
		return "low oil"
	}
	return ""
}

// refill pours oil from the post's tank into the robot's, at the
// charger's pace; a dry post leaves the robot standing there.
func (s *State) refill(r *Robot) {
	post := refuelTank(s, *r)
	fill := math.Min(chargerRefillPerSec/60, robotTankLiters-r.Tank)
	fill = math.Min(fill, tankOil(s, post))
	s.addOil(post, -fill)
	r.Tank += fill
}

// walkTowards moves the robot one tick's step towards a point and
// reports whether it arrived. The fog drags at every robot, core or
// built, slowing it the deeper the tile sits in the mist; inside a
// bubble the step is whole, and a swell's pushed band is meaner
// (sim_fog.go).
func (r *Robot) walkTowards(s *State, x, y float64) bool {
	step := robotSpeed / 60 * fogDrag(s, r.X, r.Y)
	dx, dy := x-r.X, y-r.Y
	d := math.Hypot(dx, dy)
	if d <= step {
		if d > 0 {
			r.Facing = facingFromMovement(dx, dy)
		}
		r.X, r.Y = x, y
		return true
	}
	moveX, moveY := dx/d*step, dy/d*step
	r.X += moveX
	r.Y += moveY
	if moveX != 0 || moveY != 0 {
		r.Facing = facingFromMovement(moveX, moveY)
	}
	return false
}

// takeLoad fills the robot's arms from its post's patch and sends it
// home with them. A patch that ran dry releases the robot.
func (s *State) takeLoad(r *Robot) {
	d, ok := depositAt(r.PostCol, r.PostRow)
	if !ok {
		r.clearPost()
		return
	}
	if d.Kind == kindOil && oilPoolInFog(s, r.PostCol, r.PostRow) {
		return
	}
	key := depositKey(d)
	remaining := s.Drain[key]
	capacity := robotCarryCapacity(*r, TypeLilac)
	if d.Kind == kindOil {
		capacity = robotCarryCapacity(*r, TypeOil)
	}
	take := math.Min(capacity, remaining)
	if take <= 0 {
		r.clearPost()
		return
	}
	s.Drain[key] = remaining - take
	r.Carry = take
	if d.Kind == kindOil {
		r.Cargo = TypeOil
	} else {
		r.Cargo = TypeLilac
	}
	if s.Drain[key] <= 0 {
		r.clearPost()
	}
}

// deposit empties the robot's arms into the colony's stores, as much of
// the load as their room takes; a store already full leaves the rest in
// its arms, and the robot stands at the store trying again every tick.
func (s *State) deposit(r *Robot) {
	var room float64
	switch r.Cargo {
	case TypeOil:
		room = tankRoom(s, haulTank(s, *r))
	case TypeLilac:
		room = lilacCap(s) - s.Stock.Lilac
	default:
		r.Carry, r.Cargo = 0, ""
		return
	}
	put := math.Min(r.Carry, math.Max(0, room))
	if put > 0 {
		s.Deliveries++
	}
	switch r.Cargo {
	case TypeOil:
		s.addOil(haulTank(s, *r), put)
	case TypeLilac:
		s.Stock.Lilac += put
	}
	r.Carry -= put
	if r.Carry < 0.0001 {
		r.Carry, r.Cargo = 0, ""
	}
}

// workJob puts a tick of work into the selected job; done, the building
// the job asked for rises from the ground.
func (s *State) workJob(index int) {
	if index < 0 || index >= len(s.Jobs) {
		return
	}
	job := s.Jobs[index]
	job.Left--
	if job.Left > 0 {
		s.Jobs[index] = job
		return
	}
	s.Jobs = append(s.Jobs[:index:index], s.Jobs[index+1:]...)
	if job.Kind != "" {
		s.raise(job.Kind, job.Col, job.Row)
	}
}

// parkSlot returns where the idle robot of a rank rests: in ranks of
// parkRankSize before the core's broad face, which looks down the
// world's x axis, centered on it. Past parkSlots the ranks start over.
func parkSlot(rank int) (x, y float64) {
	cx, cy := tileCenterUnits(coreCol, coreRow)
	slot := rank % parkSlots
	line, place := slot/parkRankSize, slot%parkRankSize
	return cx + parkFromCore + float64(line)*parkSpacing,
		cy + (float64(place)-float64(parkRankSize-1)/2)*parkSpacing
}

// parkCenter returns the middle of the ranks.
func parkCenter() (x, y float64) {
	cx, cy := tileCenterUnits(coreCol, coreRow)
	lines := float64(parkSlots/parkRankSize - 1)
	return cx + parkFromCore + lines*parkSpacing/2, cy
}

// remainingAt returns what is left in the deposit patch a tile belongs
// to; a tile that holds no deposit has nothing.
func remainingAt(s *State, col, row int) float64 {
	d, ok := depositAt(col, row)
	if !ok {
		return 0
	}
	return s.Drain[depositKey(d)]
}

// postRobots returns the workers assigned to the deposit patch the given
// tile belongs to, in ID order.
func postRobots(s *State, col, row int) []Robot {
	want, ok := depositAt(col, row)
	if !ok {
		return nil
	}
	var robots []Robot
	for _, id := range sortedRobotIDs(s) {
		r := s.Robots[id]
		if !r.hasPost() {
			continue
		}
		if d, ok := depositAt(r.PostCol, r.PostRow); ok && d == want {
			robots = append(robots, r)
		}
	}
	return robots
}

// pickRobot chooses the first unassigned worker. SendRobot never retasks
// an existing post or assigns a builder implicitly.
func pickRobot(s *State, col, row int) int64 {
	_, ok := depositAt(col, row)
	if !ok {
		return -1
	}
	for _, id := range sortedRobotIDs(s) {
		r := s.Robots[id]
		if r.Kind != RobotWorker || r.hasPost() {
			continue
		}
		return id
	}
	return -1
}
