package golib

import (
	"bytes"
	"encoding/binary"
	"os"
	"runtime"
	"slices"
	"sort"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// testGlyph is a glyph of testFont: a filled box, in font units, and how far
// the pen moves after it.
type testGlyph struct {
	letter                   rune
	advance                  int16
	left, bottom, right, top int16
}

// testGlyphs are the letters of testFont. Its ascent is 800 units and its
// descent 200, so at 10 pixels a unit is a hundredth of a pixel: A is 5 by 8
// pixels, standing on the baseline, 8 pixels below the top of the text.
var testGlyphs = []testGlyph{
	{letter: ' ', advance: 500},
	{letter: 'A', advance: 600, right: 500, top: 800},
	{letter: 'B', advance: 700, bottom: -200, right: 600, top: 800},
	{letter: '?', advance: 400, right: 300, top: 800},
	{letter: 'ñ', advance: 500, left: 100, right: 400, top: 300},
}

// testFont returns a TrueType font with testGlyphs, and nothing else a
// TrueType reader doesn't need, so tests need no font file with a license.
func testFont(leaveOut ...string) []byte {
	be := binary.BigEndian
	u16 := func(b []byte, v int) []byte { return be.AppendUint16(b, uint16(v)) }
	u32 := func(b []byte, v int) []byte { return be.AppendUint32(b, uint32(v)) }

	// Glyph 0 is the empty .notdef glyph; testGlyphs[i] is glyph i+1.
	var glyf, loca, hmtx []byte
	loca = u32(u32(loca, 0), 0) // where glyph 0 starts and ends
	hmtx = u16(u16(hmtx, 500), 0)
	for _, g := range testGlyphs {
		if g.right > g.left {
			glyf = u16(glyf, 1) // one contour
			for _, v := range []int16{g.left, g.bottom, g.right, g.top} {
				glyf = u16(glyf, int(v))
			}
			glyf = u16(glyf, 3) // the contour ends at point 3
			glyf = u16(glyf, 0) // no instructions
			glyf = append(glyf, 1, 1, 1, 1)
			// Clockwise, as TrueType fills: coordinates are changes from the
			// point before.
			for _, v := range []int16{g.left, 0, g.right - g.left, 0} {
				glyf = u16(glyf, int(v))
			}
			for _, v := range []int16{g.bottom, g.top - g.bottom, 0, g.bottom - g.top} {
				glyf = u16(glyf, int(v))
			}
		}
		loca = u32(loca, len(glyf))
		hmtx = u16(u16(hmtx, int(g.advance)), int(g.left))
	}
	glyphs := len(testGlyphs) + 1

	head := u32(nil, 0x00010000)
	head = u32(head, 0)
	head = u32(head, 0)
	head = u32(head, 0x5F0F3CF5)
	head = u16(u16(head, 0), 1000) // flags, units per em
	head = append(head, make([]byte, 16)...)
	head = u16(u16(u16(u16(head, 0), -200), 700), 800)
	head = u16(u16(u16(head, 0), 8), 2)
	head = u16(u16(head, 1), 0) // long offsets in loca

	hhea := u32(nil, 0x00010000)
	hhea = u16(u16(u16(hhea, 800), -200), 0) // ascent, descent, line gap
	hhea = u16(hhea, 700)
	hhea = append(hhea, make([]byte, 22)...)
	hhea = u16(hhea, glyphs)

	maxp := u16(u32(nil, 0x00005000), glyphs)

	type group struct {
		letter rune
		glyph  int
	}
	var groups []group
	for i, g := range testGlyphs {
		groups = append(groups, group{g.letter, i + 1})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].letter < groups[j].letter })
	cmap := u16(u16(nil, 0), 1)
	cmap = u32(u16(u16(cmap, 3), 10), 12) // Windows, full Unicode, at 12
	cmap = u16(u16(cmap, 12), 0)
	cmap = u32(u32(u32(cmap, 16+12*len(groups)), 0), len(groups))
	for _, g := range groups {
		cmap = u32(u32(u32(cmap, int(g.letter)), int(g.letter)), g.glyph)
	}

	tables := map[string][]byte{"cmap": cmap, "glyf": glyf, "head": head, "hhea": hhea, "hmtx": hmtx, "loca": loca, "maxp": maxp}
	tags := make([]string, 0, len(tables))
	for tag := range tables {
		if !slices.Contains(leaveOut, tag) {
			tags = append(tags, tag)
		}
	}
	slices.Sort(tags)
	font := u32(nil, 0x00010000)
	font = u16(u16(u16(u16(font, len(tags)), 64), 3), 16*len(tags)-64)
	offset := 12 + 16*len(tags)
	var body []byte
	for _, tag := range tags {
		data := tables[tag]
		font = append(font, tag...)
		font = u32(u32(u32(font, 0), offset+len(body)), len(data))
		body = append(body, data...)
		for len(body)%4 != 0 {
			body = append(body, 0)
		}
	}
	return append(font, body...)
}

