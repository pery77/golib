package golib

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// readMap reads the map named name, failing the test if that stops Run.
func readMap(t *testing.T, name string) *Map {
	t.Helper()
	m := NewMap(name)
	m.Width()
	if err := takeError(); err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return m
}

// mapFile is a map of columns by rows tiles of 2 by 2 pixels, with one tile
// layer, "ground", whose data element is data, and the tileset t.png.
func mapFile(columns, rows int, data string) []byte {
	return fmt.Appendf(nil, `<?xml version="1.0" encoding="UTF-8"?>
<map version="1.10" orientation="orthogonal" renderorder="right-down" width="%[1]d" height="%[2]d" tilewidth="2" tileheight="2" infinite="0">
 <tileset firstgid="1" name="t" tilewidth="2" tileheight="2" tilecount="4" columns="2">
  <image source="t.png" width="4" height="4"/>
 </tileset>
 <layer id="1" name="ground" width="%[1]d" height="%[2]d">
  %[3]s
 </layer>
</map>`, columns, rows, data)
}

const levelMap = `<?xml version="1.0" encoding="UTF-8"?>
<map version="1.10" tiledversion="1.11.2" orientation="orthogonal" renderorder="right-down" width="4" height="3" tilewidth="2" tileheight="2" infinite="0" backgroundcolor="#102030" nextlayerid="3" nextobjectid="1">
 <properties>
  <property name="title" value="Cave"/>
  <property name="gravity" type="float" value="9.5"/>
 </properties>
 <tileset firstgid="1" source="../tiles/terrain.tsx"/>
 <tileset firstgid="9" name="extra" tilewidth="2" tileheight="2" tilecount="2" columns="2">
  <image source="../tiles/extra.png" width="4" height="2"/>
 </tileset>
 <layer id="1" name="ground" width="4" height="3">
  <properties>
   <property name="solid" type="bool" value="true"/>
  </properties>
  <data encoding="csv">
0,1,2,3,
4,5,6,7,
8,9,10,2147483650
</data>
 </layer>
</map>
`

const terrainTileset = `<?xml version="1.0" encoding="UTF-8"?>
<tileset version="1.10" name="terrain" tilewidth="2" tileheight="2" tilecount="8" columns="4">
 <image source="terrain.png" width="8" height="4"/>
 <tile id="1" type="wall">
  <properties>
   <property name="solid" type="bool" value="true"/>
   <property name="hp" type="int" value="3"/>
  </properties>
 </tile>
 <tile id="4" class="water"/>
</tileset>
`

