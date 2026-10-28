package main

import "golib"

type costPart struct {
	name    string
	words   string
	color   golib.Color
	missing bool
}

func resourceCosts(s *State, lilac, oil float64) []costPart {
	parts := []costPart{{
		name:    "lilac",
		words:   si(lilac, "kg"),
		color:   lilacColor,
		missing: s.Stock.Lilac < lilac,
	}}
	if oil > 0 {
		parts = append(parts, costPart{
			name:    "oil",
			words:   si(oil, "L"),
			color:   oilColor,
			missing: oilTotal(s) < oil,
		})
	}
	return parts
}

func costPartsWidth(
	screen *golib.Screen,
	parts []costPart,
	size float32,
) float32 {
	width := float32(0)
	for i, part := range parts {
		if i > 0 {
			width += screen.TextWidth(" + ", size, uiText)
		}
		width += screen.TextWidth(part.words, size, uiText)
	}
	return width
}

func drawCostParts(
	screen *golib.Screen,
	parts []costPart,
	x, y, size float32,
) {
	for i, part := range parts {
		if i > 0 {
			separator := " + "
			screen.DrawText(separator, x, y, size, panelDimColor, uiText)
			x += screen.TextWidth(separator, size, uiText)
		}
		width := screen.TextWidth(part.words, size, uiText)
		screen.DrawText(part.words, x, y, size, part.color, uiText)
		if part.missing {
			const slack = 2
			screen.DrawRectangleOutline(golib.Rectangle{
				X: x - slack, Y: y - slack,
				Width: width + 2*slack, Height: size + 2*slack,
			}, 1, dangerColor)
		}
		x += width
	}
}
