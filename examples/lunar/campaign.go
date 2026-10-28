package main

import "fmt"

type shipSpec struct {
	Name, Role                         string
	Price                              int
	Thrust, Fuel, Turn, Landing, Cargo float32
}

var ships = [...]shipSpec{
	{"KESTREL", "LIGHT EXPLORER", 0, 44, 110, 108, 22, 1},
	{"MANTA", "PRECISION TRANSPORT", 2800, 53, 150, 95, 27, 1.35},
	{"ATLAS", "HEAVY FREIGHTER", 7200, 61, 195, 78, 31, 1.8},
}

type regionSpec struct {
	Name, Tag, Detail string
	Deliveries        int
	Gravity, Wind     float32
	Pay, BaseCost     int
}

var regions = [...]regionSpec{
	{"TRANQUILITY", "01 / THE NEARSIDE", "Wide pads. Gentle gravity. Your first foothold.", 0, 14, 0, 460, 800},
	{"GLASS BASIN", "02 / IMPACT FIELDS", "A fractured valley under a violet horizon.", 2, 17, 0, 650, 1400},
	{"CINDER RIDGE", "03 / VOLCANIC SHELF", "High gravity. Narrow approaches. Better pay.", 5, 21, 0, 900, 2200},
	{"ION REACH", "04 / ELECTRIC FRONTIER", "Charged dust creates a lateral drift.", 9, 18, 5, 1200, 3200},
	{"POLAR NIGHT", "05 / THE FAR SIDE", "Deep craters. The last link in your network.", 14, 23, 3, 1600, 4800},
}

var cargoNames = [...]string{"SUPPLY RUN", "RESEARCH CARGO", "EXPRESS FREIGHT"}
var upgradeNames = [...]string{"ENGINE", "FUEL CELLS", "LANDING GEAR", "CARGO BAY"}
var upgradeDetails = [...]string{"+12% thrust per level", "+22% fuel per level", "+1.25 m/s tolerance per level", "+18% contract pay per level"}

type campaign struct {
	Version                                       int
	Credits, Deliveries, Earned, Perfect, Crashes int
	Selected                                      int
	Owned                                         [3]bool
	Upgrades                                      [3][4]int
	Bases                                         [5]int
	Best                                          [5]int
	Muted, EffectsOff, Completed                  bool
}

func newCampaign() campaign {
	return campaign{Version: 1, Credits: 350, Owned: [3]bool{true, false, false}}
}

func (p *campaign) normalize() {
	p.Version = 1
	p.Credits = max(0, p.Credits)
	p.Deliveries = max(0, p.Deliveries)
	p.Owned[0] = true
	if p.Selected < 0 || p.Selected >= len(ships) || !p.Owned[p.Selected] {
		p.Selected = 0
	}
	for i := range p.Upgrades {
		for j := range p.Upgrades[i] {
			p.Upgrades[i][j] = max(0, min(4, p.Upgrades[i][j]))
		}
	}
	for i := range p.Bases {
		p.Bases[i] = max(0, min(3, p.Bases[i]))
	}
}

func (p campaign) stats() shipSpec {
	v := ships[p.Selected]
	u := p.Upgrades[p.Selected]
	v.Thrust *= 1 + float32(u[0])*0.12
	v.Fuel *= 1 + float32(u[1])*0.22
	v.Landing += float32(u[2]) * 5
	v.Cargo *= 1 + float32(u[3])*0.18
	return v
}

func (p campaign) unlocked(region int) bool {
	return region >= 0 && region < len(regions) && p.Deliveries >= regions[region].Deliveries
}

func (p campaign) upgradeCost(track int) int {
	if track < 0 || track >= 4 {
		return 0
	}
	level := p.Upgrades[p.Selected][track]
	return (300 + p.Selected*160 + track*40) * (level + 1)
}

func (p *campaign) buyUpgrade(track int) bool {
	if track < 0 || track >= 4 || p.Upgrades[p.Selected][track] >= 4 {
		return false
	}
	price := p.upgradeCost(track)
	if p.Credits < price {
		return false
	}
	p.Credits -= price
	p.Upgrades[p.Selected][track]++
	return true
}

func (p *campaign) buyShip(i int) bool {
	if i < 0 || i >= len(ships) {
		return false
	}
	if !p.Owned[i] {
		if p.Credits < ships[i].Price {
			return false
		}
		p.Credits -= ships[i].Price
		p.Owned[i] = true
	}
	p.Selected = i
	return true
}

func (p campaign) baseCost(i int) int { return regions[i].BaseCost * (p.Bases[i] + 1) }

func (p *campaign) buyBase(i int) bool {
	if !p.unlocked(i) || p.Bases[i] >= 3 || p.Credits < p.baseCost(i) {
		return false
	}
	p.Credits -= p.baseCost(i)
	p.Bases[i]++
	return true
}

func (p campaign) dividend() int {
	result := 0
	for i, level := range p.Bases {
		result += level * (110 + i*70)
	}
	return result
}

func (p campaign) colonies() int {
	n := 0
	for _, level := range p.Bases {
		if level > 0 {
			n++
		}
	}
	return n
}

type contract struct{ Region, Kind int }

func (c contract) reward(p campaign) int {
	return int(float32(regions[c.Region].Pay) * (1 + float32(c.Kind)*0.25) * p.stats().Cargo)
}
func (c contract) deadline() float32 { return 65 + float32(c.Region)*7 }

type settlement struct {
	Contract, Precision, Fuel, Express, Dividends, Total int
	Grade, Message                                       string
}

// settle is idempotent per flight, so revisiting a debrief cannot pay twice.
func (p *campaign) settle(w *world) settlement {
	if w.settled || w.state == flying {
		return settlement{}
	}
	w.settled = true
	if w.state == crashed {
		p.Crashes++
		return settlement{Grade: "LOST", Message: w.reason}
	}
	score := w.landingScore
	grade := "C"
	if score >= 90 {
		grade = "S"
	} else if score >= 72 {
		grade = "A"
	} else if score >= 48 {
		grade = "B"
	}
	base := w.contract.reward(*p)
	r := settlement{
		Contract: base, Precision: base * score / 200,
		Fuel:      int(float32(base) * 0.18 * w.fuel / w.spec.Fuel),
		Dividends: p.dividend(), Grade: grade,
		Message: fmt.Sprintf("Cargo secured at %s.", regions[w.contract.Region].Name),
	}
	if w.contract.Kind == 2 && w.elapsed <= w.contract.deadline() {
		r.Express = base / 3
	}
	r.Total = r.Contract + r.Precision + r.Fuel + r.Express + r.Dividends
	p.Credits += r.Total
	p.Earned += r.Total
	p.Deliveries++
	if grade == "S" {
		p.Perfect++
	}
	p.Best[w.contract.Region] = max(p.Best[w.contract.Region], score)
	return r
}
