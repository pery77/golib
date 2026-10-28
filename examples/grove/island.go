package main

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"golib"
)

// islandFile is the island's layout, a text file anyone can change in a text
// editor: assets/levels/island.txt says what each character stands for.
const islandFile = "levels/island.txt"

// How the things on the island are sized, for the eye and for the rules.
const (
	treeScale   = 1.5  // the kit's tree is 1.9 m tall: a little taller than half again
	pineScale   = 1.6  // and its pine 2 m
	treeRadius  = 0.3  // meters: a trunk, which the astronaut can't walk through
	rockScale   = 1.4  // the kit's rocks are 0.65 m across
	rockRadius  = 0.45 // meters: what the astronaut bumps into
	flagScale   = 2.4  // the kit's flag is 0.9 m tall
	coinScale   = 1.3  // the kit's coin is 0.4 m across
	coinHover   = 0.45 // meters above the ground a coin's middle floats
	flowerCount = 2    // flowers scattered on a plain cell, besides the w cells, on average out of 10 cells
)

// tree is a tree on the island: round or a pine, at its foot.
type tree struct {
	position golib.Vector3
	pine     bool
	yaw      float32 // degrees: each tree turned its own way, so the forest doesn't repeat
	scale    float32
}

// island is the ground, cell by cell, and where everything on it starts.
// Cells are squares cell meters a side on the ground; row 0 is the far end
// (+Z), and column 0 is +X, so the layout file reads as a player at the
// start sees the island.
type island struct {
	heights     [][]int // blocks high, by row and column; 0 is the sea
	rows, cols  int
	cell        float32 // meters a side
	blockHeight float32 // meters: one block's height
	start       golib.Vector3
	flag        golib.Vector3
	coins       []golib.Vector3 // where each coin floats
	rocks       []golib.Vector3 // the foot of each rock
	trees       []tree
	flowers     []golib.Transform
}

// loadIsland reads the island from the layout file in the assets folder,
// sized by the grass block's model.
func loadIsland() (*island, error) {
	data, err := golib.ReadAsset(islandFile)
	if err != nil {
		return nil, err
	}
	if err := blockModel.Err(); err != nil {
		return nil, err
	}
	lowest, upmost := blockModel.Bounds()
	return parseIsland(string(data), upmost.X-lowest.X, upmost.Y-lowest.Y)
}

// parseIsland reads a layout, as island.txt has it, with cells cell meters a
// side and blocks blockHeight meters high.
func parseIsland(text string, cell, blockHeight float32) (*island, error) {
	var lines []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return nil, errors.New("island: the layout has no lines")
	}
	l := &island{rows: len(lines), cols: len(lines[0]), cell: cell, blockHeight: blockHeight}
	foundStart, foundFlag := false, false
	for row, line := range lines {
		if len(line) != l.cols {
			return nil, fmt.Errorf("island: line %d is %d characters long, the first is %d", row+1, len(line), l.cols)
		}
		heights := make([]int, l.cols)
		for col, c := range line {
			height := 1
			switch {
			case c == '.':
				height = 0
			case c >= '1' && c <= '9':
				height = int(c - '0')
			case c >= 'A' && c <= 'Z':
				height = 2
			case c < 'a' || c > 'z':
				return nil, fmt.Errorf("island: line %d has %q, which stands for nothing", row+1, c)
			}
			heights[col] = height
			ground := l.center(row, col)
			ground.Y = float32(height) * blockHeight
			switch c | 0x20 { // the letter in lowercase
			case 's':
				l.start, foundStart = ground, true
			case 'f':
				l.flag, foundFlag = ground, true
			case 'c':
				l.coins = append(l.coins, ground.Add(golib.Vector3{Y: coinHover}))
			case 'r':
				l.rocks = append(l.rocks, ground)
			case 't', 'p':
				h := hash(row, col)
				t := tree{position: ground, pine: c|0x20 == 'p', yaw: h * 360, scale: treeScale}
				if t.pine {
					t.scale = pineScale
				}
				t.scale *= 0.85 + 0.3*hash(col, row)
				// A little off the cell's middle, so the forest isn't a grid.
				t.position = t.position.Add(golib.Vector3{X: (h - 0.5) * 0.5, Z: (hash(row+7, col) - 0.5) * 0.5})
				l.trees = append(l.trees, t)
			case 'w':
				l.flowers = append(l.flowers, l.flowersAt(row, col, ground, 3)...)
			}
			if height > 0 && (c >= '1' && c <= '9') && hash(row*3, col*5) < flowerCount/10.0 {
				l.flowers = append(l.flowers, l.flowersAt(row, col, ground, 1)...)
			}
		}
		l.heights = append(l.heights, heights)
	}
	switch {
	case !foundStart:
		return nil, errors.New("island: the layout has no start (s)")
	case !foundFlag:
		return nil, errors.New("island: the layout has no flag (f or F)")
	}
	return l, nil
}

