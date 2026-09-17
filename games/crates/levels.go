package main

import (
	"errors"
	"fmt"
	"strings"

	"golib"
)

// levelFiles returns the levels in the order they are played: every Tiled map
// in assets/maps/, sorted by name, which is why they are named level01.tmx and
// up. To add a level, save a new map there, and nothing else: golib.ListAssets
// finds it. The tileset they share, tiles.tsx, is not a level.
func levelFiles() ([]string, error) {
	names, err := golib.ListAssets("maps")
	if err != nil {
		return nil, err
	}
	var levels []string
	for _, name := range names {
		if strings.HasSuffix(name, ".tmx") {
			levels = append(levels, name)
		}
	}
	if len(levels) == 0 {
		return nil, errors.New("there are no levels in assets/maps/: save a Tiled map there, named level01.tmx")
	}
	return levels, nil
}

// levelMaps holds the map of every level, in the order of the levels
// loadLevels returns, for play.go to draw the floor with. Each map is made
// once, as GoLib asks.
var levelMaps []*golib.Map

// The layers of a level map, and the tile classes the tileset gives its tiles.
// The rules read classes, never tile IDs, so the tileset can be redrawn.
const (
	floorLayer = "floor"  // tiles of class floor, goal and wall; empty cells are outside the room
	thingLayer = "things" // tiles of class crate, and one of class start

	floorClass = "floor"
	goalClass  = "goal"
	wallClass  = "wall"
	crateClass = "crate"
	startClass = "start"
)

// The largest level that fits on the screen, in cells, with the heading above
// it and the hints below it.
const (
	maxColumns = screenWidth / tileSize
	maxRows    = 9
)

// loadLevels reads every level, in the order they are played, and returns the
// first mistake found in them.
func loadLevels() ([]*layout, error) {
	names, err := levelFiles()
	if err != nil {
		return nil, err
	}
	levels := make([]*layout, len(names))
	maps := make([]*golib.Map, len(names))
	for i, name := range names {
		maps[i] = golib.NewMap(name)
		if levels[i], err = loadLayout(maps[i], name); err != nil {
			return nil, err
		}
	}
	levelMaps = maps
	return levels, nil
}

// loadLayout reads the level in map m, whose file is name, and checks that it
// can be played.
func loadLayout(m *golib.Map, name string) (*layout, error) {
	if err := m.Err(); err != nil {
		return nil, err // missing, not a Tiled map, or one GoLib can't read
	}
	if m.TileWidth() != tileSize || m.TileHeight() != tileSize {
		return nil, fmt.Errorf("%s: its tiles are %g by %g pixels, not %d by %d", name, m.TileWidth(), m.TileHeight(), tileSize, tileSize)
	}
	l := &layout{
		name:    name,
		title:   m.Properties().String("title"),
		par:     m.Properties().Int("par"),
		columns: int(m.Width() / tileSize),
		rows:    int(m.Height() / tileSize),
	}
	if l.columns > maxColumns || l.rows > maxRows {
		return nil, fmt.Errorf("%s is %d by %d cells: the screen fits %d by %d", name, l.columns, l.rows, maxColumns, maxRows)
	}
	l.ground = make([]ground, l.columns*l.rows)
	goals, starts := 0, 0
	for row := range l.rows {
		for column := range l.columns {
			c := cell{column, row}
			switch class := m.Tile(floorLayer, column, row).Class; class {
			case "":
				l.ground[row*l.columns+column] = outside
			case floorClass:
				l.ground[row*l.columns+column] = floor
			case goalClass:
				l.ground[row*l.columns+column] = goal
				goals++
			case wallClass:
				l.ground[row*l.columns+column] = wall
			default:
				return nil, fmt.Errorf("%s: the %s layer has a tile of class %q at column %d, row %d: use floor, goal or wall", name, floorLayer, class, column, row)
			}
			thing := m.Tile(thingLayer, column, row).Class
			if thing == "" {
				continue
			}
			if !l.at(c).walkable() {
				return nil, fmt.Errorf("%s: the %s at column %d, row %d isn't on floor or a goal", name, thing, column, row)
			}
			switch thing {
			case crateClass:
				l.crates = append(l.crates, c)
			case startClass:
				l.start = c
				starts++
			default:
				return nil, fmt.Errorf("%s: the %s layer has a tile of class %q at column %d, row %d: use crate or start", name, thingLayer, thing, column, row)
			}
		}
	}
	switch {
	case starts != 1:
		return nil, fmt.Errorf("%s has %d tiles of class start in the %s layer: it needs exactly one", name, starts, thingLayer)
	case len(l.crates) == 0:
		return nil, fmt.Errorf("%s has no crates", name)
	case len(l.crates) != goals:
		return nil, fmt.Errorf("%s has %d crates and %d goals: it needs as many of each", name, len(l.crates), goals)
	case l.title == "":
		return nil, fmt.Errorf("%s has no title: give the map a string property named title", name)
	}
	return l, nil
}
