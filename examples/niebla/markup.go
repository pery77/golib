package main

import (
	"strings"

	"golib"
)

// markupSpan is a run of text in one color, ready to draw.
type markupSpan struct {
	text  string
	color golib.Color
}

// parseMarkup splits text into colored spans. "[name]" writes on in the
// palette color "name", and "[/]" goes back to the color before it, so
// colors nest; a name the palette doesn't know prints as it is. Example:
//
//	parseMarkup("an [oil]oil pool[/] holds", base)
func parseMarkup(text string, base golib.Color) []markupSpan {
	var spans []markupSpan
	var stack []golib.Color
	color := base
	start := 0
	flush := func(end int) {
		if end > start {
			spans = append(spans, markupSpan{text: text[start:end], color: color})
		}
	}
	for i := 0; i < len(text); i++ {
		if text[i] != '[' {
			continue
		}
		end := strings.IndexByte(text[i:], ']')
		if end < 0 {
			break
		}
		tag := text[i+1 : i+end]
		if !knownTag(tag) {
			continue
		}
		flush(i)
		if tag == "/" {
			if len(stack) > 0 {
				color = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			} else {
				color = base
			}
		} else {
			stack = append(stack, color)
			color = markupPalette[tag]
		}
		i += end
		start = i + 1
	}
	flush(len(text))
	return spans
}

// knownTag reports whether text holds a tag parseMarkup acts on.
func knownTag(tag string) bool {
	if tag == "/" {
		return true
	}
	_, ok := markupPalette[tag]
	return ok
}

// drawMarkup draws text with parseMarkup's colors, span by span, and
// returns the x just past it. TextWidth counts a span's trailing spaces,
// so advancing by it lands the next span where DrawText would have put it.
func drawMarkup(screen *golib.Screen, text string, x, y, size float32, base golib.Color) float32 {
	for _, span := range parseMarkup(text, base) {
		screen.DrawText(span.text, x, y, size, span.color, uiText)
		x += screen.TextWidth(span.text, size, uiText)
	}
	return x
}

func drawMarkupWrapped(
	screen *golib.Screen, text string, x, y, maxWidth, size float32,
	base golib.Color,
) float32 {
	start := x
	for _, span := range parseMarkup(text, base) {
		for _, word := range strings.Fields(span.text) {
			part := word
			if x > start {
				part = " " + word
			}
			width := screen.TextWidth(part, size, uiText)
			if x > start && x+width > start+maxWidth {
				x = start
				y += size + 3
				part = word
				width = screen.TextWidth(part, size, uiText)
			}
			screen.DrawText(part, x, y, size, span.color, uiText)
			x += width
		}
	}
	return y + size
}