func TestMapTiles(t *testing.T) {
	useAssets(t, map[string][]byte{
		"maps/level.tmx":    []byte(levelMap),
		"tiles/terrain.tsx": []byte(terrainTileset),
		"tiles/terrain.png": pngFile(t, 8, 4),
		"tiles/extra.png":   pngFile(t, 4, 2),
	})
	level := readMap(t, "maps/level.tmx")
	if level.Width() != 8 || level.Height() != 6 || level.TileWidth() != 2 || level.TileHeight() != 2 {
		t.Errorf("map is %v by %v with tiles of %v by %v, want 8 by 6 with 2 by 2", level.Width(), level.Height(), level.TileWidth(), level.TileHeight())
	}
	if level.background != (Color{R: 0x10, G: 0x20, B: 0x30, A: 255}) {
		t.Errorf("background = %v", level.background)
	}
	if props := level.Properties(); props.String("title") != "Cave" || props.Float("gravity") != 9.5 {
		t.Errorf("map properties = %v", props)
	}
	if !level.LayerProperties("ground").Bool("solid") {
		t.Errorf("layer properties = %v", level.LayerProperties("ground"))
	}

	wall := Tile{Tileset: "terrain", ID: 1, Class: "wall", Properties: Properties{"solid": "true", "hp": "3"}}
	tests := []struct {
		column, row int
		want        Tile
	}{
		{0, 0, Tile{}},
		{1, 0, Tile{Tileset: "terrain", ID: 0, Properties: Properties{}}},
		{2, 0, wall},
		{1, 1, Tile{Tileset: "terrain", ID: 4, Class: "water", Properties: Properties{}}},
		{0, 2, Tile{Tileset: "terrain", ID: 7, Properties: Properties{}}},
		{1, 2, Tile{Tileset: "extra", ID: 0, Properties: Properties{}}},
		{2, 2, Tile{Tileset: "extra", ID: 1, Properties: Properties{}}},
		{3, 2, wall}, // flipped
		{-1, 0, Tile{}},
		{4, 0, Tile{}},
		{0, 3, Tile{}},
	}
	for _, test := range tests {
		if got := level.Tile("ground", test.column, test.row); !reflect.DeepEqual(got, test.want) {
			t.Errorf("Tile(%d, %d) = %+v, want %+v", test.column, test.row, got, test.want)
		}
	}
	if !level.Tile("ground", 0, 0).Empty() || level.Tile("ground", 2, 0).Empty() {
		t.Error("Empty is wrong")
	}
	if got := level.Tile("ground", 2, 0).Properties.Int("hp"); got != 3 {
		t.Errorf("hp = %d, want 3", got)
	}
	if got := level.TileAt("ground", 5.9, 1.5); !reflect.DeepEqual(got, wall) {
		t.Errorf("TileAt(5.9, 1.5) = %+v, want the wall", got)
	}
	if got := level.TileAt("ground", -0.1, 0); !got.Empty() {
		t.Errorf("TileAt(-0.1, 0) = %+v, want no tile", got)
	}

	var cells []MapTile
	for _, tile := range level.TilesIn("ground", Rectangle{X: 1, Y: 1, Width: 2, Height: 2}) {
		cells = append(cells, MapTile{Rectangle: tile.Rectangle, Column: tile.Column, Row: tile.Row})
	}
	wantCells := []MapTile{
		{Rectangle: Rectangle{X: 2, Y: 0, Width: 2, Height: 2}, Column: 1, Row: 0},
		{Rectangle: Rectangle{X: 0, Y: 2, Width: 2, Height: 2}, Column: 0, Row: 1},
		{Rectangle: Rectangle{X: 2, Y: 2, Width: 2, Height: 2}, Column: 1, Row: 1},
	}
	if !reflect.DeepEqual(cells, wantCells) {
		t.Errorf("TilesIn = %+v, want %+v", cells, wantCells)
	}
	// Touching a cell's edge isn't overlapping it.
	if tiles := level.TilesIn("ground", Rectangle{X: 2, Y: 0, Width: 2, Height: 2}); len(tiles) != 1 || tiles[0].Column != 1 {
		t.Errorf("TilesIn at the edges = %+v, want tile 1, 0 only", tiles)
	}
	if tiles := level.TilesIn("ground", Rectangle{X: -10, Y: -10, Width: 5, Height: 5}); len(tiles) != 0 {
		t.Errorf("TilesIn outside = %+v", tiles)
	}

	if err := takeError(); err != nil {
		t.Errorf("reported error: %v", err)
	}
	terrain := level.tilesets[0]
	if terrain.source != "tiles/terrain.tsx" || terrain.firstGID != 1 || level.tilesets[1].firstGID != 9 {
		t.Errorf("tilesets: %q from %d, then from %d", terrain.source, terrain.firstGID, level.tilesets[1].firstGID)
	}
	if n, err := terrain.sprite.frameCount(); n != 8 || err != nil {
		t.Errorf("terrain has %d frames (%v), want 8", n, err)
	}
	if got := terrain.sprite.frames[5]; got != image.Rect(2, 2, 4, 4) {
		t.Errorf("terrain tile 5 is at %v", got)
	}
}

func TestMapTileData(t *testing.T) {
	want := []uint32{1, 0, 2147483649, 3}
	raw := make([]byte, 16)
	for i, gid := range want {
		binary.LittleEndian.PutUint32(raw[4*i:], gid)
	}
	var zlibData, gzipData bytes.Buffer
	zw := zlib.NewWriter(&zlibData)
	zw.Write(raw)
	zw.Close()
	gw := gzip.NewWriter(&gzipData)
	gw.Write(raw)
	gw.Close()
	encode := base64.StdEncoding.EncodeToString

	tests := map[string]string{
		"csv": `<data encoding="csv">1,0,
2147483649,3</data>`,
		"xml":    `<data><tile gid="1"/><tile/><tile gid="2147483649"/><tile gid="3"/></data>`,
		"base64": `<data encoding="base64">` + encode(raw) + `</data>`,
		"zlib":   `<data encoding="base64" compression="zlib">` + "\n   " + encode(zlibData.Bytes()) + "\n  " + `</data>`,
		"gzip":   `<data encoding="base64" compression="gzip">` + encode(gzipData.Bytes()) + `</data>`,
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			useAssets(t, map[string][]byte{"map.tmx": mapFile(2, 2, data), "t.png": pngFile(t, 4, 4)})
			level := readMap(t, "map.tmx")
			if got := level.layers[0].gids; !slices.Equal(got, want) {
				t.Errorf("tiles = %v, want %v", got, want)
			}
		})
	}

	failures := map[string]string{
		`<data encoding="base64" compression="zstd">KLUv/QBYAAA=</data>`: "the tile data is compressed with Zstandard, which GoLib can't read: in Tiled, pick another Tile Layer Format",
		`<data encoding="csv">1,2,3</data>`:                              "the tile data has 3 tiles, but the layer is 4 tiles",
		`<data encoding="csv">1,x,3,4</data>`:                            `the tile data has "x", which isn't a tile ID`,
		`<data encoding="base64">AAAA</data>`:                            "the tile data is cut short",
		`<data encoding="base64">!!</data>`:                              "the tile data isn't valid base64",
		`<data encoding="base64" compression="zlib">AAAA</data>`:         "the tile data isn't valid zlib",
		`<data encoding="hex">00</data>`:                                 `the tile data is encoded as "hex", which GoLib doesn't know`,
		``:                                                               "it has no tile data",
	}
	for data, want := range failures {
		useAssets(t, map[string][]byte{"map.tmx": mapFile(2, 2, data), "t.png": pngFile(t, 4, 4)})
		level := NewMap("map.tmx")
		if level.Width() != 0 {
			t.Errorf("%s: a map that can't be read has a width", data)
		}
		wantError(t, `golib.NewMap("map.tmx"): layer "ground": `+want)
	}
}

