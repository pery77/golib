package golib

import (
	"cmp"
	"errors"
	"fmt"
	"image"
	"math"
	"slices"
	"strings"
	"sync"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Vector2 is a point, or a direction, in pixels.
type Vector2 struct {
	X, Y float32
}

// Map is a level made in Tiled: a .tmx file in the game's assets folder, with
// its tile layers, object layers, image layers and groups, and the tilesets
// it uses, from the map itself or from .tsx files.
//
//	var level = golib.NewMap("maps/level1.tmx")
//
// Draw it with [Screen.DrawMap], ask what is where with [Map.TileAt],
// [Map.TilesIn] and [Map.Objects], and read the custom properties Tiled gives
// it. The file is read the first time the map is used, so games make their
// maps once, as package variables or in main. A missing file, or one GoLib
// can't read, stops Run with an error that says what to fix.
//
// GoLib reads orthogonal maps, saved as XML, with tile layer data in CSV,
// base64, or base64 compressed with zlib or gzip, and images in PNG. An
// infinite map is read as a fixed one that covers all its tiles: everything
// moves so that its top-left tile is at 0, 0.
type Map struct {
	name string

	mu              sync.Mutex
	read            bool
	err             error
	tileWidth       int
	tileHeight      int
	width           int // in pixels
	height          int
	background      Color
	renderOrder     string
	parallaxOriginX float32
	parallaxOriginY float32
	properties      Properties
	tilesets        []*tileset
	layers          []*mapLayer // in drawing order, groups taken apart
}

type layerKind int

const (
	tileLayer layerKind = iota
	objectLayer
	imageLayer
)

var layerKindNames = []string{"a tile layer", "an object layer", "an image layer"}

// mapLayer is a layer of a map, with the settings of its groups applied.
type mapLayer struct {
	name       string // as Tiled shows it
	path       string // with its groups: "group/name"
	kind       layerKind
	visible    bool
	tint       Color // with the layer's opacity
	offsetX    float32
	offsetY    float32
	parallaxX  float32
	parallaxY  float32
	properties Properties

	columns, rows int
	gids          []uint32 // row by row; 0 is no tile

	objects []mapObject
	byIndex bool // draw objects in the order they are listed, not from the top down

	image            *Sprite
	repeatX, repeatY bool

	// chunks makes the grid of an infinite map's tile layer, once the map's
	// extent is known.
	chunks func(minColumn, minRow, columns, rows int) []uint32
}

type mapObject struct {
	MapObject
	gid              uint32  // a tile object's tile, with its flip flags
	originX, originY float32 // where Tiled puts the object, which rotation turns around
	alignX, alignY   float32 // the origin, as parts of the object's width and height
}

// Tile is a tile of a map: which one it is, and what its tileset says about
// it. The tile of an empty cell has an empty Tileset.
type Tile struct {
	Tileset    string     // The name of its tileset; "" for no tile.
	ID         int        // Its ID in its tileset, as Tiled shows it.
	Class      string     // Its class in the tileset (called type in older files).
	Properties Properties // Its custom properties in the tileset.
}

// Empty reports whether there is no tile.
func (t Tile) Empty() bool {
	return t.Tileset == ""
}

// MapTile is a tile in a tile layer, and where it is: its column and row, and
// its rectangle in the map's pixels.
type MapTile struct {
	Tile
	Rectangle
	Column, Row int
}

// MapObject is an object in an object layer.
type MapObject struct {
	// Rectangle is where the object is, in the map's pixels: its top-left
	// corner and size, before rotation. Tiled puts the origin of a tile
	// object at its bottom-left corner; GoLib moves it to the top-left one,
	// like every other object. A point has no size.
	Rectangle
	ID         int
	Name       string
	Class      string    // Called type in older files.
	Layer      string    // The name of its object layer.
	Shape      string    // "rectangle", "ellipse", "point", "polygon", "polyline", "text" or "tile".
	Rotation   float32   // Degrees, clockwise, around the point Tiled shows as its position.
	Points     []Vector2 // For a polygon or a polyline, its corners, from the top-left corner of the object.
	Text       string    // For a text object.
	Tile       Tile      // For a tile object.
	Visible    bool
	Properties Properties
}

// NewMap returns the Tiled map in the game's assets folder named name, which
// is relative to that folder and uses forward slashes, as in [ReadAsset].
func NewMap(name string) *Map {
	return &Map{name: name}
}

// Width returns the map's width, in pixels.
func (m *Map) Width() float32 {
	return m.value(func() float32 { return float32(m.width) })
}

// Height returns the map's height, in pixels.
func (m *Map) Height() float32 {
	return m.value(func() float32 { return float32(m.height) })
}

// TileWidth returns the width of the map's grid, in pixels.
func (m *Map) TileWidth() float32 {
	return m.value(func() float32 { return float32(m.tileWidth) })
}

// TileHeight returns the height of the map's grid, in pixels.
func (m *Map) TileHeight() float32 {
	return m.value(func() float32 { return float32(m.tileHeight) })
}

func (m *Map) value(get func() float32) float32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.prepare() != nil {
		return 0
	}
	return get()
}

