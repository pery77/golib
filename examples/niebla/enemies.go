package main

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"golib"
)

// The rivals' picture: the marks their scouts leave on the ground, their
// vehicles under their repulsors' pockets, the guard posts' shots, and
// the words that tell the player what they are up to. All of it reads
// the state (sim_enemies.go) and changes nothing.

const (
	reportShowTicks = 15 * 60 // ticks a report stays on the screen: 15 s

	markScale = 1.3 // the scout's doodle, about 40 u long
)

func latestReport(s *State) (Report, bool) {
	if len(s.Reports) == 0 {
		return Report{}, false
	}
	return s.Reports[len(s.Reports)-1], true
}

// drawMarks paints what the scouts left on the ground, in spray paint:
// the oldest drawing there is, flat on the ground, so everything walks
// over it.
func drawMarks(s *State, screen *golib.Screen, zoom float32) {
	thick := 2 * dotRadius(0.9, zoom, 0.75)
	for _, id := range sortedMarkIDs(s) {
		m := s.Marks[id]
		for _, stroke := range markStrokes() {
			for i := 1; i < len(stroke); i++ {
				ax, ay := project(
					float32(m.X+stroke[i-1].X*markScale),
					float32(m.Y+stroke[i-1].Y*markScale))
				bx, by := project(
					float32(m.X+stroke[i].X*markScale),
					float32(m.Y+stroke[i].Y*markScale))
				screen.DrawLine(ax, ay, bx, by, thick, markColor)
			}
		}
	}
}

// markStrokes returns the doodle's strokes, in units around its middle:
// two rounds and a long shape between them, closed by an arc.
func markStrokes() [][]PipePoint {
	arc := func(cx, cy, r, from, to float64) []PipePoint {
		const steps = 12
		points := make([]PipePoint, 0, steps+1)
		for i := 0; i <= steps; i++ {
			angle := from + (to-from)*float64(i)/steps
			points = append(points,
				PipePoint{cx + math.Cos(angle)*r, cy + math.Sin(angle)*r})
		}
		return points
	}
	shaft := []PipePoint{{-6, -4}}
	shaft = append(shaft, arc(16, 0, 4, -math.Pi/2, math.Pi/2)...)
	shaft = append(shaft, PipePoint{-6, 4})
	return [][]PipePoint{
		arc(-10, -5, 5, 0, 2*math.Pi),
		arc(-10, 5, 5, 0, 2*math.Pi),
		shaft,
	}
}

// drawEnemies paints the rivals over the fog, so a party reads from far
// out as what it is: a pocket of clear air moving through the mist. The
// pockets go first, then the structures and vehicles back to front, then
// the squads' marks (squads.go).
func drawEnemies(
	s *State, screen *golib.Screen, camera *golib.Camera, zoom float32,
) {
	type spot struct {
		e    Enemy
		x, y float32
	}
	spots := make([]spot, 0, len(s.Enemies))
	for _, id := range sortedEnemyIDs(s) {
		e := s.Enemies[id]
		x, y := project(float32(e.X), float32(e.Y))
		spots = append(spots, spot{e, x, y})
		if reach := enemySpecOf(e.Kind).bubble; reach > 0 {
			tiles := float32(reach / unitsPerTile)
			ellipseOutline(screen, x, y, tiles, 2/zoom, enemyEdgeColor)
		}
	}
	sort.SliceStable(spots, func(i, j int) bool { return spots[i].y < spots[j].y })
	for _, sp := range spots {
		if isRivalBuilding(sp.e) {
			continue
		}
		model, _ := rivalVehicleModel(sp.e.Kind)
		center := golib.Vector2{X: sp.x, Y: sp.y}
		model.drawShadow(screen, camera, center, sp.e.Facing, zoom)
	}
	for _, sp := range spots {
		if isRivalBuilding(sp.e) {
			drawCityBuilding(s, screen, sp.e, sp.x, sp.y, zoom)
			continue
		}
		drawVehicle(screen, camera, sp.e, sp.x, sp.y, zoom)
	}
	drawCityConstruction(s, screen, zoom)
	drawSquadMarks(s, screen, zoom)
}

