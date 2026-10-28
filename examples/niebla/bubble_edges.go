package main

import (
	"math"
	"sort"

	"golib"
)

const bubbleOutlineSteps = 192

type discArc struct {
	start float64
	end   float64
}

// Each other circle covers an angular interval on this boundary. Merging
// those intervals leaves only the arcs on the outside of the union.
func visibleDiscArcs(discs []disc, index int) []discArc {
	if index < 0 || index >= len(discs) {
		return nil
	}
	circle := discs[index]
	if circle.r <= 0 {
		return nil
	}

	const (
		fullTurn = 2 * math.Pi
		epsilon  = 1e-9
	)
	covered := make([]discArc, 0, len(discs)*2)
	for otherIndex, other := range discs {
		if otherIndex == index || other.r <= 0 {
			continue
		}
		dx, dy := other.x-circle.x, other.y-circle.y
		distance := math.Hypot(dx, dy)
		if distance <= epsilon {
			if other.r > circle.r+epsilon ||
				(math.Abs(other.r-circle.r) <= epsilon && otherIndex < index) {
				return nil
			}
			continue
		}
		if distance+circle.r <= other.r+epsilon {
			return nil
		}
		if distance+other.r <= circle.r+epsilon ||
			distance >= circle.r+other.r-epsilon {
			continue
		}

		cosine := (distance*distance + circle.r*circle.r -
			other.r*other.r) / (2 * distance * circle.r)
		cosine = math.Max(-1, math.Min(1, cosine))
		half := math.Acos(cosine)
		center := math.Atan2(dy, dx)
		start := normalizeBubbleAngle(center - half)
		end := normalizeBubbleAngle(center + half)
		if start <= end {
			covered = append(covered, discArc{start, end})
			continue
		}
		covered = append(covered,
			discArc{0, end}, discArc{start, fullTurn})
	}

	sort.Slice(covered, func(i, j int) bool {
		return covered[i].start < covered[j].start
	})
	merged := make([]discArc, 0, len(covered))
	for _, arc := range covered {
		last := len(merged) - 1
		if last < 0 || arc.start > merged[last].end+epsilon {
			merged = append(merged, arc)
			continue
		}
		if arc.end > merged[last].end {
			merged[last].end = arc.end
		}
	}

	var visible []discArc
		at := 0.0
	for _, arc := range merged {
		if arc.start-at > epsilon {
			visible = append(visible, discArc{at, arc.start})
		}
		if arc.end > at {
			at = arc.end
		}
	}
	if fullTurn-at > epsilon {
		visible = append(visible, discArc{at, fullTurn})
	}
	return visible
}

func normalizeBubbleAngle(angle float64) float64 {
	angle = math.Mod(angle, 2*math.Pi)
	if angle < 0 {
		angle += 2 * math.Pi
	}
	return angle
}

func drawDiscOutline(
	screen *golib.Screen,
	discs []disc,
	index int,
	thickness float32,
	color golib.Color,
) {
	if index < 0 || index >= len(discs) {
		return
	}
	d := discs[index]
	if d.r <= 0 {
		return
	}
	const fullTurn = 2 * math.Pi
	for _, arc := range visibleDiscArcs(discs, index) {
		steps := int(math.Ceil(
			(arc.end - arc.start) / fullTurn * bubbleOutlineSteps))
		if steps < 1 {
			steps = 1
		}
		from := discOutlinePoint(d, arc.start)
		for i := 1; i <= steps; i++ {
			angle := arc.start + (arc.end-arc.start)*
				float64(i)/float64(steps)
			to := discOutlinePoint(d, angle)
			screen.DrawLine(from.X, from.Y, to.X, to.Y, thickness, color)
			from = to
		}
	}
}

func discOutlinePoint(d disc, angle float64) golib.Vector2 {
	x := float32(d.x + math.Cos(angle)*d.r)
	y := float32(d.y + math.Sin(angle)*d.r)
	screenX, screenY := projectFlat(x, y)
	return golib.Vector2{X: screenX, Y: screenY}
}