// Properties returns the map's custom properties.
func (m *Map) Properties() Properties {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.prepare() != nil {
		return Properties{}
	}
	return m.properties
}

// LayerProperties returns the custom properties of the layer named layer, with
// those of its groups.
func (m *Map) LayerProperties(layer string) Properties {
	m.mu.Lock()
	defer m.mu.Unlock()
	l := m.layer(layer, -1)
	if l == nil {
		return Properties{}
	}
	return l.properties
}

// Tile returns the tile at column, row of the tile layer named layer.
// Columns and rows count from 0 at the top-left corner; outside the layer
// there are no tiles.
func (m *Map) Tile(layer string, column, row int) Tile {
	m.mu.Lock()
	defer m.mu.Unlock()
	l := m.layer(layer, tileLayer)
	if l == nil || column < 0 || row < 0 || column >= l.columns || row >= l.rows {
		return Tile{}
	}
	return m.tile(l.gids[row*l.columns+column])
}

// TileAt returns the tile of the tile layer named layer at x, y, in the map's
// pixels.
func (m *Map) TileAt(layer string, x, y float32) Tile {
	m.mu.Lock()
	l := m.layer(layer, tileLayer)
	if l == nil {
		m.mu.Unlock()
		return Tile{}
	}
	column := int(math.Floor(float64((x - l.offsetX) / float32(m.tileWidth))))
	row := int(math.Floor(float64((y - l.offsetY) / float32(m.tileHeight))))
	m.mu.Unlock()
	return m.Tile(layer, column, row)
}

// TilesIn returns the tiles of the tile layer named layer whose cells
// overlap area, in the map's pixels, row by row. Use it for collisions:
//
//	for _, tile := range level.TilesIn("ground", player.box) {
//		// push the player out of tile.Rectangle
//	}
func (m *Map) TilesIn(layer string, area Rectangle) []MapTile {
	m.mu.Lock()
	defer m.mu.Unlock()
	l := m.layer(layer, tileLayer)
	if l == nil {
		return nil
	}
	tw, th := float32(m.tileWidth), float32(m.tileHeight)
	first := max(0, int(math.Floor(float64((area.X-l.offsetX)/tw))))
	last := min(l.columns-1, int(math.Ceil(float64((area.X+area.Width-l.offsetX)/tw)))-1)
	top := max(0, int(math.Floor(float64((area.Y-l.offsetY)/th))))
	bottom := min(l.rows-1, int(math.Ceil(float64((area.Y+area.Height-l.offsetY)/th)))-1)
	var tiles []MapTile
	for row := top; row <= bottom; row++ {
		for column := first; column <= last; column++ {
			gid := l.gids[row*l.columns+column]
			if gid&^tiledFlags == 0 {
				continue
			}
			cell := Rectangle{X: float32(column)*tw + l.offsetX, Y: float32(row)*th + l.offsetY, Width: tw, Height: th}
			if cell.Overlaps(area) {
				tiles = append(tiles, MapTile{Tile: m.tile(gid), Rectangle: cell, Column: column, Row: row})
			}
		}
	}
	return tiles
}

// Objects returns the objects of the object layer named layer, in the order
// Tiled lists them, or the objects of every object layer when layer is "".
func (m *Map) Objects(layer string) []MapObject {
	m.mu.Lock()
	defer m.mu.Unlock()
	var layers []*mapLayer
	if layer == "" {
		if m.prepare() != nil {
			return nil
		}
		layers = m.layers
	} else if l := m.layer(layer, objectLayer); l != nil {
		layers = []*mapLayer{l}
	}
	var objects []MapObject
	for _, l := range layers {
		for _, o := range l.objects {
			objects = append(objects, o.MapObject)
		}
	}
	return objects
}