func drawCityConstruction(s *State, screen *golib.Screen, zoom float32) {
	for _, id := range sortedCityIDs(s) {
		city := s.Cities[id]
		x, y, _, _, building := cityConstructionSite(s, city)
		if !building {
			continue
		}
		gx, gy := project(float32(x), float32(y))
		gray := golib.Color{R: 158, G: 169, B: 172, A: 255}
		w := dotRadius(11, zoom, 5)
		screen.DrawCircleOutline(gx, gy, w, 1/zoom, gray)
		bar := golib.Rectangle{
			X: gx - w, Y: gy + w + 2/zoom,
			Width: 2 * w, Height: 2 / zoom,
		}
		screen.DrawRectangle(bar, scarColor)
		progress := 1 - float32(city.Work)/cityBuildTicks
		bar.Width *= golib.Clamp(progress, 0, 1)
		screen.DrawRectangle(bar, gray)
	}
}

func drawCityBuilding(
	s *State,
	screen *golib.Screen,
	e Enemy,
	gx, gy, zoom float32,
) {
	gray := golib.Color{R: 119, G: 132, B: 137, A: 255}
	dark := golib.Color{R: 55, G: 65, B: 69, A: 255}
	light := golib.Color{R: 168, G: 178, B: 180, A: 255}
	k := buildingIcon(24, 18, zoom)
	switch e.Kind {
	case EnemyBase:
		isoSlab(screen, gx, gy, 30*k, 9*k, 15*k, gray, mid(gray, dark), dark)
		isoBox(screen, gx, gy-15*k*unitH, 6*k, 14*k, light, gray, dark)
	case EnemyCityRepulsor:
		isoBox(screen, gx, gy, 10*k, 24*k, gray, mid(gray, dark), dark)
		screen.DrawCircle(gx, gy-26*k*unitH, 4*k*unitW, light)
	case EnemyCityOilworks:
		isoBox(screen, gx, gy, 20*k, 12*k, gray, mid(gray, dark), dark)
		isoBox(screen, gx, gy-12*k*unitH, 5*k, 16*k, light, gray, dark)
		for bead := 0; bead < 3; bead++ {
			phase := math.Mod(float64(s.Ticks%90)/90+float64(bead)/3, 1)
			fade := float32(math.Sin(math.Pi * phase))
			y := gy - (30+float32(phase)*18)*k*unitH
			color := golib.WithOpacity(oilColor, fade*0.34)
			screen.DrawCircle(gx+float32(bead-1)*4*k*unitW,
				y, 2*k*unitW, color)
		}
	case EnemyCityMine:
		isoBox(screen, gx, gy, 23*k, 10*k, gray, mid(gray, dark), dark)
		screen.DrawLine(gx-7*k*unitW, gy-8*k*unitH,
			gx+7*k*unitW, gy-8*k*unitH, 2/zoom, light)
		for crystal := float32(-1); crystal <= 1; crystal++ {
			x := gx + crystal*6*k*unitW
			screen.DrawLine(x, gy-11*k*unitH,
				x+2*k*unitW, gy-15*k*unitH, 2/zoom, lilacColor)
		}
	case EnemyCityFactory:
		isoBox(screen, gx, gy, 26*k, 14*k, gray, mid(gray, dark), dark)
		isoBox(screen, gx+5*k*unitW, gy-14*k*unitH,
			7*k, 18*k, gray, dark, dark)
	}
	drawHealthBar(screen, gx, gy, 24*k, zoom,
		e.Health, enemySpecOf(e.Kind).health, dangerColor)
}

// drawHealthBar paints what is left of something hurt under its foot;
// nothing for what is whole.
func drawHealthBar(
	screen *golib.Screen,
	gx, gy, across, zoom float32,
	health, full float64,
	color golib.Color,
) {
	if health >= full {
		return
	}
	w := across * unitW
	bar := golib.Rectangle{
		X: gx - w/2, Y: gy + across*unitH/2 + 2/zoom, Width: w, Height: 2 / zoom,
	}
	screen.DrawRectangle(bar, fillBarColor)
	bar.Width *= float32(math.Max(0, health) / full)
	screen.DrawRectangle(bar, color)
}

