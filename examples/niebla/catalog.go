package main

import (
	"fmt"
	"math"
	"strings"

	"golib"
)

// ThingInfo is the catalog's entry for a thing type: what it is called, the
// color everything about it is written in, and the lines its card shows.
// The catalog is metadata, never state: it is the database the views read
// to print any entity. The card lines read the state, so they can say
// what remains in a deposit or what a robot is doing. A type it has no
// entry for still gets a name and a stable color out of the fallback, so
// new things print the moment they exist and get dressed later.
type ThingInfo struct {
	Name    string
	Color   golib.Color
	Unit    string                               // the SI unit of the type's Amount
	Primary bool                                 // the card starts open: deposits, buildings, the core
	Summary func(amount float64) string          // the card's headline; default si()
	Details func(s *State, thing Thing) []Detail // the expanded card's lines
}

// summarize returns the card's headline for a thing.
func (info ThingInfo) summarize(thing Thing) string {
	if thing.Caption != "" {
		return thing.Caption
	}
	if info.Summary != nil {
		return info.Summary(thing.Amount)
	}
	if info.Unit == "" {
		return fmt.Sprintf("%.0f", thing.Amount)
	}
	return si(thing.Amount, info.Unit)
}

// catalog holds an entry per thing type so far.
var catalog = map[ThingType]ThingInfo{
	TypeOil: {
		Name:    "Oil pool",
		Color:   oilColor,
		Unit:    "L",
		Primary: true,
		Details: func(s *State, thing Thing) []Detail {
			tcol, trow := cellTile(thing.CellCol, thing.CellRow)
			return []Detail{
				{"amount", fmt.Sprintf("[oil]%s[/]", si(thing.Amount, "L"))},
				{"pool", fmt.Sprintf("%.1f ha", thing.Area)},
				{"access", oilPoolAccess(s, tcol, trow)},
				{"state", depositState(thing.Amount, thing.Full)},
			}
		},
	},
	TypeLilac: {
		Name:    "Lilac vein",
		Color:   lilacColor,
		Unit:    "kg",
		Primary: true,
		Details: func(s *State, thing Thing) []Detail {
			return []Detail{
				{"amount", fmt.Sprintf("[lilac]%s[/]", si(thing.Amount, "kg"))},
				{"vein", fmt.Sprintf("%.1f ha", thing.Area)},
				{"state", depositState(thing.Amount, thing.Full)},
			}
		},
	},
	TypeCore: {
		Name:    "Repelling core",
		Color:   coreGlowColor,
		Unit:    "m",
		Primary: true,
		Summary: func(amount float64) string {
			return "r = " + si(amount, "m")
		},
		Details: func(s *State, thing Thing) []Detail {
			return []Detail{
				{"height", si(coreHeight, "m")},
				{"monolith", fmt.Sprintf("%d by %d m", coreSlabWide, coreSlabDeep)},
				{"bubble", "r = " + si(coreBubbleMeters(), "m")},
				{"upkeep", "none"},
				{"integrity", "[core]indestructible[/]"},
				{"oil tank", tankWords(s, coreTank)},
				{"available oil", fmt.Sprintf("[oil]%s[/] / %s",
					si(math.Round(oilTotal(s)), "L"), si(oilCap(s), "L"))},
				{"lilac store", fmt.Sprintf("[lilac]%s[/]", si(s.Stock.Lilac, "kg"))},
				{"robots", fmt.Sprintf("%d", len(s.Robots))},
			}
		},
	},
	TypeRobot: {
		Name:  "Robot",
		Color: robotColor,
		Details: func(s *State, thing Thing) []Detail {
			r, ok := s.Robots[thing.Ref]
			if !ok {
				return nil
			}
			model := "builder"
			switch r.Kind {
			case RobotBuilder:
				model = "builder"
			case RobotWorker:
				model = "worker"
			case RobotCombat:
				model = "trooper"
			case RobotRepair:
				model = "mechanic"
			}
			details := []Detail{
				{"task", robotCaption(s, r)},
				{"model", model},
			}
			healthName := "hull"
			if r.Kind == RobotCombat {
				healthName = "health"
			}
			health := robotMaxHealth(r.Kind)
			if health > 0 {
				details = append(details, Detail{
					healthName, fmt.Sprintf("%.0f / %.0f", math.Max(0, r.Health), health),
				})
			}
			if r.tanked() {
				details = append(details, Detail{
					"tank",
					fmt.Sprintf("[oil]%s[/] / %s",
						si(r.Tank, "L"), si(robotTankLiters, "L")),
				})
			}
			if r.Carry > 0 {
				unit, kind := "kg", r.Cargo
				if kind == TypeOil {
					unit = "L"
				}
				details = append(details, Detail{
					"carrying", fmt.Sprintf("[%s]%s[/]", kind, si(r.Carry, unit)),
				})
			}
			if r.hasPost() {
				details = append(details, Detail{
					"post", fmt.Sprintf("tile %d, %d", r.PostCol, r.PostRow),
				})
			}
			return details
		},
	},
	TypeFactory: {
		Name:    "Robot factory",
		Color:   factoryColor,
		Primary: true,
		Summary: func(amount float64) string {
			return "builds robots"
		},
		Details: func(s *State, thing Thing) []Detail {
			return []Detail{
				{"unit cost", fmt.Sprintf("[lilac]%s[/] + [oil]%s[/]",
					si(robotCostLilac, "kg"), si(robotCostOil, "L"))},
				{"builders", "build sites and lay pipes"},
				{"workers", "mine; builders carry a third as much"},
				{"pace", "one per " + si(factoryRobotTicks/60, "s")},
				{"robots", fmt.Sprintf("%d", len(s.Robots))},
			}
		},
	},
	TypeCharger: {
		Name:    "Robot charger",
		Color:   chargerColor,
		Primary: true,
		Summary: func(amount float64) string {
			return "refills tanks"
		},
		Details: func(s *State, thing Thing) []Detail {
			return []Detail{
				{"pace", si(chargerRefillPerSec, "L") + "/s"},
				{"tank", tankWords(s, thing.Ref)},
				{"serves", "from its own tank"},
				{"low tank", fmt.Sprintf("at %s", si(robotTankLiters*robotLowTankAt, "L"))},
			}
		},
	},
	TypeSilo: {
		Name:    "Oil silo",
		Color:   siloColor,
		Unit:    "L",
		Primary: true,
		Details: func(s *State, thing Thing) []Detail {
			return []Detail{
				{"tank", tankWords(s, thing.Ref)},
				{"available oil", fmt.Sprintf("[oil]%s[/] / %s",
					si(math.Round(oilTotal(s)), "L"), si(oilCap(s), "L"))},
			}
		},
	},
	TypeWarehouse: {
		Name:    "Mineral warehouse",
		Color:   warehouseColor,
		Unit:    "kg",
		Primary: true,
		Details: func(s *State, thing Thing) []Detail {
			return []Detail{
				{"adds", fmt.Sprintf("[lilac]%s[/]", si(warehouseLilacCap, "kg"))},
				{"colony lilac", fmt.Sprintf("[lilac]%s[/] / %s",
					si(s.Stock.Lilac, "kg"), si(lilacCap(s), "kg"))},
			}
		},
	},
	TypeProtector: {
		Name:    "Shadow protector",
		Color:   protectorColor,
		Unit:    "m",
		Primary: true,
		Summary: func(amount float64) string {
			return "r = " + si(amount, "m")
		},
		Details: func(s *State, thing Thing) []Detail {
			b, ok := s.Buildings[thing.Ref]
			if !ok {
				return nil
			}
			radius := protectorRadiusTiles(b) * unitsPerTile * unitMeters
			return []Detail{
				{"bubble", si(radius, "m") + " / " +
					si(protectorBubbleMeters(), "m")},
				{"oil tank", tankWords(s, b.ID)},
				{"upkeep", si(protectorOilPerSecond, "L") + "/s"},
				{"radius fades", fmt.Sprintf("below %.0f%% charge",
					protectorRadiusFadeBelow*100)},
				{"state", protectorState(b)},
				{"shelters", "buildings and robots"},
			}
		},
	},
	TypePump: {
		Name:    "Oil pump",
		Color:   pumpColor,
		Primary: true,
		Details: func(s *State, thing Thing) []Detail {
			return []Detail{
				{"pace", si(pumpLitersPerSecond, "L") + "/s"},
				{"pipe cost", fmt.Sprintf("[lilac]%s[/] per %s",
					si(pipeSectionLilac, "kg"), si(pipeSectionMeters, "m"))},
			}
		},
	},
	TypeGuard: {
		Name:    "Guard post",
		Color:   guardColor,
		Primary: true,
		Summary: func(amount float64) string {
			return "shoots rivals"
		},
		Details: func(s *State, thing Thing) []Detail {
			return []Detail{
				{"reach", si(smallArmsRangeUnits, "m")},
				{"shot", fmt.Sprintf("%.0f damage, every %.1f s",
					guardShotDamage, guardReloadTicks/60.0)},
				{"shot cost", fmt.Sprintf("[oil]%s[/], from any tank", si(guardShotOil, "L"))},
			}
		},
	},
	TypeWarFactory: {
		Name:    "War factory",
		Color:   warFactoryColor,
		Primary: true,
		Summary: func(amount float64) string {
			return "produces units"
		},
		Details: func(s *State, thing Thing) []Detail {
			details := []Detail{
				{"squad", squadWords(s, thing.Ref)},
				{"room", fmt.Sprintf("%d of %d troopers",
					len(squadMembers(s, thing.Ref)), squadSize)},
				{"trooper cost", costWords(trooperCostLilac, trooperCostOil)},
				{"pace", "one per " + si(trooperBuildTicks/60, "s")},
				{"trooper", fmt.Sprintf("%.0f health, reach %s",
					trooperHealth, si(smallArmsRangeUnits, "m"))},
				{"shot", fmt.Sprintf("%.0f damage every %.1f s, [oil]%s[/] of its tank",
					trooperShotDamage, trooperReloadTicks/60.0, si(trooperShotOil, "L"))},
			}
			if repairProtocolUnlocked(s) {
				details = append(details, []Detail{
					{"mechanic cost", costWords(mechanicCostLilac, mechanicCostOil)},
					{"mechanic pace", "one per " + si(mechanicBuildTicks/60, "s")},
					{"mechanic", fmt.Sprintf("%.0f hull, repairs %.0f damage/s",
						mechanicHealth, repairPerSecond)},
					{"repair price", fmt.Sprintf(
						"%.0f%% of blueprint cost from colony stores",
						repairCostShare*100,
					)},
					{"mechanic fuel", fmt.Sprintf("[oil]%s[/] per damage, from its tank",
						si(repairOilPerPoint, "L"))},
					{"mechanics", fmt.Sprintf("%d / %d",
						mechanicCount(s, thing.Ref), mechanicPerFactory)},
				}...)
			}
			return details
		},
	},
	TypeSquad: {
		Name:    "Squad",
		Color:   guardColor,
		Primary: true,
		Details: squadDetails,
	},
	TypeArtillery: {
		Name:    "Artillery",
		Color:   warFactoryColor,
		Primary: true,
		Summary: func(amount float64) string {
			return "shells what is seen"
		},
		Details: func(s *State, thing Thing) []Detail {
			return []Detail{
				{"reach", fmt.Sprintf("%s to %s",
					si(artilleryMinUnits, "m"), si(artilleryRangeUnits, "m"))},
				{"shell", fmt.Sprintf("%.0f damage within %s, every %s",
					artilleryShellDamage, si(shellBlastUnits, "m"),
					si(artilleryReloadTicks/60, "s"))},
				{"shell cost", costWords(artilleryShellLilac, artilleryShellOil)},
				{"fires at", "rivals within " + si(sightUnits, "m") + " of a robot or a building"},
			}
		},
	},
	TypeBase:           {Name: "Rival Nexus", Color: enemyLampColor, Primary: true, Details: enemyDetails},
	TypeCityRepulsor:   {Name: "Rival antimist post", Color: panelDimColor, Primary: true, Details: enemyDetails},
	TypeCityOilworks:   {Name: "Rival oil extractor", Color: panelDimColor, Primary: true, Details: enemyDetails},
	TypeCityMine:       {Name: "Rival mineral mine", Color: panelDimColor, Primary: true, Details: enemyDetails},
	TypeCityFactory:    {Name: "Rival war factory", Color: panelDimColor, Primary: true, Details: enemyDetails},
	TypeCityCrawler:    {Name: "Rival city crawler", Color: panelDimColor, Primary: true, Details: enemyDetails},
	TypeEnemyArtillery: {Name: "Rival mobile artillery", Color: enemyLampColor, Details: enemyDetails},
	TypeScout:          {Name: "Rival scout", Color: enemyLampColor, Details: enemyDetails},
	TypeCrawler:        {Name: "Rival crawler", Color: enemyLampColor, Details: enemyDetails},
	TypeRaider:         {Name: "Rival raider", Color: enemyLampColor, Details: enemyDetails},
	TypeSite: {
		Name:    "Building site",
		Color:   siteColor,
		Primary: true, // its Details are set in init: they read the catalog
	},
	TypePile: {
		Name:    "Loose items",
		Color:   pileColor,
		Primary: true,
		Details: func(s *State, thing Thing) []Detail {
			p, ok := s.Piles[thing.Ref]
			if !ok {
				return nil
			}
			var details []Detail
			if p.Lilac >= pileDust {
				details = append(details, Detail{
					"lilac", fmt.Sprintf("[lilac]%s[/]", si(math.Round(p.Lilac*10)/10, "kg")),
				})
			}
			if p.Oil >= pileDust {
				details = append(details, Detail{
					"oil", fmt.Sprintf("[oil]%s[/]", si(math.Round(p.Oil*10)/10, "L")),
				})
			}
			return append(details, Detail{"state", pileState(s, p)})
		},
	},
}