// Object returns the first object named name, in any object layer, and
// whether there is one: a spawn point, for example.
func (m *Map) Object(name string) (MapObject, bool) {
	for _, o := range m.Objects("") {
		if o.Name == name {
			return o, true
		}
	}
	return MapObject{}, false
}

// tile returns what a global tile ID stands for.
func (m *Map) tile(gid uint32) Tile {
	ts, id := m.tileset(gid)
	if ts == nil {
		return Tile{}
	}
	tile := Tile{Tileset: ts.name, ID: id, Properties: Properties{}}
	if info := ts.tiles[id]; info != nil {
		tile.Class, tile.Properties = info.class, info.properties
	}
	return tile
}

// tileset returns the tileset of a global tile ID, and the tile's ID in it.
func (m *Map) tileset(gid uint32) (*tileset, int) {
	gid &^= tiledFlags
	if gid == 0 {
		return nil, 0
	}
	for i := len(m.tilesets) - 1; i >= 0; i-- {
		if ts := m.tilesets[i]; int(gid) >= ts.firstGID {
			return ts, int(gid) - ts.firstGID
		}
	}
	return nil, 0
}

// layer returns the layer named name, which may start with its groups, such
// as "level 1/ground", and is of kind unless kind is -1. It reports the
// mistake and returns nil when there is no such layer. Call it with m.mu held.
func (m *Map) layer(name string, kind layerKind) *mapLayer {
	if m.prepare() != nil {
		return nil
	}
	for _, match := range []func(*mapLayer) bool{
		func(l *mapLayer) bool { return l.path == name },
		func(l *mapLayer) bool { return l.name == name },
	} {
		for _, l := range m.layers {
			if !match(l) {
				continue
			}
			if kind >= 0 && l.kind != kind {
				reportError(fmt.Errorf("golib.NewMap(%q): layer %q is %s, not %s", m.name, name, layerKindNames[l.kind], layerKindNames[kind]))
				return nil
			}
			return l
		}
	}
	var names []string
	for _, l := range m.layers {
		names = append(names, fmt.Sprintf("%q", l.path))
	}
	reportError(fmt.Errorf("golib.NewMap(%q) has no layer named %q: it has %s", m.name, name, strings.Join(names, ", ")))
	return nil
}

// prepare reads the file the first time the map is used, and reports a
// failure to Run every time it is used. Call it with m.mu held.
func (m *Map) prepare() error {
	if !m.read {
		m.read = true
		if err := m.readFile(tiledFiles{read: ReadAsset, owner: fmt.Sprintf("golib.NewMap(%q)", m.name)}); err != nil {
			m.err = fmt.Errorf("golib.NewMap(%q): %w", m.name, err)
		}
	}
	if m.err != nil {
		reportError(m.err)
	}
	return m.err
}

// readFile reads the map and the files it uses.
func (m *Map) readFile(files tiledFiles) error {
	if !strings.EqualFold(pathExt(m.name), ".tmx") {
		return errors.New("GoLib reads maps from Tiled's .tmx files: save the map as TMX in Tiled")
	}
	var x tmxMap
	if err := files.xml(m.name, &x); err != nil {
		return err
	}
	if x.Orientation != "orthogonal" {
		return fmt.Errorf("the map is %s, but GoLib draws orthogonal maps only: pick Orthogonal under Map > Map Properties in Tiled", x.Orientation)
	}
	if x.TileWidth <= 0 || x.TileHeight <= 0 {
		return fmt.Errorf("the map's grid is %d by %d pixels: fix it under Map > Map Properties in Tiled", x.TileWidth, x.TileHeight)
	}
	m.tileWidth, m.tileHeight = x.TileWidth, x.TileHeight
	m.width, m.height = x.Width*x.TileWidth, x.Height*x.TileHeight
	m.renderOrder = x.RenderOrder
	m.parallaxOriginX, m.parallaxOriginY = x.ParallaxOriginX, x.ParallaxOriginY
	m.background, _ = parseTiledColor(x.BackgroundColor)
	m.properties = x.Properties.properties(nil)

	sources := map[string]*tileset{} // the map's tilesets by .tsx file, for templates
	for _, xt := range x.Tilesets {
		ts, err := files.readTileset(xt, m.name)
		if err != nil {
			return err
		}
		m.tilesets = append(m.tilesets, ts)
		if ts.source != "" {
			sources[ts.source] = ts
		}
	}
	slices.SortStableFunc(m.tilesets, func(a, b *tileset) int { return a.firstGID - b.firstGID })

	reader := layerReader{m: m, files: files, sources: sources, minColumn: math.MaxInt, minRow: math.MaxInt}
	root := groupSettings{visible: true, tint: White, parallaxX: 1, parallaxY: 1}
	if err := reader.read(x.Layers, root, x.Infinite != 0); err != nil {
		return err
	}
	if x.Infinite != 0 {
		reader.fitInfinite()
	}
	return nil
}