// drawVehicle paints a rival rover pointing along its last movement.
func drawVehicle(
	screen *golib.Screen, camera *golib.Camera,
	e Enemy, gx, gy, zoom float32,
) {
	model, across := rivalVehicleModel(e.Kind)
	center := golib.Vector2{X: gx, Y: gy}
	model.draw(screen, camera, center, e.Facing, zoom)
	if e.Kind == EnemyRaider && e.Oil > 0 {
		scale := model.iconScale(zoom)
		size := across * unitW * scale * 0.4
		tank := center.Add(golib.Vector2{Y: -6.3 * unitH * scale})
		screen.DrawCircle(tank.X, tank.Y, size*0.22, oilColor)
	}
	drawHealthBar(screen, gx, gy, across, zoom, e.Health,
		enemySpecOf(e.Kind).health, dangerColor)
}

func rivalVehicleModel(kind EnemyKind) (worldSprite, float32) {
	model, across := rivalRaiderModel, float32(8)
	switch kind {
	case EnemyArtillery:
		model, across = rivalArtilleryModel, 16
	case EnemyCrawler:
		model, across = rivalCrawlerModel, 16
	case EnemyCityCrawler:
		model, across = rivalCityCrawlerModel, 16
	case EnemyScout:
		model, across = rivalScoutModel, 6
	}
	return model, across
}

// compassWord names the way from the core to a spot as the screen shows
// it: north is up.
func compassWord(x, y float64) string {
	cx, cy := tileCenterUnits(coreCol, coreRow)
	dx, dy := x-cx, y-cy
	angle := math.Atan2(-(dx+dy)/2, dx-dy)
	words := []string{
		"east", "north-east", "north", "north-west",
		"west", "south-west", "south", "south-east",
	}
	sector := int(math.Round(angle/(math.Pi/4))+8) % 8
	return words[sector]
}

// threatWords says what the rivals in the region are doing, for the
// HUD; "" while there are none.
func threatWords(s *State) string {
	for _, id := range sortedPartyIDs(s) {
		p := s.Parties[id]
		members := partyMembers(s, id)
		where := ""
		lead := Enemy{}
		if len(members) == 0 {
			if p.Stage != StageBuild {
				continue
			}
			city, exists := s.Cities[p.City]
			if !exists {
				continue
			}
			where = compassWord(city.X, city.Y)
		} else {
			lead = members[0]
			where = compassWord(lead.X, lead.Y)
		}
		switch p.Stage {
		case StageApproach:
			return "something moves in the mist, " + where
		case StageCamp:
			left := (p.Wait + 59) / 60
			return fmt.Sprintf("raiders camped %s, moving in %d:%02d",
				where, left/60, left%60)
		case StageRaid:
			if lead.Kind == EnemyScout {
				return "an intruder, " + where
			}
			return "raid under way, " + where
		case StageUnload:
			return "rival force unloading at its city, " + where
		case StageBuild:
			left := (p.Wait + 59) / 60
			return fmt.Sprintf(
				"rival city assembling a force, %s, next unit in %d:%02d",
				where, left/60, left%60,
			)
		case StageRebuild:
			left := (p.Wait + 59) / 60
			return fmt.Sprintf("rival force completing its ranks, %s, %d:%02d",
				where, left/60, left%60)
		case StageRegroup:
			left := (p.Wait + 59) / 60
			return fmt.Sprintf("rival force regrouping, %s, attack in %d:%02d",
				where, left/60, left%60)
		case StageSettled:
			continue
		default:
			return "rivals leaving, " + where
		}
	}
	for _, id := range sortedCityIDs(s) {
		city := s.Cities[id]
		if city.Ruined {
			continue
		}
		where := compassWord(city.X, city.Y)
		if cityNeedsCrawler(s, city) && cityHasStructures(s, city) {
			return fmt.Sprintf("rival city %s, rebuilding crawler", where)
		}
		stage, building := cityNextBuildingStage(s, city)
		if city.Stage == len(cityBuildOrder) && building &&
			cityHasStructures(s, city) {
			return fmt.Sprintf("rival city %s, rebuilding %s",
				where, cityBuildingName(cityBuildOrder[stage]))
		}
		if city.AnnounceUntil <= s.Ticks {
			continue
		}
		if building {
			return fmt.Sprintf("rival city %s, building %s",
				where, cityBuildingName(cityBuildOrder[stage]))
		}
		left := city.NextSortie - s.Ticks
		if left < 0 {
			left = 0
		}
		return fmt.Sprintf("rival city %s, next force in %d:%02d",
			where, left/3600, left/60%60)
	}
	return ""
}

