package main

import (
	"math"
	"testing"
)

func TestProjectionFitsTheRegionOnScreen(t *testing.T) {
	if x, _ := projectTile(coreCol, coreRow); x != regionOriginX {
		t.Errorf("the core projects at x %v, want the middle %v", x, regionOriginX)
	}
	corners := [][2]float32{
		{0, 0},
		{regionCols - 1, 0},
		{regionCols - 1, regionRows - 1},
		{0, regionRows - 1},
	}
	for _, c := range corners {
		x, y := projectTile(c[0], c[1])
		if x < 0 || x > float32(screenWidth) ||
			y < 0 || y > float32(screenHeight) {
			t.Errorf("corner %v, %v projects at %v, %v, off the screen",
				c[0], c[1], x, y)
		}
	}
}

// TestUnitsFillATile pins the world's unit: unitsPerTile of them span one
// tile, so the core's 5 u pole is a tile across and a 10 u building is two.
func TestUnitsFillATile(t *testing.T) {
	x0, y0 := project(0, 0)
	x1, y1 := project(unitsPerTile, 0)
	if math.Abs(float64(x1-x0-tileW/2)) > 0.01 || math.Abs(float64(y1-y0-tileH/2)) > 0.01 {
		t.Errorf("%d units span %v by %v pixels, want one tile, %v by %v",
			unitsPerTile, x1-x0, y1-y0, tileW/2, tileH/2)
	}
}

func TestRegionOnScreenCoversTheDiamond(t *testing.T) {
	bounds := regionOnScreen()
	corners := [][2]float32{
		{0, 0},
		{regionCols - 1, 0},
		{regionCols - 1, regionRows - 1},
		{0, regionRows - 1},
	}
	for _, c := range corners {
		x, y := projectTile(c[0], c[1])
		if x < bounds.X || x > bounds.X+bounds.Width ||
			y < bounds.Y || y > bounds.Y+bounds.Height {
			t.Errorf("corner %v, %v projects at %v, %v, outside the camera bounds %v",
				c[0], c[1], x, y, bounds)
		}
	}
}

func TestCirclesFlattenAsTheProjectionSays(t *testing.T) {
	halfW, halfH := ellipseSemiAxes(1)
	want := float32(tileW) / float32(tileH)
	if got := halfW / halfH; math.Abs(float64(got-want)) > 0.001 {
		t.Errorf("a world circle flattens to %v wide for %v high, want %v",
			got, want, 1/want)
	}
}

func TestThingsAtKnowsTheRegion(t *testing.T) {
	s := newGame()
	var firstOil, firstLilac [2]int
	foundOil, foundLilac := false, false
	for row := 0; row < regionRows && !(foundOil && foundLilac); row++ {
		for col := 0; col < regionCols; col++ {
			if !foundOil && tileAt(col, row) == kindOil {
				firstOil = [2]int{col, row}
				foundOil = true
			}
			if !foundLilac && tileAt(col, row) == kindLilac {
				firstLilac = [2]int{col, row}
				foundLilac = true
			}
		}
	}
	if !foundOil || !foundLilac {
		t.Fatalf("the layout has no oil pool or no lilac vein to test")
	}

	// A deposit tile shows its whole patch's card, at the patch's amount.
	at := func(tile [2]int) []Thing {
		col, row := tileCell(tile[0], tile[1])
		return thingsAt(s, col, row)
	}
	if things := at(firstOil); len(things) != 1 || things[0].Type != TypeOil {
		t.Errorf("thingsAt an oil tile returned %v, want one oil thing", things)
	} else {
		patch, _ := depositAt(firstOil[0], firstOil[1])
		if things[0].Amount != depositFull(patch) {
			t.Errorf("the oil patch holds %v, want %v", things[0].Amount, depositFull(patch))
		}
		if things[0].ID == "" {
			t.Errorf("an oil thing without an ID, want one to remember it by")
		}
		// Another tile of the same patch shows the same card: the patch
		// is one thing.
		other, ok := otherPatchTile(patch, firstOil)
		if !ok {
			t.Fatalf("the oil patch has no second tile to test with")
		}
		again := at(other)
		if len(again) != 1 || again[0].ID != things[0].ID {
			t.Errorf("thingsAt %v, %v returned %v, want the patch's one card %v",
				other[0], other[1], again, things[0])
		}
	}

	if things := at(firstLilac); len(things) != 1 || things[0].Type != TypeLilac {
		t.Errorf("thingsAt a lilac tile returned %v, want one lilac thing", things)
	} else {
		patch, _ := depositAt(firstLilac[0], firstLilac[1])
		if things[0].Amount != depositFull(patch) {
			t.Errorf("the lilac patch holds %v, want %v", things[0].Amount, depositFull(patch))
		}
	}

	core := at([2]int{coreCol, coreRow})
	if len(core) != 1 || core[0].Type != TypeCore {
		t.Fatalf("thingsAt the core returned %v, want one core thing", core)
	}
	if core[0].Amount != coreBubbleMeters() {
		t.Errorf("the core's headline is %v, want the bubble radius %v",
			core[0].Amount, coreBubbleMeters())
	}

	if things := thingsAt(s, 0, 0); things != nil {
		t.Errorf("thingsAt bare ground returned %v, want none", things)
	}

	// Each robot's card shows on the cell it stands on.
	for _, id := range sortedRobotIDs(s) {
		r := s.Robots[id]
		col, row := robotCell(r)
		found := false
		for _, thing := range thingsAt(s, col, row) {
			if thing.Type == TypeRobot && thing.Ref == id {
				found = true
			}
		}
		if !found {
			t.Errorf("robot %d stands on cell %d, %d but its card is missing",
				id, col, row)
		}
	}
}