// pathExt returns the extension of a slash-separated path.
func pathExt(name string) string {
	if i := strings.LastIndexAny(name, "./"); i >= 0 && name[i] == '.' {
		return name[i:]
	}
	return ""
}

// groupSettings are what the groups around a layer add to it.
type groupSettings struct {
	path                 string
	visible              bool
	tint                 Color
	offsetX, offsetY     float32
	parallaxX, parallaxY float32
	properties           Properties
}

// layerReader turns a map's layers into mapLayers.
type layerReader struct {
	m                 *Map
	files             tiledFiles
	sources           map[string]*tileset
	minColumn, minRow int // of an infinite map's tiles
	maxColumn, maxRow int
}

func (r *layerReader) read(layers []tmxLayer, group groupSettings, infinite bool) error {
	for _, x := range layers {
		kind := x.XMLName.Local
		if kind != "layer" && kind != "objectgroup" && kind != "imagelayer" && kind != "group" {
			continue // such as editorsettings
		}
		settings := groupSettings{
			path:       x.Name,
			visible:    group.visible && (x.Visible == nil || *x.Visible != 0),
			tint:       group.tint,
			offsetX:    group.offsetX + x.OffsetX,
			offsetY:    group.offsetY + x.OffsetY,
			parallaxX:  group.parallaxX,
			parallaxY:  group.parallaxY,
			properties: x.Properties.properties(group.properties),
		}
		if group.path != "" {
			settings.path = group.path + "/" + x.Name
		}
		if tint, ok := parseTiledColor(x.TintColor); ok {
			settings.tint = Color{
				R: uint8(int(settings.tint.R) * int(tint.R) / 255),
				G: uint8(int(settings.tint.G) * int(tint.G) / 255),
				B: uint8(int(settings.tint.B) * int(tint.B) / 255),
				A: uint8(int(settings.tint.A) * int(tint.A) / 255),
			}
		}
		if x.Opacity != nil {
			settings.tint = colorWithOpacity(settings.tint, *x.Opacity)
		}
		if x.ParallaxX != nil {
			settings.parallaxX *= *x.ParallaxX
		}
		if x.ParallaxY != nil {
			settings.parallaxY *= *x.ParallaxY
		}
		if kind == "group" {
			if err := r.read(x.Layers, settings, infinite); err != nil {
				return err
			}
			continue
		}
		layer := &mapLayer{
			name:       x.Name,
			path:       settings.path,
			visible:    settings.visible,
			tint:       settings.tint,
			offsetX:    settings.offsetX,
			offsetY:    settings.offsetY,
			parallaxX:  settings.parallaxX,
			parallaxY:  settings.parallaxY,
			properties: settings.properties,
		}
		var err error
		switch kind {
		case "layer":
			layer.kind = tileLayer
			err = r.readTiles(layer, x, infinite)
		case "objectgroup":
			layer.kind = objectLayer
			layer.byIndex = x.DrawOrder == "index"
			err = r.readObjects(layer, x)
		case "imagelayer":
			layer.kind = imageLayer
			err = r.readImage(layer, x)
		}
		if err != nil {
			return fmt.Errorf("layer %q: %w", settings.path, err)
		}
		r.m.layers = append(r.m.layers, layer)
	}
	return nil
}

func (r *layerReader) readTiles(layer *mapLayer, x tmxLayer, infinite bool) error {
	if x.Data == nil {
		return errors.New("it has no tile data: save the map again from Tiled")
	}
	if !infinite {
		gids, err := decodeTileData(x.Data.Encoding, x.Data.Compression, x.Data.Text, x.Data.Tiles, x.Width*x.Height)
		layer.columns, layer.rows, layer.gids = x.Width, x.Height, gids
		return err
	}
	// An infinite map keeps its tiles in chunks, at any column and row.
	// They go into one grid once the map's extent is known.
	type chunk struct {
		x, y, width, height int
		gids                []uint32
	}
	var chunks []chunk
	for _, c := range x.Data.Chunks {
		gids, err := decodeTileData(x.Data.Encoding, x.Data.Compression, c.Text, c.Tiles, c.Width*c.Height)
		if err != nil {
			return err
		}
		chunks = append(chunks, chunk{c.X, c.Y, c.Width, c.Height, gids})
		for i, gid := range gids {
			if gid != 0 {
				column, row := c.X+i%c.Width, c.Y+i/c.Width
				r.minColumn, r.minRow = min(r.minColumn, column), min(r.minRow, row)
				r.maxColumn, r.maxRow = max(r.maxColumn, column), max(r.maxRow, row)
			}
		}
	}
	layer.chunks = func(minColumn, minRow, columns, rows int) []uint32 {
		grid := make([]uint32, columns*rows)
		for _, c := range chunks {
			for i, gid := range c.gids {
				column, row := c.x+i%c.width-minColumn, c.y+i/c.width-minRow
				if gid != 0 && column >= 0 && row >= 0 && column < columns && row < rows {
					grid[row*columns+column] = gid
				}
			}
		}
		return grid
	}
	return nil
}

