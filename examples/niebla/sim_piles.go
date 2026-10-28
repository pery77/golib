package main

import (
	"math"
	"sort"
)

// Demolition and loose items. What is built can be unbuilt: a building
// ordered down is worked on by a builder and leaves the state when the
// work is done, and everything it was made of or held falls on its cell
// as one pile, which the robots haul back to the stores. Nothing goes
// straight back: a refund is a haul.
const (
	demolishWorkTicks = 300 // ticks of robot work to take a building down: 5 s

	demolishRefund  = 1.0  // the part of a blueprint's cost that falls to the ground
	unitWreckRefund = 0.25 // the part of a lost unit's resources recovered

	pileDust = 0.0001 // under this, a pile's amount counts as nothing

	storeStandoff = 11.0 // u from a store's middle to where a robot unloads
)

func sortedPileIDs(s *State) []int64 {
	ids := make([]int64, 0, len(s.Piles))
	for id := range s.Piles {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// pileAt returns the pile lying on a cell, if any.
func pileAt(s *State, col, row int) (Pile, bool) {
	for _, id := range sortedPileIDs(s) {
		if p := s.Piles[id]; p.Col == col && p.Row == row {
			return p, true
		}
	}
	return Pile{}, false
}

// dropPile leaves loose items on a cell, joining the pile already there
// when there is one. Nothing to drop leaves no pile.
func (s *State) dropPile(col, row int, oil, lilac float64) {
	if oil < pileDust && lilac < pileDust {
		return
	}
	// A save from before the piles loads with no table for them.
	if s.Piles == nil {
		s.Piles = map[int64]Pile{}
	}
	p, ok := pileAt(s, col, row)
	if !ok {
		p = Pile{ID: s.NextID, Col: col, Row: row}
		s.NextID++
	}
	p.Oil += oil
	p.Lilac += lilac
	s.Piles[p.ID] = p
}

func (s *State) dropRobotWreck(r Robot) {
	lilac, oil := 0.0, 0.0
	switch r.Kind {
	case RobotBuilder, RobotWorker:
		lilac, oil = robotCostLilac, robotCostOil
	case RobotCombat:
		lilac, oil = trooperCostLilac, trooperCostOil
	case RobotRepair:
		lilac, oil = mechanicCostLilac, mechanicCostOil
	}

	oil += r.Tank
	switch r.Cargo {
	case TypeOil:
		oil += r.Carry
	case TypeLilac:
		lilac += r.Carry
	}

	col, row := robotCell(r)
	s.dropPile(
		col,
		row,
		oil*unitWreckRefund,
		lilac*unitWreckRefund,
	)
}

// canDemolish reports whether a building may go. A protector stays
// while it is the only bubble over another building or a site, so the
// fog's law — nothing but a protector outside a bubble — can't be
// broken by taking one away.
func canDemolish(s *State, b Building) bool {
	if b.Kind != BuildingProtector {
		return true
	}
	for _, id := range sortedBuildingIDs(s) {
		other := s.Buildings[id]
		if other.ID == b.ID || other.Kind == BuildingProtector {
			continue
		}
		x, y := cellCenterUnits(other.Col, other.Row)
		if !shelteredWithout(s, x, y, b.ID) {
			return false
		}
	}
	for _, job := range s.Jobs {
		if job.Kind == BuildingProtector {
			continue
		}
		x, y := cellCenterUnits(job.Col, job.Row)
		if !shelteredWithout(s, x, y, b.ID) {
			return false
		}
	}
	return true
}

// workDemolish puts a tick of a builder's work into taking a building
// down: the work only goes in while the building may legally go, so a
// protector whose bubble alone shelters something waits where it stands
// until that is no longer so. Work spent, the building comes down and
// leaves its pile (takeDown).
func (s *State) workDemolish(id int64) {
	b, ok := s.Buildings[id]
	if !ok || !canDemolish(s, b) {
		return
	}
	if b.Demolish > 0 {
		b.Demolish--
	}
	if b.Demolish > 0 {
		s.Buildings[id] = b
		return
	}
	s.takeDown(b, demolishRefund, BuildingDemolished)
}

// spillOverflow moves the lilac the stores no longer have a roof for
// onto a cell: lilac is one stock under many roofs, so what a warehouse
// held is the part that stops fitting when its roof goes. Oil has a
// place, and a demolished tank drops its own (Demolish).
func (s *State) spillOverflow(col, row int) {
	lilac := math.Max(0, s.Stock.Lilac-lilacCap(s))
	s.Stock.Lilac -= lilac
	s.dropPile(col, row, 0, lilac)
}

// freeRoom returns the room in all physical stores for a cargo, counting
// what robots already carry home, so nobody loads what won't fit.
func freeRoom(s *State, cargo ThingType) float64 {
	room := allOilCap(s) - allOilTotal(s)
	if cargo == TypeLilac {
		room = lilacCap(s) - s.Stock.Lilac
	}
	for _, id := range sortedRobotIDs(s) {
		if r := s.Robots[id]; r.Cargo == cargo {
			room -= r.Carry
		}
	}
	return math.Max(0, room)
}

// pileOffer returns what a robot would take from a pile in one trip:
// one kind, lilac first, as much as its arms and the stores' room
// allow. An amount of zero means the pile has nothing for now.
func pileOffer(s *State, p Pile, r Robot) (cargo ThingType, amount float64) {
	lilac := math.Min(
		math.Min(p.Lilac, robotCarryCapacity(r, TypeLilac)),
		freeRoom(s, TypeLilac),
	)
	if lilac >= pileDust {
		return TypeLilac, lilac
	}
	oil := math.Min(
		math.Min(p.Oil, robotCarryCapacity(r, TypeOil)),
		freeRoom(s, TypeOil),
	)
	if oil >= pileDust {
		return TypeOil, oil
	}
	return "", 0
}

// nearestPile returns the closest pile that holds something the stores
// have room for; IDs break ties.
func nearestPile(s *State, r Robot) (Pile, bool) {
	var best Pile
	found, bestDist := false, 0.0
	for _, id := range sortedPileIDs(s) {
		p := s.Piles[id]
		if _, amount := pileOffer(s, p, r); amount <= 0 {
			continue
		}
		x, y := cellCenterUnits(p.Col, p.Row)
		if d := math.Hypot(r.X-x, r.Y-y); !found || d < bestDist {
			best, found, bestDist = p, true, d
		}
	}
	return best, found
}

// takeFromPile fills the robot's arms from the pile it was loading at.
// A pile that left in the meantime, or has nothing the stores can take,
// leaves the robot empty-handed. The last item takes the pile with it.
func (s *State) takeFromPile(r *Robot) {
	p, ok := s.Piles[r.Pile]
	r.Pile = 0
	if !ok {
		return
	}
	cargo, amount := pileOffer(s, p, *r)
	if amount <= 0 {
		return
	}
	if cargo == TypeLilac {
		p.Lilac -= amount
	} else {
		p.Oil -= amount
	}
	r.Carry, r.Cargo = amount, cargo
	if p.Oil < pileDust && p.Lilac < pileDust {
		delete(s.Piles, p.ID)
		return
	}
	s.Piles[p.ID] = p
}

// storeSpot returns where a robot unloads: by the nearest store of its
// cargo's kind, the core one among them. Lilac is one stock, so the
// nearest warehouse or the core is only where the walking ends; oil goes
// into the tank it is carried to, the nearest with room for it
// (haulTank). Robots spread around a store by ID, as builders do.
func storeSpot(s *State, r Robot) (x, y float64) {
	spot, _ := tankSpot(s, coreTank)
	if r.Cargo == TypeOil {
		spot, _ = tankSpot(s, haulTank(s, r))
	} else {
		best := pointGap(PipePoint{r.X, r.Y}, spot)
		for _, id := range sortedBuildingIDs(s) {
			b := s.Buildings[id]
			if b.Kind != BuildingWarehouse {
				continue
			}
			cx, cy := cellCenterUnits(b.Col, b.Row)
			if d := math.Hypot(r.X-cx, r.Y-cy); d < best {
				best, spot = d, PipePoint{cx, cy}
			}
		}
	}
	angle := float64(r.ID) * goldenAngle
	return spot.X + math.Cos(angle)*storeStandoff,
		spot.Y + math.Sin(angle)*storeStandoff
}
