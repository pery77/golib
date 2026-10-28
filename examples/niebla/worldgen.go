package main

import (
	"math"
	"math/bits"
	"sort"
)

// The region is generated: relief, ground cover and deposits are a pure
// function of a seed, so State holds the seed alone and a save reproduces
// its ground. Nothing here draws, and nothing here reads the state.

// Relief tuning. The landforms are decided by wave function collapse on
// blocks of cells; the ground's height lives on the cells' corners, so a
// cell whose four corners agree is flat, and a building asks for one.
const (
	reliefBlock  = 4 // cells on a side of a landform block: 100 m
	reliefCols   = regionCellCols / reliefBlock
	reliefRows   = regionCellRows / reliefBlock
	reliefLevels = 4 // 0 is a basin, 1 the plain, 2 and 3 the hills
	reliefPlain  = 1
	levelHeight  = 4 // u of height to a level

	reliefMarks     = 16   // hilltops and basins pinned before the collapse
	reliefMarkNear  = 3.5  // tiles from the core, the nearest a mark stands
	reliefMarkFar   = 11   // tiles, the farthest
	reliefCorePlain = 2.2  // tiles around the core that are plain, always
	reliefWarpCells = 2.6  // cells the blocks' edges wander by
	reliefWarpWave  = 11.0 // cells, the wander's wavelength
)

// How much a level weighs in a collapse, and how much more for each
// neighbor already collapsed to it. The plain's pull is what leaves
// great flats; the hills' weak one keeps them hills.
var (
	reliefWeight   = [reliefLevels]float64{0.5, 6, 1.2, 0.35}
	reliefAffinity = [reliefLevels]float64{3.5, 6, 2.3, 2.1}
)

// Deposit tuning. A deposit is a field of richness over cells, 1 at its
// heart and thinning out to specks at its rim; what it holds is its
// richness times the ore's density.
const (
	oilPerRichCell   = 70  // liters in a cell of richness 1
	lilacPerRichCell = 240 // kilograms

	depositTileOre  = 0.8 // richness a tile needs to belong to a deposit
	depositApart    = 3.2 // tiles between two hearts, at the least
	depositCoreGap  = 0.9 // tiles around the core's middle that hold no ore
	depositAttempts = 400
)

// depositPlan is where a deposit may stand and how large it may grow.
type depositPlan struct {
	kind         byte
	near, far    float64 // tiles from the core to its heart
	small, large float64 // its radius, in cells
}

// The region's deposits: one of each kind in the bubble's comfort, and
// two of each out where the hauls are long, the richer the farther.
var depositPlans = []depositPlan{
	{kindOil, 1.7, 2.4, 6.5, 7.5},
	{kindLilac, 1.7, 2.4, 6.5, 7.5},
	{kindOil, 7.2, 8.4, 8, 12},
	{kindLilac, 7.2, 8.4, 8, 12},
	{kindOil, 7.2, 8.4, 8, 12},
	{kindLilac, 7.2, 8.4, 8, 12},
}

// OreCell is one cell of a deposit's body.
type OreCell struct {
	Col, Row int
	Rich     float32 // 0 to 1
}

// Region is the generated ground.
type Region struct {
	Seed         int64
	tiles        [regionRows][regionCols]byte
	levels       []int8    // relief levels on the cells' corners
	cover        []float32 // vegetation per cell, 0 bare to 1 thick
	deposits     []Deposit
	bodies       [][]OreCell // each deposit's cells
	depositIndex map[[2]int]int
	ore          map[[2]int]float32 // the cells that hold ore, and how much
}

// rng is splitmix64: the generator's own numbers, so a seed gives the
// same region on every machine and Go version.
type rng struct{ s uint64 }

