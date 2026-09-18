//go:build js

package device

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"syscall/js"
)

// Text on the web is drawn one textured quad per letter, from an atlas, and
// laid out by the same arithmetic raylib uses, so a line of text starts and
// ends where it does on the desktop and centered text lands on the same
// pixel.
//
// The built-in font's atlas is raylib's own, taken out of it by makefont.go,
// so those letters are the same pixels on both backends. A font from a .ttf
// or .otf file is drawn by the browser instead, which shapes letters a little
// differently from raylib: the text says the same thing in the same place,
// give or take a pixel.

//go:embed defaultfont.png
var defaultFontPNG []byte

// defaultFontID is the Font every backend hands back for the built-in font.
const defaultFontID = 1

// webFont is a font drawn at one size, ready to draw with.
type webFont struct {
	base    float32 // the height its letters were drawn at
	texture Texture
	glyphs  map[rune]defaultFontGlyph
	unknown defaultFontGlyph // the letter a missing one is drawn as
}

// fonts holds every font on the graphics card, by its number.
var (
	fonts    = map[uint32]*webFont{}
	nextFont = uint32(defaultFontID + 1)
)

// lineSpacing is the space raylib leaves under a line of text, in pixels,
// which package golib passes on to games as the gap between lines.
const lineSpacing = 2

// DefaultFont returns the built-in font, which needs no file.
func DefaultFont() Font {
	loadBuiltin()
	return Font{ID: defaultFontID}
}

// loadBuiltin puts raylib's own atlas on the graphics card and indexes its
// letters, the first time text is drawn.
func loadBuiltin() {
	if fonts[defaultFontID] != nil {
		return
	}
	font := &webFont{base: defaultFontBaseSize, glyphs: make(map[rune]defaultFontGlyph, len(defaultFontGlyphs))}
	for _, glyph := range defaultFontGlyphs {
		if _, seen := font.glyphs[glyph.Value]; !seen {
			font.glyphs[glyph.Value] = glyph
		}
	}
	font.unknown = font.glyphs['?']
	fonts[defaultFontID] = font
	picture, err := png.Decode(bytes.NewReader(defaultFontPNG))
	if err != nil {
		return // the atlas is built into the program; it cannot go missing
	}
	bounds := picture.Bounds()
	pixels := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(pixels, pixels.Bounds(), picture, bounds.Min, draw.Src)
	font.texture = NewTexture(pixels.Pix, bounds.Dx(), bounds.Dy())
}

// NewFont draws the letters in runes from a font file held in memory, at size
// pixels high, and returns them ready to draw with, or says why it couldn't.
// The browser reads the file and draws the letters; GoLib keeps the result as
// one picture, as raylib does.
func NewFont(format string, data []byte, size int, runes []rune) (Font, error) {
	loadBuiltin()
	file := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(file, data)
	made := readFont(file, size, string(runes))
	if made.IsNull() || !made.Truthy() {
		return Font{}, fmt.Errorf("a browser could not read this %s file as a font", format)
	}
	if reason := made.Get("error"); reason.Truthy() {
		return Font{}, fmt.Errorf("a browser could not read it: %s", reason.String())
	}

	width, height := made.Get("width").Int(), made.Get("height").Int()
	pixels := make([]byte, width*height*4)
	js.CopyBytesToGo(pixels, made.Get("pixels"))
	font := &webFont{base: float32(size), glyphs: map[rune]defaultFontGlyph{}}
	metrics := made.Get("glyphs")
	for i := 0; i < metrics.Length(); i += 8 {
		letter := rune(metrics.Index(i).Int())
		font.glyphs[letter] = defaultFontGlyph{
			Value:    letter,
			X:        float32(metrics.Index(i + 1).Float()),
			Y:        float32(metrics.Index(i + 2).Float()),
			Width:    float32(metrics.Index(i + 3).Float()),
			Height:   float32(metrics.Index(i + 4).Float()),
			OffsetX:  int32(metrics.Index(i + 5).Int()),
			OffsetY:  int32(metrics.Index(i + 6).Int()),
			AdvanceX: int32(metrics.Index(i + 7).Int()),
		}
	}
	font.unknown = font.glyphs['?']
	font.texture = NewTexture(pixels, width, height)
	id := nextFont
	nextFont++
	fonts[id] = font
	return Font{ID: id}, nil
}

