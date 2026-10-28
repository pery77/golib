package main

import (
	"math"
	"sort"

	"golib"
)

// The fog on the screen. Inside a repulsor's circle the air is clear and
// the ground shows as it is; everywhere else a haze hangs, and past the
// fog's line it thickens to the fog the region is surrounded by. It is
// drawn in layers, each the whole region minus the clear circles, so the
// circles' edges are round at every zoom.

const (
	mistHaze   = 0.3 // the haze's opacity outside every circle, before the line
	mistLayers = 16  // steps the fog takes from the haze to whole, past the line
	mistStepPx = 3.0 // screen pixels of a strip of mist, across
)

// disc is a circle on the flat ground, in units.
type disc struct{ x, y, r float64 }

// colonyClearDiscs lists the core's and protectors' bubbles. A circle
// follows what carries it up the relief, as its outline does.
func colonyClearDiscs(s *State) []disc {
	cx, cy := tileCenterUnits(coreCol, coreRow)
	discs := []disc{{cx, cy, coreBubbleRadius * unitsPerTile}}
	for _, id := range sortedBuildingIDs(s) {
		if b := s.Buildings[id]; b.Kind == BuildingProtector {
			x, y := cellCenterUnits(b.Col, b.Row)
			if radius := protectorRadiusTiles(b); radius > 0 {
				discs = append(discs,
					liftedDisc(x, y, radius*unitsPerTile))
			}
		}
	}
	return discs
}

func liftedDisc(x, y, radius float64) disc {
	height := float64(land.heightAt(float32(x), float32(y)))
	return disc{x - height, y - height, radius}
}

// clearDiscs lists every circle the fog stays out of: the colony's and
// every rival repulsor's.
func clearDiscs(s *State) []disc {
	discs := colonyClearDiscs(s)
	for _, id := range sortedEnemyIDs(s) {
		e := s.Enemies[id]
		if reach := enemySpecOf(e.Kind).bubble; reach > 0 {
			discs = append(discs,
				liftedDisc(e.X, e.Y, reach))
		}
	}
	return discs
}

// drawFogCover lays the fog over everything but the clear circles: the
// haze first, then the layers that thicken it past the line - the line
// of now, so a swell's pushed band is mist for as long as it lasts. It
// runs after the robots, so whatever walks into the fog is swallowed.
func drawFogCover(s *State, screen *golib.Screen, zoom float32, view golib.Rectangle) {
	holes := clearDiscs(s)
	drawMist(screen, zoom, view, holes, mistHaze)
	cx, cy := tileCenterUnits(coreCol, coreRow)
	start := float64(fogLineNow(s)) - 0.7
	for k := 1; k <= mistLayers; k++ {
		// Each layer brings the fog from (k-1)/n to k/n of the way to whole.
		before := float64(k-1) / mistLayers
		opacity := (1.0 / mistLayers) / (1 - before)
		reach := (start + fogFadeTiles*float64(k)/mistLayers) * unitsPerTile
		drawMist(screen, zoom, view,
			append(holes[:len(holes):len(holes)], disc{cx, cy, reach}),
			float32(opacity))
	}
}