func TestFontFiles(t *testing.T) {
	font := testFont()
	short := slices.Clone(font[:len(font)-8])
	many := slices.Clone(font)
	many[5] = 200 // tables
	useAssets(t, map[string][]byte{
		"fonts/test.ttf":       font,
		"fonts/test.woff":      font,
		"fonts/text.ttf":       []byte("not a font at all"),
		"fonts/short.ttf":      short,
		"fonts/many.ttf":       many,
		"fonts/tiny.otf":       []byte("OTTO"),
		"fonts/collection.ttf": append([]byte("ttcf"), font...),
	})
	for _, name := range []string{"fonts/test.ttf", "fonts/text.ttf", "fonts/short.ttf", "fonts/many.ttf", "fonts/tiny.otf"} {
		data, _ := os.ReadFile("assets/" + name)
		if err := checkFontTables(data); (err == nil) != (name == "fonts/test.ttf") {
			t.Errorf("checkFontTables(%s) = %v", name, err)
		}
	}
	test := NewFont("fonts/test.ttf")
	if err := test.prepare(); err != nil || !bytes.Equal(test.data, font) {
		t.Errorf("prepare: %v, %d bytes", err, len(test.data))
	}

	tests := map[string]string{
		"fonts/missing.ttf":    `golib.NewFont("fonts/missing.ttf"): golib.ReadAsset: assets/fonts/missing.ttf not found`,
		"fonts/test.woff":      `golib.NewFont("fonts/test.woff"): GoLib reads fonts from .ttf and .otf files, not ".woff" ones`,
		"fonts/text.ttf":       `golib.NewFont("fonts/text.ttf"): the file isn't a TrueType or OpenType font, or it is damaged`,
		"fonts/collection.ttf": `golib.NewFont("fonts/collection.ttf"): the file is a font collection`,
	}
	for name, want := range tests {
		font := NewFont(name)
		if _, err := font.atlas(10, "A"); err == nil || err.Error()[:len(want)] != want {
			t.Errorf("%s: error %v, want %q", name, err, want)
		}
		if _, err := font.atlas(10, "A"); err == nil {
			t.Errorf("%s: the second use has no error", name)
		}
		font.unload()
		if font.read || font.err != nil {
			t.Errorf("%s: unload kept read %v, error %v", name, font.read, font.err)
		}
	}
}

