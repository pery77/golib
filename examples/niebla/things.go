package main

import (
	"fmt"
	"math"
	"strings"
)

// The world speaks SI: one unit of the world is one meter, so a tile is
// 200 m across, oil is measured in liters and lilac in kilograms. What a
// deposit holds is its cells' richness times the ore's density
// (worldgen.go).
const (
	unitMeters = 1 // one world unit, in m

	// The core is a monolith in the old proportions, 1 by 4 by 9.
	coreSlabDeep = 4  // m through the monolith
	coreSlabWide = 16 // m across its broad face
	coreHeight   = 36 // m of monolith above the ground
)

// coreBubbleMeters returns the core's bubble radius in meters.
func coreBubbleMeters() float64 {
	return float64(coreBubbleRadius * unitsPerTile * unitMeters)
}

// protectorBubbleMeters returns a shadow protector's bubble radius in
// meters.
func protectorBubbleMeters() float64 {
	return float64(protectorBubbleTiles * unitsPerTile * unitMeters)
}

// ThingType names a kind of thing the world can hold: a resource in the
// ground, a building, a robot. The catalog (catalog.go) holds what each
// type is called and the color it is written in.
type ThingType string

// The thing types of the region so far.
const (
	TypeOil   ThingType = "oil"
	TypeLilac ThingType = "lilac"
	TypeCore  ThingType = "core"
	TypeRobot ThingType = "robot"
	TypeSquad ThingType = "squad"

	TypeFactory   ThingType = "factory"
	TypeCharger   ThingType = "charger"
	TypeSilo      ThingType = "silo"
	TypeWarehouse ThingType = "warehouse"
	TypeProtector ThingType = "protector"
	TypePump      ThingType = "pump"
	TypeGuard     ThingType = "guard"

	TypeWarFactory ThingType = "warfactory"
	TypeArtillery  ThingType = "artillery"

	TypeScout          ThingType = "scout" // the rivals' vehicles
	TypeCrawler        ThingType = "crawler"
	TypeCityCrawler    ThingType = "citycrawler"
	TypeRaider         ThingType = "raider"
	TypeBase           ThingType = "base"
	TypeCityRepulsor   ThingType = "cityrepulsor"
	TypeCityOilworks   ThingType = "cityoilworks"
	TypeCityMine       ThingType = "citymine"
	TypeCityFactory    ThingType = "cityfactory"
	TypeEnemyArtillery ThingType = "enemyartillery"

	TypeSite ThingType = "site" // a building still being raised
	TypePile ThingType = "pile" // loose items on the ground
)

// buildingType maps a building kind to the thing type its cards read.
func buildingType(kind BuildingKind) ThingType {
	switch kind {
	case BuildingFactory:
		return TypeFactory
	case BuildingCharger:
		return TypeCharger
	case BuildingSilo:
		return TypeSilo
	case BuildingWarehouse:
		return TypeWarehouse
	case BuildingProtector:
		return TypeProtector
	case BuildingPump:
		return TypePump
	case BuildingGuard:
		return TypeGuard
	case BuildingWarFactory:
		return TypeWarFactory
	case BuildingArtillery:
		return TypeArtillery
	}
	return TypeRobot
}

// Detail is one line of a thing's expanded card: a label and a value. The
// value may carry markup, such as "[oil]900 L[/]".
type Detail struct {
	Label string
	Value string
}

// Thing is one thing the world holds, seen from a tile: the view-side
// snapshot the cards render. It carries no behavior; what it means and
// how it reads live in the catalog. Robots come from the state; a
// deposit's amount is what remains of its whole patch in it.
type Thing struct {
	Type    ThingType
	ID      string  // stable in the region, so views can remember it
	Amount  float64 // the type's headline quantity, in its SI unit
	Caption string  // a headline that replaces the summary, a robot's task
	Ref     int64   // the entity's ID in the state: robots, buildings, piles
	CellCol int     // deposits: heart cell; sites: the cell the job stands on
	CellRow int     //
	Full    float64 // deposits: what the body held at first
	Area    float64 // deposits: the ground its ore covers, in hectares
}

// thingsAt returns what stands on a cell, the unit the player picks and
// counts by: the deposit or core under it, then its building, site and
// pile, then the robots on it by ID. A deposit's card spans its whole
// patch, but buildings and sites belong only to their own cell.
func thingsAt(s *State, col, row int) []Thing {
	var things []Thing
	tcol, trow := cellTile(col, row)
	pump := pumpOnCell(s, col, row)
	switch tileAt(tcol, trow) {
	case kindOil, kindLilac:
		if !pump {
			d, _ := depositAt(tcol, trow)
			thing := depositThing(d)
			thing.Amount = remainingAt(s, tcol, trow)
			things = append(things, thing)
		}
	case kindCore:
		things = append(things, coreAt(tcol, trow))
	}
	for _, id := range sortedBuildingIDs(s) {
		b := s.Buildings[id]
		if b.Col != col || b.Row != row {
			continue
		}
		thing := buildingThing(b)
		switch b.Kind {
		case BuildingPump:
			thing.Caption = pumpStatus(s, b)
		case BuildingSilo:
			thing.Amount = math.Round(b.Oil)
		case BuildingCharger:
			thing.Caption = si(math.Round(b.Oil), "L")
		case BuildingProtector:
			thing.Amount = protectorRadiusTiles(b) *
				unitsPerTile * unitMeters
		}
		things = append(things, thing)
	}
	for _, job := range s.Jobs {
		if job.Col != col || job.Row != row {
			continue
		}
		things = append(things, siteThing(job))
	}
	if p, littered := pileAt(s, col, row); littered {
		things = append(things, pileThing(p))
	}
	for _, r := range robotsOnCell(s, col, row) {
		things = append(things, robotThing(s, r))
	}
	for _, e := range enemiesOnCell(s, col, row) {
		things = append(things, enemyThing(s, e))
	}
	return things
}

