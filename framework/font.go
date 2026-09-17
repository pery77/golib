package golib

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"path"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// fontFormats are the font files GoLib reads, by extension.
var fontFormats = []string{".ttf", ".otf"}

// maxFontSizes is how many sizes of one font GoLib keeps ready to draw. Each
// is a texture with the font's letters drawn at that size; when a game uses
// one more, the size used longest ago goes.
const maxFontSizes = 8

// Font is a TrueType or OpenType font in the game's assets folder, for
// [Screen.DrawText] and [Screen.TextWidth]:
//
//	var titleFont = golib.NewFont("fonts/title.ttf") // games/<game>/assets/fonts/title.ttf
//
//	screen.DrawText("Hello", 20, 20, 32, golib.White, golib.TextOptions{Font: titleFont})
//
// The file is read the first time text is drawn or measured with the font.
// So that text stays sharp, GoLib draws the font's letters at each size the
// game uses, the first time it uses that size: draw text at a few sizes, not
// at a new size in every frame. Letters beyond English, such as ñ or ü, are
// added the first time they are drawn. A letter the font doesn't have shows
// as its question mark.
type Font struct {
	name    string
	read    bool // the file has been read, successfully or not
	err     error
	data    []byte
	sizes   map[int]*fontSize // by height in pixels
	clock   int               // counts uses, to find the size used longest ago
	tracked bool              // in loadedFonts
}

// fontSize is a font drawn at one size.
type fontSize struct {
	font  rl.Font
	runes []rune // the letters it was asked for, in order
	asked map[rune]bool
	used  int // the font's clock when it was last used
}

// NewFont returns the font in the game's assets folder named name, which is
// relative to that folder and uses forward slashes, as in [ReadAsset]: a .ttf
// or .otf file. A file that is missing or can't be read stops Run with an
// error the first time text is drawn with the font.
func NewFont(name string) *Font {
	return &Font{name: name}
}

// TextOptions changes how [Screen.DrawText] draws text and how
// [Screen.TextWidth] measures it. Fields left at their zero value change
// nothing.
type TextOptions struct {
	// Font is the font to draw with. Default: GoLib's built-in pixel font,
	// which is 10 pixels high.
	Font *Font

	// Align says which part of each line DrawText puts at x: its start, with
	// AlignLeft, the default, its middle, with AlignCenter, or its end, with
	// AlignRight.
	Align TextAlign
}

// TextAlign says which part of each line of text [Screen.DrawText] puts at x.
type TextAlign int

// Text alignments.
const (
	AlignLeft   TextAlign = iota // x is where each line starts
	AlignCenter                  // x is the middle of each line
	AlignRight                   // x is where each line ends
)

// textLineGap is the space between lines of text, in pixels: raylib's.
const textLineGap = 2

// textFont returns the raylib font to draw text at size with, and the space
// between its letters. call names the method, for messages.
func textFont(call, text string, size float32, options []TextOptions) (rl.Font, float32) {
	if len(options) > 1 {
		reportError(fmt.Errorf("golib: Screen.%s got %d TextOptions: pass at most one", call, len(options)))
	}
	if len(options) == 0 || options[0].Font == nil {
		return rl.GetFontDefault(), textSpacing(size)
	}
	font, err := options[0].Font.atlas(size, text)
	if err != nil {
		// Draw with the built-in font this time: Run stops after the frame.
		reportError(err)
		return rl.GetFontDefault(), textSpacing(size)
	}
	return font, 0
}

// textSpacing matches raylib's DrawText: one pixel between letters for every
// 10 pixels of text height. Only the built-in font needs it.
func textSpacing(size float32) float32 {
	return float32(math.Floor(float64(max(size, 10)) / 10))
}

// atlas returns the font drawn at size, the nearest whole number of pixels,
// with every letter of text in it. The window must be open.
func (f *Font) atlas(size float32, text string) (rl.Font, error) {
	if err := f.prepare(); err != nil {
		return rl.Font{}, err
	}
	pixels := max(1, int(math.Round(float64(size))))
	f.clock++
	sized := f.sizes[pixels]
	if sized == nil {
		if len(f.sizes) >= maxFontSizes {
			f.dropOldest()
		}
		sized = &fontSize{asked: map[rune]bool{}}
		for letter := rune(' '); letter <= '~'; letter++ {
			sized.ask(letter)
		}
		f.sizes[pixels] = sized
	}
	sized.used = f.clock
	missing := false
	for _, letter := range text {
		if letter >= ' ' && letter != utf8.RuneError && !sized.asked[letter] {
			sized.ask(letter)
			missing = true
		}
	}
	if sized.font.Texture.ID == 0 || missing {
		if err := f.build(sized, pixels); err != nil {
			return rl.Font{}, err
		}
	}
	return sized.font, nil
}

