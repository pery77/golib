package main

import (
	"math"
	"sort"
)

// Shots. Nothing here hits at the speed of light: a gun puts a bullet in
// the air, which flies to its target and hurts it on arrival, and an
// artillery piece lobs a shell at a spot of ground, which bursts there
// and hurts whoever of the other side stands in the blast - so a shell
// aimed at somebody who moved on is a shell wasted. Shots are state, so a
// save keeps what is in the air.
//
// Rival fire can hurt buildings and defenders: a building falls into a
// wreck's pile when its damage reaches its health, and only mechanics
// repair one, spending oil from their tanks and the building's proportional
// repair cost from colony stores. The core takes nothing.

// Tuning: the shots' numbers, with units in the name.
const (
	bulletSpeed            = 480.0 // u/s
	shellSpeed             = 150.0 // u/s along the ground: a shell takes its time
	shellMuzzleOffsetUnits = 24.0  // u ahead of the firing unit

	shellBlastUnits = 50.0 // u of a shell's blast

	// The colony's artillery: a building that shells the rivals it can
	// reach and somebody of the colony sees.
	artilleryCostLilac   = 300.0 // kg
	artilleryCostOil     = 80.0  // L
	artilleryRangeUnits  = 1500.0
	artilleryMinUnits    = 200.0 // u under which a piece can't drop a shell
	artilleryReloadTicks = 360   // ticks between two shells: 6 s
	artilleryShellDamage = 60.0
	artilleryShellLilac  = 10.0  // kg a shell costs
	artilleryShellOil    = 2.0   // L a shell costs
	sightUnits           = 450.0 // u around a robot or a building of the colony that it sees

	// What a building stands before it falls, and how it comes back.
	buildingHealthPoints  = 200.0
	protectorHealthPoints = 300.0
	repairPerSecond       = 6.0 // damage a mechanic repairs
	repairOilPerPoint     = 0.2 // L a mechanic spends per point repaired
	repairCostShare       = 0.5 // share of construction cost for a full repair
	wreckRefund           = 0.5 // the part of its cost a destroyed building leaves
)

// ShotKind names what flies.
type ShotKind string

const (
	ShotBullet ShotKind = "bullet" // flies to a target and hurts it
	ShotShell  ShotKind = "shell"  // flies to a spot and bursts there
)

// Shot is one thing in the air, on its way from where it was fired to
// where it lands. A bullet follows its target; a shell doesn't.
type Shot struct {
	ID           int64
	Kind         ShotKind
	X, Y         float64 // where it is over the ground, in units
	FromX, FromY float64
	ToX, ToY     float64
	Enemy        int64 // bullets: the rival vehicle it flies to
	Robot        int64 // bullets: the defender it flies to
	Building     int64 // bullets: the guard post it flies to
	Damage       float64
	Rival        bool // the rivals fired it: a shell's blast hurts the other side
}

