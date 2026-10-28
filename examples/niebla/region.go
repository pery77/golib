package main

import "math"

// Tuning: the numbers that define the region, with units in the name.
const (
	regionCols = 25 // tiles across
	regionRows = 25 // tiles down

	tileW = 48 // tile width, in screen pixels
	tileH = 24 // tile height, in screen pixels

	// The world measures itself in units, and one unit is one meter
	// (things.go). A tile is 200 u across, so the region is 5 km from
	// side to side and a deposit is a few hundred meters across.
	// Zoomed out the whole region fits on the screen as icons, zoomed in
	// a tile fills it. A robot is 6 u across, the core's pole 10 u, a
	// future building 40 u.
	unitsPerTile = 200

	coreCol = 12 // where the core sits
	coreRow = 12

	coreBubbleRadius = 4.0  // tiles; nothing is digested inside it
	fogLineRadius    = 10.5 // tiles; the fog's front stands here, for now
	fogFadeTiles     = 2.4  // tiles; how far the fog fades in past the line
)

// The size of one unit on the screen, before the camera zooms.
const (
	unitW = float32(tileW) / unitsPerTile
	unitH = float32(tileH) / unitsPerTile
)

// Where the region's top corner lands on the screen, so the whole diamond
// fits with a margin.
const (
	regionOriginX = 640
	regionOriginY = 72
)

// The tile kinds. A tile is ground unless the core stands on it or a
// deposit's body reaches into it.
const (
	kindGround = '.'
	kindOil    = 'o'
	kindLilac  = 'L'
	kindCore   = 'C'
)

// defaultSeed is the region a new State is dealt when nobody names one,
// and the one a save from before the seeds wakes up in.
const defaultSeed = 0

// land is the region the game stands on: static data the rules and the
// view read freely, all of it a pure function of State.Seed (worldgen.go).
// It never enters the state; the seed does. useRegion keeps it the
// seed's, and Apply calls it, so no action ever runs on another region's
// ground.
var land = generateRegion(defaultSeed)

func useRegion(seed int64) {
	if land.Seed != seed {
		land = generateRegion(seed)
	}
}

// tileAt returns the kind of the tile at a column and row. Outside the
// region there is only ground, which the fog covers anyway.
func tileAt(col, row int) byte {
	if col < 0 || row < 0 || col >= regionCols || row >= regionRows {
		return kindGround
	}
	return land.tiles[row][col]
}

// tileDistance returns how far a tile is from the core, in tiles.
func tileDistance(col, row int) float32 {
	return float32(math.Hypot(float64(col-coreCol), float64(row-coreRow)))
}

// projectFlat returns where the world point at x, y units would land on
// the screen were the ground flat, as DESIGN.md says: screenX =
// (x-y)*unitW/2, screenY = (x+y)*unitH/2. The fog and the bubbles, which
// are no part of the ground, are drawn with it.
func projectFlat(x, y float32) (sx, sy float32) {
	return (x-y)*unitW/2 + regionOriginX, (x+y)*unitH/2 + regionOriginY
}

// project returns where the ground at x, y units lands on the screen:
// projectFlat, lifted by the relief. Whatever stands on the ground is
// drawn from it.
func project(x, y float32) (sx, sy float32) {
	sx, sy = projectFlat(x, y)
	return sx, sy - land.heightAt(x, y)*unitH
}

// projectTile returns where a tile's top corner lands on the screen, on
// flat ground.
func projectTile(col, row float32) (x, y float32) {
	return projectFlat(col*unitsPerTile, row*unitsPerTile)
}

// unproject undoes project: the ground under a projected point. The lift
// depends on the ground it lands on, so it closes in on it; the relief is
// gentle, and a few rounds leave nothing to see.
func unproject(px, py float32) (x, y float32) {
	lift := float32(0)
	for i := 0; i < 4; i++ {
		a := (px - regionOriginX) / (unitW / 2)
		b := (py + lift - regionOriginY) / (unitH / 2)
		x, y = (a+b)/2, (b-a)/2
		lift = land.heightAt(x, y) * unitH
	}
	return x, y
}

// jitter returns a stable number from a tile's place and a salt, from -0.5
// to 0.5, so props sit off the middle of their tile without anything stored:
// the same tile always draws the same way.
func jitter(col, row, salt int) float32 {
	h := col*374761393 + row*668265263 + salt*1442695041
	h ^= h >> 13
	h *= 1274126177
	h ^= h >> 16
	return float32(h&0xffff)/float32(0xffff) - 0.5
}

// ellipseSemiAxes returns the half width and half height on the screen of a
// world circle of a radius in tiles, which the projection flattens.
func ellipseSemiAxes(radius float32) (halfW, halfH float32) {
	k := radius * float32(math.Sqrt2) / 2
	return k * tileW, k * tileH
}

// Deposit is one vein or pool: a body of ore cells the colony treats as
// a single thing, however many tiles it reaches into. Many robots may work
// it; one card describes it, and what remains lives under one key, its
// heart's tile. It is comparable, which is how the rules ask whether two
// tiles are one body.
type Deposit struct {
	Kind       byte    // kindOil or kindLilac
	Index      int     // its place in land.deposits
	Col, Row   int     // the bounding box's top corner tile
	Cols, Rows int     // its extent, in tiles
	HeartCol   int     // the richest cell: where the pump stands
	HeartRow   int     //
	Cells      int     // how many cells hold ore
	Full       float64 // what it holds at first, in its SI unit
}

// depositAt returns the deposit a tile belongs to, and whether it holds
// one.
func depositAt(col, row int) (Deposit, bool) {
	i, ok := land.depositIndex[[2]int{col, row}]
	if !ok {
		return Deposit{}, false
	}
	return land.deposits[i], true
}

// oreAt returns how much ore a cell was generated with: 0 where the
// deposit's body holds none, which is also where nothing draws.
func oreAt(col, row int) float32 {
	return land.ore[[2]int{col, row}]
}

// depositKey names a deposit's remaining amount inside State.Drain.
func depositKey(d Deposit) string {
	return drainKey(cellTile(d.HeartCol, d.HeartRow))
}

// depositFull returns what a deposit holds at first, in its SI unit: the
// richness of its cells times the ore's density.
func depositFull(d Deposit) float64 {
	return d.Full
}