// fitInfinite puts the tiles of an infinite map into grids that start at its
// top-left tile, and moves everything else by as much.
func (r *layerReader) fitInfinite() {
	m := r.m
	if r.minColumn > r.maxColumn {
		r.minColumn, r.minRow, r.maxColumn, r.maxRow = 0, 0, -1, -1
	}
	columns, rows := r.maxColumn-r.minColumn+1, r.maxRow-r.minRow+1
	shiftX, shiftY := float32(-r.minColumn*m.tileWidth), float32(-r.minRow*m.tileHeight)
	m.width, m.height = columns*m.tileWidth, rows*m.tileHeight
	for _, l := range m.layers {
		switch l.kind {
		case tileLayer:
			l.columns, l.rows = columns, rows
			l.gids = l.chunks(r.minColumn, r.minRow, columns, rows)
			l.chunks = nil
		case objectLayer:
			for i := range l.objects {
				o := &l.objects[i]
				o.X += shiftX
				o.Y += shiftY
				o.originX += shiftX
				o.originY += shiftY
			}
		case imageLayer:
			l.offsetX += shiftX
			l.offsetY += shiftY
		}
	}
}

func (r *layerReader) readObjects(layer *mapLayer, x tmxLayer) error {
	for _, xo := range x.Objects {
		if xo.Template != "" {
			merged, err := r.applyTemplate(xo)
			if err != nil {
				return err
			}
			xo = merged
		}
		o := mapObject{MapObject: MapObject{
			ID:         xo.ID,
			Layer:      layer.name,
			Shape:      "rectangle",
			Visible:    xo.Visible == nil || *xo.Visible != 0,
			Properties: xo.Properties.properties(nil),
		}}
		o.X, o.Y = xo.X, xo.Y
		o.originX, o.originY = xo.X, xo.Y
		if xo.Name != nil {
			o.Name = *xo.Name
		}
		if xo.Class != nil {
			o.Class = *xo.Class
		} else if xo.Type != nil {
			o.Class = *xo.Type
		}
		if xo.Width != nil {
			o.Width = *xo.Width
		}
		if xo.Height != nil {
			o.Height = *xo.Height
		}
		if xo.Rotation != nil {
			o.Rotation = *xo.Rotation
		}
		var err error
		switch {
		case xo.GID != nil:
			o.Shape = "tile"
			o.gid = *xo.GID
			o.Tile = r.m.tile(o.gid)
			ts, id := r.m.tileset(o.gid)
			if ts == nil {
				return fmt.Errorf("object %d shows tile %d, which no tileset of the map has: fix it in Tiled", o.ID, o.gid&^tiledFlags)
			}
			// A tile object takes its tile's size unless it says otherwise,
			// and its tileset says where its origin is.
			tw, th := ts.size(id)
			if xo.Width == nil {
				o.Width = float32(tw)
			}
			if xo.Height == nil {
				o.Height = float32(th)
			}
			o.alignX, o.alignY = alignment(ts.alignment)
			// Tile properties come first; the object's own replace them.
			o.Properties = xo.Properties.properties(o.Tile.Properties)
			if o.Class == "" {
				o.Class = o.Tile.Class
			}
		case xo.Ellipse != nil:
			o.Shape = "ellipse"
		case xo.Point != nil:
			o.Shape = "point"
		case xo.Polygon != nil:
			o.Shape = "polygon"
			o.Points, err = parsePoints(xo.Polygon.Points)
		case xo.Polyline != nil:
			o.Shape = "polyline"
			o.Points, err = parsePoints(xo.Polyline.Points)
		case xo.Text != nil:
			o.Shape = "text"
			o.Text = xo.Text.Text
		}
		if err != nil {
			return err
		}
		o.X -= o.alignX * o.Width
		o.Y -= o.alignY * o.Height
		layer.objects = append(layer.objects, o)
	}
	return nil
}