func (s *fontSize) ask(letter rune) {
	s.asked[letter] = true
	s.runes = append(s.runes, letter)
}

// build draws the letters sized asks for, replacing the ones drawn before.
func (f *Font) build(sized *fontSize, pixels int) error {
	// raylib warns about every letter taller than the size, which many fonts
	// have, such as capitals with accents.
	rl.SetTraceLogLevel(rl.LogError)
	font := rl.LoadFontFromMemory(strings.ToLower(path.Ext(f.name)), f.data, int32(pixels), sized.runes)
	rl.SetTraceLogLevel(rl.LogWarning)
	// raylib gives its own font back when it can't read the file.
	if font.Texture.ID == 0 || font.Texture.ID == rl.GetFontDefault().Texture.ID {
		f.err = fmt.Errorf("golib.NewFont(%q): raylib could not read the font: check that the file is a TrueType or OpenType font", f.name)
		return f.err
	}
	unloadFont(sized.font)
	if !f.tracked {
		f.tracked = true
		loadedFonts.add(f)
	}
	sized.font = font
	return nil
}

// dropOldest frees the size used longest ago.
func (f *Font) dropOldest() {
	oldest := -1
	for pixels, sized := range f.sizes {
		if oldest < 0 || sized.used < f.sizes[oldest].used {
			oldest = pixels
		}
	}
	unloadFont(f.sizes[oldest].font)
	delete(f.sizes, oldest)
}

// unloadFont frees a font drawn at one size. raylib may still hold text
// drawn with it, waiting to go to the graphics card, so that goes first.
func unloadFont(font rl.Font) {
	if font.Texture.ID != 0 {
		rl.DrawRenderBatchActive()
		rl.UnloadFont(font)
	}
}

// prepare reads the file the first time the font is used.
func (f *Font) prepare() error {
	if !f.read {
		f.read = true
		f.sizes = map[int]*fontSize{}
		if err := f.readFile(); err != nil {
			f.err = fmt.Errorf("golib.NewFont(%q): %w", f.name, err)
		}
	}
	return f.err
}

func (f *Font) readFile() error {
	format := strings.ToLower(path.Ext(f.name))
	if !slices.Contains(fontFormats, format) {
		return fmt.Errorf("GoLib reads fonts from %s files, not %q ones", strings.Join(fontFormats, " and "), format)
	}
	data, err := ReadAsset(f.name)
	if err != nil {
		return err
	}
	if err := checkFontTables(data); err != nil {
		return err
	}
	f.data = data
	return nil
}

// checkFontTables checks that data starts as a TrueType or OpenType font
// does, with its tables inside the file. raylib's font reader trusts the
// file, and would read past its end otherwise.
func checkFontTables(data []byte) error {
	if len(data) >= 4 && string(data[:4]) == "ttcf" {
		return errors.New("the file is a font collection: GoLib reads one font per file, so save the font you want on its own as .ttf or .otf")
	}
	notFont := errors.New("the file isn't a TrueType or OpenType font, or it is damaged")
	if len(data) < 12 {
		return notFont
	}
	switch string(data[:4]) {
	case "\x00\x01\x00\x00", "true", "OTTO":
	default:
		return notFont
	}
	tables := int(binary.BigEndian.Uint16(data[4:]))
	if len(data) < 12+16*tables {
		return notFont
	}
	for i := range tables {
		record := data[12+16*i:]
		offset, length := binary.BigEndian.Uint32(record[8:]), binary.BigEndian.Uint32(record[12:])
		if uint64(offset)+uint64(length) > uint64(len(data)) {
			return notFont
		}
	}
	return nil
}

// unload frees every size of the font, so that the file is read again if a
// game runs again. The window must still be open.
func (f *Font) unload() {
	for _, sized := range f.sizes {
		if sized.font.Texture.ID != 0 {
			rl.UnloadFont(sized.font)
		}
	}
	f.read, f.err, f.data, f.sizes, f.clock, f.tracked = false, nil, nil, nil, 0, false
}

// loadedFonts are the fonts with sizes drawn, for Run to free when it ends.
var loadedFonts fontList

type fontList struct {
	sync.Mutex
	fonts []*Font
}

func (l *fontList) add(f *Font) {
	l.Lock()
	defer l.Unlock()
	l.fonts = append(l.fonts, f)
}

// unloadAll frees every loaded font. The window must still be open.
func (l *fontList) unloadAll() {
	l.Lock()
	fonts := l.fonts
	l.fonts = nil
	l.Unlock()
	for _, f := range fonts {
		f.unload()
	}
}
