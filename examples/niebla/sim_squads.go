package main

import "math"

// Military units: a war factory builds troopers, up to six in its squad,
// and, after the repair protocol, one mechanic. A squad guards a spot or
// attacks a rival party, a vehicle of it first; nobody places a trooper by
// hand. Mechanics work alone, repairing buildings for oil. Troopers shoot
// rivals in reach and pay each shot from their tanks. The rivals' guns
// shoot back at troopers, mechanics and guard posts; their shells can also
// damage buildings.

// Tuning: the squads' numbers, with units in the name.
const (
	warFactoryCostLilac = 250.0 // kg
	warFactoryCostOil   = 50.0  // L

	trooperCostLilac   = 80.0 // kg
	trooperCostOil     = 60.0 // L
	trooperBuildTicks  = 900  // ticks to build one: 15 s
	squadSize          = 6    // troopers to a war factory
	trooperHealth      = 80.0
	trooperReloadTicks = 30 // ticks between two shots
	trooperShotDamage  = 6.0
	trooperShotOil     = 0.2 // L a shot burns, out of the trooper's own tank

	mechanicCostLilac  = 100.0 // kg
	mechanicCostOil    = 50.0  // L
	mechanicBuildTicks = 900   // ticks to build one: 15 s
	mechanicPerFactory = 1
	mechanicHealth     = 60.0

	trooperHealPerSecond = 2.0 // health a trooper mends resting at its spot, under a bubble
	squadStandoff        = 0.8 // the share of its reach an attacker closes to
)

// SquadOrder is what a squad was told to do.
type SquadOrder string

const (
	OrderGuard  SquadOrder = "guard"  // stand at a spot
	OrderAttack SquadOrder = "attack" // go after a rival party
)

// Squad is a war factory's troopers as one unit: its order, and what the
// order is about. It lives under its war factory's ID. A squad nobody
// has ordered yet has no entry, and guards its war factory.
type Squad struct {
	Home  int64 // the war factory's entity ID
	Order SquadOrder
	X, Y  float64 // guard: the spot, in units
	Party int64   // attack: the rival party
	Focus int64   // attack: the vehicle to shoot first
}

// squadOf returns a war factory's squad: the one in the state, or the
// one that guards its own door.
func squadOf(s *State, home int64) Squad {
	if sq, ok := s.Squads[home]; ok {
		return sq
	}
	sq := Squad{Home: home, Order: OrderGuard}
	sq.X, sq.Y = parkCenter()
	if b, ok := s.Buildings[home]; ok {
		sq.X, sq.Y = cellCenterUnits(b.Col, b.Row)
	}
	return sq
}

// squadMembers lists a squad's troopers, in ID order.
func squadMembers(s *State, home int64) []Robot {
	var members []Robot
	for _, id := range sortedRobotIDs(s) {
		if r := s.Robots[id]; r.Kind == RobotCombat && r.Squad == home {
			members = append(members, r)
		}
	}
	return members
}

// squadRoom reports whether a war factory may build one more trooper.
func squadRoom(s *State, b Building) bool {
	building := 0
	if b.Work > 0 && robotWorkKind(b) == RobotCombat {
		building = 1
	}
	return len(squadMembers(s, b.ID))+building < squadSize
}

// stepSquads keeps every order about something: an attack on a party
// that is gone turns back into guarding the war factory's door, and a
// focus that fell passes to the party's leader.
func stepSquads(s *State) {
	for _, id := range sortedSquadIDs(s) {
		sq := s.Squads[id]
		if _, stands := s.Buildings[id]; !stands {
			delete(s.Squads, id)
			continue
		}
		if sq.Order != OrderAttack {
			continue
		}
		if sq.Party == 0 {
			if _, alive := s.Enemies[sq.Focus]; !alive {
				delete(s.Squads, id)
			}
			continue
		}
		members := partyMembers(s, sq.Party)
		if len(members) == 0 {
			delete(s.Squads, id)
			continue
		}
		if _, alive := s.Enemies[sq.Focus]; !alive {
			sq.Focus = members[0].ID
			s.Squads[id] = sq
		}
	}
}

func sortedSquadIDs(s *State) []int64 {
	ids := make([]int64, 0, len(s.Squads))
	for id := range s.Squads {
		ids = append(ids, id)
	}
	sortIDs(ids)
	return ids
}

// squadded claims a trooper's day: all of it that its tank leaves.
func (r *Robot) squadded(s *State) bool {
	return r.Kind == RobotCombat
}