// otherPatchTile returns another tile of the given deposit patch.
func otherPatchTile(d Deposit, not [2]int) ([2]int, bool) {
	for _, tile := range depositTiles(d) {
		if tile != not {
			return tile, true
		}
	}
	return [2]int{}, false
}

func TestTileAtWorldUndoesProject(t *testing.T) {
	for _, c := range [][2]int{
		{0, 0}, {regionCols - 1, regionRows - 1}, {coreCol, coreRow}, {7, 3},
	} {
		x, y := projectTile(float32(c[0])+0.5, float32(c[1])+0.5)
		col, row, inside := tileAtWorld(x, y)
		if col != c[0] || row != c[1] || !inside {
			t.Errorf("the middle of tile %d, %d lands back on %d, %d, inside %v",
				c[0], c[1], col, row, inside)
		}
	}
	if _, _, inside := tileAtWorld(regionOriginX-4000, regionOriginY); inside {
		t.Errorf("a point far from the region landed inside it")
	}
}

func TestSiFormatsInternational(t *testing.T) {
	cases := []struct {
		value float64
		unit  string
		want  string
	}{
		{900, "L", "900 L"},
		{300, "kg", "300 kg"},
		{20, "m", "20 m"},
		{14.5, "m", "14.5 m"},
		{1200, "L", "1.2 kL"},
		{1500, "kg", "1.5 t"},
	}
	for _, c := range cases {
		if got := si(c.value, c.unit); got != c.want {
			t.Errorf("si(%v, %q) = %q, want %q", c.value, c.unit, got, c.want)
		}
	}
}

func TestCatalogColorsAreStable(t *testing.T) {
	for _, kind := range []ThingType{TypeOil, TypeLilac, TypeCore, "robot"} {
		if catalogInfo(kind).Color != catalogInfo(kind).Color {
			t.Errorf("the color of %q changes between reads", kind)
		}
	}
	if name := catalogInfo("robot").Name; name != "Robot" {
		t.Errorf("a type without an entry is named %q, want \"Robot\"", name)
	}
	if info := catalogInfo(TypeOil); info.Name != "Oil pool" || info.Unit != "L" {
		t.Errorf("the oil entry reads %q, %q, want \"Oil pool\", \"L\"",
			info.Name, info.Unit)
	}
	if got := catalogInfo(TypeCore).summarize(Thing{Amount: coreBubbleMeters()}); got != "r = 800 m" {
		t.Errorf("the core's headline is %q, want \"r = 800 m\"", got)
	}
	if got := catalogInfo(TypeOil).summarize(Thing{Amount: 900}); got != "900 L" {
		t.Errorf("the oil headline is %q, want \"900 L\"", got)
	}
	if got := catalogInfo(TypeRobot).summarize(Thing{Caption: "oil run"}); got != "oil run" {
		t.Errorf("the robot headline is %q, want its caption \"oil run\"", got)
	}
}
