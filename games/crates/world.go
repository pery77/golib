package main

// The rules of Sokoban, as plain Go with no input or drawing, so that
// world_test.go can play them directly. levels.go reads layouts from the
// level maps.

// cell is a square of a level's grid. Column 0 is the leftmost, row 0 the top.
type cell struct {
	column, row int
}

// next returns the neighboring cell in direction d.
func (c cell) next(d direction) cell {
	dx, dy := d.offset()
	return cell{c.column + dx, c.row + dy}
}

// direction is a way the player can walk.
type direction int

const (
	noDirection direction = iota
	up
	down
	left
	right
)

// offset returns how a step in direction d changes the column and the row.
func (d direction) offset() (dx, dy int) {
	switch d {
	case up:
		return 0, -1
	case down:
		return 0, 1
	case left:
		return -1, 0
	case right:
		return 1, 0
	}
	return 0, 0
}

// opposite returns the direction that undoes a step in direction d.
func (d direction) opposite() direction {
	switch d {
	case up:
		return down
	case down:
		return up
	case left:
		return right
	case right:
		return left
	}
	return noDirection
}

// ground is what a cell of a level is made of.
type ground uint8

const (
	outside ground = iota // beyond the walls, where nothing ever goes
	floor
	goal // floor where a crate should end
	wall
)

// walkable reports whether the player and crates can stand on g.
func (g ground) walkable() bool {
	return g == floor || g == goal
}

// layout is a level as its map describes it. It doesn't change while the
// level is played.
type layout struct {
	name          string // the map file, such as "maps/level01.tmx"; progress is saved under it
	title         string // the level's name, shown above it
	par           int    // the fewest moves that finish the level
	columns, rows int
	ground        []ground // row by row
	start         cell     // where the player starts
	crates        []cell   // where the crates start
}

// at returns what the cell c is made of; outside the grid, it is outside.
func (l *layout) at(c cell) ground {
	if c.column < 0 || c.row < 0 || c.column >= l.columns || c.row >= l.rows {
		return outside
	}
	return l.ground[c.row*l.columns+c.column]
}

// move is one step the player took, kept so it can be undone.
type move struct {
	dir    direction
	pushed int       // the crate the step pushed, or -1
	facing direction // where the player faced before the step
}

// moveResult says what a step did, so the play scene can animate it and play
// a sound.
type moveResult struct {
	moved  bool // the player walked one cell
	pushed int  // the crate the player pushed, or -1
	landed bool // the pushed crate ended on a goal
}

// world is a level being played: where the player and the crates are, and
// the steps that brought them there. It knows nothing about the keyboard or
// the screen.
type world struct {
	layout  *layout
	player  cell
	facing  direction // where the player looks, for drawing
	crates  []cell
	moves   int    // steps taken, pushes included, minus the steps undone
	pushes  int    // steps that pushed a crate, minus those undone
	history []move // every step not undone, oldest first
}

// newWorld returns the level l at its start.
func newWorld(l *layout) world {
	return world{
		layout: l,
		player: l.start,
		facing: down,
		crates: append([]cell(nil), l.crates...),
	}
}

// crateAt returns the index of the crate on cell c, or -1.
func (w *world) crateAt(c cell) int {
	for i, crate := range w.crates {
		if crate == c {
			return i
		}
	}
	return -1
}

// free reports whether a crate can be pushed onto c: floor or a goal, with no
// crate on it.
func (w *world) free(c cell) bool {
	return w.layout.at(c).walkable() && w.crateAt(c) < 0
}

// step walks the player one cell in direction d, pushing the crate in the
// way if the cell behind it is free. A player facing a wall, or a crate that
// can't move, turns to face it and stays. A finished level takes no more
// steps.
func (w *world) step(d direction) moveResult {
	result := moveResult{pushed: -1}
	if d == noDirection || w.solved() {
		return result
	}
	facing := w.facing
	w.facing = d
	target := w.player.next(d)
	if !w.layout.at(target).walkable() {
		return result
	}
	pushed := w.crateAt(target)
	if pushed >= 0 {
		beyond := target.next(d)
		if !w.free(beyond) {
			return result
		}
		w.crates[pushed] = beyond
		w.pushes++
		result.pushed = pushed
		result.landed = w.layout.at(beyond) == goal
	}
	w.player = target
	w.moves++
	w.history = append(w.history, move{dir: d, pushed: pushed, facing: facing})
	result.moved = true
	return result
}

// undo takes back the last step, and the push it made, if any. It reports
// whether there was a step to take back.
func (w *world) undo() bool {
	if len(w.history) == 0 {
		return false
	}
	last := w.history[len(w.history)-1]
	w.history = w.history[:len(w.history)-1]
	back := last.dir.opposite()
	if last.pushed >= 0 {
		w.crates[last.pushed] = w.crates[last.pushed].next(back)
		w.pushes--
	}
	w.player = w.player.next(back)
	w.facing = last.facing
	w.moves--
	return true
}

// cratesOnGoals returns how many crates stand on a goal.
func (w *world) cratesOnGoals() int {
	count := 0
	for _, c := range w.crates {
		if w.layout.at(c) == goal {
			count++
		}
	}
	return count
}

// solved reports whether every crate stands on a goal, which finishes the
// level.
func (w *world) solved() bool {
	return w.cratesOnGoals() == len(w.crates)
}