// stepSquad is a trooper's tick under its squad's order: to its place
// around the guarded spot, where it mends under a bubble, or after the
// vehicle its squad is to shoot first, as close as squadStandoff of its
// reach.
func (r *Robot) stepSquad(s *State) {
	sq := squadOf(s, r.Squad)
	place := 0
	for _, other := range squadMembers(s, r.Squad) {
		if other.ID < r.ID {
			place++
		}
	}
	dx, dy := formationOffset(place + 1)
	if target, ok := s.Enemies[sq.Focus]; ok && sq.Order == OrderAttack {
		if math.Hypot(target.X-r.X, target.Y-r.Y) >
			smallArmsRangeUnits*squadStandoff {
			r.walkTowards(s, target.X+dx, target.Y+dy)
		}
		return
	}
	if r.walkTowards(s, sq.X+dx, sq.Y+dy) && inSafeZone(s, r.X, r.Y) {
		r.Health = math.Min(trooperHealth, r.Health+trooperHealPerSecond/60)
	}
}

// shoot reloads a trooper's gun and fires it when it is ready, has oil
// for the shot and somebody in reach: its squad's focus before anybody
// else.
func (r *Robot) shoot(s *State) {
	if r.Reload > 0 {
		r.Reload--
		return
	}
	target, found := s.Enemies[squadOf(s, r.Squad).Focus]
	if !found || math.Hypot(target.X-r.X, target.Y-r.Y) >
		smallArmsRangeUnits {
		target, found = nearestEnemy(s, r.X, r.Y, smallArmsRangeUnits)
	}
	if !found || r.Tank < trooperShotOil {
		r.Aim = 0
		return
	}
	r.Tank -= trooperShotOil
	s.recordCost(costRobot, r.ID, r.X, r.Y, 0, trooperShotOil, true)
	r.Reload, r.Aim = trooperReloadTicks, target.ID
	s.fire(Shot{
		Kind: ShotBullet, FromX: r.X, FromY: r.Y, ToX: target.X, ToY: target.Y,
		Enemy: target.ID, Damage: trooperShotDamage,
	})
}

// stepEnemyGuns is the rivals shooting back: a vehicle with a gun fires
// at the nearest trooper, mechanic or guard post in its reach, whichever
// stands closer, and at nothing else.
func stepEnemyGuns(s *State) {
	for _, id := range sortedEnemyIDs(s) {
		e := s.Enemies[id]
		if e.Kind == EnemyArtillery {
			s.fireCityArtillery(e)
			continue
		}
		spec := enemySpecOf(e.Kind)
		if spec.damage <= 0 {
			continue
		}
		if e.Reload > 0 {
			e.Reload--
			s.Enemies[id] = e
			continue
		}
		defender, defenderIn := nearestColonyUnit(s, e.X, e.Y, spec.gunRange)
		post, postIn := nearestGuard(s, e.X, e.Y, spec.gunRange)
		ptx, pty := cellCenterUnits(post.Col, post.Row)
		switch {
		case defenderIn && (!postIn ||
			math.Hypot(defender.X-e.X, defender.Y-e.Y) <=
				math.Hypot(ptx-e.X, pty-e.Y)):
			e.Reload, e.Aim = spec.reload, defender.ID
			s.Enemies[id] = e
			s.fire(Shot{
				Kind: ShotBullet, FromX: e.X, FromY: e.Y,
				ToX: defender.X, ToY: defender.Y,
				Robot: defender.ID, Damage: spec.damage, Rival: true,
			})
		case postIn:
			e.Reload, e.Aim = spec.reload, post.ID
			s.Enemies[id] = e
			s.fire(Shot{
				Kind: ShotBullet, FromX: e.X, FromY: e.Y,
				ToX: ptx, ToY: pty,
				Building: post.ID, Damage: spec.damage, Rival: true,
			})
		default:
			e.Aim = 0
			s.Enemies[id] = e
		}
	}
}

// nearestColonyUnit returns the nearest trooper or mechanic within reach;
// IDs break ties.
func nearestColonyUnit(s *State, x, y, reach float64) (Robot, bool) {
	var best Robot
	found, bestGap := false, reach
	for _, id := range sortedRobotIDs(s) {
		r := s.Robots[id]
		if r.Kind != RobotCombat && r.Kind != RobotRepair {
			continue
		}
		if gap := math.Hypot(r.X-x, r.Y-y); gap <= bestGap && (!found || gap < bestGap) {
			best, found, bestGap = r, true, gap
		}
	}
	return best, found
}

// nearestGuard returns the guard post closest to a spot, within a
// reach; IDs break ties.
func nearestGuard(s *State, x, y, reach float64) (Building, bool) {
	var best Building
	found, bestGap := false, reach
	for _, id := range sortedBuildingIDs(s) {
		b := s.Buildings[id]
		if b.Kind != BuildingGuard {
			continue
		}
		bx, by := cellCenterUnits(b.Col, b.Row)
		if gap := math.Hypot(bx-x, by-y); gap <= bestGap && (!found || gap < bestGap) {
			best, found, bestGap = b, true, gap
		}
	}
	return best, found
}