// reportWords words a report for the player.
func reportWords(r Report) string {
	where := compassWord(r.X, r.Y)
	switch r.Kind {
	case ReportScout:
		return fmt.Sprintf("A scout siphoned [oil]%s[/] off your tanks and left its mark. "+
			"They know you are here. A guard post would stop the next visit.",
			si(math.Round(r.Oil), "L"))
	case ReportCamp:
		return fmt.Sprintf("[danger]Raiders have camped to the %s.[/] "+
			"They are getting ready: so should you.", where)
	case ReportRaid:
		return "[danger]The raiders are moving in[/], for the nearest tank with oil in it."
	case ReportLeft:
		if r.Oil < 1 {
			return "The rivals left empty-handed."
		}
		return fmt.Sprintf("The raiders got away with [oil]%s[/].",
			si(math.Round(r.Oil), "L"))
	case ReportDestroyed:
		return "The rivals are gone to the last vehicle. What they carried lies where they fell."
	case ReportSettled:
		return fmt.Sprintf("[danger]A rival city is establishing to the %s.[/] "+
			"Its extractors and war factory will make forces here.", where)
	case ReportGun:
		return fmt.Sprintf("A rival building to the %s is complete.", where)
	case ReportBaseDown:
		return fmt.Sprintf(
			"The rival city to the %s was razed. A crawler will seek a new site.",
			where,
		)
	case ReportSortie:
		return fmt.Sprintf("[danger]A rival battalion is attacking from the %s.[/]",
			where)
	case ReportCityIncoming:
		return fmt.Sprintf("[danger]A crawler approaches from the %s.[/] "+
			"It is looking for a place to establish a city.", where)
	case ReportCityBuilding:
		index := int(r.Stage)
		if index >= 0 && index < len(cityBuildOrder) {
			return fmt.Sprintf("The rival city to the %s completed its %s.",
				where, cityBuildingName(cityBuildOrder[index]))
		}
	case ReportCityCrawler:
		return fmt.Sprintf(
			"A construction crawler returned to the rival city to the %s.",
			where,
		)
	case ReportRazed:
		return fmt.Sprintf("[danger]A shell brought a building down, %s.[/] Half of it lies there as a pile.", where)
	case ReportPumpEaten:
		return "[danger]mites ate the pump![/] Build a protector over the pool first."
	case ReportMiteEaten:
		return fmt.Sprintf("[danger]The mites consumed a structure to the %s.[/]",
			where)
	}
	return ""
}

func drawReport(s *State, screen *golib.Screen, top, right float32) {
	r, visible := currentReport(s)
	if !visible {
		return
	}
	words := reportWords(r)
	var plain strings.Builder
	for _, span := range parseMarkup(words, panelTextColor) {
		plain.WriteString(span.text)
	}
	size := statusTextSize()
	width := min(screen.TextWidth(plain.String(), size, uiText),
		max(40, right-36))
	lines := techWrap(screen, plain.String(), width, size)
	x := (16 + right - width) / 2
	plate := golib.Rectangle{
		X: x - 10, Y: top, Width: width + 20,
		Height: float32(len(lines))*(size+3) + 12,
	}
	screen.DrawRectangle(plate, panelColor)
	screen.DrawRectangleOutline(plate, 1, dangerColor)
	drawMarkupWrapped(screen, words, x, top+6, width, size, panelTextColor)
}

func currentReport(s *State) (Report, bool) {
	if len(s.Reports) == 0 {
		return Report{}, false
	}
	r := s.Reports[len(s.Reports)-1]
	return r, s.Ticks-r.Tick < reportLifetime(r)
}

func reportLifetime(report Report) int64 {
	if report.Kind == ReportSettled {
		return cityAnnouncementTicks
	}
	return reportShowTicks
}