func (r *rng) next() uint64 {
	r.s += 0x9e3779b97f4a7c15
	z := r.s
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func (r *rng) float() float64 {
	return float64(r.next()>>11) / (1 << 53)
}

func (r *rng) between(low, high float64) float64 {
	return low + (high-low)*r.float()
}

// lattice returns a stable number from 0 to 1 for a point of a grid.
func lattice(salt uint64, x, y int) float64 {
	h := salt ^ uint64(int64(x))*0x9e3779b97f4a7c15 ^
		uint64(int64(y))*0xc2b2ae3d27d4eb4f
	h = (h ^ (h >> 30)) * 0xbf58476d1ce4e5b9
	h = (h ^ (h >> 27)) * 0x94d049bb133111eb
	h ^= h >> 31
	return float64(h>>11) / (1 << 53)
}

// valueNoise returns smooth noise from 0 to 1, one feature to the unit.
func valueNoise(salt uint64, x, y float64) float64 {
	x0, y0 := math.Floor(x), math.Floor(y)
	fx, fy := x-x0, y-y0
	fx = fx * fx * (3 - 2*fx)
	fy = fy * fy * (3 - 2*fy)
	ix, iy := int(x0), int(y0)
	top := lattice(salt, ix, iy)*(1-fx) + lattice(salt, ix+1, iy)*fx
	bottom := lattice(salt, ix, iy+1)*(1-fx) + lattice(salt, ix+1, iy+1)*fx
	return top*(1-fy) + bottom*fy
}

// fbm stacks octaves of valueNoise, each half as strong and twice as
// fine. It stays within 0 and 1.
func fbm(salt uint64, x, y float64, octaves int) float64 {
	sum, strength, total := 0.0, 1.0, 0.0
	for i := 0; i < octaves; i++ {
		sum += valueNoise(salt+uint64(i)*7919, x, y) * strength
		total += strength
		strength /= 2
		x, y = x*2, y*2
	}
	return sum / total
}

func generateRegion(seed int64) *Region {
	g := &Region{Seed: seed, depositIndex: map[[2]int]int{}, ore: map[[2]int]float32{}}
	r := &rng{s: uint64(seed)*0x2545f4914f6cdd1d + 0x1234567}
	for row := range g.tiles {
		for col := range g.tiles[row] {
			g.tiles[row][col] = kindGround
		}
	}
	g.tiles[coreRow][coreCol] = kindCore
	g.raiseRelief(r)
	g.growCover(r)
	for _, plan := range depositPlans {
		g.placeDeposit(r, plan)
	}
	return g
}

func (g *Region) level(vcol, vrow int) int8 {
	return g.levels[vrow*(regionCellCols+1)+vcol]
}

// flatCell reports whether a cell's four corners stand at one height.
func (g *Region) flatCell(col, row int) bool {
	if col < 0 || row < 0 || col >= regionCellCols || row >= regionCellRows {
		return false
	}
	l := g.level(col, row)
	return g.level(col+1, row) == l &&
		g.level(col, row+1) == l &&
		g.level(col+1, row+1) == l
}

// cornerHeight returns the ground's height at a cell corner, in units
// over the plain.
func (g *Region) cornerHeight(vcol, vrow int) float32 {
	return float32(g.level(vcol, vrow)-reliefPlain) * levelHeight
}

// heightAt returns the ground's height at a world point, in units over
// the plain, blended between the corners of the cell it falls on.
func (g *Region) heightAt(x, y float32) float32 {
	cx := clampf(x/buildingCell, 0, regionCellCols)
	cy := clampf(y/buildingCell, 0, regionCellRows)
	col := int(math.Min(float64(cx), regionCellCols-1))
	row := int(math.Min(float64(cy), regionCellRows-1))
	fx, fy := cx-float32(col), cy-float32(row)
	top := g.cornerHeight(col, row)*(1-fx) + g.cornerHeight(col+1, row)*fx
	bottom := g.cornerHeight(col, row+1)*(1-fx) + g.cornerHeight(col+1, row+1)*fx
	return top*(1-fy) + bottom*fy
}

// coverAt returns how much grows on a cell, 0 bare to 1 thick.
func (g *Region) coverAt(col, row int) float32 {
	return g.cover[row*regionCellCols+col]
}

// raiseRelief decides the landforms by wave function collapse and lays
// them on the cells' corners. Each block of cells is a wave over the
// relief levels; neighbors, the diagonal ones too, may differ by a level
// at the most, so a slope is never steeper than one level to the cell.
// The blocks around the core are pinned to the plain and a few hilltops
// and basins are pinned out in the region; then the block with the least
// left to decide collapses, to a level drawn by weight, and what that
// rules out spreads to its neighbors. The constraint keeps every wave an
// unbroken run of levels, so the collapse never contradicts itself.
func (g *Region) raiseRelief(r *rng) {
	const all = 1<<reliefLevels - 1
	waves := make([]uint8, reliefCols*reliefRows)
	for i := range waves {
		waves[i] = all
	}
	at := func(col, row int) int { return row*reliefCols + col }
	inside := func(col, row int) bool {
		return col >= 0 && row >= 0 && col < reliefCols && row < reliefRows
	}
	var stack []int
	narrow := func(i int, allowed uint8) {
		if next := waves[i] & allowed; next != waves[i] {
			waves[i] = next
			stack = append(stack, i)
		}
	}
	spread := func() {
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			w := waves[i]
			reach := (w | w<<1 | w>>1) & all
			col, row := i%reliefCols, i/reliefCols
			for dr := -1; dr <= 1; dr++ {
				for dc := -1; dc <= 1; dc++ {
					if (dc != 0 || dr != 0) && inside(col+dc, row+dr) {
						narrow(at(col+dc, row+dr), reach)
					}
				}
			}
		}
	}
	coreTiles := func(col, row int) float64 {
		const blockTiles = float64(reliefBlock*buildingCell) / unitsPerTile
		return math.Hypot(
			(float64(col)+0.5)*blockTiles-(coreCol+0.5),
			(float64(row)+0.5)*blockTiles-(coreRow+0.5))
	}

	for row := 0; row < reliefRows; row++ {
		for col := 0; col < reliefCols; col++ {
			if coreTiles(col, row) <= reliefCorePlain {
				narrow(at(col, row), 1<<reliefPlain)
			}
		}
	}
	spread()
	for placed, tries := 0, 0; placed < reliefMarks && tries < 1000; tries++ {
		col, row := int(r.float()*reliefCols), int(r.float()*reliefRows)
		far := coreTiles(col, row)
		if far < reliefMarkNear || far > reliefMarkFar {
			continue
		}
		mark := uint8(1 << (reliefLevels - 1))
		if placed%3 == 2 {
			mark = 1
		}
		if waves[at(col, row)]&mark == 0 {
			continue
		}
		narrow(at(col, row), mark)
		spread()
		placed++
	}

	for {
		// The least left to decide: the fewest levels, then the most
		// decided neighbors, then the generator's pick among equals.
		pick, best, equals := -1, math.MaxInt, 0
		for i, w := range waves {
			if bits.OnesCount8(w) < 2 {
				continue
			}
			col, row := i%reliefCols, i/reliefCols
			decided := 0
			for dr := -1; dr <= 1; dr++ {
				for dc := -1; dc <= 1; dc++ {
					if inside(col+dc, row+dr) &&
						bits.OnesCount8(waves[at(col+dc, row+dr)]) == 1 {
						decided++
					}
				}
			}
			score := bits.OnesCount8(w)*16 - decided
			if score < best {
				best, equals = score, 0
			}
			if score == best {
				equals++
				if r.float()*float64(equals) < 1 {
					pick = i
				}
			}
		}
		if pick < 0 {
			break
		}
		col, row := pick%reliefCols, pick/reliefCols
		var weights [reliefLevels]float64
		total := 0.0
		for l := 0; l < reliefLevels; l++ {
			if waves[pick]&(1<<l) == 0 {
				continue
			}
			weights[l] = reliefWeight[l]
			for dr := -1; dr <= 1; dr++ {
				for dc := -1; dc <= 1; dc++ {
					if inside(col+dc, row+dr) &&
						waves[at(col+dc, row+dr)] == 1<<l {
						weights[l] *= reliefAffinity[l]
					}
				}
			}
			total += weights[l]
		}
		draw := r.float() * total
		chosen := 0
		for l := 0; l < reliefLevels; l++ {
			if weights[l] == 0 {
				continue
			}
			chosen = l
			if draw < weights[l] {
				break
			}
			draw -= weights[l]
		}
		narrow(pick, 1<<chosen)
		spread()
	}

	// Onto the corners: each takes the level of the block it falls on,
	// after a smooth wander that takes the blocks' straight edges away.
	// The wander moves two neighboring corners less than a block apart,
	// so they land on the same block or on neighbors, and the slopes stay
	// one level to the cell.
	warpX, warpY := r.next(), r.next()
	g.levels = make([]int8, (regionCellCols+1)*(regionCellRows+1))
	block := func(v float64, count int) int {
		return int(math.Max(0, math.Min(float64(count-1), math.Floor(v/reliefBlock))))
	}
	for vrow := 0; vrow <= regionCellRows; vrow++ {
		for vcol := 0; vcol <= regionCellCols; vcol++ {
			x, y := float64(vcol), float64(vrow)
			nx, ny := x/reliefWarpWave, y/reliefWarpWave
			wx := x + (valueNoise(warpX, nx, ny)-0.5)*2*reliefWarpCells
			wy := y + (valueNoise(warpY, nx, ny)-0.5)*2*reliefWarpCells
			w := waves[at(block(wx, reliefCols), block(wy, reliefRows))]
			g.levels[vrow*(regionCellCols+1)+vcol] = int8(bits.TrailingZeros8(w))
		}
	}
}