func sortedShotIDs(s *State) []int64 {
	ids := make([]int64, 0, len(s.Shots))
	for id := range s.Shots {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// fire puts a shot in the air.
func (s *State) fire(shot Shot) {
	// A save from before the shots loads with no table for them.
	if s.Shots == nil {
		s.Shots = map[int64]Shot{}
	}
	shot.ID = s.NextID
	s.NextID++
	shot.X, shot.Y = shot.FromX, shot.FromY
	s.Shots[shot.ID] = shot
}

func shellLaunchPoint(
	fromX, fromY, toX, toY float64,
) (x, y float64) {
	dx, dy := toX-fromX, toY-fromY
	distance := math.Hypot(dx, dy)
	if distance == 0 {
		return fromX, fromY
	}
	offset := math.Min(shellMuzzleOffsetUnits, distance/2)
	return fromX + dx/distance*offset, fromY + dy/distance*offset
}

// stepShots flies every shot a tick forward and lands the ones that
// arrive.
func stepShots(s *State) {
	for _, id := range sortedShotIDs(s) {
		shot := s.Shots[id]
		speed := shellSpeed
		if shot.Kind == ShotBullet {
			speed = bulletSpeed
			if e, ok := s.Enemies[shot.Enemy]; ok {
				shot.ToX, shot.ToY = e.X, e.Y
			}
			if r, ok := s.Robots[shot.Robot]; ok {
				shot.ToX, shot.ToY = r.X, r.Y
			}
			if b, ok := s.Buildings[shot.Building]; ok {
				shot.ToX, shot.ToY = cellCenterUnits(b.Col, b.Row)
			}
		}
		dx, dy := shot.ToX-shot.X, shot.ToY-shot.Y
		gap := math.Hypot(dx, dy)
		if step := speed / 60; gap > step {
			shot.X += dx / gap * step
			shot.Y += dy / gap * step
			s.Shots[id] = shot
			continue
		}
		delete(s.Shots, id)
		s.land(shot)
	}
}

// land is a shot arriving: a bullet hurts its target if it still stands,
// a shell hurts everybody of the other side within its blast - rival
// vehicles or city structures for the colony's, troopers, mechanics or
// buildings for the rivals'.
func (s *State) land(shot Shot) {
	if shot.Kind == ShotBullet {
		s.hurtEnemy(shot.Enemy, shot.Damage)
		s.hurtColonyUnit(shot.Robot, shot.Damage)
		if shot.Rival {
			s.hurtBuildingByRival(shot.Building, shot.Damage)
		} else {
			s.hurtBuilding(shot.Building, shot.Damage)
		}
		return
	}
	near := func(x, y float64) bool {
		return math.Hypot(x-shot.ToX, y-shot.ToY) <= shellBlastUnits
	}
	if !shot.Rival {
		for _, id := range sortedEnemyIDs(s) {
			if e := s.Enemies[id]; near(e.X, e.Y) {
				s.hurtEnemy(id, shot.Damage)
			}
		}
		return
	}
	for _, id := range sortedRobotIDs(s) {
		if r := s.Robots[id]; near(r.X, r.Y) {
			s.hurtColonyUnit(id, shot.Damage)
		}
	}
	for _, id := range sortedBuildingIDs(s) {
		b := s.Buildings[id]
		if x, y := cellCenterUnits(b.Col, b.Row); near(x, y) {
			s.hurtBuildingByRival(id, shot.Damage)
		}
	}
}

func (s *State) hurtBuildingByRival(id int64, damage float64) {
	if _, stands := s.Buildings[id]; stands && damage > 0 {
		s.Raids.RivalBuildingHit = true
	}
	s.hurtBuilding(id, damage)
}

// hurtEnemy takes health off a rival vehicle, and the vehicle with it
// when none is left. It does nothing for a vehicle that isn't there.
func (s *State) hurtEnemy(id int64, damage float64) {
	e, ok := s.Enemies[id]
	if !ok {
		return
	}
	e.Health -= damage
	s.Enemies[id] = e
	if e.Health <= 0 {
		s.killEnemy(id)
	}
}

// hurtColonyUnit applies rival fire to troopers and mechanics; one that
// falls leaves a quarter of its cost and onboard resources in a wreck.
func (s *State) hurtColonyUnit(id int64, damage float64) {
	r, ok := s.Robots[id]
	if !ok || (r.Kind != RobotCombat && r.Kind != RobotRepair) {
		return
	}
	r.Health -= damage
	s.Robots[id] = r
	if r.Health <= 0 {
		s.recordRobotDeath(r)
		delete(s.Robots, id)
		s.dropRobotWreck(r)
	}
}

// buildingHealth returns what a kind of building stands before it falls.
func buildingHealth(kind BuildingKind) float64 {
	if kind == BuildingProtector {
		return protectorHealthPoints
	}
	return buildingHealthPoints
}

func buildingRepairCost(kind BuildingKind, damage float64) (lilac, oil float64) {
	health := buildingHealth(kind)
	if damage <= 0 || health <= 0 {
		return 0, 0
	}

	fraction := math.Min(damage/health, 1) * repairCostShare
	lilac, oil = buildingCost(kind)
	return lilac * fraction, oil * fraction
}

// hurtBuilding adds to the damage a building has taken; one that has
// taken all it stands falls, and the colony is told.
func (s *State) hurtBuilding(id int64, damage float64) {
	b, ok := s.Buildings[id]
	if !ok {
		return
	}
	b.Damage += damage
	s.Buildings[id] = b
	if b.Damage < buildingHealth(b.Kind) {
		return
	}
	x, y := cellCenterUnits(b.Col, b.Row)
	s.takeDown(b, wreckRefund, BuildingDestroyed)
	s.report(ReportRazed, 0, x, y)
}

// damagedBuilding returns the oldest damaged building for the mechanics.
func damagedBuilding(s *State) (Building, bool) {
	for _, id := range sortedBuildingIDs(s) {
		if b := s.Buildings[id]; b.Damage > 0 {
			return b, true
		}
	}
	return Building{}, false
}

// mend repairs a building for one mechanic tick, spending colony resources
// and the mechanic's tank in proportion to the damage it removes.
func (s *State) mend(id int64, mechanic *Robot) {
	if b, ok := s.Buildings[id]; ok {
		repair := math.Min(repairPerSecond/60, b.Damage)
		repair = math.Min(repair, mechanic.Tank/repairOilPerPoint)
		constructionLilac, constructionOil := buildingCost(b.Kind)
		health := buildingHealth(b.Kind)
		lilacPerPoint := constructionLilac * repairCostShare / health
		oilPerPoint := constructionOil * repairCostShare / health
		if lilacPerPoint > 0 {
			repair = math.Min(repair, s.Stock.Lilac/lilacPerPoint)
		}
		if oilPerPoint > 0 {
			repair = math.Min(repair, oilTotal(s)/oilPerPoint)
		}
		if repair <= 0 {
			return
		}
		lilac, repairOil := buildingRepairCost(b.Kind, repair)
		b.Damage = math.Max(0, b.Damage-repair)
		mechanicOil := repair * repairOilPerPoint
		mechanic.Tank = math.Max(0,
			mechanic.Tank-mechanicOil)
		s.Stock.Lilac = math.Max(0, s.Stock.Lilac-lilac)
		s.payOil(repairOil)
		s.Buildings[id] = b
		x, y := cellCenterUnits(b.Col, b.Row)
		s.recordCost(costBuilding, b.ID, x, y, lilac, repairOil, true)
		s.recordCost(costRobot, mechanic.ID, mechanic.X, mechanic.Y,
			0, mechanicOil, true)
	}
}

// seen reports whether somebody of the colony sees a spot: a robot or a
// building within sightUnits of it, or the core.
func seen(s *State, x, y float64) bool {
	cx, cy := tileCenterUnits(coreCol, coreRow)
	if math.Hypot(x-cx, y-cy) <= sightUnits {
		return true
	}
	for _, id := range sortedRobotIDs(s) {
		if r := s.Robots[id]; math.Hypot(x-r.X, y-r.Y) <= sightUnits {
			return true
		}
	}
	for _, id := range sortedBuildingIDs(s) {
		bx, by := cellCenterUnits(s.Buildings[id].Col, s.Buildings[id].Row)
		if math.Hypot(x-bx, y-by) <= sightUnits {
			return true
		}
	}
	return false
}

// stepArtillery reloads the colony's pieces and fires the ones that are
// ready, can pay for a shell and have a target: a rival somebody sees,
// no nearer than the piece's minimum and no farther than its reach, city
// structures before mobile forces, and then the nearest.
func stepArtillery(s *State) {
	for _, id := range sortedBuildingIDs(s) {
		b := s.Buildings[id]
		if b.Kind != BuildingArtillery {
			continue
		}
		if b.Reload > 0 {
			b.Reload--
			s.Buildings[id] = b
			continue
		}
		x, y := cellCenterUnits(b.Col, b.Row)
		target, found := artilleryTarget(s, x, y)
		if !found || s.Stock.Lilac < artilleryShellLilac || oilTotal(s) < artilleryShellOil {
			continue
		}
		s.Stock.Lilac -= artilleryShellLilac
		s.payOil(artilleryShellOil)
		s.recordCost(costBuilding, b.ID, x, y,
			artilleryShellLilac, artilleryShellOil, true)
		b.Reload, b.Aim = artilleryReloadTicks, target.ID
		s.Buildings[id] = b
		muzzleX, muzzleY := shellLaunchPoint(x, y, target.X, target.Y)
		s.fire(Shot{
			Kind:  ShotShell,
			FromX: muzzleX, FromY: muzzleY,
			ToX: target.X, ToY: target.Y,
			Damage: artilleryShellDamage,
		})
	}
}

func artilleryTarget(s *State, x, y float64) (Enemy, bool) {
	var best Enemy
	found, bestStructure, bestGap := false, false, 0.0
	for _, id := range sortedEnemyIDs(s) {
		e := s.Enemies[id]
		gap := math.Hypot(e.X-x, e.Y-y)
		if gap < artilleryMinUnits || gap > artilleryRangeUnits || !seen(s, e.X, e.Y) {
			continue
		}
		structure := isRivalBuilding(e)
		if !found || (structure && !bestStructure) ||
			(structure == bestStructure && gap < bestGap) {
			best, found, bestStructure, bestGap = e, true, structure, gap
		}
	}
	return best, found
}
