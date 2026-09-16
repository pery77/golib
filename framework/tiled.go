package golib

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"io"
	"math"
	"path"
	"strconv"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Tiled's files are read as its documentation describes:
// https://doc.mapeditor.org/en/stable/reference/tmx-map-format/
// A map (.tmx) can use tilesets of its own or from .tsx files, and objects
// can come from templates (.tx). Paths inside a file are relative to it.

// The flags Tiled keeps in the top bits of a tile's global ID.
const (
	tiledFlipX     = 1 << 31
	tiledFlipY     = 1 << 30
	tiledFlipXY    = 1 << 29 // the anti-diagonal: x and y swap
	tiledRotateHex = 1 << 28 // hexagonal maps only
	tiledFlags     = tiledFlipX | tiledFlipY | tiledFlipXY | tiledRotateHex
)

// The XML of Tiled's files, as encoding/xml reads it.

type tmxMap struct {
	Orientation     string        `xml:"orientation,attr"`
	RenderOrder     string        `xml:"renderorder,attr"`
	Width           int           `xml:"width,attr"`
	Height          int           `xml:"height,attr"`
	TileWidth       int           `xml:"tilewidth,attr"`
	TileHeight      int           `xml:"tileheight,attr"`
	Infinite        int           `xml:"infinite,attr"`
	BackgroundColor string        `xml:"backgroundcolor,attr"`
	ParallaxOriginX float32       `xml:"parallaxoriginx,attr"`
	ParallaxOriginY float32       `xml:"parallaxoriginy,attr"`
	Properties      tmxProperties `xml:"properties"`
	Tilesets        []tmxTileset  `xml:"tileset"`
	Layers          []tmxLayer    `xml:",any"`
}

// tmxLayer is a tile layer, an object layer, an image layer or a group,
// which its XMLName tells apart. Groups hold more of them.
type tmxLayer struct {
	XMLName    xml.Name
	Name       string        `xml:"name,attr"`
	Visible    *int          `xml:"visible,attr"`
	Opacity    *float32      `xml:"opacity,attr"`
	TintColor  string        `xml:"tintcolor,attr"`
	OffsetX    float32       `xml:"offsetx,attr"`
	OffsetY    float32       `xml:"offsety,attr"`
	ParallaxX  *float32      `xml:"parallaxx,attr"`
	ParallaxY  *float32      `xml:"parallaxy,attr"`
	Width      int           `xml:"width,attr"`
	Height     int           `xml:"height,attr"`
	RepeatX    int           `xml:"repeatx,attr"`
	RepeatY    int           `xml:"repeaty,attr"`
	DrawOrder  string        `xml:"draworder,attr"`
	Properties tmxProperties `xml:"properties"`
	Data       *tmxData      `xml:"data"`
	Objects    []tmxObject   `xml:"object"`
	Image      *tmxImage     `xml:"image"`
	Layers     []tmxLayer    `xml:",any"`
}

type tmxData struct {
	Encoding    string     `xml:"encoding,attr"`
	Compression string     `xml:"compression,attr"`
	Text        string     `xml:",chardata"`
	Tiles       []tmxGID   `xml:"tile"`
	Chunks      []tmxChunk `xml:"chunk"`
}

type tmxChunk struct {
	X      int      `xml:"x,attr"`
	Y      int      `xml:"y,attr"`
	Width  int      `xml:"width,attr"`
	Height int      `xml:"height,attr"`
	Text   string   `xml:",chardata"`
	Tiles  []tmxGID `xml:"tile"`
}

type tmxGID struct {
	GID uint32 `xml:"gid,attr"`
}

type tmxTileset struct {
	FirstGID        int    `xml:"firstgid,attr"`
	Source          string `xml:"source,attr"`
	Name            string `xml:"name,attr"`
	TileWidth       int    `xml:"tilewidth,attr"`
	TileHeight      int    `xml:"tileheight,attr"`
	Spacing         int    `xml:"spacing,attr"`
	Margin          int    `xml:"margin,attr"`
	TileCount       int    `xml:"tilecount,attr"`
	Columns         int    `xml:"columns,attr"`
	ObjectAlignment string `xml:"objectalignment,attr"`
	TileOffset      struct {
		X int `xml:"x,attr"`
		Y int `xml:"y,attr"`
	} `xml:"tileoffset"`
	Image *tmxImage `xml:"image"`
	Tiles []tmxTile `xml:"tile"`
}