// enemyDetails are a rival vehicle's card lines: how much of it is left,
// what it carries and what it has stolen.
func enemyDetails(s *State, thing Thing) []Detail {
	e, ok := s.Enemies[thing.Ref]
	if !ok {
		return nil
	}
	spec := enemySpecOf(e.Kind)
	details := []Detail{
		{"health", fmt.Sprintf("%.0f / %.0f", math.Max(0, e.Health), spec.health)},
	}
	if spec.bubble > 0 {
		details = append(details, Detail{"repulsor", "r = " + si(spec.bubble, "m")})
	} else if e.Kind == EnemyCityCrawler {
		details = append(details, Detail{"repulsor", "none: construction vehicle"})
	} else {
		details = append(details, Detail{"repulsor", "none: it lives under its crawler's"})
	}
	if e.Kind == EnemyBase {
		details = append(details,
			Detail{"city", "produces mobile forces"},
			Detail{"static weapons", "none"})
	}
	if e.City != 0 {
		if city, ok := s.Cities[e.City]; ok {
			details = append(details,
				Detail{"city oil", si(math.Round(city.Oil), "L")},
				Detail{"city mineral", si(math.Round(city.Lilac), "kg")},
			)
			if name := cityConstructionName(s, city); name != "" {
				left := (city.Work + 59) / 60
				details = append(details, Detail{
					"building", fmt.Sprintf("%s, %d:%02d",
						name, left/60, left%60),
				})
			} else {
				left := city.NextSortie - s.Ticks
				if left < 0 {
					left = 0
				}
				details = append(details,
					Detail{"oil reserve", si(math.Round(city.OilDeposit), "L")},
					Detail{"mineral reserve", si(math.Round(city.LilacDeposit), "kg")},
					Detail{"forces sent", fmt.Sprintf("%d", city.Sorties)},
				)
				if status := cityPartyStatus(s, city.ID); status != "" {
					details = append(details, Detail{"active force", status})
				} else {
					details = append(details, Detail{
						"next force", fmt.Sprintf("%d:%02d", left/3600, left/60%60),
					})
				}
			}
		}
	}
	if spec.oilCap > 0 {
		details = append(details, Detail{
			"stolen", fmt.Sprintf("[oil]%s[/] / %s",
				si(math.Round(e.Oil*10)/10, "L"), si(spec.oilCap, "L")),
		})
	}
	return details
}