// pumpOnCell reports whether a pump or its site occupies this cell.
func pumpOnCell(s *State, col, row int) bool {
	for _, id := range sortedBuildingIDs(s) {
		b := s.Buildings[id]
		if b.Kind == BuildingPump && b.Col == col && b.Row == row {
			return true
		}
	}
	for _, job := range s.Jobs {
		if job.Kind == BuildingPump && job.Col == col && job.Row == row {
			return true
		}
	}
	return false
}

// enemiesOnCell returns the rival vehicles standing on a cell, in ID
// order.
func enemiesOnCell(s *State, col, row int) []Enemy {
	var found []Enemy
	for _, id := range sortedEnemyIDs(s) {
		e := s.Enemies[id]
		if int(math.Floor(e.X/buildingCell)) == col &&
			int(math.Floor(e.Y/buildingCell)) == row {
			found = append(found, e)
		}
	}
	return found
}

// enemyThing is a rival vehicle's card, headlined by what its party is
// at.
func enemyThing(s *State, e Enemy) Thing {
	caption := stageWords(s.Parties[e.Party].Stage)
	if e.City != 0 && e.Party == 0 {
		caption = cityBuildingCaption(s, e)
	}
	return Thing{
		Type:    ThingType(e.Kind),
		ID:      fmt.Sprintf("enemy-%d", e.ID),
		Ref:     e.ID,
		Caption: caption,
	}
}

// stageWords says a party's stage the way a card reads it.
func stageWords(stage PartyStage) string {
	switch stage {
	case StageApproach:
		return "coming in"
	case StageCamp:
		return "camped, getting ready"
	case StageUnload:
		return "unloading at its city"
	case StageBuild:
		return "being assembled at its city"
	case StageRebuild:
		return "completing its ranks"
	case StageRegroup:
		return "regrouping before its next attack"
	case StageRaid:
		return "after your oil"
	case StageSettled:
		return "dug in"
	}
	return "leaving"
}

// robotsOnCell returns the robots standing on a cell, in ID order.
func robotsOnCell(s *State, col, row int) []Robot {
	var found []Robot
	for _, id := range sortedRobotIDs(s) {
		if r := s.Robots[id]; r.standsOn(col, row) {
			found = append(found, r)
		}
	}
	return found
}

// depositThing is a deposit's card: named for its kind, ID'd by its
// heart's tile, carrying what it held at first and the ground its ore
// covers.
func depositThing(d Deposit) Thing {
	kind := TypeOil
	if d.Kind == kindLilac {
		kind = TypeLilac
	}
	return Thing{
		Type:    kind,
		ID:      fmt.Sprintf("%s@%s", kind, depositKey(d)),
		CellCol: d.HeartCol,
		CellRow: d.HeartRow,
		Full:    d.Full,
		Area:    float64(d.Cells) * buildingCell * buildingCell / 10000,
	}
}

func coreAt(col, row int) Thing {
	return Thing{
		Type:   TypeCore,
		ID:     fmt.Sprintf("core@%d,%d", col, row),
		Amount: coreBubbleMeters(),
	}
}

// buildingThing is a raised building's card, ID'd by its entity ID so it
// stays the same card across saves. The catalog says what each kind's
// card reads; the factory's caption counts down its robot when busy.
func buildingThing(b Building) Thing {
	kind := buildingType(b.Kind)
	thing := Thing{
		Type: kind,
		ID:   fmt.Sprintf("%s-%d", kind, b.ID),
		Ref:  b.ID,
	}
	if b.Demolish > 0 {
		thing.Caption = fmt.Sprintf("demolishing, %d s", (b.Demolish+59)/60)
	} else if b.Work > 0 {
		unit := "robot"
		switch robotWorkKind(b) {
		case RobotCombat:
			unit = "trooper"
		case RobotRepair:
			unit = "mechanic"
		}
		thing.Caption = fmt.Sprintf("building %s, %d s", unit, b.Work/60)
	}
	return thing
}

