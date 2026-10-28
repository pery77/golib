package main

import (
	"fmt"
	"golib"
)

type controlScene struct {
	app                         *session
	tab, region, kind, selected int
}

func newControlScene(a *session) *controlScene { return &controlScene{app: a, selected: 8} }
func (s *controlScene) buttons() []button {
	p := s.app.progress
	var b []button
	switch s.tab {
	case 0:
		for i, r := range regions {
			hint := fmt.Sprintf("%d DELIVERIES TO UNLOCK", r.Deliveries)
			if p.unlocked(i) {
				hint = r.Tag
			}
			entry := buttonAt(40, 198+float32(i)*83, 302, 72, r.Name, hint)
			b = append(b, entry)
		}
		for i, name := range cargoNames {
			r := contract{Region: s.region, Kind: i}.reward(p)
			hint := fmt.Sprintf("%s CR + BONUSES", money(r))
			b = append(b, buttonAt(386+float32(i)*282, 433, 267, 83, name, hint))
		}
		launch := buttonAt(386, 558, 831, 61, "LAUNCH CONTRACT  /  "+cargoNames[s.kind], "")
		launch.disabled = !p.unlocked(s.region)
		b = append(b, launch)
	case 1:
		for i, ship := range ships {
			name := "BUY  /  " + money(ship.Price) + " CR"
			if p.Owned[i] {
				name = "EQUIP " + ship.Name
			}
			if p.Selected == i {
				name = "EQUIPPED"
			}
			b = append(b, buttonAt(60+float32(i)*404, 370, 354, 48, name, ""))
		}
		for i := range upgradeNames {
			price := p.upgradeCost(i)
			name := "UPGRADE  /  " + money(price) + " CR"
			if p.Upgrades[p.Selected][i] >= 4 {
				name = "MAXIMUM LEVEL"
			}
			b = append(b, buttonAt(58+float32(i)*302, 568, 277, 47, name, ""))
		}
	case 2:
		for i := range regions {
			action := "ESTABLISH"
			if p.Bases[i] > 0 {
				action = "EXPAND"
			}
			if p.Bases[i] >= 3 {
				action = "FULLY DEVELOPED"
			}
			b = append(b, buttonAt(940, 196+float32(i)*82, 277, 55, action, ""))
		}
	}
	return b
}
func (s *controlScene) changeTab(tab int) { s.tab = tab; s.selected = 0; clickSound.Play() }
func (s *controlScene) Update(in *golib.Input, dt float32) {
	s.app.update(in, dt)
	if back(in) {
		golib.SwitchScene(newTitleScene(s.app))
		return
	}
	if in.KeyPressed(golib.KeyOne) {
		s.changeTab(0)
	}
	if in.KeyPressed(golib.KeyTwo) {
		s.changeTab(1)
	}
	if in.KeyPressed(golib.KeyThree) {
		s.changeTab(2)
	}
	if in.KeyPressed(golib.KeyTab) || in.GamepadPressed(0, golib.GamepadRightBumper) {
		s.changeTab((s.tab + 1) % 3)
	}
	if in.GamepadPressed(0, golib.GamepadLeftBumper) {
		s.changeTab((s.tab + 2) % 3)
	}
	mx, my := in.MousePosition()
	if in.MousePressed(golib.MouseLeft) {
		for i := 0; i < 3; i++ {
			if (golib.Rectangle{X: 40 + float32(i)*226, Y: 103, Width: 211, Height: 44}).Contains(mx, my) {
				s.changeTab(i)
				return
			}
		}
		if (golib.Rectangle{X: 1136, Y: 105, Width: 105, Height: 40}).Contains(mx, my) {
			golib.SwitchScene(newTitleScene(s.app))
			return
		}
	}
	action := menuInput(in, s.buttons(), &s.selected)
	if action < 0 {
		return
	}
	p := &s.app.progress
	switch s.tab {
	case 0:
		if action < 5 {
			s.region = action
			return
		}
		if action < 8 {
			s.kind = action - 5
			s.selected = 8
			return
		}
		if p.unlocked(s.region) {
			engineSound.Stop()
			golib.SwitchScene(newPlayScene(s.app, contract{Region: s.region, Kind: s.kind}))
			return
		}
	case 1:
		if action < 3 {
			if !p.buyShip(action) {
				s.app.tell("Insufficient credits. Complete contracts to grow your fleet.")
				alertSound.Play()
				return
			}
			s.app.tell(ships[action].Name + " equipped and ready.")
		} else {
			track := action - 3
			if !p.buyUpgrade(track) {
				if p.Upgrades[p.Selected][track] >= 4 {
					s.app.tell("This system is already fully upgraded.")
				} else {
					s.app.tell("Insufficient credits for this upgrade.")
				}
				alertSound.Play()
				return
			}
			s.app.tell(upgradeNames[track] + " upgraded. Changes apply to your next flight.")
		}
		buySound.Play()
		s.app.save()
	case 2:
		if !p.buyBase(action) {
			switch {
			case !p.unlocked(action):
				s.app.tell(fmt.Sprintf("Complete %d deliveries to access this region.", regions[action].Deliveries))
			case p.Bases[action] >= 3:
				s.app.tell("This colony is fully developed.")
			default:
				s.app.tell("Insufficient credits. Colony dividends will help fund expansion.")
			}
			alertSound.Play()
			return
		}
		buySound.Play()
		s.app.tell(fmt.Sprintf("%s is now level %d. Dividends grow on every successful delivery.", regions[action].Name, p.Bases[action]))
		if p.colonies() == 5 && !p.Completed {
			p.Completed = true
			s.app.save()
			golib.SwitchScene(&completionScene{app: s.app})
			return
		}
		s.app.save()
	}
}
func (s *controlScene) Draw(screen *golib.Screen) {
	a := s.app
	p := a.progress
	drawSky(screen, a, s.region, 0)
	rect(screen, 0, 0, 1280, 157, fade(ink, 0.9))
	text(screen, "MISSION CONTROL", 40, 32, 30, white)
	text(screen, "SELENE / INDEPENDENT OPERATIONS", 41, 70, 14, muted)
	right(screen, money(p.Credits)+" CR", 1239, 28, 32, cyan)
	right(screen, fmt.Sprintf("%02d DELIVERIES   /   %d COLONIES   /   +%s CR DIVIDENDS", p.Deliveries, p.colonies(), money(p.dividend())), 1238, 71, 14, muted)
	for i, name := range []string{"01  CONTRACTS", "02  HANGAR", "03  COLONIES"} {
		drawButton(screen, buttonAt(40+float32(i)*226, 103, 211, 44, name, ""), s.tab == i)
	}
	drawButton(screen, buttonAt(1136, 105, 105, 40, "HOME", ""), false)
	switch s.tab {
	case 0:
		s.drawContracts(screen)
	case 1:
		s.drawHangar(screen)
	case 2:
		s.drawColonies(screen)
	}
	footer(screen, a)
}
func (s *controlScene) drawContracts(screen *golib.Screen) {
	p := s.app.progress
	r := regions[s.region]
	text(screen, "DESTINATION", 40, 172, 14, muted)
	text(screen, "CONTRACT BRIEFING", 386, 172, 14, muted)
	buttons := s.buttons()
	for i := 0; i < 5; i++ {
		drawButton(screen, buttons[i], s.selected == i || s.region == i)
	}
	box(screen, golib.Rectangle{X: 386, Y: 198, Width: 831, Height: 211})
	drawPlanet(screen, 1109, 301, 77, s.region)
	text(screen, r.Name, 408, 221, 30, white)
	text(screen, r.Detail, 408, 263, 16, muted)
	badge(screen, fmt.Sprintf("GRAVITY %.2f", r.Gravity/14), 408, 300, cyan)
	badge(screen, fmt.Sprintf("BEST LANDING %d / 100", p.Best[s.region]), 590, 300, muted)
	status := "CLEARED FOR INSERTION"
	c := cyan
	if !p.unlocked(s.region) {
		status = fmt.Sprintf("LOCKED / %d MORE DELIVERIES", r.Deliveries-p.Deliveries)
		c = orange
	}
	text(screen, status, 408, 359, 18, c)
	for i := 5; i < 8; i++ {
		drawButton(screen, buttons[i], s.selected == i || s.kind == i-5)
	}
	hints := []string{
		"Standard cargo. Reliable pay. A generous landing envelope.",
		"Fragile instruments. 25% more pay; 22% softer touchdown required.",
		fmt.Sprintf("Rush shipment. 50%% more pay, plus a bonus under %.0f seconds.", (contract{Region: s.region}).deadline()),
	}
	text(screen, hints[s.kind], 387, 530, 16, muted)
	drawButton(screen, buttons[8], s.selected == 8)
	text(screen, "SELECT WITH MOUSE OR ARROWS  /  ENTER TO CONFIRM  /  TAB TO CHANGE VIEW", 387, 642, 12, muted)
}
func (s *controlScene) drawHangar(screen *golib.Screen) {
	p := s.app.progress
	buttons := s.buttons()
	for i, ship := range ships {
		x := float32(40 + i*404)
		box(screen, golib.Rectangle{X: x, Y: 174, Width: 394, Height: 258})
		text(screen, ship.Name, x+20, 190, 27, white)
		text(screen, ship.Role, x+20, 226, 13, muted)
		drawShip(screen, golib.Vector2{X: x + 194, Y: 301}, -8, 1.85, i, 0, s.app.time)
		text(screen, fmt.Sprintf("CARGO x%.2f  /  TANK %.0f", ship.Cargo, ship.Fuel), x+20, 344, 14, muted)
		drawButton(screen, buttons[i], s.selected == i || p.Selected == i)
	}
	text(screen, "UPGRADE "+ships[p.Selected].Name+" / EACH SHIP KEEPS ITS OWN SYSTEMS", 40, 449, 16, cyan)
	for i, name := range upgradeNames {
		x := float32(40 + i*302)
		box(screen, golib.Rectangle{X: x, Y: 480, Width: 292, Height: 150})
		text(screen, name, x+18, 496, 20, white)
		detail := upgradeDetails[i]
		text(screen, detail, x+18, 526, 14, muted)
		for j := 0; j < 4; j++ {
			c := lineColor
			if j < p.Upgrades[p.Selected][i] {
				c = cyan
			}
			rect(screen, x+18+float32(j)*65, 552, 58, 4, c)
		}
		drawButton(screen, buttons[i+3], s.selected == i+3)
	}
	text(screen, "Permanent upgrades. Fresh fuel and full insurance are included with every contract.", 40, 647, 14, muted)
}
func (s *controlScene) drawColonies(screen *golib.Screen) {
	p := s.app.progress
	text(screen, "BUILD A NETWORK. LET EVERY LANDING EARN MORE.", 40, 171, 18, cyan)
	buttons := s.buttons()
	for i, r := range regions {
		y := float32(191 + i*82)
		box(screen, golib.Rectangle{X: 40, Y: y, Width: 1178, Height: 74})
		c := cyan
		if !p.unlocked(i) {
			c = muted
		}
		text(screen, fmt.Sprintf("%02d", i+1), 57, y+25, 22, c)
		text(screen, r.Name, 104, y+14, 20, white)
		status := fmt.Sprintf("LEVEL %d / 3", p.Bases[i])
		if !p.unlocked(i) {
			status = fmt.Sprintf("REQUIRES %d DELIVERIES", r.Deliveries)
		}
		text(screen, status, 105, y+43, 13, muted)
		drawBase(screen, 458, y+60, 0.55, p.Bases[i], c, s.app.time)
		text(screen, fmt.Sprintf("+%d CR / DELIVERY", p.Bases[i]*(110+i*70)), 548, y+16, 16, c)
		text(screen, fmt.Sprintf("NEXT +%d CR / DELIVERY", 110+i*70), 548, y+44, 12, muted)
		price := money(p.baseCost(i)) + " CR"
		if p.Bases[i] >= 3 {
			price = "MAX"
		}
		right(screen, price, 914, y+28, 18, white)
		drawButton(screen, buttons[i], s.selected == i)
	}
	text(screen, "Dividends are paid after every successful contract, in every region. Establish all five to win.", 40, 627, 16, muted)
	text(screen, "A growing economy powered by your piloting. Expand each colony through three levels.", 40, 652, 14, muted)
}