func squadDetails(s *State, thing Thing) []Detail {
	if _, ok := s.Buildings[thing.Ref]; !ok {
		return nil
	}
	sq := squadOf(s, thing.Ref)
	details := []Detail{
		{"troopers", fmt.Sprintf("%d / %d",
			len(squadMembers(s, thing.Ref)), squadSize)},
	}
	if sq.Order == OrderAttack {
		details = append(details, Detail{"order", "attack"})
		if enemy, ok := s.Enemies[sq.Focus]; ok {
			details = append(details, Detail{
				"target", string(enemy.Kind) + " first",
			})
		}
		return details
	}
	details = append(details, Detail{
		"order", fmt.Sprintf("guard at %s, %s",
			si(sq.X, "m"), si(sq.Y, "m")),
	})
	return details
}

// siteDetails are a site's card lines. They name the building from the
// catalog itself, so init hangs them on the entry: inside the catalog's
// own value they would be an initialization cycle.
func siteDetails(s *State, thing Thing) []Detail {
	job, ok := siteJob(s, thing)
	if !ok {
		return nil
	}
	lilac, oil := buildingCost(job.Kind)
	return []Detail{
		{"raising", catalogInfo(buildingType(job.Kind)).Name},
		{"work left", si(float64((job.Left+59)/60), "s")},
		{"paid", costWords(lilac, oil)},
	}
}