const objectsMap = `<?xml version="1.0" encoding="UTF-8"?>
<map version="1.10" orientation="orthogonal" width="10" height="10" tilewidth="16" tileheight="16">
 <tileset firstgid="1" name="props" tilewidth="24" tileheight="32" tilecount="2" columns="0">
  <grid orientation="orthogonal" width="1" height="1"/>
  <tile id="0" class="chest">
   <properties>
    <property name="gold" type="int" value="5"/>
    <property name="locked" type="bool" value="true"/>
   </properties>
   <image source="chest.png" width="24" height="32"/>
  </tile>
  <tile id="3">
   <image source="tree.png" width="10" height="20"/>
  </tile>
 </tileset>
 <objectgroup id="2" name="things">
  <object id="1" name="spawn" type="start" x="10" y="20">
   <point/>
  </object>
  <object id="2" name="zone" class="water" x="5" y="6" width="30" height="40" rotation="45">
   <properties>
    <property name="speed" type="float" value="0.5"/>
    <property name="door" type="class" propertytype="Door">
     <properties>
      <property name="locked" type="bool" value="true"/>
     </properties>
    </property>
    <property name="note">two
lines</property>
   </properties>
  </object>
  <object id="3" x="1" y="2" width="3" height="4">
   <ellipse/>
  </object>
  <object id="4" name="path" x="100" y="50">
   <polyline points="0,0 10,5 -3,8.5"/>
  </object>
  <object id="5" x="0" y="0">
   <polygon points="0,0 4,0 4,4"/>
  </object>
  <object id="6" name="sign" x="7" y="8" width="50" height="20">
   <text wrap="1">Hello
there</text>
  </object>
  <object id="7" name="chest" gid="1" x="40" y="100" width="24" height="32">
   <properties>
    <property name="gold" type="int" value="9"/>
   </properties>
  </object>
  <object id="8" gid="2147483652" x="60" y="80" visible="0"/>
 </objectgroup>
</map>
`

func TestMapObjects(t *testing.T) {
	useAssets(t, map[string][]byte{"objects.tmx": []byte(objectsMap), "chest.png": pngFile(t, 24, 32), "tree.png": pngFile(t, 10, 20)})
	level := readMap(t, "objects.tmx")
	chest := Tile{Tileset: "props", ID: 0, Class: "chest", Properties: Properties{"gold": "5", "locked": "true"}}
	want := []MapObject{
		{Rectangle: Rectangle{X: 10, Y: 20}, ID: 1, Name: "spawn", Class: "start", Layer: "things", Shape: "point", Visible: true, Properties: Properties{}},
		{Rectangle: Rectangle{X: 5, Y: 6, Width: 30, Height: 40}, ID: 2, Name: "zone", Class: "water", Layer: "things", Shape: "rectangle", Rotation: 45, Visible: true,
			Properties: Properties{"speed": "0.5", "door.locked": "true", "note": "two\nlines"}},
		{Rectangle: Rectangle{X: 1, Y: 2, Width: 3, Height: 4}, ID: 3, Layer: "things", Shape: "ellipse", Visible: true, Properties: Properties{}},
		{Rectangle: Rectangle{X: 100, Y: 50}, ID: 4, Name: "path", Layer: "things", Shape: "polyline", Visible: true, Properties: Properties{},
			Points: []Vector2{{0, 0}, {10, 5}, {-3, 8.5}}},
		{ID: 5, Layer: "things", Shape: "polygon", Visible: true, Properties: Properties{}, Points: []Vector2{{0, 0}, {4, 0}, {4, 4}}},
		{Rectangle: Rectangle{X: 7, Y: 8, Width: 50, Height: 20}, ID: 6, Name: "sign", Layer: "things", Shape: "text", Text: "Hello\nthere", Visible: true, Properties: Properties{}},
		// Tile objects move their origin from the bottom-left corner to the
		// top-left one, and take their tile's class, properties and size.
		{Rectangle: Rectangle{X: 40, Y: 68, Width: 24, Height: 32}, ID: 7, Name: "chest", Class: "chest", Layer: "things", Shape: "tile", Tile: chest, Visible: true,
			Properties: Properties{"gold": "9", "locked": "true"}},
		{Rectangle: Rectangle{X: 60, Y: 60, Width: 10, Height: 20}, ID: 8, Layer: "things", Shape: "tile",
			Tile: Tile{Tileset: "props", ID: 3, Properties: Properties{}}, Properties: Properties{}},
	}
	got := level.Objects("things")
	if len(got) != len(want) {
		t.Fatalf("got %d objects, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if !reflect.DeepEqual(got[i], want[i]) {
			t.Errorf("object %d:\n got %+v\nwant %+v", i, got[i], want[i])
		}
	}
	if all := level.Objects(""); len(all) != len(want) {
		t.Errorf("Objects(\"\") has %d objects, want %d", len(all), len(want))
	}
	if o, found := level.Object("chest"); !found || o.ID != 7 {
		t.Errorf("Object(\"chest\") = %+v, %v", o, found)
	}
	if o, found := level.Object("dragon"); found {
		t.Errorf("Object(\"dragon\") = %+v", o)
	}
	if got := level.Objects("things")[1].Properties; got.Float("speed") != 0.5 || !got.Bool("door.locked") {
		t.Errorf("zone properties = %v", got)
	}
	if err := takeError(); err != nil {
		t.Errorf("reported error: %v", err)
	}

	// A collection of images packs them into one picture.
	props := level.tilesets[0]
	if n, err := props.sprite.frameCount(); n != 2 || err != nil {
		t.Fatalf("props has %d frames (%v), want 2", n, err)
	}
	if props.frameOf[0] != 0 || props.frameOf[3] != 1 || props.sprite.frames[1] != image.Rect(25, 0, 35, 20) {
		t.Errorf("frames %v at %v", props.frameOf, props.sprite.frames)
	}
	if w, h := props.size(3); w != 10 || h != 20 {
		t.Errorf("tile 3 is %d by %d, want 10 by 20", w, h)
	}
	if _, found := props.frame(1, 0); found {
		t.Error("props has a tile 1")
	}
}