// flowersAt scatters count patches of flowers on a cell, away from its middle.
func (l *island) flowersAt(row, col int, ground golib.Vector3, count int) []golib.Transform {
	var out []golib.Transform
	for i := range count {
		h := hash(row*11+i, col*13+i)
		angle := float64(h) * 2 * math.Pi
		reach := l.cell * (0.18 + 0.2*hash(col+i, row))
		at := ground.Add(golib.Vector3{X: reach * float32(math.Cos(angle)), Z: reach * float32(math.Sin(angle))})
		size := 0.8 + 0.5*hash(row+i, col*3)
		out = append(out, golib.Transform{Position: at, Rotation: golib.Rotation{Yaw: h * 720}, Scale: golib.Vector3{X: size, Y: size, Z: size}})
	}
	return out
}

// hash is a number from 0 to 1 that looks random but is always the same for
// the same row and column, so the island looks the same every game.
func hash(a, b int) float32 {
	x := uint32(a)*73856093 ^ uint32(b)*19349663
	x ^= x >> 13
	x *= 0x5bd1e995
	x ^= x >> 15
	return float32(x%10000) / 10000
}

// center is the middle of a cell on the sea's floor, at Y 0.
func (l *island) center(row, col int) golib.Vector3 {
	return golib.Vector3{
		X: (float32(l.cols-1)/2 - float32(col)) * l.cell,
		Z: (float32(l.rows-1)/2 - float32(row)) * l.cell,
	}
}

// cellAt is the row and column of the cell under a point on the ground, and
// false outside the layout.
func (l *island) cellAt(x, z float32) (row, col int, ok bool) {
	col = int(math.Floor(float64(float32(l.cols)/2 - x/l.cell)))
	row = int(math.Floor(float64(float32(l.rows)/2 - z/l.cell)))
	return row, col, row >= 0 && row < l.rows && col >= 0 && col < l.cols
}

// top is how high the ground of a cell is, in meters: 0 for the sea and
// outside the layout.
func (l *island) top(row, col int) float32 {
	if row < 0 || row >= l.rows || col < 0 || col >= l.cols {
		return 0
	}
	return float32(l.heights[row][col]) * l.blockHeight
}

// highestUnder is the highest top of the cells that a square, half meters
// either way from x, z, lies over, no higher than below: the ground a body
// standing there rests on. It is 0 over the sea.
func (l *island) highestUnder(x, z, half, below float32) float32 {
	var best float32
	l.eachCell(x, z, half, func(row, col int) {
		if t := l.top(row, col); t <= below && t > best {
			best = t
		}
	})
	return best
}

// highestAround is the highest top of the cells a square, half meters either
// way from x, z, lies over: what a body there would bump into.
func (l *island) highestAround(x, z, half float32) float32 {
	var best float32
	l.eachCell(x, z, half, func(row, col int) {
		best = max(best, l.top(row, col))
	})
	return best
}

// eachCell calls do for every cell a square, half meters either way from
// x, z, lies over.
func (l *island) eachCell(x, z, half float32, do func(row, col int)) {
	r0, c0, _ := l.cellAt(x+half, z+half)
	r1, c1, _ := l.cellAt(x-half, z-half)
	for row := r0; row <= r1; row++ {
		for col := c0; col <= c1; col++ {
			do(row, col)
		}
	}
}
