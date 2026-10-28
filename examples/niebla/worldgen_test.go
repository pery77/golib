package main

import (
	"math"
	"reflect"
	"sort"
	"testing"
)

// depositTiles returns the tiles a deposit reaches into, row by row.
func depositTiles(d Deposit) [][2]int {
	var tiles [][2]int
	for row := d.Row; row < d.Row+d.Rows; row++ {
		for col := d.Col; col < d.Col+d.Cols; col++ {
			if found, ok := depositAt(col, row); ok && found == d {
				tiles = append(tiles, [2]int{col, row})
			}
		}
	}
	return tiles
}

// testSeeds are the regions the generator's laws are checked on.
const testSeeds = 40

func TestASeedGivesTheSameRegion(t *testing.T) {
	a, b := generateRegion(7), generateRegion(7)
	if !reflect.DeepEqual(a, b) {
		t.Error("the same seed generated two different regions")
	}
	if reflect.DeepEqual(a.levels, generateRegion(8).levels) {
		t.Error("two seeds generated the same relief")
	}
}

// TestSlopesAreOneLevelToTheCell pins the collapse's constraint where
// the game meets it: no cell's corners stand more than a level apart.
func TestSlopesAreOneLevelToTheCell(t *testing.T) {
	for seed := int64(0); seed < testSeeds; seed++ {
		g := generateRegion(seed)
		for row := 0; row < regionCellRows; row++ {
			for col := 0; col < regionCellCols; col++ {
				low, high := int8(math.MaxInt8), int8(math.MinInt8)
				for _, l := range []int8{
					g.level(col, row), g.level(col+1, row),
					g.level(col, row+1), g.level(col+1, row+1),
				} {
					low, high = min(low, l), max(high, l)
				}
				if high-low > 1 {
					t.Fatalf("seed %d: cell %d, %d spans levels %d to %d",
						seed, col, row, low, high)
				}
			}
		}
	}
}

// TestTheReliefLeavesGreatPlains pins what building asks of the ground:
// most of it flat, all of it flat and plain around the core, and yet
// some relief in every region.
func TestTheReliefLeavesGreatPlains(t *testing.T) {
	const cellsPerTile = unitsPerTile / buildingCell
	for seed := int64(0); seed < testSeeds; seed++ {
		g := generateRegion(seed)
		flat, levels := 0, map[int8]bool{}
		for row := 0; row < regionCellRows; row++ {
			for col := 0; col < regionCellCols; col++ {
				levels[g.level(col, row)] = true
				if g.flatCell(col, row) {
					flat++
					continue
				}
				far := math.Hypot(
					float64(col)+0.5-(coreCol+0.5)*cellsPerTile,
					float64(row)+0.5-(coreRow+0.5)*cellsPerTile)
				if far < 1.5*cellsPerTile {
					t.Fatalf("seed %d: a slope at cell %d, %d, by the core",
						seed, col, row)
				}
			}
		}
		part := float64(flat) / (regionCellCols * regionCellRows)
		if part < 0.75 || part > 0.98 {
			t.Errorf("seed %d: %.0f%% of the ground is flat, want 75 to 98",
				seed, part*100)
		}
		if len(levels) < 3 {
			t.Errorf("seed %d: the relief has %d levels, want 3 or more",
				seed, len(levels))
		}
	}
}

// TestDepositsStandWhereTheColonyNeedsThem pins the deposits' spread:
// one pool and one vein whole inside the core's bubble, two more of each
// with their hearts past 7 tiles (over a kilometer) out and inside the
// fog line, every heart on flat ground for the pump, and no tile shared.
func TestDepositsStandWhereTheColonyNeedsThem(t *testing.T) {
	const cellsPerTile = unitsPerTile / buildingCell
	for seed := int64(0); seed < testSeeds; seed++ {
		g := generateRegion(seed)
		if len(g.deposits) != len(depositPlans) {
			t.Fatalf("seed %d: %d deposits, want %d",
				seed, len(g.deposits), len(depositPlans))
		}
		inside, far := map[byte]int{}, map[byte]int{}
		tiles := 0
		for _, d := range g.deposits {
			if !g.flatCell(d.HeartCol, d.HeartRow) {
				t.Errorf("seed %d: the heart of deposit %d stands on a slope",
					seed, d.Index)
			}
			heart := math.Hypot(
				float64(d.HeartCol)+0.5-(coreCol+0.5)*cellsPerTile,
				float64(d.HeartRow)+0.5-(coreRow+0.5)*cellsPerTile) / cellsPerTile
			reach := 0.0
			for _, c := range g.bodies[d.Index] {
				reach = math.Max(reach, math.Hypot(
					float64(c.Col)+0.5-(coreCol+0.5)*cellsPerTile,
					float64(c.Row)+0.5-(coreRow+0.5)*cellsPerTile)/cellsPerTile)
				tcol, trow := cellTile(c.Col, c.Row)
				if g.tiles[trow][tcol] != d.Kind ||
					g.depositIndex[[2]int{tcol, trow}] != d.Index {
					t.Fatalf("seed %d: cell %d, %d of deposit %d lies on a tile that isn't its own",
						seed, c.Col, c.Row, d.Index)
				}
			}
			switch {
			case reach <= coreBubbleRadius:
				inside[d.Kind]++
			case heart > 7 && heart < fogLineRadius:
				far[d.Kind]++
			default:
				t.Errorf("seed %d: deposit %d has its heart %.1f tiles out and reaches %.1f",
					seed, d.Index, heart, reach)
			}
		}
		for _, kind := range []byte{kindOil, kindLilac} {
			if inside[kind] != 1 || far[kind] != 2 {
				t.Errorf("seed %d: %c has %d deposits inside the bubble and %d far, want 1 and 2",
					seed, kind, inside[kind], far[kind])
			}
		}
		for row := range g.tiles {
			for col := range g.tiles[row] {
				if g.tiles[row][col] == kindOil || g.tiles[row][col] == kindLilac {
					tiles++
				}
			}
		}
		if tiles != len(g.depositIndex) {
			t.Errorf("seed %d: %d deposit tiles, %d of them indexed",
				seed, tiles, len(g.depositIndex))
		}
		if g.tiles[coreRow][coreCol] != kindCore {
			t.Errorf("seed %d: the core's tile is %c", seed, g.tiles[coreRow][coreCol])
		}
	}
}

