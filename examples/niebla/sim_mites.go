package main

import "math"

const (
	miteDamagePerSecond   = 2.0         // health points per second at full exposure
	mitePileLifetimeTicks = 3 * 60 * 60 // ticks a pile lasts in full fog
)

func miteSwellFactor(s *State) float64 {
	return 1 + math.Max(0, math.Min(1, s.Fog.Pressure))
}

func miteExposureAt(s *State, x, y float64) float64 {
	if inSafeZone(s, x, y) || insideEnemyRepulsor(s, x, y) {
		return 0
	}
	return math.Max(fogHazeExposure, fogAt(s, x, y))
}

func insideEnemyRepulsor(s *State, x, y float64) bool {
	for _, id := range sortedEnemyIDs(s) {
		e := s.Enemies[id]
		reach := enemySpecOf(e.Kind).bubble
		if reach > 0 && math.Hypot(e.X-x, e.Y-y) <= reach {
			return true
		}
	}
	return false
}

func stepMiteWear(s *State) {
	stepMiteBuildingWear(s)
	stepMiteSiteWear(s)
	stepMiteCitySiteWear(s)
	stepMitePileWear(s)
	stepMitePipeWear(s)
}

func stepMiteBuildingWear(s *State) {
	for _, id := range sortedBuildingIDs(s) {
		b := s.Buildings[id]
		if b.Kind == BuildingProtector && b.Oil > 0 {
			continue
		}
		x, y := cellCenterUnits(b.Col, b.Row)
		exposure := miteExposureAt(s, x, y)
		if exposure <= 0 {
			continue
		}
		damage := miteDamagePerSecond * exposure * miteSwellFactor(s) / 60
		s.hurtBuildingWithMites(id, damage)
	}
}

func (s *State) hurtBuildingWithMites(id int64, damage float64) {
	b, ok := s.Buildings[id]
	if !ok {
		return
	}
	b.Damage += damage
	if b.Damage < buildingHealth(b.Kind) {
		s.Buildings[id] = b
		return
	}
	x, y := cellCenterUnits(b.Col, b.Row)
	s.takeDown(b, wreckRefund, BuildingDestroyed)
	s.report(ReportMiteEaten, 0, x, y)
}

func stepMiteSiteWear(s *State) {
	var lost []Job
	for i, job := range s.Jobs {
		x, y := cellCenterUnits(job.Col, job.Row)
		exposure := miteExposureAt(s, x, y)
		if exposure <= 0 {
			continue
		}
		job.Damage += miteDamagePerSecond * exposure * miteSwellFactor(s) / 60
		if job.Damage >= buildingHealth(job.Kind) {
			lost = append(lost, job)
			continue
		}
		s.Jobs[i] = job
	}
	for _, job := range lost {
		Apply(s, CancelJob{Col: job.Col, Row: job.Row})
		s.report(ReportMiteEaten, 0,
			float64(job.Col)*buildingCell+buildingCell/2,
			float64(job.Row)*buildingCell+buildingCell/2,
		)
	}
}

func stepMiteCitySiteWear(s *State) {
	for _, id := range sortedCityIDs(s) {
		city := s.Cities[id]
		x, y, _, health, building := cityConstructionSite(s, city)
		if !building {
			continue
		}
		exposure := miteExposureAt(s, x, y)
		if exposure <= 0 {
			continue
		}
		city.MiteDamage +=
			miteDamagePerSecond * exposure * miteSwellFactor(s) / 60
		if city.MiteDamage >= health {
			city.MiteDamage = 0
			city.Work = cityBuildTicks
			s.report(ReportMiteEaten, 0, x, y)
		}
		s.Cities[id] = city
	}
}

func stepMitePileWear(s *State) {
	for _, id := range sortedPileIDs(s) {
		p := s.Piles[id]
		x, y := cellCenterUnits(p.Col, p.Row)
		exposure := miteExposureAt(s, x, y)
		if exposure <= 0 {
			continue
		}
		remaining := float64(mitePileLifetimeTicks) - p.MiteTicks
		if remaining <= 0 {
			delete(s.Piles, id)
			continue
		}
		elapsed := exposure * miteSwellFactor(s)
		if elapsed >= remaining {
			delete(s.Piles, id)
			continue
		}
		fraction := (remaining - elapsed) / remaining
		p.Oil *= fraction
		p.Lilac *= fraction
		p.MiteTicks += elapsed
		if p.Oil < pileDust && p.Lilac < pileDust {
			delete(s.Piles, id)
			continue
		}
		s.Piles[id] = p
	}
}

func stepMitePipeWear(s *State) {
	for _, id := range sortedPipeIDs(s) {
		p := s.Pipes[id]
		path, ok := pipeSpine(s, p)
		if !ok {
			continue
		}
		length := pathLength(path)
		laidLength, exposedLength := 0.0, 0.0
		for section := int64(0); section < p.Sections; section++ {
			if sectionLeft(p, section) > 0 {
				continue
			}
			start := float64(section) * pipeSectionMeters
			part := math.Min(pipeSectionMeters, length-start)
			if part <= 0 {
				continue
			}
			laidLength += part
			point := pathPointAt(path, start+part/2)
			exposure := miteExposureAt(s, point.X, point.Y)
			exposedLength += part * exposure
		}
		if laidLength <= 0 || exposedLength <= 0 {
			continue
		}
		damage := miteDamagePerSecond * miteSwellFactor(s) *
			exposedLength / laidLength / 60
		p.Damage += damage
		if p.Damage < buildingHealthPoints {
			s.Pipes[id] = p
			continue
		}
		point := pathPointAt(path, length/2)
		delete(s.Pipes, id)
		s.report(ReportMiteEaten, 0, point.X, point.Y)
	}
}