// drawMist paints one layer of fog over the region, all of it but the
// discs. The region is cut in strips along the world's y axis; on each
// edge of a strip the discs leave gaps, and a gap on one edge joins its
// twin on the other as a quad, so the discs' rims come out as polygons
// with a corner per strip. Where the two edges disagree on how many gaps
// there are - a disc begins or ends inside the strip - the strip's middle
// decides for both. Strips no disc touches are most of the deep fog, and
// a run of them goes out as one quad.
func drawMist(
	screen *golib.Screen,
	zoom float32,
	view golib.Rectangle,
	discs []disc,
	opacity float32,
) {
	color := golib.WithOpacity(fogColor, opacity)
	lowX, lowY, highX, highY := viewUnits(view)
	step := float64(mistStepPx / (zoom * unitH))
	first := int(math.Floor(lowY / step))
	last := int(math.Ceil(highY / step))
	quad := func(y0, y1 float64, top, bottom [2]float64) {
		ax, ay := projectFlat(float32(top[0]), float32(y0))
		bx, by := projectFlat(float32(top[1]), float32(y0))
		cx, cy := projectFlat(float32(bottom[1]), float32(y1))
		dx, dy := projectFlat(float32(bottom[0]), float32(y1))
		screen.DrawPolygon([]golib.Vector2{
			{X: ax, Y: ay}, {X: bx, Y: by}, {X: cx, Y: cy}, {X: dx, Y: dy},
		}, color)
	}
	whole := [2]float64{lowX, highX}
	runFrom, running := 0.0, false
	for i := first; i < last; i++ {
		y0 := math.Max(lowY, float64(i)*step)
		y1 := math.Min(highY, float64(i+1)*step)
		top := mistGaps(discs, y0, lowX, highX)
		bottom := mistGaps(discs, y1, lowX, highX)
		if len(top) != len(bottom) {
			top = mistGaps(discs, (y0+y1)/2, lowX, highX)
			bottom = top
		}
		if len(top) == 1 && top[0] == whole && bottom[0] == whole {
			if !running {
				runFrom, running = y0, true
			}
			continue
		}
		if running {
			quad(runFrom, y0, whole, whole)
			running = false
		}
		for j := range top {
			quad(y0, y1, top[j], bottom[j])
		}
	}
	if running {
		quad(runFrom, highY, whole, whole)
	}
}

// mistGaps returns the stretches of the line at y, between lowX and
// highX, that no disc covers.
func mistGaps(discs []disc, y, lowX, highX float64) [][2]float64 {
	var covered [][2]float64
	for _, d := range discs {
		if off := math.Abs(y - d.y); off < d.r {
			half := math.Sqrt(d.r*d.r - off*off)
			covered = append(covered, [2]float64{d.x - half, d.x + half})
		}
	}
	sort.Slice(covered, func(i, j int) bool { return covered[i][0] < covered[j][0] })
	var gaps [][2]float64
	at := lowX
	for _, c := range covered {
		if c[0] > at {
			gaps = append(gaps, [2]float64{at, math.Min(c[0], highX)})
		}
		at = math.Max(at, c[1])
		if at >= highX {
			return gaps
		}
	}
	return append(gaps, [2]float64{at, highX})
}

// unprojectFlat undoes projectFlat: the flat ground, in units, under a
// projected point.
func unprojectFlat(px, py float32) (x, y float64) {
	a := float64((px - regionOriginX) / (unitW / 2))
	b := float64((py - regionOriginY) / (unitH / 2))
	return (a + b) / 2, (b - a) / 2
}

// inDiscs reports whether a projected point lies inside one of the discs.
func inDiscs(discs []disc, px, py float32) bool {
	x, y := unprojectFlat(px, py)
	for _, d := range discs {
		if math.Hypot(x-d.x, y-d.y) < d.r {
			return true
		}
	}
	return false
}

// viewUnits returns the box of flat ground, in units, that holds what
// the view shows of the region.
func viewUnits(view golib.Rectangle) (lowX, lowY, highX, highY float64) {
	lowX, lowY = math.Inf(1), math.Inf(1)
	highX, highY = math.Inf(-1), math.Inf(-1)
	for _, corner := range [][2]float32{
		{view.X, view.Y}, {view.X + view.Width, view.Y},
		{view.X, view.Y + view.Height}, {view.X + view.Width, view.Y + view.Height},
	} {
		x, y := unprojectFlat(corner[0], corner[1])
		lowX, highX = math.Min(lowX, x), math.Max(highX, x)
		lowY, highY = math.Min(lowY, y), math.Max(highY, y)
	}
	most := float64(regionCols * unitsPerTile)
	return math.Max(0, lowX), math.Max(0, lowY),
		math.Min(most, highX), math.Min(most, highY)
}
