package main

import "math"

// The fog's law, after DESIGN.md's "The fog": it breathes. It never
// creeps toward the core, and the colony never maintains a wall of
// repulsors - conflict is temporal, not positional. Whole cycles of
// calm pass; then a swell rises for a while: the line presses in, the
// pushed band drags harder, and tanked robots outside a bubble burn
// their tanks faster. Each swell leaves the next one sooner, longer
// and deeper, but the bubbles never give an inch, and no ground is
// lost for good.
//
// The weather is state, not scenery: the fog's phase lives in State,
// so a save reproduces its mists and an action log replays them.
// Nothing here reads the clock or draws.

// Tuning: the fog's breath, with units in the name. Swells rise at a
// cycle's end, whole, so the HUD's forecast is exact: while it says
// "next cycle", the swell rises at that very boundary.
const (
	fogCycleTicks = 1800 // ticks a cycle lasts: 30 s

	fogHazeExposure       = 0.5 // the clear haze's share of the fog's full wear
	fogStillGraceTicks    = 120 // ticks a unit may stand before mites hurt it
	fogStillBurnPerSecond = 1.0 // L/s at full exposure, while standing still

	fogSwellPeriod    = 18.0 // cycles of calm before the first swell
	fogSwellQuickener = 0.90 // each swell shortens the next calm by this
	fogSwellMinPeriod = 4.0  // cycles of calm at the least

	fogSwellTicks       = 900  // ticks the first swell lasts: 15 s
	fogSwellTicksGrowth = 180  // ticks longer each swell: 3 s
	fogSwellTicksMax    = 3600 // ticks a swell lasts at the most: a minute

	fogSwellReach  = 2.0  // tiles the line presses in, the first swell
	fogSwellGrowth = 0.35 // tiles more of reach each swell
	fogSwellMargin = 0.75 // tiles of ground the line never takes off the bubble

	fogSwellRampTicks = 300 // ticks the line takes to press in, and to let go: 5 s

	fogSwellSpeedFactor = 0.25 // speed left in the pushed band; fog that was already there keeps fogSpeedFactor
	fogSwellBurn        = 1.5  // tanked units' burn outside a bubble
)

// stepFog moves the weather one tick forward: the swell drains tick by
// tick, the cycle's clock runs always, and the swell that is due rises
// at the boundary, whole.
func stepFog(s *State) {
	stepPressure(s)
	if s.Fog.SwellLeft > 0 && !s.Fog.Held {
		s.Fog.SwellLeft--
	}
	s.Fog.CycleLeft--
	if s.Fog.CycleLeft > 0 {
		return
	}
	s.Fog.Cycle++
	s.Fog.CycleLeft = fogCycleTicks
	if s.Fog.SwellLeft > 0 {
		return // a swell spans the boundary; its calm starts after it
	}
	if s.Fog.NextIn > 1 {
		s.Fog.NextIn--
		return
	}
	left := swellTicks(s)
	s.Fog.Swells++
	s.Fog.SwellLeft = left
	s.Fog.NextIn = swellPeriod(s)
}

// stepPressure moves the swell's pressure a tick toward where the
// weather wants it: all the way in while a swell is up, all the way out
// in the calm. The swell starts and ends on its tick, whole; what it
// does to the ground comes and goes with the pressure.
func stepPressure(s *State) {
	step := 1.0 / fogSwellRampTicks
	if s.Fog.SwellLeft > 0 {
		s.Fog.Pressure = math.Min(1, s.Fog.Pressure+step)
		return
	}
	s.Fog.Pressure = math.Max(0, s.Fog.Pressure-step)
}

// swellTicks returns how long a swell lasts: the longer, the more have
// passed.
func swellTicks(s *State) int64 {
	ticks := fogSwellTicks + fogSwellTicksGrowth*s.Fog.Swells
	if ticks > fogSwellTicksMax {
		ticks = fogSwellTicksMax
	}
	return ticks
}

// swellPeriod returns how many cycles of calm follow a swell: the
// shorter, the more have passed.
func swellPeriod(s *State) float64 {
	period := fogSwellPeriod
	for i := int64(0); i < s.Fog.Swells && period > fogSwellMinPeriod; i++ {
		period *= fogSwellQuickener
	}
	if period < fogSwellMinPeriod {
		period = fogSwellMinPeriod
	}
	return period
}

// swellReach returns how many tiles the line presses in: deeper each
// swell, but never up to the bubble - the core's ground is not
// negotiable.
func swellReach(s *State) float32 {
	reach := fogSwellReach + fogSwellGrowth*float32(s.Fog.Swells)
	if most := float32(fogLineRadius - coreBubbleRadius - fogSwellMargin); reach > most {
		reach = most
	}
	return reach
}

// fogLineNow returns where the fog's front stands, in tiles from the
// core: the calm line, pressed in as far as the swell's pressure says.
func fogLineNow(s *State) float32 {
	return fogLineRadius - swellReach(s)*float32(s.Fog.Pressure)
}

// fogExposureAt returns the colony's fog wear at a point: none inside a
// colony bubble, half strength in the haze and full strength in deep fog.
func fogExposureAt(s *State, x, y float64) float64 {
	if inSafeZone(s, x, y) {
		return 0
	}
	return math.Max(fogHazeExposure, fogAt(s, x, y))
}

// oilPoolInFog reports whether an oil pool's heart is covered, including
// the clear haze outside the core and colony protectors.
func oilPoolInFog(s *State, col, row int) bool {
	d, ok := depositAt(col, row)
	if !ok || d.Kind != kindOil {
		return false
	}
	x, y := cellCenterUnits(d.HeartCol, d.HeartRow)
	return fogExposureAt(s, x, y) > 0
}

func oilPoolAccess(s *State, col, row int) string {
	if oilPoolInFog(s, col, row) {
		return "covered by fog"
	}
	return "clear"
}

// fogDistanceAt returns how far the tile under a world point sits from
// the core, in tiles.
func fogDistanceAt(x, y float64) float32 {
	col := int(math.Floor(x / unitsPerTile))
	row := int(math.Floor(y / unitsPerTile))
	return tileDistance(col, row)
}

// fogDrag returns the share of its speed a robot keeps where it stands:
// the calm fog slows every walker the same, and a swell's pushed band -
// ground the calm fog would leave clear - is meaner still. The bubbles
// return the step whole.
func fogDrag(s *State, x, y float64) float64 {
	fog := fogAt(s, x, y)
	if fog <= 0 {
		return 1
	}
	factor := fogSpeedFactor
	if s.Fog.Pressure > 0 && fogCover(fogDistanceAt(x, y), fogLineRadius) <= 0 {
		factor = fogSwellSpeedFactor
	}
	return 1 - (1-factor)*fog
}
