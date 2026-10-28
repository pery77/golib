package main

import "math"

// Oil has a place. Lilac is one stock under many roofs, but oil sits in
// tanks - the core's, each silo's, each charger's and each protector's -
// and gets from one to another in a robot's arms or down a pipe.
// Protector oil is reserved for its building; other oil pays colony
// costs, the core's first.
const (
	coreTank int64 = 0 // the core's tank, where a building's ID would go

	chargerOilCap = 200.0 // L a charger holds for the robots it refills
)

// tankCapOf returns what a kind of building holds in oil; 0 is no tank.
func tankCapOf(kind BuildingKind) float64 {
	switch kind {
	case BuildingSilo:
		return siloOilCap
	case BuildingCharger:
		return chargerOilCap
	case BuildingProtector:
		return protectorOilCap
	}
	return 0
}

func initialBuildingOil(kind BuildingKind) float64 {
	if kind == BuildingProtector {
		return protectorCostOil
	}
	return 0
}

// tankCap returns what a tank holds at most; 0 for what is no tank.
func tankCap(s *State, tank int64) float64 {
	if tank == coreTank {
		return coreOilCap
	}
	return tankCapOf(s.Buildings[tank].Kind)
}

// tankOil returns the oil in a tank.
func tankOil(s *State, tank int64) float64 {
	if tank == coreTank {
		return s.Stock.Oil
	}
	return s.Buildings[tank].Oil
}

// tankRoom returns the room a tank has left.
func tankRoom(s *State, tank int64) float64 {
	return math.Max(0, tankCap(s, tank)-tankOil(s, tank))
}

// addOil pours oil into a tank, or out of it when the amount is
// negative. The caller minds the tank's room and its bottom.
func (s *State) addOil(tank int64, amount float64) {
	if tank == coreTank {
		s.Stock.Oil += amount
		return
	}
	if b, ok := s.Buildings[tank]; ok {
		b.Oil += amount
		s.Buildings[tank] = b
	}
}

// oilTanks lists oil available for colony spending: the core's, then
// stores that aren't reserved for a building's own operation.
func oilTanks(s *State) []int64 {
	tanks := []int64{coreTank}
	for _, id := range sortedBuildingIDs(s) {
		kind := s.Buildings[id].Kind
		if tankCapOf(kind) > 0 && kind != BuildingProtector {
			tanks = append(tanks, id)
		}
	}
	return tanks
}

func allOilTanks(s *State) []int64 {
	tanks := []int64{coreTank}
	for _, id := range sortedBuildingIDs(s) {
		if tankCapOf(s.Buildings[id].Kind) > 0 {
			tanks = append(tanks, id)
		}
	}
	return tanks
}

// tankSpot returns the middle of a tank, in units.
func tankSpot(s *State, tank int64) (PipePoint, bool) {
	if tank == coreTank {
		x, y := tileCenterUnits(coreCol, coreRow)
		return PipePoint{x, y}, true
	}
	b, ok := s.Buildings[tank]
	if !ok || tankCapOf(b.Kind) <= 0 {
		return PipePoint{}, false
	}
	x, y := cellCenterUnits(b.Col, b.Row)
	return PipePoint{x, y}, true
}

// oilTotal returns oil available for colony spending.
func oilTotal(s *State) float64 {
	total := 0.0
	for _, tank := range oilTanks(s) {
		total += tankOil(s, tank)
	}
	return total
}

// oilCap returns how much oil the colony can hold for spending.
func oilCap(s *State) float64 {
	cap := 0.0
	for _, tank := range oilTanks(s) {
		cap += tankCap(s, tank)
	}
	return cap
}

func allOilTotal(s *State) float64 {
	total := 0.0
	for _, tank := range allOilTanks(s) {
		total += tankOil(s, tank)
	}
	return total
}

func allOilCap(s *State) float64 {
	cap := 0.0
	for _, tank := range allOilTanks(s) {
		cap += tankCap(s, tank)
	}
	return cap
}

// payOil takes oil the colony spends out of its tanks, the core's first.
// The caller has checked oilTotal.
func (s *State) payOil(amount float64) {
	for _, tank := range oilTanks(s) {
		take := math.Min(amount, tankOil(s, tank))
		s.addOil(tank, -take)
		amount -= take
	}
}

// nearestTank returns the closest to a spot of the tanks that serve,
// one that is ready before any that isn't; IDs break ties, the core
// first. The core always serves, so there is always an answer.
func nearestTank(s *State, x, y float64, serves, ready func(tank int64) bool) int64 {
	return nearestTankAmong(s, oilTanks(s), x, y, serves, ready)
}

func nearestTankAmong(
	s *State,
	tanks []int64,
	x, y float64,
	serves, ready func(tank int64) bool,
) int64 {
	best, bestGap, bestReady := coreTank, math.Inf(1), false
	for _, tank := range tanks {
		if tank != coreTank && !serves(tank) {
			continue
		}
		spot, _ := tankSpot(s, tank)
		gap := math.Hypot(spot.X-x, spot.Y-y)
		ok := ready(tank)
		if (ok && !bestReady) || (ok == bestReady && gap < bestGap) {
			best, bestGap, bestReady = tank, gap, ok
		}
	}
	return best
}

// haulTank returns the tank a robot carries its oil to: the nearest one
// with room for some of it, or the nearest one when all are full.
func haulTank(s *State, r Robot) int64 {
	return nearestOilTank(s, r.X, r.Y,
		func(int64) bool { return true },
		func(tank int64) bool { return tankRoom(s, tank) >= pileDust })
}

func nearestOilTank(
	s *State,
	x, y float64,
	serves, ready func(tank int64) bool,
) int64 {
	return nearestTankAmong(s, allOilTanks(s), x, y, serves, ready)
}

// refuelTank returns the tank a robot refills at: the nearest charger or
// the core, one with oil in it before a dry one.
func refuelTank(s *State, r Robot) int64 {
	return nearestTank(s, r.X, r.Y,
		func(tank int64) bool { return s.Buildings[tank].Kind == BuildingCharger },
		func(tank int64) bool { return tankOil(s, tank) > 0 })
}
