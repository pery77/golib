//go:build js

package device

import (
	"bytes"
	_ "embed"
	"errors"
	"image"
	"image/draw"
	"image/png"
)

// Text on the web is drawn from the same atlas as on the desktop, one
// textured quad per letter, and laid out by the same arithmetic raylib uses,
// so a line of text starts and ends where it does on the desktop and centered
// text lands on the same pixel. defaultfont.png and defaultfont.go come from
// makefont.go, which takes them out of raylib.
//
// Fonts from .ttf and .otf files are stage 3 of the web build: nothing here
// rasterizes a font file yet.

//go:embed defaultfont.png
var defaultFontPNG []byte

// defaultFontID is the Font every backend hands back for the built-in font.
const defaultFontID = 1

// builtin is the atlas on the graphics card, put there the first time text is
// drawn, and the letters by their character.
var builtin struct {
	texture Texture
	loaded  bool
	byRune  map[rune]int
	unknown int // the letter raylib falls back to, its question mark
}

// lineSpacing is the space raylib leaves under a line of text, in pixels,
// which package golib passes on to games as the gap between lines.
const lineSpacing = 2

// DefaultFont returns the built-in font, which needs no file.
func DefaultFont() Font {
	return Font{ID: defaultFontID}
}

// NewFont reports that fonts from files are not in the web build yet. Package
// golib says so in the game's own words, naming the file.
func NewFont(format string, data []byte, size int, runes []rune) (Font, error) {
	return Font{}, errors.New("a web build has no fonts from files yet: draw with the built-in font, or play the game on the desktop")
}

// UnloadFont has nothing to free: the built-in font stays for the whole game.
func UnloadFont(font Font) {}

// FontID identifies the texture a font's letters are drawn on.
func FontID(font Font) uint32 {
	return font.ID
}

// loadBuiltin puts the atlas on the graphics card and indexes its letters,
// the first time text is drawn.
func loadBuiltin() {
	if builtin.loaded {
		return
	}
	builtin.loaded = true
	builtin.byRune = make(map[rune]int, len(defaultFontGlyphs))
	for i, glyph := range defaultFontGlyphs {
		if _, seen := builtin.byRune[glyph.Value]; !seen {
			builtin.byRune[glyph.Value] = i
		}
	}
	builtin.unknown = builtin.byRune['?']
	picture, err := png.Decode(bytes.NewReader(defaultFontPNG))
	if err != nil {
		return // the atlas is built into the program; it cannot go missing
	}
	bounds := picture.Bounds()
	pixels := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(pixels, pixels.Bounds(), picture, bounds.Min, draw.Src)
	builtin.texture = NewTexture(pixels.Pix, bounds.Dx(), bounds.Dy())
}

// glyphOf returns the letter for a character, falling back to the question
// mark as raylib does for a character the font doesn't have.
func glyphOf(letter rune) defaultFontGlyph {
	if i, ok := builtin.byRune[letter]; ok {
		return defaultFontGlyphs[i]
	}
	return defaultFontGlyphs[builtin.unknown]
}

// DrawText draws text with its top-left corner at x, y, size pixels high,
// with spacing pixels between its letters. It walks the string the way
// raylib's DrawTextEx does, so both backends put the letters in the same
// places.
func DrawText(font Font, text string, x, y, size, spacing float32, c Color) {
	loadBuiltin()
	if builtin.texture.ID == 0 {
		return
	}
	scale := size / defaultFontBaseSize
	offsetX, offsetY := float32(0), float32(0)
	for _, letter := range text {
		if letter == '\n' {
			offsetY += size + lineSpacing
			offsetX = 0
			continue
		}
		glyph := glyphOf(letter)
		if letter != ' ' && letter != '\t' {
			drawGlyph(glyph, x+offsetX, y+offsetY, scale, c)
		}
		if glyph.AdvanceX == 0 {
			offsetX += glyph.Width*scale + spacing
		} else {
			offsetX += float32(glyph.AdvanceX)*scale + spacing
		}
	}
}

// drawGlyph draws one letter with its pen position at x, y.
func drawGlyph(glyph defaultFontGlyph, x, y, scale float32, c Color) {
	const pad = defaultFontPadding
	source := Rectangle{
		X:      glyph.X - pad,
		Y:      glyph.Y - pad,
		Width:  glyph.Width + 2*pad,
		Height: glyph.Height + 2*pad,
	}
	dest := Rectangle{
		X:      x + float32(glyph.OffsetX)*scale - pad*scale,
		Y:      y + float32(glyph.OffsetY)*scale - pad*scale,
		Width:  (glyph.Width + 2*pad) * scale,
		Height: (glyph.Height + 2*pad) * scale,
	}
	DrawTexture(builtin.texture, source, dest, Vector2{}, 0, c)
}

// TextWidth returns how wide DrawText would draw text, in pixels. It counts
// the way raylib's MeasureTextEx does, down to adding the spacing once per
// byte of the longest line rather than once per letter, so that text centered
// on the web lands where it lands on the desktop.
func TextWidth(font Font, text string, size, spacing float32) float32 {
	loadBuiltin()
	if text == "" {
		return 0
	}
	scale := size / defaultFontBaseSize
	var width, widest float32
	bytesInLine, longestLine := 0, 0
	for i, letter := range text {
		// raylib counts the bytes it walks, not the letters.
		bytesInLine += runeBytes(text, i)
		if letter != '\n' {
			glyph := glyphOf(letter)
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