type completionScene struct {
	app      *session
	selected int
}

func (s *completionScene) Update(in *golib.Input, dt float32) {
	s.app.update(in, dt)
	if menuInput(in, []button{buttonAt(428, 545, 424, 58, "KEEP BUILDING THE FRONTIER", "")}, &s.selected) >= 0 {
		golib.SwitchScene(newControlScene(s.app))
	}
}
func (s *completionScene) Draw(screen *golib.Screen) {
	drawSky(screen, s.app, 4, 0)
	drawPlanet(screen, 640, 302, 204, 4)
	rect(screen, 0, 0, 1280, 720, fade(ink, 0.55))
	badge(screen, "CAMPAIGN COMPLETE", 545, 126, cyan)
	center(screen, "FIVE OUTPOSTS. ONE FRONTIER.", 640, 202, 36, white)
	center(screen, "From a single lander to a world connected.", 640, 268, 22, muted)
	for i := 0; i < 5; i++ {
		x := float32(310 + i*165)
		screen.DrawCircle(x, 377, 8, cyan)
		screen.DrawCircleOutline(x, 377, 23, 1, cyan)
		if i < 4 {
			screen.DrawLine(x+25, 377, x+140, 377, 1, cyan)
		}
		center(screen, fmt.Sprintf("0%d", i+1), x, 419, 18, white)
	}
	center(screen, fmt.Sprintf("%d DELIVERIES   /   %s CR EARNED   /   %d PERFECT LANDINGS", s.app.progress.Deliveries, money(s.app.progress.Earned), s.app.progress.Perfect), 640, 481, 18, cyan)
	drawButton(screen, buttonAt(428, 545, 424, 58, "KEEP BUILDING THE FRONTIER", ""), true)
	footer(screen, s.app)
}
