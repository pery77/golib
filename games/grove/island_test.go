package main

import (
	"testing"

	"golib"
)

func TestTheIslandLoads(t *testing.T) {
	l, err := loadIsland()
	if err != nil {
		t.Fatal(err)
	}
	if len(l.coins) < 15 || len(l.rocks) < 6 || len(l.trees) < 20 {
		t.Errorf("the island has %d coins, %d rocks and %d trees: want a forest with plenty to find", len(l.coins), len(l.rocks), len(l.trees))
	}
	if _, err := parseIsland("..\n.x\n", 2, 1); err == nil {
		t.Error("a layout with an unknown character loaded")
	}
	if _, err := parseIsland("1s\n1\n", 2, 1); err == nil {
		t.Error("a layout with lines of different lengths loaded")
	}
}

// reachable is every cell the astronaut can get to from the start, one cell
// to the next, stepping down any height and climbing at most one block.
func reachable(l *island) map[[2]int]bool {
	row, col, _ := l.cellAt(l.start.X, l.start.Z)
	seen := map[[2]int]bool{{row, col}: true}
	queue := [][2]int{{row, col}}
	for len(queue) > 0 {
		at := queue[0]
		queue = queue[1:]
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			next := [2]int{at[0] + d[0], at[1] + d[1]}
			if seen[next] || l.top(next[0], next[1]) == 0 || l.top(next[0], next[1]) > l.top(at[0], at[1])+l.blockHeight {
				continue
			}
			seen[next] = true
			queue = append(queue, next)
		}
	}
	return seen
}

func TestEverythingCanBeReached(t *testing.T) {
	l, err := loadIsland()
	if err != nil {
		t.Fatal(err)
	}
	// A jump must clear one block.
	if rise := float32(jumpSpeed * jumpSpeed / (2 * gravity)); rise < l.blockHeight+0.15 {
		t.Fatalf("a jump rises %.2f m, not enough for a block %.2f m high", rise, l.blockHeight)
	}
	can := reachable(l)
	check := func(what string, at golib.Vector3) {
		row, col, ok := l.cellAt(at.X, at.Z)
		if !ok || !can[[2]int{row, col}] {
			t.Errorf("the %s at row %d, column %d can't be reached from the start", what, row, col)
		}
	}
	check("flag", l.flag)
	for _, c := range l.coins {
		check("coin", c)
	}
	for _, r := range l.rocks {
		check("rock", r)
	}
}

func TestTheLayoutReadsAsTheStartSeesIt(t *testing.T) {
	// From behind the astronaut at the start, facing up the island, a cell
	// to the right in the layout is on the right of the screen, and a line
	// higher up is further away.
	l, err := loadIsland()
	if err != nil {
		t.Fatal(err)
	}
	w := newWorld(l)
	camera := golib.NewCamera3D(screenWidth, screenHeight)
	follow(camera, &w.player, l, dt)
	camera.Snap()
	row, col, _ := l.cellAt(l.start.X, l.start.Z)
	here, _ := camera.ToScreen(l.center(row-2, col))
	right, _ := camera.ToScreen(l.center(row-2, col+2))
	if right.X <= here.X {
		t.Errorf("two columns to the right in the layout is at x %.0f on the screen, left of %.0f", right.X, here.X)
	}
	if w.player.yaw != 0 || l.center(row-2, col).Z <= l.start.Z {
		t.Error("the start doesn't face up the layout")
	}
}