func TestMapTemplates(t *testing.T) {
	useAssets(t, map[string][]byte{
		"maps/level.tmx": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<map version="1.10" orientation="orthogonal" width="4" height="4" tilewidth="2" tileheight="2">
 <tileset firstgid="1" name="extra" tilewidth="2" tileheight="2" tilecount="2" columns="2">
  <image source="../tiles/extra.png" width="4" height="2"/>
 </tileset>
 <tileset firstgid="3" source="../tiles/terrain.tsx"/>
 <objectgroup id="1" name="crates">
  <object id="1" template="templates/crate.tx" x="4" y="6">
   <properties>
    <property name="weight" type="int" value="20"/>
   </properties>
  </object>
  <object id="2" template="templates/crate.tx" name="big" x="0" y="10" width="4" height="4"/>
  <object id="3" template="templates/zone.tx" x="1" y="1"/>
 </objectgroup>
</map>
`),
		"maps/templates/crate.tx": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<template>
 <tileset firstgid="1" source="../../tiles/terrain.tsx"/>
 <object name="crate" type="box" gid="3" width="2" height="2">
  <properties>
   <property name="weight" type="int" value="10"/>
   <property name="fragile" type="bool" value="true"/>
  </properties>
 </object>
</template>
`),
		"maps/templates/zone.tx": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<template>
 <object name="zone" width="8" height="6">
  <ellipse/>
 </object>
</template>
`),
		"tiles/terrain.tsx": []byte(terrainTileset),
		"tiles/terrain.png": pngFile(t, 8, 4),
		"tiles/extra.png":   pngFile(t, 4, 2),
	})
	level := readMap(t, "maps/level.tmx")
	crate := Tile{Tileset: "terrain", ID: 2, Properties: Properties{}}
	want := []MapObject{
		{Rectangle: Rectangle{X: 4, Y: 4, Width: 2, Height: 2}, ID: 1, Name: "crate", Class: "box", Layer: "crates", Shape: "tile", Tile: crate, Visible: true,
			Properties: Properties{"weight": "20", "fragile": "true"}},
		{Rectangle: Rectangle{X: 0, Y: 6, Width: 4, Height: 4}, ID: 2, Name: "big", Class: "box", Layer: "crates", Shape: "tile", Tile: crate, Visible: true,
			Properties: Properties{"weight": "10", "fragile": "true"}},
		{Rectangle: Rectangle{X: 1, Y: 1, Width: 8, Height: 6}, ID: 3, Name: "zone", Layer: "crates", Shape: "ellipse", Visible: true, Properties: Properties{}},
	}
	if got := level.Objects("crates"); !reflect.DeepEqual(got, want) {
		t.Errorf("objects:\n got %+v\nwant %+v", got, want)
	}
	if err := takeError(); err != nil {
		t.Errorf("reported error: %v", err)
	}
}

const groupsMap = `<?xml version="1.0" encoding="UTF-8"?>
<map version="1.10" orientation="orthogonal" width="2" height="2" tilewidth="8" tileheight="8" parallaxoriginx="3" parallaxoriginy="4">
 <editorsettings>
  <export target="level.json" format="json"/>
 </editorsettings>
 <tileset firstgid="1" name="t" tilewidth="8" tileheight="8" tilecount="1" columns="1">
  <image source="t.png" width="8" height="8"/>
 </tileset>
 <group id="1" name="level" offsetx="4" offsety="2" opacity="0.5" visible="0">
  <properties>
   <property name="zone" value="cave"/>
   <property name="depth" type="int" value="1"/>
  </properties>
  <group id="2" name="back" tintcolor="#ff8000" parallaxx="0.5">
   <layer id="3" name="ground" width="2" height="2" offsetx="1" parallaxx="0.5" parallaxy="2">
    <properties>
     <property name="depth" type="int" value="2"/>
    </properties>
    <data encoding="csv">1,0,0,1</data>
   </layer>
  </group>
  <objectgroup id="4" name="spots" draworder="index">
   <object id="1" name="a" x="0" y="0"/>
  </objectgroup>
 </group>
 <imagelayer id="5" name="sky" repeatx="1">
  <image source="t.png" width="8" height="8"/>
 </imagelayer>
 <objectgroup id="6" name="ground-objects">
  <object id="2" name="b" x="1" y="1"/>
 </objectgroup>
 <layer id="7" name="ground" width="2" height="2">
  <data encoding="csv">0,1,1,0</data>
 </layer>
 <imagelayer id="8" name="empty"/>
</map>
`

func TestMapLayers(t *testing.T) {
	useAssets(t, map[string][]byte{"groups.tmx": []byte(groupsMap), "t.png": pngFile(t, 8, 8)})
	level := readMap(t, "groups.tmx")
	var paths []string
	for _, l := range level.layers {
		paths = append(paths, l.path)
	}
	if want := []string{"level/back/ground", "level/spots", "sky", "ground-objects", "ground", "empty"}; !slices.Equal(paths, want) {
		t.Fatalf("layers = %q, want %q", paths, want)
	}
	nested := level.layers[0]
	if nested.name != "ground" || nested.kind != tileLayer || nested.visible {
		t.Errorf("nested layer: %q, kind %d, visible %v", nested.name, nested.kind, nested.visible)
	}
	if want := (Color{R: 255, G: 128, B: 0, A: 128}); nested.tint != want {
		t.Errorf("nested tint = %v, want %v", nested.tint, want)
	}
	if nested.offsetX != 5 || nested.offsetY != 2 || nested.parallaxX != 0.25 || nested.parallaxY != 2 {
		t.Errorf("nested layer: offset %v, %v, parallax %v, %v", nested.offsetX, nested.offsetY, nested.parallaxX, nested.parallaxY)
	}
	if !level.layers[1].byIndex || level.layers[3].byIndex {
		t.Error("draw order is wrong")
	}
	if sky := level.layers[2]; sky.kind != imageLayer || !sky.repeatX || sky.repeatY || sky.image == nil || !sky.visible || sky.tint != White {
		t.Errorf("sky = %+v", sky)
	}
	if level.layers[5].image != nil {
		t.Error("an image layer without an image has one")
	}
	if level.parallaxOriginX != 3 || level.parallaxOriginY != 4 {
		t.Errorf("parallax origin = %v, %v", level.parallaxOriginX, level.parallaxOriginY)
	}

	// Layers are found by their path first, then by their name.
	if props := level.LayerProperties("level/back/ground"); props.Int("depth") != 2 || props.String("zone") != "cave" {
		t.Errorf("nested properties = %v", props)
	}
	if props := level.LayerProperties("ground"); len(props) != 0 {
		t.Errorf("top-level properties = %v", props)
	}
	if level.LayerProperties("spots").Int("depth") != 1 {
		t.Errorf("spots properties = %v", level.LayerProperties("spots"))
	}
	if got := level.Tile("ground", 0, 0); !got.Empty() {
		t.Errorf("top-level ground 0, 0 = %+v", got)
	}
	if got := level.Tile("level/back/ground", 0, 0); got.Empty() {
		t.Error("nested ground 0, 0 is empty")
	}
	// Tile positions include the layer's offset.
	if got := level.TileAt("level/back/ground", 5, 2); got.Empty() {
		t.Error("TileAt(5, 2) is empty")
	}
	if got := level.TileAt("level/back/ground", 4.9, 2); !got.Empty() {
		t.Errorf("TileAt(4.9, 2) = %+v", got)
	}
	if tiles := level.TilesIn("level/back/ground", Rectangle{X: 13, Y: 10, Width: 1, Height: 1}); len(tiles) != 1 || tiles[0].Rectangle != (Rectangle{X: 13, Y: 10, Width: 8, Height: 8}) {
		t.Errorf("TilesIn = %+v", tiles)
	}
	var names []string
	for _, o := range level.Objects("") {
		names = append(names, o.Layer+"/"+o.Name)
	}
	if want := []string{"spots/a", "ground-objects/b"}; !slices.Equal(names, want) {
		t.Errorf("objects = %q, want %q", names, want)
	}
	if err := takeError(); err != nil {
		t.Errorf("reported error: %v", err)
	}

	if tile := level.Tile("spots", 0, 0); !tile.Empty() {
		t.Errorf("Tile of an object layer = %+v", tile)
	}
	wantError(t, `golib.NewMap("groups.tmx"): layer "spots" is an object layer, not a tile layer`)
	if objects := level.Objects("sky"); objects != nil {
		t.Errorf("Objects of an image layer = %+v", objects)
	}
	wantError(t, `layer "sky" is an image layer, not an object layer`)
	level.TilesIn("level", Rectangle{})
	wantError(t, `golib.NewMap("groups.tmx") has no layer named "level": it has "level/back/ground", "level/spots", "sky", "ground-objects", "ground", "empty"`)
	level.LayerProperties("back/ground")
	wantError(t, `no layer named "back/ground"`)
	(&Screen{}).DrawMapLayer(level, "nope", 0, 0)
	wantError(t, `no layer named "nope"`)
}

func TestMapInfinite(t *testing.T) {
	useAssets(t, map[string][]byte{"t.png": pngFile(t, 8, 8), "infinite.tmx": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<map version="1.10" orientation="orthogonal" width="30" height="20" tilewidth="4" tileheight="4" infinite="1">
 <tileset firstgid="1" name="t" tilewidth="4" tileheight="4" tilecount="4" columns="2">
  <image source="t.png" width="8" height="8"/>
 </tileset>
 <layer id="1" name="a" width="30" height="20">
  <data encoding="csv">
   <chunk x="-2" y="-2" width="2" height="2">
0,0,
0,1
</chunk>
   <chunk x="0" y="0" width="2" height="2">
0,0,
0,2
</chunk>
  </data>
 </layer>
 <layer id="2" name="b" width="30" height="20" offsetx="1">
  <data>
   <chunk x="2" y="-2" width="2" height="2"><tile gid="3"/><tile/><tile/><tile/></chunk>
  </data>
 </layer>
 <objectgroup id="3" name="things">
  <object id="1" name="start" x="-4" y="-8"/>
 </objectgroup>
 <imagelayer id="4" name="sky" offsetx="1" offsety="1">
  <image source="t.png" width="8" height="8"/>
 </imagelayer>
</map>
`)})
	level := readMap(t, "infinite.tmx")
	// The tiles span columns -1 to 2 and rows -2 to 1.
	if level.Width() != 16 || level.Height() != 16 {
		t.Errorf("map is %v by %v, want 16 by 16", level.Width(), level.Height())
	}
	for _, test := range []struct {
		layer       string
		column, row int
		id          int
	}{{"a", 0, 1, 0}, {"a", 2, 3, 1}, {"b", 3, 0, 2}} {
		if got := level.Tile(test.layer, test.column, test.row); got.Empty() || got.ID != test.id {
			t.Errorf("Tile(%q, %d, %d) = %+v, want tile %d", test.layer, test.column, test.row, got, test.id)
		}
	}
	if n := len(level.TilesIn("a", Rectangle{Width: 16, Height: 16})); n != 2 {
		t.Errorf("layer a has %d tiles, want 2", n)
	}
	if start, _ := level.Object("start"); start.X != 0 || start.Y != 0 {
		t.Errorf("start is at %v, %v, want 0, 0", start.X, start.Y)
	}
	if sky := level.layers[3]; sky.offsetX != 5 || sky.offsetY != 9 {
		t.Errorf("sky is at %v, %v, want 5, 9", sky.offsetX, sky.offsetY)
	}
	if err := takeError(); err != nil {
		t.Errorf("reported error: %v", err)
	}

	useAssets(t, map[string][]byte{"empty.tmx": []byte(`<map orientation="orthogonal" width="30" height="20" tilewidth="4" tileheight="4" infinite="1">
 <layer id="1" name="a" width="30" height="20"><data encoding="csv"></data></layer>
</map>`)})
	empty := readMap(t, "empty.tmx")
	if empty.Width() != 0 || empty.Height() != 0 || !empty.Tile("a", 0, 0).Empty() {
		t.Errorf("empty infinite map is %v by %v", empty.Width(), empty.Height())
	}
	if err := takeError(); err != nil {
		t.Errorf("reported error: %v", err)
	}
}