// siteThing is the card of a building still being raised, ID'd by its
// cell: a job has no entity ID, and a cell holds one site at most.
func siteThing(job Job) Thing {
	done := 100 - job.Left*100/buildingWorkTicks
	return Thing{
		Type:    TypeSite,
		ID:      fmt.Sprintf("site@%d,%d", job.Col, job.Row),
		Caption: fmt.Sprintf("%s, %d%%", job.Kind, done),
		CellCol: job.Col,
		CellRow: job.Row,
	}
}

// siteJob returns the job a site's card stands for.
func siteJob(s *State, thing Thing) (Job, bool) {
	for _, job := range s.Jobs {
		if job.Col == thing.CellCol && job.Row == thing.CellRow {
			return job, true
		}
	}
	return Job{}, false
}

// pileThing is a pile's card, headlined by what lies there.
func pileThing(p Pile) Thing {
	return Thing{
		Type:    TypePile,
		ID:      fmt.Sprintf("pile-%d", p.ID),
		Ref:     p.ID,
		Caption: pileWords(p, " + "),
	}
}

// pileWords writes a pile's contents, the kinds it holds only.
func pileWords(p Pile, joint string) string {
	var words []string
	if p.Lilac >= pileDust {
		words = append(words, si(math.Round(p.Lilac*10)/10, "kg"))
	}
	if p.Oil >= pileDust {
		words = append(words, si(math.Round(p.Oil*10)/10, "L"))
	}
	return strings.Join(words, joint)
}

// standsOn reports whether the robot's position falls on this cell.
func (r Robot) standsOn(col, row int) bool {
	c, rw := robotCell(r)
	return c == col && rw == row
}

func robotThing(s *State, r Robot) Thing {
	return Thing{
		Type:    TypeRobot,
		ID:      fmt.Sprintf("robot-%d", r.ID),
		Ref:     r.ID,
		Caption: robotCaption(s, r),
	}
}

// robotCaption is the robot card's headline: what it is up to. It reads
// the state in the same priority order the rules do (sim_robots.go), so
// the words always say what the robot is doing.
func robotCaption(s *State, r Robot) string {
	switch r.taskNow(s).name {
	case taskHaul:
		if word := storageWord(s, r); word != "" {
			return word
		}
		return "hauling " + cargoWord(r.Cargo)
	case taskRefuel:
		return chargeStatus(s, r)
	case taskLoad:
		if r.Pile != 0 {
			return "loading loose items"
		}
		return "loading " + postWord(r)
	case taskBuild:
		if task, ok := constructionFor(s, r); ok {
			switch task.kind {
			case constructionSite:
				return "building"
			case constructionDemolition:
				return "taking down"
			}
		}
		return "laying pipe"
	case taskRepair:
		if _, damaged := damagedBuilding(s); damaged {
			return "repairing"
		}
		return "standing by"
	case taskCollect:
		return "fetching loose items"
	case taskPost:
		if oilPoolInFog(s, r.PostCol, r.PostRow) {
			return "waiting for fog"
		}
		return postWord(r) + " run"
	case taskSquad:
		if squadOf(s, r.Squad).Order == OrderAttack {
			return "attacking"
		}
		return "guarding"
	default:
		return "idle"
	}
}

// storageWord names a robot standing at a store that is full, with
// cargo in its arms.
func storageWord(s *State, r Robot) string {
	if r.Carry <= 0 {
		return ""
	}
	x, y := storeSpot(s, r)
	if math.Hypot(r.X-x, r.Y-y) >= 0.5 {
		return ""
	}
	full := tankRoom(s, haulTank(s, r))
	if r.Cargo == TypeLilac {
		full = lilacCap(s) - s.Stock.Lilac
	}
	if full < 0.0001 {
		return "waiting for storage"
	}
	return ""
}

func postWord(r Robot) string {
	if tileAt(r.PostCol, r.PostRow) == kindOil {
		return "oil"
	}
	return "lilac"
}

func cargoWord(cargo ThingType) string {
	if cargo == TypeOil {
		return "oil"
	}
	return "lilac"
}

// tileAtWorld returns the tile under the world point at x, y units, and
// whether the point is inside the region. It undoes project: a tile's
// diamond on the screen is the square [col, col+1) by [row, row+1) in tiles.
func tileAtWorld(x, y float32) (col, row int, inside bool) {
	ux, uy := unproject(x, y)
	col = int(math.Floor(float64(ux / unitsPerTile)))
	row = int(math.Floor(float64(uy / unitsPerTile)))
	return col, row, col >= 0 && row >= 0 && col < regionCols && row < regionRows
}

// si writes a quantity in its SI unit: whole numbers plainly, fractions
// with one decimal, thousands with the k prefix. Kilograms that grow past
// a thousand become tonnes, so no one reads "kkg".
func si(value float64, unit string) string {
	switch {
	case unit == "kg" && value >= 1000:
		return fmt.Sprintf("%.1f t", value/1000)
	case value >= 1000:
		return fmt.Sprintf("%.1f k%s", value/1000, unit)
	case value == math.Trunc(value):
		return fmt.Sprintf("%.0f %s", value, unit)
	default:
		return fmt.Sprintf("%.1f %s", value, unit)
	}
}