// readFont waits for the browser to read the font file and draw its letters,
// which it does in its own time, as it does with sounds.
func readFont(file js.Value, size int, letters string) js.Value {
	done := make(chan js.Value, 1)
	var answer js.Func
	answer = js.FuncOf(func(_ js.Value, args []js.Value) any {
		answer.Release()
		done <- args[0]
		return nil
	})
	js_().Call("readFont", file, size, letters, answer)
	return <-done
}

// UnloadFont frees a font drawn at one size. The built-in one stays: it is
// the same picture for every game and every size.
func UnloadFont(font Font) {
	if font.ID == defaultFontID || font.ID == 0 {
		return
	}
	if held := fonts[font.ID]; held != nil {
		UnloadTexture(held.texture)
		delete(fonts, font.ID)
	}
}

// FontID identifies the texture a font's letters are drawn on.
func FontID(font Font) uint32 {
	return font.ID
}

// glyphOf returns the letter for a character, falling back to the question
// mark as raylib does for a character the font doesn't have.
func (f *webFont) glyphOf(letter rune) defaultFontGlyph {
	if glyph, ok := f.glyphs[letter]; ok {
		return glyph
	}
	return f.unknown
}

// DrawText draws text with its top-left corner at x, y, size pixels high,
// with spacing pixels between its letters. It walks the string the way
// raylib's DrawTextEx does, so both backends put the letters in the same
// places.
func DrawText(font Font, text string, x, y, size, spacing float32, c Color) {
	held := fonts[font.ID]
	if held == nil || held.texture.ID == 0 {
		return
	}
	scale := size / held.base
	offsetX, offsetY := float32(0), float32(0)
	for _, letter := range text {
		if letter == '\n' {
			offsetY += size + lineSpacing
			offsetX = 0
			continue
		}
		glyph := held.glyphOf(letter)
		if letter != ' ' && letter != '\t' {
			held.drawGlyph(glyph, x+offsetX, y+offsetY, scale, c)
		}
		if glyph.AdvanceX == 0 {
			offsetX += glyph.Width*scale + spacing
		} else {
			offsetX += float32(glyph.AdvanceX)*scale + spacing
		}
	}
}

// drawGlyph draws one letter with its pen position at x, y.
func (f *webFont) drawGlyph(glyph defaultFontGlyph, x, y, scale float32, c Color) {
	source := Rectangle{X: glyph.X, Y: glyph.Y, Width: glyph.Width, Height: glyph.Height}
	dest := Rectangle{
		X:      x + float32(glyph.OffsetX)*scale,
		Y:      y + float32(glyph.OffsetY)*scale,
		Width:  glyph.Width * scale,
		Height: glyph.Height * scale,
	}
	DrawTexture(f.texture, source, dest, Vector2{}, 0, c)
}

// TextWidth returns how wide DrawText would draw text, in pixels. It counts
// the way raylib's MeasureTextEx does, down to adding the spacing once per
// byte of the longest line rather than once per letter, so that text centered
// on the web lands where it lands on the desktop.
func TextWidth(font Font, text string, size, spacing float32) float32 {
	held := fonts[font.ID]
	if held == nil || text == "" {
		return 0
	}
	scale := size / held.base
	var width, widest float32
	bytesInLine, longestLine := 0, 0
	for i, letter := range text {
		// raylib counts the bytes it walks, not the letters.
		bytesInLine += runeBytes(text, i)
		if letter != '\n' {
			glyph := held.glyphOf(letter)
			if glyph.AdvanceX > 0 {
				width += float32(glyph.AdvanceX)
			} else {
				width += glyph.Width + float32(glyph.OffsetX)
			}
		} else {
			widest = max(widest, width)
			bytesInLine, width = 0, 0
		}
		longestLine = max(longestLine, bytesInLine)
	}
	widest = max(widest, width)
	return widest*scale + float32(longestLine-1)*spacing
}

// runeBytes returns how many bytes the character starting at i takes, for the
// byte counting raylib's MeasureTextEx does.
func runeBytes(text string, i int) int {
	for next := i + 1; next < len(text); next++ {
		if text[next]&0xc0 != 0x80 { // not a continuation byte
			return next - i
		}
	}
	return len(text) - i
}