// alignment returns where a tileset's tile objects have their origin, as
// parts of their width and height. Orthogonal maps put it at the bottom-left
// corner unless the tileset says otherwise.
func alignment(name string) (float32, float32) {
	switch name {
	case "topleft":
		return 0, 0
	case "top":
		return 0.5, 0
	case "topright":
		return 1, 0
	case "left":
		return 0, 0.5
	case "center":
		return 0.5, 0.5
	case "right":
		return 1, 0.5
	case "bottom":
		return 0.5, 1
	case "bottomright":
		return 1, 1
	default:
		return 0, 1
	}
}

// applyTemplate returns the object with what its template says filled in.
func (r *layerReader) applyTemplate(xo tmxObject) (tmxObject, error) {
	name, err := relative(r.m.name, xo.Template)
	if err != nil {
		return xo, err
	}
	var t tmxTemplate
	if err := r.files.xml(name, &t); err != nil {
		return xo, err
	}
	merged := t.Object
	merged.ID, merged.X, merged.Y, merged.Template = xo.ID, xo.X, xo.Y, ""
	if xo.Name != nil {
		merged.Name = xo.Name
	}
	if xo.Type != nil {
		merged.Type = xo.Type
	}
	if xo.Class != nil {
		merged.Class = xo.Class
	}
	if xo.Width != nil {
		merged.Width = xo.Width
	}
	if xo.Height != nil {
		merged.Height = xo.Height
	}
	if xo.Rotation != nil {
		merged.Rotation = xo.Rotation
	}
	if xo.Visible != nil {
		merged.Visible = xo.Visible
	}
	// The object's properties come after the template's, so they win.
	merged.Properties.Properties = append(slices.Clone(t.Object.Properties.Properties), xo.Properties.Properties...)
	gid := merged.GID
	if xo.GID != nil {
		gid = xo.GID
	} else if gid != nil {
		// The template's tile is counted in the template's tileset: find
		// the same tileset in the map.
		if t.Tileset == nil || t.Tileset.Source == "" {
			return xo, fmt.Errorf("template %s has a tile but no .tsx tileset: GoLib can't find its tile", name)
		}
		source, err := relative(name, t.Tileset.Source)
		if err != nil {
			return xo, err
		}
		ts := r.sources[source]
		if ts == nil {
			return xo, fmt.Errorf("template %s uses the tileset %s, which the map doesn't: add that tileset to the map in Tiled", name, source)
		}
		shifted := (*gid&^tiledFlags - uint32(t.Tileset.FirstGID) + uint32(ts.firstGID)) | *gid&tiledFlags
		gid = &shifted
	}
	merged.GID = gid
	if xo.Ellipse != nil || xo.Point != nil || xo.Polygon != nil || xo.Polyline != nil || xo.Text != nil {
		merged.Ellipse, merged.Point, merged.Polygon, merged.Polyline, merged.Text = xo.Ellipse, xo.Point, xo.Polygon, xo.Polyline, xo.Text
	}
	return merged, nil
}

func (r *layerReader) readImage(layer *mapLayer, x tmxLayer) error {
	layer.repeatX, layer.repeatY = x.RepeatX != 0, x.RepeatY != 0
	if x.Image == nil || x.Image.Source == "" {
		return nil // an image layer without an image draws nothing
	}
	name, err := relative(r.m.name, x.Image.Source)
	if err != nil {
		return err
	}
	trans := x.Image.Trans
	files := r.files
	layer.image = newGeneratedSprite(fmt.Sprintf("%s, image layer %q", r.files.owner, layer.path), func(s *Sprite) error {
		pixels, err := readTiledImage(files, name, trans)
		if err != nil {
			return err
		}
		s.pixels, s.width, s.height = pixels, pixels.Rect.Dx(), pixels.Rect.Dy()
		s.frames = []image.Rectangle{pixels.Rect}
		return nil
	})
	return nil
}

