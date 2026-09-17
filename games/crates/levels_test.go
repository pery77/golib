package main

import (
	"slices"
	"strings"
	"testing"
)

// parseLayout builds a level from text, one string per row, in the usual
// Sokoban notation: # wall, space floor, . goal, $ crate, * crate on a goal,
// @ player, + player on a goal, and - outside the room. Tests use it for small
// rooms of their own.
func parseLayout(t *testing.T, rows ...string) *layout {
	t.Helper()
	l := &layout{name: "test", title: "Test", rows: len(rows)}
	for _, row := range rows {
		l.columns = max(l.columns, len(row))
	}
	l.ground = make([]ground, l.columns*l.rows)
	for r, row := range rows {
		for c, char := range row {
			here := cell{c, r}
			kind := floor
			switch char {
			case '#':
				kind = wall
			case '-':
				kind = outside
			case '.', '*', '+':
				kind = goal
			}
			l.ground[r*l.columns+c] = kind
			switch char {
			case '$', '*':
				l.crates = append(l.crates, here)
			case '@', '+':
				l.start = here
			}
		}
	}
	return l
}

// maxSolverCrates is the most crates solve handles.
const maxSolverCrates = 7

// position is a state of a level for solve: the player's cell, then the
// crates' cells in increasing order, each as row*columns+column.
type position [1 + maxSolverCrates]uint8

// solve finds the fewest moves that finish level l. It returns them, or nil
// if the level can't be finished, and how many positions it looked at. It
// tries every position breadth first, leaving out crates pushed where they
// could never reach a goal.
func solve(t *testing.T, l *layout) (solution []direction, searched int) {
	t.Helper()
	if l.columns*l.rows > 255 || len(l.crates) > maxSolverCrates {
		t.Fatalf("%s is too big for the solver: %d cells, %d crates", l.name, l.columns*l.rows, len(l.crates))
	}
	index := func(c cell) uint8 { return uint8(c.row*l.columns + c.column) }
	cellOf := func(i uint8) cell { return cell{int(i) % l.columns, int(i) / l.columns} }
	directions := []direction{up, down, left, right}

	// live marks the cells from which a crate can still be pushed onto some
	// goal: from each goal, walk back the way crates come, where a player has
	// room to push.
	live := make([]bool, l.columns*l.rows)
	var queue []cell
	for i, g := range l.ground {
		if g == goal {
			live[i] = true
			queue = append(queue, cellOf(uint8(i)))
		}
	}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		for _, d := range directions {
			from := c.next(d.opposite())
			pusher := from.next(d.opposite())
			if l.at(from).walkable() && l.at(pusher).walkable() && !live[index(from)] {
				live[index(from)] = true
				queue = append(queue, from)
			}
		}
	}

	var start position
	start[0] = index(l.start)
	for i, c := range l.crates {
		start[1+i] = index(c)
	}
	crates := len(l.crates)
	slices.Sort(start[1 : 1+crates])

	type parent struct {
		from position
		dir  direction
	}
	parents := map[position]parent{start: {}}
	frontier := []position{start}
	for len(frontier) > 0 {
		var next []position
		for _, p := range frontier {
			done := true
			for _, c := range p[1 : 1+crates] {
				if l.ground[c] != goal {
					done = false
					break
				}
			}
			if done {
				for p != start {
					solution = append(solution, parents[p].dir)
					p = parents[p].from
				}
				slices.Reverse(solution)
				return solution, len(parents)
			}
			player := cellOf(p[0])
			for _, d := range directions {
				target := player.next(d)
				if !l.at(target).walkable() {
					continue
				}
				q := p
				q[0] = index(target)
				if i := slices.Index(p[1:1+crates], index(target)); i >= 0 {
					beyond := target.next(d)
					if !l.at(beyond).walkable() || !live[index(beyond)] || slices.Contains(p[1:1+crates], index(beyond)) {
						continue
					}
					q[1+i] = index(beyond)
					slices.Sort(q[1 : 1+crates])
				}
				if _, seen := parents[q]; !seen {
					parents[q] = parent{from: p, dir: d}
					next = append(next, q)
				}
			}
		}
		frontier = next
	}
	return nil, len(parents)
}

// directionNames spells directions for test messages and golib shot scripts.
var directionNames = map[direction]string{up: "Up", down: "Down", left: "Left", right: "Right"}

func spell(solution []direction) string {
	names := make([]string, len(solution))
	for i, d := range solution {
		names[i] = directionNames[d]
	}
	return strings.Join(names, " ")
}

func TestSolveFindsTheFewestMoves(t *testing.T) {
	l := parseLayout(t,
		"#######",
		"#@ $ .#",
		"#######",
	)
	solution, _ := solve(t, l)
	if got := spell(solution); got != "Right Right Right" {
		t.Errorf("solution %q, want Right Right Right", got)
	}

	stuck := parseLayout(t,
		"#####",
		"#@ $#",
		"#  .#",
		"#####",
	)
	if solution, _ := solve(t, stuck); solution != nil {
		t.Errorf("a crate in a corner was solved with %q", spell(solution))
	}
}

func TestLevelsCanBeFinished(t *testing.T) {
	levels, err := loadLevels()
	if err != nil {
		t.Fatal(err)
	}
	for i, l := range levels {
		solution, searched := solve(t, l)
		if solution == nil {
			t.Errorf("level %d, %s, can't be finished (%d positions tried)", i+1, l.name, searched)
			continue
		}
		t.Logf("level %d, %q: %d crates, %d moves, %d positions: %s", i+1, l.title, len(l.crates), len(solution), searched, spell(solution))
		if l.par != len(solution) {
			t.Errorf("level %d, %s, has par %d, but it can be finished in %d moves: set the map's par property to %d", i+1, l.name, l.par, len(solution), len(solution))
		}

		// Play the solution through the rules, to check that they agree.
		w := newWorld(l)
		for _, d := range solution {
			w.step(d)
		}
		if !w.solved() || w.moves != len(solution) {
			t.Errorf("level %d: playing the solution leaves it solved: %v, after %d moves", i+1, w.solved(), w.moves)
		}
	}
}

func TestLevelsGetHarder(t *testing.T) {
	levels, err := loadLevels()
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(levels); i++ {
		before, now := levels[i-1], levels[i]
		if len(now.crates) < len(before.crates) {
			t.Errorf("level %d has %d crates, fewer than level %d's %d", i+1, len(now.crates), i, len(before.crates))
		}
		if now.par <= before.par {
			t.Errorf("level %d has par %d, no more than level %d's %d", i+1, now.par, i, before.par)
		}
	}
	if first := levels[0]; len(first.crates) != 1 {
		t.Errorf("level 1 has %d crates: start with one", len(first.crates))
	}
}
