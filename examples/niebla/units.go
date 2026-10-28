package main

import (
	"math"
	"sort"

	"golib"
)

func drawRobots(
	s *State, screen *golib.Screen, camera *golib.Camera, zoom float32,
) {
	type spot struct {
		id int64
		p  golib.Vector2
	}
	spots := make([]spot, 0, len(s.Robots))
	for _, id := range sortedRobotIDs(s) {
		r := s.Robots[id]
		x, y := project(float32(r.X), float32(r.Y))
		spots = append(spots, spot{id, golib.Vector2{X: x, Y: y}})
	}
	sort.SliceStable(
		spots,
		func(i, j int) bool { return spots[i].p.Y < spots[j].p.Y },
	)
	for _, sp := range spots {
		r := s.Robots[sp.id]
		model := robotModel(r.Kind)
		model.drawShadow(screen, camera, sp.p, r.Facing, zoom)
	}
	for _, sp := range spots {
		r := s.Robots[sp.id]
		p := sp.p
		radius := dotRadius(3, zoom, 3)
		model := robotModel(r.Kind)
		model.draw(screen, camera, p, r.Facing, zoom)
		if r.tanked() && chargeStatus(s, r) == "refueling" &&
			s.Ticks/20%2 == 0 {
			scale := model.iconScale(zoom)
			head := p.Add(golib.Vector2{Y: -4.0 * scale * unitH})
			lamp := groundFacingPoint(head, 1.1, 0,
				3*unitW*scale, r.Facing)
			screen.DrawCircle(lamp.X, lamp.Y, radius*0.21,
				robotDarkColor)
		}
		if r.Carry > 0 {
			cargo := lilacColor
			if r.Cargo == TypeOil {
				cargo = oilColor
			}
			scale := model.iconScale(zoom)
			roof := p.Add(golib.Vector2{Y: -6.4 * scale * unitH})
			pack := groundFacingPoint(roof, -0.57, 0,
				3*unitW*scale, r.Facing)
			screen.DrawCircle(pack.X, pack.Y, radius*0.32, cargo)
		}
		maxHealth := robotMaxHealth(r.Kind)
		statusY := p.Y + radius*1.5
		healthColor := robotColor
		switch r.Kind {
		case RobotWorker:
			healthColor = factoryColor
		case RobotCombat:
			healthColor = guardColor
		case RobotRepair:
			healthColor = factoryColor
		}
		if maxHealth > 0 && r.Health < maxHealth {
			drawUnitBar(screen, golib.Rectangle{
				X: p.X - radius, Y: statusY,
				Width: 2 * radius, Height: 2 / zoom,
			}, r.Health/maxHealth, healthColor)
		}
		if r.Kind == RobotWorker {
			drawUnitBar(screen, golib.Rectangle{
				X: p.X - radius, Y: statusY + 4/zoom,
				Width: 2 * radius, Height: 2 / zoom,
			}, r.Tank/robotTankLiters, oilColor)
		}
	}
}

// drawUnitBar paints one status bar under a unit: the dark well first,
// then the fill, as wide as part of the whole bar is.
func drawUnitBar(
	screen *golib.Screen, bar golib.Rectangle, part float64,
	color golib.Color,
) {
	screen.DrawRectangle(bar, fillBarColor)
	bar.Width *= float32(math.Max(0, part))
	screen.DrawRectangle(bar, color)
}