// pipeEndName names what a pipe starts or ends at.
func pipeEndName(s *State, end int64) string {
	if end == coreTank {
		return "the core"
	}
	return string(s.Buildings[end].Kind)
}

// pipeNote says a pipe in a few words, seen from one of its ends: where
// it goes or comes from, how long it is, and what it is doing.
func pipeNote(s *State, p Pipe, seenFrom int64) string {
	way, other := "to", p.To
	if p.To == seenFrom {
		way, other = "from", p.From
	}
	doing := "idle"
	switch {
	case p.Left > 0:
		doing = fmt.Sprintf("laid %.0f%%", pipeLaidPart(p)*100)
	case pipeFlowing(p):
		doing = "[oil]flowing[/]"
	}
	return fmt.Sprintf("%s %s, %s, %s", way, pipeEndName(s, other),
		si(float64(p.Sections)*pipeSectionMeters, "m"), doing)
}

// tankWords writes what a tank holds against what it holds at most.
func tankWords(s *State, tank int64) string {
	return fmt.Sprintf("[oil]%s[/] / %s",
		si(math.Round(tankOil(s, tank)*10)/10, "L"), si(tankCap(s, tank), "L"))
}

// costWords writes a cost in the resources' colors, the kinds it asks
// only.
func costWords(lilac, oil float64) string {
	words := fmt.Sprintf("[lilac]%s[/]", si(lilac, "kg"))
	if oil > 0 {
		words += fmt.Sprintf(" + [oil]%s[/]", si(oil, "L"))
	}
	return words
}