// growCover spreads the vegetation: broad zones of more and less, thicker
// in the basins where the water sits, thinner up the hills and thinnest
// on the slopes.
func (g *Region) growCover(r *rng) {
	const (
		zoneWave   = 28.0 // cells, the zones' wavelength
		basinBonus = 0.16
		slopeToll  = 0.22
	)
	salt := r.next()
	g.cover = make([]float32, regionCellCols*regionCellRows)
	for row := 0; row < regionCellRows; row++ {
		for col := 0; col < regionCellCols; col++ {
			v := fbm(salt, float64(col)/zoneWave, float64(row)/zoneWave, 3)
			v = (v-0.5)*2.4 + 0.5
			v += basinBonus * float64(reliefPlain-g.level(col, row))
			if !g.flatCell(col, row) {
				v -= slopeToll
			}
			g.cover[row*regionCellCols+col] = float32(math.Max(0, math.Min(1, v)))
		}
	}
}

// placeDeposit finds a deposit its place and grows it there: a heart on
// flat ground, for the pump, at the plan's distance from the core, apart
// from the other hearts, with a body that shares no tile with another's.
func (g *Region) placeDeposit(r *rng, plan depositPlan) {
	const cellsPerTile = unitsPerTile / buildingCell
	for try := 0; try < depositAttempts; try++ {
		angle := r.between(0, 2*math.Pi)
		far := r.between(plan.near, plan.far)
		radius := r.between(plan.small, plan.large)
		turn := r.between(0, math.Pi)
		salt := r.next()
		heartCol := int((coreCol + 0.5 + far*math.Cos(angle)) * cellsPerTile)
		heartRow := int((coreRow + 0.5 + far*math.Sin(angle)) * cellsPerTile)
		if !g.flatCell(heartCol, heartRow) || g.crowded(heartCol, heartRow) {
			continue
		}
		body := g.growBody(plan.kind, heartCol, heartRow, radius, turn, salt)
		if body == nil {
			continue
		}
		d := Deposit{
			Kind: plan.kind, Index: len(g.deposits),
			HeartCol: heartCol, HeartRow: heartRow,
			Cells: len(body),
		}
		density := float64(lilacPerRichCell)
		if plan.kind == kindOil {
			density = oilPerRichCell
		}
		minCol, minRow, maxCol, maxRow := regionCols, regionRows, -1, -1
		rich := 0.0
		for _, c := range body {
			rich += float64(c.Rich)
			tcol, trow := cellTile(c.Col, c.Row)
			g.tiles[trow][tcol] = plan.kind
			g.depositIndex[[2]int{tcol, trow}] = d.Index
			minCol, maxCol = min(minCol, tcol), max(maxCol, tcol)
			minRow, maxRow = min(minRow, trow), max(maxRow, trow)
		}
		d.Col, d.Row = minCol, minRow
		d.Cols, d.Rows = maxCol-minCol+1, maxRow-minRow+1
		d.Full = math.Round(rich*density/10) * 10
		g.deposits = append(g.deposits, d)
		g.bodies = append(g.bodies, body)
		for _, c := range body {
			g.ore[[2]int{c.Col, c.Row}] = c.Rich
		}
		return
	}
}