func TestMapTilesets(t *testing.T) {
	useAssets(t, map[string][]byte{
		"spaced.png": pngFile(t, 7, 4),
		"plain.png":  pngFile(t, 6, 2),
		"sets.tmx": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<map version="1.10" orientation="orthogonal" width="1" height="1" tilewidth="2" tileheight="2">
 <tileset firstgid="1" name="spaced" tilewidth="2" tileheight="2" margin="1" spacing="1" objectalignment="center">
  <tileoffset x="3" y="-4"/>
  <image source="spaced.png" trans="000000"/>
  <tile id="0">
   <animation>
    <frame tileid="1" duration="100"/>
    <frame tileid="0" duration="300"/>
   </animation>
  </tile>
 </tileset>
 <tileset firstgid="10" source="plain.tsx"/>
 <layer id="1" name="a" width="1" height="1"><data encoding="csv">10</data></layer>
 <objectgroup id="2" name="o">
  <object id="1" gid="1" x="10" y="10" width="4" height="4"/>
 </objectgroup>
</map>
`),
		"plain.tsx": []byte(`<tileset name="plain" tilewidth="2" tileheight="2">
 <image source="plain.png"/>
</tileset>`),
	})
	level := readMap(t, "sets.tmx")
	spaced, plain := level.tilesets[0], level.tilesets[1]
	if n, err := spaced.sprite.frameCount(); n != 2 || err != nil {
		t.Fatalf("spaced has %d frames (%v), want 2", n, err)
	}
	if want := []image.Rectangle{image.Rect(1, 1, 3, 3), image.Rect(4, 1, 6, 3)}; !slices.Equal(spaced.sprite.frames, want) {
		t.Errorf("spaced frames = %v, want %v", spaced.sprite.frames, want)
	}
	if spaced.offsetX != 3 || spaced.offsetY != -4 {
		t.Errorf("tile offset = %d, %d", spaced.offsetX, spaced.offsetY)
	}
	// The trans color, black, is transparent.
	if a0, a1 := spaced.sprite.pixels.NRGBAAt(0, 0).A, spaced.sprite.pixels.NRGBAAt(1, 0).A; a0 != 0 || a1 != 255 {
		t.Errorf("alpha of pixels 0 and 1 = %d, %d, want 0, 255", a0, a1)
	}
	// A tileset that doesn't say how many tiles it has has as many as fit.
	if n, err := plain.sprite.frameCount(); n != 3 || err != nil {
		t.Errorf("plain has %d frames (%v), want 3", n, err)
	}
	if got := level.Tile("a", 0, 0); got.Tileset != "plain" || got.ID != 0 {
		t.Errorf("tile = %+v", got)
	}
	for _, test := range []struct {
		time  float32
		frame int
	}{{0, 1}, {0.099, 1}, {0.1, 0}, {0.399, 0}, {0.4, 1}, {1.25, 1}, {1.35, 0}, {-1, 1}} {
		if frame, found := spaced.frame(0, test.time); frame != test.frame || !found {
			t.Errorf("tile 0 at %v s: frame %d, %v, want %d", test.time, frame, found, test.frame)
		}
	}
	if frame, _ := spaced.frame(1, 5); frame != 1 {
		t.Errorf("tile 1 at 5 s: frame %d, want 1", frame)
	}
	// The tileset puts the origin of its tile objects in their middle.
	if o, _ := level.Object(""); o.Rectangle != (Rectangle{X: 8, Y: 8, Width: 4, Height: 4}) {
		t.Errorf("object = %+v", o.Rectangle)
	}
	if err := takeError(); err != nil {
		t.Errorf("reported error: %v", err)
	}
}

func TestMapErrors(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"level.json", nil, `golib.NewMap("level.json"): GoLib reads maps from Tiled's .tmx files`},
		{"missing.tmx", nil, `golib.NewMap("missing.tmx"): golib.ReadAsset: assets/missing.tmx not found`},
		{"json.tmx", map[string]string{"json.tmx": `{"type": "map"}`}, `json.tmx isn't a Tiled file GoLib can read`},
		{"iso.tmx", map[string]string{"iso.tmx": `<map orientation="isometric" width="1" height="1" tilewidth="2" tileheight="1"/>`},
			`the map is isometric, but GoLib draws orthogonal maps only: pick Orthogonal`},
		{"grid.tmx", map[string]string{"grid.tmx": `<map orientation="orthogonal" width="1" height="1" tilewidth="0" tileheight="1"/>`},
			`the map's grid is 0 by 1 pixels`},
		{"maps/far.tmx", map[string]string{"maps/far.tmx": `<map orientation="orthogonal" width="1" height="1" tilewidth="2" tileheight="2">
 <tileset firstgid="1" source="../../tiles.tsx"/></map>`}, `maps/far.tmx names "../../tiles.tsx", which is outside the assets folder`},
		{"notiles.tmx", map[string]string{"notiles.tmx": `<map orientation="orthogonal" width="1" height="1" tilewidth="2" tileheight="2">
 <objectgroup name="o"><object id="4" gid="1" x="0" y="0"/></objectgroup></map>`}, `layer "o": object 4 shows tile 1, which no tileset of the map has`},
		{"badpoints.tmx", map[string]string{"badpoints.tmx": `<map orientation="orthogonal" width="1" height="1" tilewidth="2" tileheight="2">
 <objectgroup name="o"><object id="1" x="0" y="0"><polygon points="0,0 4"/></object></objectgroup></map>`}, `a shape has the point "4"`},
		{"sizeless.tmx", map[string]string{"sizeless.tmx": `<map orientation="orthogonal" width="1" height="1" tilewidth="2" tileheight="2">
 <tileset firstgid="1" name="s"><image source="s.png"/></tileset></map>`}, `tileset "s" has tiles of 0 by 0 pixels`},
		{"template.tmx", map[string]string{
			"template.tmx": `<map orientation="orthogonal" width="1" height="1" tilewidth="2" tileheight="2">
 <objectgroup name="o"><object id="1" template="t.tx" x="0" y="0"/></objectgroup></map>`,
			"t.tx": `<template><tileset firstgid="1" source="t.tsx"/><object gid="1"/></template>`,
		}, `template t.tx uses the tileset t.tsx, which the map doesn't`},
	}
	for _, test := range tests {
		files := map[string][]byte{}
		for name, text := range test.files {
			files[name] = []byte(text)
		}
		useAssets(t, files)
		level := NewMap(test.name)
		level.Objects("")
		wantError(t, test.want)
		// Every use reports the mistake again.
		level.Properties()
		wantError(t, test.want)
	}

	// Images are read when they are first drawn.
	useAssets(t, map[string][]byte{"bmp.tmx": []byte(`<map orientation="orthogonal" width="1" height="1" tilewidth="2" tileheight="2">
 <tileset firstgid="1" name="s" tilewidth="2" tileheight="2"><image source="s.bmp"/></tileset></map>`)})
	level := readMap(t, "bmp.tmx")
	if _, _, ok := level.tilesets[0].texture(0, 0); ok {
		t.Error("a BMP tileset has a texture")
	}
	wantError(t, `golib.NewMap("bmp.tmx"), tileset "s": s.bmp: GoLib reads PNG images only`)
	level.tilesets[0].texture(0, 0)
	wantError(t, `s.bmp: GoLib reads PNG images only`)

	(&Screen{}).DrawMap(nil, 0, 0)
	wantError(t, "golib: Screen.DrawMap got a nil map: make maps with golib.NewMap")
}

func TestProperties(t *testing.T) {
	p := Properties{"n": "3", "f": "2.75", "neg": "-4.5", "yes": "true", "rgb": "#00ff00", "argb": "#80ff0000", "bad": "#12345", "word": "hello"}
	tests := []struct {
		got, want any
	}{
		{p.String("word"), "hello"},
		{p.String("missing"), ""},
		{p.Int("n"), 3},
		{p.Int("f"), 2},
		{p.Int("neg"), -4},
		{p.Int("word"), 0},
		{p.Float("f"), float32(2.75)},
		{p.Float("n"), float32(3)},
		{p.Float("word"), float32(0)},
		{p.Bool("yes"), true},
		{p.Bool("n"), false},
		{p.Bool("missing"), false},
		{p.Color("rgb"), Color{G: 255, A: 255}},
		{p.Color("argb"), Color{R: 255, A: 128}},
		{p.Color("bad"), Color{}},
		{p.Color("word"), Color{}},
		{Properties(nil).Int("n"), 0},
	}
	for i, test := range tests {
		if test.got != test.want {
			t.Errorf("test %d: got %v, want %v", i, test.got, test.want)
		}
	}
}

func TestTiledPaths(t *testing.T) {
	tests := []struct {
		from, file, want string
	}{
		{"level.tmx", "tiles.tsx", "tiles.tsx"},
		{"maps/level.tmx", "../tiles/t.tsx", "tiles/t.tsx"},
		{"maps/level.tmx", `..\tiles\t.tsx`, "tiles/t.tsx"},
		{"maps/a/level.tmx", "./b/../t.png", "maps/a/t.png"},
		{"level.tmx", "../t.png", ""},
		{"level.tmx", "/t.png", ""},
		{"level.tmx", "C:/art/t.png", ""},
		{"level.tmx", `D:\art\t.png`, ""},
	}
	for _, test := range tests {
		got, err := relative(test.from, test.file)
		if got != test.want || (err != nil) != (test.want == "") {
			t.Errorf("relative(%q, %q) = %q, %v, want %q", test.from, test.file, got, err, test.want)
		}
	}
	if got := pathExt("maps.v2/level"); got != "" {
		t.Errorf("pathExt = %q", got)
	}
}

func TestWholePixel(t *testing.T) {
	for _, test := range []struct{ in, want float32 }{{670.5, 671}, {683.333, 683}, {-0.5, 0}, {-3.66667, -4}, {2, 2}, {-1.5, -1}} {
		if got := wholePixel(test.in); got != test.want {
			t.Errorf("wholePixel(%v) = %v, want %v", test.in, got, test.want)
		}
	}
}

func TestMapErr(t *testing.T) {
	useAssets(t, map[string][]byte{
		"maps/level.tmx":  []byte(mapFile(2, 2, `<data encoding="csv">1,2,3,4</data>`)),
		"maps/t.png":      pngFile(t, 4, 4),
		"maps/broken.tmx": []byte("<map>"),
		"maps/level.json": []byte("{}"),
	})
	if err := readMap(t, "maps/level.tmx").Err(); err != nil {
		t.Errorf("a map that loads: Err() = %v", err)
	}
	for name, want := range map[string]string{
		"maps/missing.tmx": "assets/maps/missing.tmx not found",
		"maps/broken.tmx":  "golib.NewMap(\"maps/broken.tmx\")",
		"maps/level.json":  "save the map as TMX in Tiled",
	} {
		err := NewMap(name).Err()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Err() of %s = %v, want one containing %q", name, err, want)
		}
		// Err leaves the game running: only using the map stops Run.
		if pending := takeError(); pending != nil {
			t.Errorf("Err() of %s reported %v to Run", name, pending)
		}
		if level := NewMap(name); level.Width() != 0 || takeError() == nil {
			t.Errorf("using %s: width %v, and Run was not told", name, level.Width())
		}
	}
}