// pileState says whether the robots can take a pile home.
func pileState(s *State, p Pile) string {
	if _, amount := pileOffer(s, p, Robot{Kind: RobotWorker}); amount <= 0 {
		return "waiting for storage"
	}
	return "to be hauled"
}

// depositState says how a deposit tile reads after the robots worked it.
func depositState(amount, full float64) string {
	switch {
	case amount <= 0:
		return "dry"
	case amount < full:
		return "worked"
	}
	return "intact"
}

// catalogInfo returns the entry for a thing type. A type the catalog has no
// entry for yet takes its name from the type and its color from the type's
// name, which never changes.
func catalogInfo(kind ThingType) ThingInfo {
	if info, ok := catalog[kind]; ok {
		return info
	}
	name := string(kind)
	if name != "" {
		name = strings.ToUpper(name[:1]) + name[1:]
	}
	return ThingInfo{Name: name, Color: stableColor(string(kind))}
}

// stableColor turns a name into a color that stays the same for that name:
// the hue spreads the types around the wheel, washed out and bright enough
// to sit on the game's cold palette.
func stableColor(name string) golib.Color {
	h := 2166136261
	for _, r := range name {
		h ^= int(r)
		h *= 16777619
	}
	if h < 0 {
		h = -h
	}
	return hsv(float32(h%360)/360, 0.45, 0.85)
}

// hsv turns a color from hue (0 to 1), saturation and value into RGB.
func hsv(h, s, v float32) golib.Color {
	sector := h * 6
	i := int(sector) % 6
	f := sector - float32(int(sector))
	p := v * (1 - s)
	q := v * (1 - f*s)
	t := v * (1 - (1-f)*s)
	to8 := func(x float32) uint8 { return uint8(x * 255) }
	switch i {
	case 0:
		return golib.Color{R: to8(v), G: to8(t), B: to8(p), A: 255}
	case 1:
		return golib.Color{R: to8(q), G: to8(v), B: to8(p), A: 255}
	case 2:
		return golib.Color{R: to8(p), G: to8(v), B: to8(t), A: 255}
	case 3:
		return golib.Color{R: to8(p), G: to8(q), B: to8(v), A: 255}
	case 4:
		return golib.Color{R: to8(t), G: to8(p), B: to8(v), A: 255}
	default:
		return golib.Color{R: to8(v), G: to8(p), B: to8(q), A: 255}
	}
}

// markupPalette holds the colors text can name with markup, "[name]...[/]":
// one per thing type, from the catalog, plus a few that say how text is
// written rather than what it is about.
var markupPalette = map[string]golib.Color{}

func init() {
	site := catalog[TypeSite]
	site.Details = siteDetails
	catalog[TypeSite] = site
	for kind, info := range catalog {
		markupPalette[string(kind)] = info.Color
	}
	markupPalette["dim"] = panelDimColor
	markupPalette["light"] = panelTextColor
	markupPalette["fog"] = fogColor
	markupPalette["danger"] = dangerColor
}