// crowded reports whether a heart would stand too close to another.
func (g *Region) crowded(heartCol, heartRow int) bool {
	const cellsPerTile = unitsPerTile / buildingCell
	for _, d := range g.deposits {
		apart := math.Hypot(
			float64(d.HeartCol-heartCol), float64(d.HeartRow-heartRow))
		if apart < depositApart*cellsPerTile {
			return true
		}
	}
	return false
}

// growBody grows a deposit's cells around its heart: a stretched, turned
// blob whose edge a noise bends, mottled all over, and broken into
// specks toward the rim. Veins stretch more than pools. Cells on tiles
// too poor to count, or cut off from the heart's tile, are dropped; nil
// means the body doesn't fit here.
func (g *Region) growBody(
	kind byte,
	heartCol, heartRow int,
	radius, turn float64,
	salt uint64,
) []OreCell {
	const cellsPerTile = unitsPerTile / buildingCell
	stretch := 1.55
	if kind == kindOil {
		stretch = 1.15
	}
	reach := int(radius*stretch*1.4) + 1
	sin, cos := math.Sin(turn), math.Cos(turn)
	coreX := (coreCol + 0.5) * cellsPerTile
	coreY := (coreRow + 0.5) * cellsPerTile

	var body []OreCell
	tileOre := map[[2]int]float64{}
	for row := heartRow - reach; row <= heartRow+reach; row++ {
		for col := heartCol - reach; col <= heartCol+reach; col++ {
			dx, dy := float64(col-heartCol), float64(row-heartRow)
			along := (dx*cos + dy*sin) / (radius * stretch)
			across := (dy*cos - dx*sin) / (radius / stretch)
			bend := (fbm(salt, float64(col)/5, float64(row)/5, 2) - 0.5) * 0.9
			rich := 1 - (math.Hypot(along, across) + bend)
			if rich <= 0 {
				continue
			}
			rich = math.Pow(math.Min(rich, 1), 0.8)
			rich *= 0.55 + 0.45*valueNoise(salt+1, float64(col)/2, float64(row)/2)
			if rich < 0.3 && lattice(salt+2, col, row) < (0.3-rich)/0.3*0.7 {
				continue
			}
			if rich < 0.06 {
				continue
			}
			if col < 0 || row < 0 || col >= regionCellCols || row >= regionCellRows {
				return nil
			}
			if math.Hypot(float64(col)+0.5-coreX, float64(row)+0.5-coreY) <
				depositCoreGap*cellsPerTile {
				continue
			}
			body = append(body, OreCell{Col: col, Row: row, Rich: float32(rich)})
			tcol, trow := cellTile(col, row)
			tileOre[[2]int{tcol, trow}] += rich
		}
	}

	// The tiles that count, flooded from the heart's.
	hcol, hrow := cellTile(heartCol, heartRow)
	member := map[[2]int]bool{}
	queue := [][2]int{{hcol, hrow}}
	for len(queue) > 0 {
		t := queue[0]
		queue = queue[1:]
		if member[t] || tileOre[t] < depositTileOre {
			continue
		}
		if g.tiles[t[1]][t[0]] != kindGround {
			return nil
		}
		member[t] = true
		queue = append(queue,
			[2]int{t[0] + 1, t[1]}, [2]int{t[0] - 1, t[1]},
			[2]int{t[0], t[1] + 1}, [2]int{t[0], t[1] - 1})
	}
	if !member[[2]int{hcol, hrow}] {
		return nil
	}
	kept := body[:0]
	heart := false
	for _, c := range body {
		tcol, trow := cellTile(c.Col, c.Row)
		if !member[[2]int{tcol, trow}] {
			continue
		}
		if c.Col == heartCol && c.Row == heartRow {
			c.Rich = 1
			heart = true
		}
		kept = append(kept, c)
	}
	if !heart {
		return nil
	}
	// Back to front, the order the crystals are drawn in.
	sort.SliceStable(kept, func(i, j int) bool {
		return kept[i].Col+kept[i].Row < kept[j].Col+kept[j].Row
	})
	return kept
}