func TestFontsInAWindow(t *testing.T) {
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Skip("no display to open a window on")
	}
	// OpenGL draws from the thread that opened the window, and tests run on
	// any thread: keep this one.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	config := Config{Title: "font test", Width: 64, Height: 32}
	if err := openWindow(config, true); err != nil {
		t.Skip(err)
	}
	defer rl.CloseWindow()
	render := newRenderer(config)
	defer render.close()
	useAssets(t, map[string][]byte{"test.ttf": testFont(), "nocmap.ttf": testFont("cmap")})

	font := NewFont("test.ttf")
	options := TextOptions{Font: font}
	screen := &Screen{width: 64, height: 32}
	// A is 6 pixels wide with the space after it, B is 7.
	if got := screen.TextWidth("AB", 10, options); got != 13 {
		t.Errorf("TextWidth(AB, 10) = %v, want 13", got)
	}
	if got := screen.TextWidth("AB\nBBB", 20, options); got != 42 {
		t.Errorf("TextWidth of two lines at 20 = %v, want 42, the longest", got)
	}
	sized := font.sizes[10]
	if sized == nil || sized.font.CharsCount != 4 || len(sized.runes) != 95 {
		t.Fatalf("size 10: %+v, want 4 glyphs from 95 letters asked", sized)
	}
	first := sized.font.Texture.ID

	// A letter beyond ASCII is added once; one the font doesn't have is
	// asked for once too.
	screen.TextWidth("ñ", 10, options)
	if sized.font.CharsCount != 5 || sized.font.Texture.ID == first {
		t.Errorf("after ñ: %d glyphs, texture %d (was %d)", sized.font.CharsCount, sized.font.Texture.ID, first)
	}
	screen.TextWidth("日", 10, options)
	second := sized.font.Texture.ID
	screen.TextWidth("日ñAB", 10, options)
	if sized.font.Texture.ID != second || len(sized.runes) != 97 {
		t.Errorf("letters asked again: texture %d (was %d), %d letters", sized.font.Texture.ID, second, len(sized.runes))
	}

	// The text is drawn at whole pixels, with its top at y.
	rl.BeginTextureMode(render.scene)
	screen.Clear(Blank)
	screen.DrawText("A", 10.4, 3.5, 10, White, options)
	screen.DrawText("B", 30, 0, 10, Red) // the built-in font
	rl.EndTextureMode()
	image := rl.LoadImageFromTexture(render.scene.Texture)
	rl.ImageFlipVertical(image)
	for _, test := range []struct {
		x, y int32
		lit  bool
	}{{10, 4, true}, {14, 4, true}, {10, 11, true}, {14, 11, true}, {9, 4, false}, {15, 4, false}, {10, 3, false}, {10, 12, false}} {
		if lit := rl.GetImageColor(*image, test.x, test.y).A > 0; lit != test.lit {
			t.Errorf("pixel %d, %d lit: %v, want %v", test.x, test.y, lit, test.lit)
		}
	}
	builtIn := 0
	for y := int32(0); y < 10; y++ {
		for x := int32(30); x < 40; x++ {
			if rl.GetImageColor(*image, x, y).A > 0 {
				builtIn++
			}
		}
	}
	if builtIn == 0 {
		t.Error("the built-in font drew nothing")
	}
	rl.UnloadImage(image)

	// Only the sizes used last stay.
	for size := float32(11); size <= 19; size++ {
		screen.TextWidth("A", size, options)
	}
	if len(font.sizes) != maxFontSizes || font.sizes[10] != nil || font.sizes[11] != nil || font.sizes[19] == nil {
		var sizes []int
		for size := range font.sizes {
			sizes = append(sizes, size)
		}
		slices.Sort(sizes)
		t.Errorf("sizes kept: %v", sizes)
	}
	// Sizes are rounded.
	screen.TextWidth("A", 18.6, options)
	if len(font.sizes) != maxFontSizes || font.sizes[12] == nil {
		t.Errorf("18.6 wasn't drawn at 19: %d sizes", len(font.sizes))
	}
	if err := takeError(); err != nil {
		t.Errorf("reported error: %v", err)
	}

	broken := NewFont("nocmap.ttf")
	if got, want := screen.TextWidth("AB", 10, TextOptions{Font: broken}), screen.TextWidth("AB", 10); got != want {
		t.Errorf("a broken font measured %v, want %v, as the built-in font", got, want)
	}
	wantError(t, `golib.NewFont("nocmap.ttf"): raylib could not read the font`)
	screen.TextWidth("AB", 10, options, options)
	wantError(t, "golib: Screen.TextWidth got 2 TextOptions: pass at most one")

	loadedFonts.unloadAll()
	if font.read || font.sizes != nil || font.tracked || len(loadedFonts.fonts) != 0 {
		t.Errorf("after unloadAll: read %v, %d sizes, tracked %v", font.read, len(font.sizes), font.tracked)
	}
}