type tmxImage struct {
	Source string `xml:"source,attr"`
	Trans  string `xml:"trans,attr"`
	Width  int    `xml:"width,attr"`
	Height int    `xml:"height,attr"`
}

type tmxTile struct {
	ID         int           `xml:"id,attr"`
	Type       string        `xml:"type,attr"`
	Class      string        `xml:"class,attr"`
	Properties tmxProperties `xml:"properties"`
	Image      *tmxImage     `xml:"image"`
	Animation  []struct {
		TileID   int `xml:"tileid,attr"`
		Duration int `xml:"duration,attr"`
	} `xml:"animation>frame"`
}

type tmxObject struct {
	ID         int           `xml:"id,attr"`
	Name       *string       `xml:"name,attr"`
	Type       *string       `xml:"type,attr"`
	Class      *string       `xml:"class,attr"`
	X          float32       `xml:"x,attr"`
	Y          float32       `xml:"y,attr"`
	Width      *float32      `xml:"width,attr"`
	Height     *float32      `xml:"height,attr"`
	Rotation   *float32      `xml:"rotation,attr"`
	GID        *uint32       `xml:"gid,attr"`
	Visible    *int          `xml:"visible,attr"`
	Template   string        `xml:"template,attr"`
	Properties tmxProperties `xml:"properties"`
	Ellipse    *struct{}     `xml:"ellipse"`
	Point      *struct{}     `xml:"point"`
	Polygon    *tmxPoints    `xml:"polygon"`
	Polyline   *tmxPoints    `xml:"polyline"`
	Text       *struct {
		Text string `xml:",chardata"`
	} `xml:"text"`
}

type tmxPoints struct {
	Points string `xml:"points,attr"`
}

type tmxProperties struct {
	Properties []tmxProperty `xml:"property"`
}

type tmxProperty struct {
	Name       string         `xml:"name,attr"`
	Type       string         `xml:"type,attr"`
	Value      *string        `xml:"value,attr"`
	Text       string         `xml:",chardata"`
	Properties *tmxProperties `xml:"properties"`
}

type tmxTemplate struct {
	Tileset *tmxTileset `xml:"tileset"`
	Object  tmxObject   `xml:"object"`
}

// Properties are the custom properties Tiled gives a map, a layer, a tile or
// an object, by name, as the text Tiled saves: "true" for a bool, "#ff8000"
// or "#80ff8000" for a color, the ID of an object for an object. The members
// of a class property are named "property.member". Read them with the
// methods, which give the zero value for a property that is missing or
// doesn't parse.
//
//	if tile.Properties.Bool("solid") { ... }
//	damage := object.Properties.Int("damage")
type Properties map[string]string

// String returns the property as text.
func (p Properties) String(name string) string {
	return p[name]
}

// Int returns a whole-number property.
func (p Properties) Int(name string) int {
	n, err := strconv.Atoi(strings.TrimSpace(p[name]))
	if err != nil {
		f, err := strconv.ParseFloat(strings.TrimSpace(p[name]), 64)
		if err != nil {
			return 0
		}
		return int(f)
	}
	return n
}

// Float returns a number property.
func (p Properties) Float(name string) float32 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(p[name]), 32)
	return float32(f)
}

// Bool returns a true or false property.
func (p Properties) Bool(name string) bool {
	b, _ := strconv.ParseBool(strings.TrimSpace(p[name]))
	return b
}

// Color returns a color property, written as #rrggbb or #aarrggbb.
func (p Properties) Color(name string) Color {
	c, _ := parseTiledColor(p[name])
	return c
}

// parseTiledColor reads a color as Tiled writes it: #rrggbb or #aarrggbb,
// with or without the #.
func parseTiledColor(text string) (Color, bool) {
	text = strings.TrimPrefix(strings.TrimSpace(text), "#")
	value, err := strconv.ParseUint(text, 16, 32)
	if err != nil || (len(text) != 6 && len(text) != 8) {
		return Color{}, false
	}
	c := Color{R: uint8(value >> 16), G: uint8(value >> 8), B: uint8(value), A: 255}
	if len(text) == 8 {
		c.A = uint8(value >> 24)
	}
	return c, true
}

// properties turns the XML properties into Properties, on top of base.
func (t tmxProperties) properties(base Properties) Properties {
	props := Properties{}
	for name, value := range base {
		props[name] = value
	}
	t.addTo(props, "")
	return props
}