// DrawMap draws every visible layer of level with the map's top-left corner
// at x, y, starting with its background color, if Tiled gives it one. To
// follow a camera, pass the camera's position, negated:
//
//	screen.DrawMap(level, -camera.X, -camera.Y)
//
// Tile layers, image layers and tile objects are drawn; other objects aren't,
// because games use them for places and areas. Animated tiles follow the
// game's time, and layers move at their parallax factor. Only the tiles on the
// screen are drawn, so large maps are fine. Layers and objects are drawn at
// whole pixels, rounding x and y, so that tiles leave no gaps between them.
func (s *Screen) DrawMap(level *Map, x, y float32) {
	s.drawMap(level, nil, x, y)
}

// DrawMapLayer draws the layer named layer of level, as DrawMap draws it,
// even if it is hidden in Tiled. Draw a map one layer at a time to put
// sprites between its layers, such as a player behind the leaves of a tree.
func (s *Screen) DrawMapLayer(level *Map, layer string, x, y float32) {
	s.drawMap(level, &layer, x, y)
}

func (s *Screen) drawMap(level *Map, only *string, x, y float32) {
	if level == nil {
		reportError(errors.New("golib: Screen.DrawMap got a nil map: make maps with golib.NewMap"))
		return
	}
	level.mu.Lock()
	var layers []*mapLayer
	if only != nil {
		if l := level.layer(*only, -1); l != nil {
			layers = []*mapLayer{l}
		}
	} else if level.prepare() == nil {
		if level.background.A > 0 {
			rl.DrawRectangleRec(rl.Rectangle{X: x, Y: y, Width: float32(level.width), Height: float32(level.height)}, level.background)
		}
		for _, l := range level.layers {
			if l.visible {
				layers = append(layers, l)
			}
		}
	}
	level.mu.Unlock()
	for _, l := range layers {
		// A layer with a parallax factor other than 1 moves slower or
		// faster than the camera, as Tiled shows it.
		centerX, centerY := s.width/2-x, s.height/2-y
		lx := wholePixel(x + l.offsetX + (1-l.parallaxX)*(centerX-level.parallaxOriginX))
		ly := wholePixel(y + l.offsetY + (1-l.parallaxY)*(centerY-level.parallaxOriginY))
		switch l.kind {
		case tileLayer:
			s.drawTiles(level, l, lx, ly)
		case objectLayer:
			s.drawTileObjects(level, l, lx, ly)
		case imageLayer:
			s.drawImageLayer(l, lx, ly)
		}
	}
}

// drawTiles draws the tiles of l on the screen, with the layer's top-left
// corner at x, y.
func (s *Screen) drawTiles(level *Map, l *mapLayer, x, y float32) {
	tw, th := float32(level.tileWidth), float32(level.tileHeight)
	// Tiles can be larger than the grid, and stick out up and to the right.
	var reachX, reachY float32
	for _, ts := range level.tilesets {
		reachX = max(reachX, float32(ts.tileWidth)-tw+float32(max(ts.offsetX, -ts.offsetX)))
		reachY = max(reachY, float32(ts.tileHeight)-th+float32(max(ts.offsetY, -ts.offsetY)))
	}
	firstColumn := max(0, int(math.Floor(float64((-x-reachX)/tw))))
	lastColumn := min(l.columns-1, int(math.Floor(float64((s.width-x+reachX)/tw))))
	firstRow := max(0, int(math.Floor(float64((-y-reachY)/th))))
	lastRow := min(l.rows-1, int(math.Floor(float64((s.height-y+reachY)/th))))

	columnStep, rowStep := 1, 1
	startColumn, startRow := firstColumn, firstRow
	if strings.HasPrefix(level.renderOrder, "left") {
		columnStep, startColumn = -1, lastColumn
	}
	if strings.HasSuffix(level.renderOrder, "up") {
		rowStep, startRow = -1, lastRow
	}
	for row := startRow; row >= firstRow && row <= lastRow; row += rowStep {
		for column := startColumn; column >= firstColumn && column <= lastColumn; column += columnStep {
			gid := l.gids[row*l.columns+column]
			if gid&^tiledFlags == 0 {
				continue
			}
			ts, id := level.tileset(gid)
			if ts == nil {
				continue
			}
			w, h := ts.size(id)
			// A tile sits on the bottom-left corner of its cell.
			left := x + float32(column)*tw + float32(ts.offsetX)
			top := y + float32(row+1)*th - float32(h) + float32(ts.offsetY)
			s.drawTile(ts, id, gid, rl.Rectangle{X: left, Y: top, Width: float32(w), Height: float32(h)}, l.tint)
		}
	}
}

