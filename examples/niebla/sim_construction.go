package main

import "math"

type constructionKind uint8

const (
	constructionSite constructionKind = iota + 1
	constructionDemolition
	constructionPipe
)

type constructionTask struct {
	kind     constructionKind
	index    int
	col, row int
	id       int64
	section  int64
	spot     PipePoint
}

func (r *Robot) clearConstruction() {
	r.BuildJob = false
	r.BuildCol, r.BuildRow = 0, 0
	r.Demolition = 0
	r.Pipe, r.Section = 0, 0
}

func (r *Robot) reserveConstruction(task constructionTask) {
	r.clearConstruction()
	switch task.kind {
	case constructionSite:
		r.BuildJob = true
		r.BuildCol, r.BuildRow = task.col, task.row
	case constructionDemolition:
		r.Demolition = task.id
	case constructionPipe:
		r.Pipe, r.Section = task.id, task.section
	}
}

func claimedConstruction(s *State, r Robot) (constructionTask, bool) {
	if r.BuildJob {
		for i, job := range s.Jobs {
			if job.Col == r.BuildCol && job.Row == r.BuildRow &&
				job.Left > 0 {
				x, y := cellCenterUnits(job.Col, job.Row)
				return constructionTask{
					kind: constructionSite, index: i,
					col: job.Col, row: job.Row, spot: PipePoint{x, y},
				}, true
			}
		}
	}
	if r.Demolition != 0 {
		if b, ok := s.Buildings[r.Demolition]; ok && b.Demolish > 0 {
			x, y := cellCenterUnits(b.Col, b.Row)
			return constructionTask{
				kind: constructionDemolition, id: b.ID,
				spot: PipePoint{x, y},
			}, true
		}
	}
	if r.Pipe != 0 {
		if p, ok := s.Pipes[r.Pipe]; ok &&
			sectionLeft(p, r.Section) > 0 {
			path, stands := pipeSpine(s, p)
			if stands {
				return constructionTask{
					kind: constructionPipe, id: p.ID,
					section: r.Section,
					spot:    sectionSpot(path, r.Section),
				}, true
			}
		}
	}
	return constructionTask{}, false
}

func sameConstruction(a, b constructionTask) bool {
	if a.kind != b.kind {
		return false
	}
	switch a.kind {
	case constructionSite:
		return a.col == b.col && a.row == b.row
	case constructionDemolition:
		return a.id == b.id
	case constructionPipe:
		return a.id == b.id && a.section == b.section
	}
	return false
}

func freeConstruction(s *State, r Robot) (constructionTask, bool) {
	claimed := make([]constructionTask, 0, len(s.Robots))
	for _, id := range sortedRobotIDs(s) {
		if id == r.ID {
			continue
		}
		if task, ok := claimedConstruction(s, s.Robots[id]); ok {
			claimed = append(claimed, task)
		}
	}

	var best constructionTask
	bestPriority, bestDistance := 2, math.Inf(1)
	consider := func(task constructionTask, priority int) {
		for _, taken := range claimed {
			if sameConstruction(task, taken) {
				return
			}
		}
		distance := pointGap(PipePoint{r.X, r.Y}, task.spot)
		if priority < bestPriority ||
			(priority == bestPriority && distance < bestDistance) {
			best, bestPriority, bestDistance = task, priority, distance
		}
	}

	for i, job := range s.Jobs {
		if job.Left <= 0 {
			continue
		}
		x, y := cellCenterUnits(job.Col, job.Row)
		priority := 1
		if job.Kind == BuildingProtector {
			priority = 0
		}
		consider(constructionTask{
			kind: constructionSite, index: i,
			col: job.Col, row: job.Row, spot: PipePoint{x, y},
		}, priority)
	}
	for _, id := range sortedBuildingIDs(s) {
		b := s.Buildings[id]
		if b.Demolish <= 0 {
			continue
		}
		x, y := cellCenterUnits(b.Col, b.Row)
		consider(constructionTask{
			kind: constructionDemolition, id: id,
			spot: PipePoint{x, y},
		}, 1)
	}
	for _, id := range sortedPipeIDs(s) {
		p := s.Pipes[id]
		if p.Left <= 0 {
			continue
		}
		path, stands := pipeSpine(s, p)
		if !stands {
			continue
		}
		for section := int64(0); section < p.Sections; section++ {
			if sectionLeft(p, section) <= 0 {
				continue
			}
			consider(constructionTask{
				kind: constructionPipe, id: id, section: section,
				spot: sectionSpot(path, section),
			}, 1)
		}
	}
	return best, bestPriority < 2
}

func constructionFor(s *State, r Robot) (constructionTask, bool) {
	if task, ok := claimedConstruction(s, r); ok {
		return task, true
	}
	return freeConstruction(s, r)
}