func (t tmxProperties) addTo(props Properties, prefix string) {
	for _, p := range t.Properties {
		name := prefix + p.Name
		if p.Properties != nil {
			p.Properties.addTo(props, name+".")
			continue
		}
		value := p.Text
		if p.Value != nil {
			value = *p.Value
		}
		props[name] = value
	}
}

// tiledFiles reads the files a map uses, relative to the assets folder.
type tiledFiles struct {
	read  func(name string) ([]byte, error)
	owner string // the call that made the map, for messages
}

// relative returns the asset name of a path written in the file named from.
func relative(from, file string) (string, error) {
	file = strings.ReplaceAll(file, `\`, "/")
	joined := path.Join(path.Dir(from), file)
	// Tiled writes a path on another drive, or on the web, as it is.
	if strings.HasPrefix(joined, "../") || joined == ".." || path.IsAbs(file) || strings.Contains(file, ":") {
		return "", fmt.Errorf("%s names %q, which is outside the assets folder: keep maps, tilesets and images inside it", from, file)
	}
	return joined, nil
}

func (files tiledFiles) xml(name string, into any) error {
	data, err := files.read(name)
	if err != nil {
		return err
	}
	if err := xml.Unmarshal(data, into); err != nil {
		return fmt.Errorf("%s isn't a Tiled file GoLib can read (%v): save it again from Tiled as XML (.tmx, .tsx or .tx), not JSON", name, err)
	}
	return nil
}

// tileset is a tileset of a map, ready to draw from.
type tileset struct {
	firstGID      int
	name          string
	source        string // the .tsx file, or "" for a tileset inside the map
	tileWidth     int
	tileHeight    int
	offsetX       int
	offsetY       int
	alignment     string
	sprite        *Sprite     // the tiles, as frames
	frameOf       map[int]int // for a collection of images, the frame of each tile ID; nil otherwise
	tiles         map[int]*tileInfo
	width, height map[int]int // for a collection of images, each tile's size
}

type tileInfo struct {
	class      string
	properties Properties
	animation  []tileFrame
	duration   float32
}

type tileFrame struct {
	id       int
	duration float32
}

// frame returns the sprite frame that shows tile id, after time seconds of
// its animation.
func (t *tileset) frame(id int, time float32) (int, bool) {
	if info := t.tiles[id]; info != nil && info.duration > 0 {
		time = float32(math.Mod(float64(max(time, 0)), float64(info.duration)))
		for _, f := range info.animation {
			time -= f.duration
			if time < 0 {
				id = f.id
				break
			}
		}
	}
	if t.frameOf != nil {
		frame, found := t.frameOf[id]
		return frame, found
	}
	return id, id >= 0
}

// texture returns the texture that holds tile id after time seconds of its
// animation, and the tile's place in it. ok is false when there is nothing to
// draw: the tileset has no such tile, or its image can't be read, which is
// reported.
func (t *tileset) texture(id int, time float32) (texture rl.Texture2D, place image.Rectangle, ok bool) {
	frame, found := t.frame(id, time)
	if !found {
		return texture, place, false
	}
	// A tileset's file may leave out how many tiles it has: its image
	// tells.
	count, err := t.sprite.frameCount()
	if err == nil && frame >= count {
		return texture, place, false
	}
	if err == nil {
		texture, place, err = t.sprite.frameTexture(frame)
	}
	if err != nil {
		reportError(err)
		return texture, place, false
	}
	return texture, place, true
}

// size returns the pixel size of tile id.
func (t *tileset) size(id int) (int, int) {
	if t.frameOf != nil {
		return t.width[id], t.height[id]
	}
	return t.tileWidth, t.tileHeight
}

// readTileset makes a tileset from its XML, which is in the file named from.
func (files tiledFiles) readTileset(x tmxTileset, from string) (*tileset, error) {
	if x.Source != "" {
		source, err := relative(from, x.Source)
		if err != nil {
			return nil, err
		}
		var external tmxTileset
		if err := files.xml(source, &external); err != nil {
			return nil, err
		}
		external.FirstGID = x.FirstGID
		ts, err := files.readTileset(external, source)
		if ts != nil {
			ts.source = source
		}
		return ts, err
	}
	ts := &tileset{
		firstGID:   x.FirstGID,
		name:       x.Name,
		tileWidth:  x.TileWidth,
		tileHeight: x.TileHeight,
		offsetX:    x.TileOffset.X,
		offsetY:    x.TileOffset.Y,
		alignment:  x.ObjectAlignment,
		tiles:      map[int]*tileInfo{},
	}
	for _, tile := range x.Tiles {
		info := &tileInfo{class: tile.Class, properties: tile.Properties.properties(nil)}
		if info.class == "" {
			info.class = tile.Type
		}
		for _, f := range tile.Animation {
			frame := tileFrame{id: f.TileID, duration: float32(f.Duration) / 1000}
			info.animation = append(info.animation, frame)
			info.duration += frame.duration
		}
		ts.tiles[tile.ID] = info
	}

	if x.Image != nil {
		imageName, err := relative(from, x.Image.Source)
		if err != nil {
			return nil, err
		}
		if x.TileWidth <= 0 || x.TileHeight <= 0 {
			return nil, fmt.Errorf("tileset %q has tiles of %d by %d pixels: fix it in Tiled", x.Name, x.TileWidth, x.TileHeight)
		}
		trans := x.Image.Trans
		ts.sprite = newGeneratedSprite(fmt.Sprintf("%s, tileset %q", files.owner, x.Name), func(s *Sprite) error {
			pixels, err := readTiledImage(files, imageName, trans)
			if err != nil {
				return err
			}
			s.pixels = pixels
			s.width, s.height = x.TileWidth, x.TileHeight
			columns := x.Columns
			if columns <= 0 {
				columns = (pixels.Rect.Dx() - 2*x.Margin + x.Spacing) / (x.TileWidth + x.Spacing)
			}
			count := x.TileCount
			if count <= 0 {
				rows := (pixels.Rect.Dy() - 2*x.Margin + x.Spacing) / (x.TileHeight + x.Spacing)
				count = columns * rows
			}
			s.frames = nil
			for id := range count {
				left := x.Margin + (id%columns)*(x.TileWidth+x.Spacing)
				top := x.Margin + (id/columns)*(x.TileHeight+x.Spacing)
				s.frames = append(s.frames, image.Rect(left, top, left+x.TileWidth, top+x.TileHeight))
			}
			return nil
		})
		return ts, nil
	}

	// A collection of images: each tile has an image of its own. They go
	// into one picture, in rows.
	ts.frameOf, ts.width, ts.height = map[int]int{}, map[int]int{}, map[int]int{}
	var names []string
	var ids []int
	for _, tile := range x.Tiles {
		if tile.Image == nil {
			continue
		}
		name, err := relative(from, tile.Image.Source)
		if err != nil {
			return nil, err
		}
		ts.frameOf[tile.ID] = len(names)
		ts.width[tile.ID], ts.height[tile.ID] = tile.Image.Width, tile.Image.Height
		names = append(names, name)
		ids = append(ids, tile.ID)
	}
	trans := map[int]string{}
	for _, tile := range x.Tiles {
		if tile.Image != nil {
			trans[tile.ID] = tile.Image.Trans
		}
	}
	ts.sprite = newGeneratedSprite(fmt.Sprintf("%s, tileset %q", files.owner, x.Name), func(s *Sprite) error {
		var images []*image.NRGBA
		for i, name := range names {
			pixels, err := readTiledImage(files, name, trans[ids[i]])
			if err != nil {
				return err
			}
			ts.width[ids[i]], ts.height[ids[i]] = pixels.Rect.Dx(), pixels.Rect.Dy()
			images = append(images, pixels)
		}
		s.pixels, s.frames = packImages(images)
		return nil
	})
	return ts, nil
}

// readTiledImage reads a PNG image a map uses, making the color trans, if
// given, transparent.
func readTiledImage(files tiledFiles, name, trans string) (*image.NRGBA, error) {
	if !strings.EqualFold(path.Ext(name), ".png") {
		return nil, fmt.Errorf("%s: GoLib reads PNG images only: save the image as PNG and pick it again in Tiled", name)
	}
	data, err := files.read(name)
	if err != nil {
		return nil, err
	}
	sprite := &Sprite{name: name}
	if err := sprite.decodePNG(data); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	pixels := sprite.pixels
	if key, ok := parseTiledColor(trans); ok && trans != "" {
		for i := 0; i < len(pixels.Pix); i += 4 {
			if pixels.Pix[i] == key.R && pixels.Pix[i+1] == key.G && pixels.Pix[i+2] == key.B {
				pixels.Pix[i+3] = 0
			}
		}
	}
	return pixels, nil
}

// packImages puts images into one picture, in rows no wider than 4096
// pixels, with a gap between them, and returns where each one went.
func packImages(images []*image.NRGBA) (*image.NRGBA, []image.Rectangle) {
	const maxWidth = 4096
	var places []image.Rectangle
	x, y, rowHeight, width := 0, 0, 0, 1
	for _, img := range images {
		w, h := img.Rect.Dx(), img.Rect.Dy()
		if x > 0 && x+w > maxWidth {
			x, y, rowHeight = 0, y+rowHeight+1, 0
		}
		places = append(places, image.Rect(x, y, x+w, y+h))
		x += w + 1
		rowHeight = max(rowHeight, h)
		width = max(width, x-1)
	}
	picture := image.NewNRGBA(image.Rect(0, 0, width, max(y+rowHeight, 1)))
	for i, img := range images {
		for row := range img.Rect.Dy() {
			copy(picture.Pix[picture.PixOffset(places[i].Min.X, places[i].Min.Y+row):], img.Pix[img.PixOffset(0, row):img.PixOffset(img.Rect.Dx(), row)])
		}
	}
	return picture, places
}

// decodeTileData returns the global tile IDs of a layer's data or chunk.
func decodeTileData(encoding, compression, text string, tiles []tmxGID, count int) ([]uint32, error) {
	var gids []uint32
	switch encoding {
	case "":
		for _, t := range tiles {
			gids = append(gids, t.GID)
		}
	case "csv":
		for _, field := range strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' || r == ' ' || r == '\t' }) {
			gid, err := strconv.ParseUint(field, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("the tile data has %q, which isn't a tile ID: save the map again from Tiled", field)
			}
			gids = append(gids, uint32(gid))
		}
	case "base64":
		data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(text))
		if err != nil {
			return nil, fmt.Errorf("the tile data isn't valid base64 (%v): save the map again from Tiled", err)
		}
		var reader io.Reader = bytes.NewReader(data)
		switch compression {
		case "":
		case "zlib":
			if reader, err = zlib.NewReader(reader); err != nil {
				return nil, fmt.Errorf("the tile data isn't valid zlib (%v): save the map again from Tiled", err)
			}
		case "gzip":
			if reader, err = gzip.NewReader(reader); err != nil {
				return nil, fmt.Errorf("the tile data isn't valid gzip (%v): save the map again from Tiled", err)
			}
		case "zstd":
			return nil, errors.New("the tile data is compressed with Zstandard, which GoLib can't read: in Tiled, pick another Tile Layer Format under Map > Map Properties, such as CSV")
		default:
			return nil, fmt.Errorf("the tile data is compressed with %q, which GoLib doesn't know: pick CSV as the Tile Layer Format in Tiled", compression)
		}
		raw := make([]byte, 4*count)
		if _, err := io.ReadFull(reader, raw); err != nil {
			return nil, fmt.Errorf("the tile data is cut short (%v): save the map again from Tiled", err)
		}
		for i := range count {
			gids = append(gids, binary.LittleEndian.Uint32(raw[4*i:]))
		}
	default:
		return nil, fmt.Errorf("the tile data is encoded as %q, which GoLib doesn't know: pick CSV as the Tile Layer Format in Tiled", encoding)
	}
	if len(gids) != count {
		return nil, fmt.Errorf("the tile data has %d tiles, but the layer is %d tiles: save the map again from Tiled", len(gids), count)
	}
	return gids, nil
}

// parsePoints reads a polygon's or a polyline's points: "0,0 10,5 3,8".
func parsePoints(text string) ([]Vector2, error) {
	var points []Vector2
	for _, pair := range strings.Fields(text) {
		xs, ys, found := strings.Cut(pair, ",")
		x, errX := strconv.ParseFloat(xs, 32)
		y, errY := strconv.ParseFloat(ys, 32)
		if !found || errX != nil || errY != nil {
			return nil, fmt.Errorf("a shape has the point %q: save the map again from Tiled", pair)
		}
		points = append(points, Vector2{X: float32(x), Y: float32(y)})
	}
	return points, nil
}

// colorWithOpacity returns tint made opacity times as opaque.
func colorWithOpacity(tint Color, opacity float32) Color {
	tint.A = uint8(math.Round(float64(tint.A) * float64(max(0, min(opacity, 1)))))
	return tint
}