// drawTile draws tile id of ts into dest, flipped as gid says.
func (s *Screen) drawTile(ts *tileset, id int, gid uint32, dest rl.Rectangle, tint Color) {
	texture, place, ok := ts.texture(id, s.time)
	if !ok {
		return
	}
	// Tiled flips the anti-diagonal first, then left to right, then top to
	// bottom. raylib flips the source, then rotates: the same, in these
	// combinations.
	rotation, flipX, flipY := float32(0), gid&tiledFlipX != 0, gid&tiledFlipY != 0
	if gid&tiledFlipXY != 0 {
		switch {
		case !flipX && !flipY:
			rotation, flipX, flipY = 90, false, true
		case flipX && !flipY:
			rotation, flipX, flipY = 90, false, false
		case !flipX && flipY:
			rotation, flipX, flipY = 270, false, false
		default:
			rotation, flipX, flipY = 90, true, false
		}
	}
	drawTexturePart(texture, place, dest, rotation, flipX, flipY, tint)
}

// drawTileObjects draws the visible tile objects of l, with the layer's
// top-left corner at x, y.
func (s *Screen) drawTileObjects(level *Map, l *mapLayer, x, y float32) {
	objects := l.objects
	if !l.byIndex {
		// Tiled draws the objects higher up first, unless the layer says
		// otherwise.
		objects = slices.Clone(objects)
		slices.SortStableFunc(objects, func(a, b mapObject) int { return cmp.Compare(a.originY, b.originY) })
	}
	for _, o := range objects {
		if o.gid == 0 || !o.Visible {
			continue
		}
		ts, id := level.tileset(o.gid)
		if ts == nil {
			continue
		}
		texture, place, ok := ts.texture(id, s.time)
		if !ok {
			continue
		}
		dest := rl.Rectangle{X: x + wholePixel(o.originX), Y: y + wholePixel(o.originY), Width: o.Width, Height: o.Height}
		origin := rl.Vector2{X: o.alignX * o.Width, Y: o.alignY * o.Height}
		source := rl.Rectangle{X: float32(place.Min.X), Y: float32(place.Min.Y), Width: float32(place.Dx()), Height: float32(place.Dy())}
		if o.gid&tiledFlipX != 0 {
			source.Width = -source.Width
		}
		if o.gid&tiledFlipY != 0 {
			source.Height = -source.Height
		}
		rl.DrawTexturePro(texture, source, dest, origin, o.Rotation, l.tint)
	}
}

// drawImageLayer draws l's image with its top-left corner at x, y, over and
// over across the screen if the layer repeats.
func (s *Screen) drawImageLayer(l *mapLayer, x, y float32) {
	if l.image == nil {
		return
	}
	texture, place, err := l.image.frameTexture(0)
	if err != nil {
		reportError(err)
		return
	}
	w, h := float32(place.Dx()), float32(place.Dy())
	if w <= 0 || h <= 0 {
		return
	}
	firstX, lastX, firstY, lastY := x, x, y, y
	if l.repeatX {
		firstX = x - w*float32(math.Ceil(float64(x/w)))
		lastX = s.width
	}
	if l.repeatY {
		firstY = y - h*float32(math.Ceil(float64(y/h)))
		lastY = s.height
	}
	for top := firstY; top <= lastY; top += h {
		for left := firstX; left <= lastX; left += w {
			drawTexturePart(texture, place, rl.Rectangle{X: left, Y: top, Width: w, Height: h}, 0, false, false, l.tint)
		}
	}
}

// wholePixel rounds v to the nearest whole pixel, halves up, as Tiled does.
// Pictures drawn half a pixel off repeat some pixels and skip others, and
// tiles drawn between pixels leave gaps between them.
func wholePixel(v float32) float32 {
	return float32(math.Floor(float64(v) + 0.5))
}

// drawTexturePart draws the part of texture at place into dest, rotated
// clockwise by rotation degrees around its middle, and flipped.
func drawTexturePart(texture rl.Texture2D, place image.Rectangle, dest rl.Rectangle, rotation float32, flipX, flipY bool, tint Color) {
	source := rl.Rectangle{X: float32(place.Min.X), Y: float32(place.Min.Y), Width: float32(place.Dx()), Height: float32(place.Dy())}
	if flipX {
		source.Width = -source.Width
	}
	if flipY {
		source.Height = -source.Height
	}
	var origin rl.Vector2
	if rotation != 0 {
		origin = rl.Vector2{X: dest.Width / 2, Y: dest.Height / 2}
		dest.X += origin.X
		dest.Y += origin.Y
	}
	rl.DrawTexturePro(texture, source, dest, origin, rotation, tint)
}