// TestDepositsThinOutTowardTheRim pins the deposits' shape: the heart is
// the richest cell, the rich cells crowd around it and the poor ones lie
// out at the rim, the deposits differ in what they hold, and the far
// ones don't hold less than the bubble's.
func TestDepositsThinOutTowardTheRim(t *testing.T) {
	for seed := int64(0); seed < testSeeds; seed++ {
		g := generateRegion(seed)
		sizes := map[float64]bool{}
		for _, d := range g.deposits {
			sizes[d.Full] = true
			body := append([]OreCell(nil), g.bodies[d.Index]...)
			sort.Slice(body, func(i, j int) bool { return body[i].Rich > body[j].Rich })
			if body[0].Col != d.HeartCol || body[0].Row != d.HeartRow {
				t.Errorf("seed %d: the richest cell of deposit %d isn't its heart",
					seed, d.Index)
			}
			away := func(cells []OreCell) float64 {
				sum := 0.0
				for _, c := range cells {
					if c.Rich > 1 || c.Rich <= 0 {
						t.Fatalf("seed %d: a cell of richness %v", seed, c.Rich)
					}
					sum += math.Hypot(
						float64(c.Col-d.HeartCol), float64(c.Row-d.HeartRow))
				}
				return sum / float64(len(cells))
			}
			quarter := len(body) / 4
			rich, poor := away(body[:quarter]), away(body[len(body)-quarter:])
			if rich > 0.8*poor {
				t.Errorf("seed %d: deposit %d has its rich cells %.1f cells from the heart and its poor ones %.1f",
					seed, d.Index, rich, poor)
			}
		}
		if len(sizes) < len(g.deposits)-1 {
			t.Errorf("seed %d: the deposits hold %d amounts between the %d of them",
				seed, len(sizes), len(g.deposits))
		}
		for _, d := range g.deposits[2:] {
			if near := g.deposits[d.Index%2]; d.Full <= near.Full*0.8 {
				t.Errorf("seed %d: the far deposit %d holds %v, the bubble's %v",
					seed, d.Index, d.Full, near.Full)
			}
		}
	}
}

// TestOreWearsFromTheRimIn pins what the player sees of a worked
// deposit: the part that shows follows what remains, and the rim goes
// first.
func TestOreWearsFromTheRimIn(t *testing.T) {
	g := generateRegion(defaultSeed)
	d := g.deposits[0]
	if cut := oreCut(g, d, d.Full); cut > 0.001 {
		t.Errorf("a full deposit is cut at %v, want 0", cut)
	}
	half := oreCut(g, d, d.Full/2)
	whole, left, gone := 0.0, 0.0, 0
	for _, c := range g.bodies[d.Index] {
		whole += float64(c.Rich)
		if c.Rich > half {
			left += float64(c.Rich - half)
		} else {
			gone++
		}
	}
	if math.Abs(left/whole-0.5) > 0.01 {
		t.Errorf("half a deposit shows %.3f of its ore", left/whole)
	}
	if gone == 0 {
		t.Error("half a deposit still shows every cell of its rim")
	}
}

func TestUnprojectUndoesProjectOnTheRelief(t *testing.T) {
	useRegion(defaultSeed)
	for row := 3; row < regionCellRows; row += 7 {
		for col := 3; col < regionCellCols; col += 7 {
			x, y := cellCenterUnits(col, row)
			px, py := project(float32(x), float32(y))
			gx, gy := unproject(px, py)
			if math.Hypot(float64(gx)-x, float64(gy)-y) > 1 {
				t.Fatalf("the middle of cell %d, %d came back as %v, %v, want %v, %v",
					col, row, gx, gy, x, y)
			}
		}
	}
}

func TestBuildingsAskForFlatGround(t *testing.T) {
	s := newGame()
	seedStock(s)
	const cellsPerTile = unitsPerTile / buildingCell
	for row := (coreRow - 4) * cellsPerTile; row < (coreRow+4)*cellsPerTile; row++ {
		for col := (coreCol - 4) * cellsPerTile; col < (coreCol+4)*cellsPerTile; col++ {
			if !land.flatCell(col, row) && canPlace(s, BuildingSilo, col, row) {
				t.Fatalf("a silo may rise on the slope at cell %d, %d", col, row)
			}
		}
	}
}

func TestAnOldSaveWakesUpWithFullDeposits(t *testing.T) {
	s := newGame()
	s.Drain = map[string]float64{"5,6": 12}
	s.enterRegion()
	for _, d := range land.deposits {
		if s.Drain[depositKey(d)] != d.Full {
			t.Errorf("deposit %d woke up holding %v, want %v",
				d.Index, s.Drain[depositKey(d)], d.Full)
		}
	}
}
