package main

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestVisibleDiscArcsKeepSeparatedOutlines(t *testing.T) {
	discs := []disc{{x: 0, y: 0, r: 4}, {x: 10, y: 0, r: 2}}
	for i := range discs {
		arcs := visibleDiscArcs(discs, i)
		if len(arcs) != 1 || arcs[0].start != 0 ||
			math.Abs(arcs[0].end-2*math.Pi) > 1e-9 {
			t.Errorf("disc %d has visible arcs %v, want one full circle", i, arcs)
		}
	}
}

func TestVisibleDiscArcsJoinOverlappingBubbles(t *testing.T) {
	discs := []disc{
		{x: 0, y: 0, r: 4},
		{x: 4.5, y: 0, r: 2},
		{x: 5.5, y: 2.5, r: 2},
	}
	for i, d := range discs {
		arcs := visibleDiscArcs(discs, i)
		if len(arcs) == 0 {
			t.Fatalf("disc %d has no outside edge", i)
		}
		for _, arc := range arcs {
			angle := (arc.start + arc.end) / 2
			x := d.x + math.Cos(angle)*d.r
			y := d.y + math.Sin(angle)*d.r
			for otherIndex, other := range discs {
				if otherIndex == i {
					continue
				}
				if math.Hypot(x-other.x, y-other.y) < other.r-1e-8 {
					t.Errorf("disc %d kept an edge inside disc %d", i, otherIndex)
				}
			}
		}
	}
	for i := range discs {
		length := visibleArcLength(visibleDiscArcs(discs, i))
		if length >= 2*math.Pi-1e-6 {
			t.Errorf("disc %d kept its full outline through an overlap", i)
		}
	}
}

func TestVisibleDiscArcsHideContainedAndDuplicateCircles(t *testing.T) {
	discs := []disc{
		{x: 0, y: 0, r: 4},
		{x: 1, y: 0, r: 1},
		{x: 0, y: 0, r: 4},
	}
	if arcs := visibleDiscArcs(discs, 1); len(arcs) != 0 {
		t.Errorf("contained disc has visible arcs %v, want none", arcs)
	}
	if arcs := visibleDiscArcs(discs, 2); len(arcs) != 0 {
		t.Errorf("duplicate disc has visible arcs %v, want none", arcs)
	}
	if arcs := visibleDiscArcs(discs, 0); len(arcs) != 1 ||
		visibleArcLength(arcs) < 2*math.Pi-1e-9 {
		t.Errorf("outer disc has visible arcs %v, want one full circle", arcs)
	}
}

func visibleArcLength(arcs []discArc) float64 {
	var length float64
	for _, arc := range arcs {
		length += arc.end - arc.start
	}
	return length
}

func TestWriteBubbleShotState(t *testing.T) {
	path := os.Getenv("NIEBLA_BUBBLE_SHOT_STATE")
	if path == "" {
		t.Skip("set NIEBLA_BUBBLE_SHOT_STATE to write a bubble shot state")
	}
	s := newGame()
	s.raise(BuildingProtector, 136, 99)
	s.raise(BuildingProtector, 144, 99)
	data, err := json.MarshalIndent(map[string]any{"state": s}, "", "  ")
	if err != nil {
		t.Fatalf("the state doesn't marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
