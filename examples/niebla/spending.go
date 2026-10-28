package main

import (
	"fmt"
	"sort"
	"strings"

	"golib"
)

const (
	spendingPeriod          = 1.0
	protectorSpendingPeriod = 4.0
	spendingLife            = 1.2
	spendingRise            = 34.0
	spendingTextSize        = 14.0
)

type spendingKey struct {
	source costSource
	id     int64
}

type spendingTotal struct {
	x, y       float64
	oil, lilac float64
}

type spendingNumber struct {
	x, y   float64
	amount float64
	unit   string
	color  golib.Color
	lane   float32
	age    float32
}

type spendingField struct {
	pending map[spendingKey]spendingTotal
	numbers []spendingNumber
	elapsed map[costSource]float32
}

func newSpendingField() *spendingField {
	return &spendingField{
		pending: map[spendingKey]spendingTotal{},
		elapsed: map[costSource]float32{},
	}
}

func (f *spendingField) update(receipts []CostReceipt, dt float32) {
	for _, receipt := range receipts {
		if receipt.Continuous {
			key := spendingKey{source: receipt.Source, id: receipt.ID}
			total := f.pending[key]
			total.x, total.y = receipt.X, receipt.Y
			total.oil += receipt.Oil
			total.lilac += receipt.Lilac
			f.pending[key] = total
			if _, ok := f.elapsed[receipt.Source]; !ok {
				f.elapsed[receipt.Source] = 0
			}
			continue
		}
		f.addNumbers(receipt.X, receipt.Y, receipt.Lilac, receipt.Oil)
	}

	sources := make([]costSource, 0, len(f.elapsed))
	for source := range f.elapsed {
		sources = append(sources, source)
	}
	sort.Slice(sources, func(i, j int) bool {
		return sources[i] < sources[j]
	})
	for _, source := range sources {
		f.elapsed[source] += dt
		period := spendingPeriod
		if source == costProtector {
			period = protectorSpendingPeriod
		}
		for f.elapsed[source] >= float32(period) {
			f.flush(source)
			f.elapsed[source] -= float32(period)
		}
	}

	numbers := f.numbers[:0]
	for _, number := range f.numbers {
		number.age += dt
		if number.age < spendingLife {
			numbers = append(numbers, number)
		}
	}
	f.numbers = numbers
}

func (f *spendingField) flush(source costSource) {
	keys := make([]spendingKey, 0, len(f.pending))
	for key := range f.pending {
		if key.source == source {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].source != keys[j].source {
			return keys[i].source < keys[j].source
		}
		return keys[i].id < keys[j].id
	})
	for _, key := range keys {
		total := f.pending[key]
		f.addNumbers(total.x, total.y, total.lilac, total.oil)
		delete(f.pending, key)
	}
}

func (f *spendingField) addNumbers(x, y, lilac, oil float64) {
	if lilac > 0 && oil > 0 {
		f.addNumber(x, y, lilac, "kg", lilacLightColor, -1)
		f.addNumber(x, y, oil, "L", oilColor, 1)
		return
	}
	if lilac > 0 {
		f.addNumber(x, y, lilac, "kg", lilacLightColor, 0)
	}
	if oil > 0 {
		f.addNumber(x, y, oil, "L", oilColor, 0)
	}
}

func (f *spendingField) addNumber(
	x, y, amount float64,
	unit string,
	color golib.Color,
	lane float32,
) {
	if amount < 0.005 {
		return
	}
	f.numbers = append(f.numbers, spendingNumber{
		x: x, y: y, amount: amount, unit: unit,
		color: color, lane: lane,
	})
}

func (f *spendingField) draw(
	camera *golib.Camera,
	screen *golib.Screen,
) {
	for _, number := range f.numbers {
		worldX, worldY := project(float32(number.x), float32(number.y))
		at := camera.ToScreen(golib.Vector2{X: worldX, Y: worldY})
		words := spendingWords(number.amount, number.unit)
		width := screen.TextWidth(words, spendingTextSize, uiTextBold)
		alpha := 1 - number.age/spendingLife
		x := at.X + number.lane*24 - width/2
		y := at.Y - 18 - spendingRise*number.age
		plate := golib.Rectangle{
			X: x - 4, Y: y - 2, Width: width + 8,
			Height: spendingTextSize + 5,
		}
		screen.DrawRectangle(plate,
			golib.WithOpacity(panelColor, alpha*0.88))
		screen.DrawText(words, x, y, spendingTextSize,
			golib.WithOpacity(number.color, alpha), uiTextBold)
	}
}

func spendingWords(amount float64, unit string) string {
	if amount < 1 {
		value := strings.TrimRight(fmt.Sprintf("%.2f", amount), "0")
		value = strings.TrimRight(value, ".")
		return fmt.Sprintf("-%s %s", value, unit)
	}
	return fmt.Sprintf("-%s", si(amount, unit))
}
