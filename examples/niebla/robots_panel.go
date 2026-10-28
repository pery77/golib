package main

import (
	"fmt"
	"sort"

	"golib"
)

const (
	robotPanelWidth       = 366
	robotPanelHeight      = 512
	robotPanelPageSize    = 5
	robotPanelRowHeight   = 32
	robotPanelGroupHeight = 20
	robotPanelButtonH     = 22
)

type robotPanelGroup struct {
	title  string
	color  golib.Color
	robots []Robot
}

type robotPanelRow struct {
	robot Robot
	area  golib.Rectangle
}

type robotPanelLayout struct {
	box         golib.Rectangle
	close       golib.Rectangle
	prev        golib.Rectangle
	next        golib.Rectangle
	assign      golib.Rectangle
	recall      golib.Rectangle
	rows        []robotPanelRow
	page        int
	pages       int
	selected    Robot
	hasSelected bool
}

func robotPanelButtonRect() golib.Rectangle {
	squad := squadBoxRect(0)
	return golib.Rectangle{
		X:     float32(screenWidth - 16 - 140),
		Y:     squad.Y + squad.Height + squadBoxGap,
		Width: 140, Height: 32,
	}
}

func robotPanelRect() golib.Rectangle {
	width := min(float32(robotPanelWidth), float32(screenWidth)-16)
	height := min(float32(robotPanelHeight), float32(screenHeight)-16)
	y := min(float32(136), float32(screenHeight)-height-8)
	return golib.Rectangle{
		X:      float32(screenWidth) - 8 - width,
		Y:      y,
		Width:  width,
		Height: height,
	}
}

func robotPanelPageCapacity() int {
	available := robotPanelRect().Height - 150
	return max(1, min(robotPanelPageSize,
		int(available/(robotPanelGroupHeight+robotPanelRowHeight))))
}

func (s *playScene) updateRobotPanel(input *golib.Input) bool {
	if !input.MousePressed(golib.MouseLeft) {
		return false
	}
	mx, my := input.MousePosition()
	if robotPanelButtonRect().Contains(mx, my) {
		s.robotsOpen = !s.robotsOpen
		s.assigningRobot = 0
		if s.robotsOpen {
			s.closeRadial()
			s.laying = pipeLaying{}
			s.ordering = 0
		}
		s.au.ui(1)
		return true
	}
	if !s.robotsOpen {
		return false
	}
	layout := robotPanelLayoutFor(s)
	if !layout.box.Contains(mx, my) {
		return false
	}
	if layout.close.Contains(mx, my) {
		s.robotsOpen = false
		s.assigningRobot = 0
		s.au.ui(1)
		return true
	}
	if layout.prev.Contains(mx, my) && s.robotsPage > 0 {
		s.robotsPage--
		return true
	}
	if layout.next.Contains(mx, my) && s.robotsPage+1 < layout.pages {
		s.robotsPage++
		return true
	}
	for _, row := range layout.rows {
		if row.area.Contains(mx, my) {
			if s.robotsPicked == row.robot.ID {
				s.robotsPicked = 0
			} else {
				s.robotsPicked = row.robot.ID
			}
			s.assigningRobot = 0
			s.au.ui(1)
			return true
		}
	}
	if layout.hasSelected && layout.assign.Contains(mx, my) {
		if s.assigningRobot == layout.selected.ID {
			s.assigningRobot = 0
		} else {
			s.assigningRobot = layout.selected.ID
		}
		s.au.ui(1)
		return true
	}
	if layout.hasSelected && layout.recall.Contains(mx, my) {
		Apply(s.state, RecallRobot{ID: layout.selected.ID})
		s.assigningRobot = 0
		s.au.ui(1)
		return true
	}
	return true
}

func (s *playScene) clampRobotPanel() {
	groups := robotPanelGroups(s.state)
	total := 0
	for _, group := range groups {
		total += len(group.robots)
	}
	pageSize := robotPanelPageCapacity()
	pages := max(1, (total+pageSize-1)/pageSize)
	if s.robotsPage >= pages {
		s.robotsPage = pages - 1
	}
	if _, ok := s.state.Robots[s.robotsPicked]; !ok {
		s.robotsPicked = 0
	}
	if _, ok := s.state.Robots[s.assigningRobot]; !ok {
		s.assigningRobot = 0
	}
}

func (s *playScene) assignRobotAtScreen(mx, my float32) bool {
	r, ok := s.state.Robots[s.assigningRobot]
	if !ok || (r.Kind != RobotBuilder && r.Kind != RobotWorker) {
		return false
	}
	point := s.camera.ToWorld(mx, my)
	col, row, inside := cellAtWorld(float64(point.X), float64(point.Y))
	if !inside {
		return false
	}
	tcol, trow := cellTile(col, row)
	if _, ok := depositAt(tcol, trow); !ok ||
		remainingAt(s.state, tcol, trow) <= 0 {
		return false
	}
	Apply(s.state, AssignRobot{
		ID: s.assigningRobot, Col: tcol, Row: trow,
	})
	return true
}

func robotPanelGroups(s *State) []robotPanelGroup {
	builders := robotPanelGroup{title: "BUILDERS", color: robotColor}
	freeWorkers := robotPanelGroup{title: "UNASSIGNED WORKERS", color: factoryColor}
	mechanics := robotPanelGroup{title: "MECHANICS", color: chargerColor}
	workersByPost := map[string][]Robot{}
	deposits := append([]Deposit(nil), land.deposits...)
	sort.Slice(deposits, func(i, j int) bool {
		if deposits[i].Kind != deposits[j].Kind {
			if deposits[i].Kind == kindOil {
				return true
			}
			if deposits[j].Kind == kindOil {
				return false
			}
			return deposits[i].Kind < deposits[j].Kind
		}
		if deposits[i].Col != deposits[j].Col {
			return deposits[i].Col < deposits[j].Col
		}
		return deposits[i].Row < deposits[j].Row
	})
	for _, id := range sortedRobotIDs(s) {
		r := s.Robots[id]
		switch r.Kind {
		case RobotBuilder:
			builders.robots = append(builders.robots, r)
		case RobotWorker:
			if !r.hasPost() {
				freeWorkers.robots = append(freeWorkers.robots, r)
				continue
			}
			d, ok := depositAt(r.PostCol, r.PostRow)
			if !ok {
				freeWorkers.robots = append(freeWorkers.robots, r)
				continue
			}
			workersByPost[depositKey(d)] = append(
				workersByPost[depositKey(d)], r,
			)
		case RobotRepair:
			mechanics.robots = append(mechanics.robots, r)
		}
	}
	groups := []robotPanelGroup{builders}
	if len(freeWorkers.robots) > 0 {
		groups = append(groups, freeWorkers)
	}
	for _, d := range deposits {
		robots := workersByPost[depositKey(d)]
		if len(robots) == 0 {
			continue
		}
		name := "OIL"
		color := oilColor
		if d.Kind == kindLilac {
			name, color = "LILAC", lilacColor
		}
		groups = append(groups, robotPanelGroup{
			title: fmt.Sprintf("%s · TILE %d,%d", name, d.Col, d.Row),
			color: color, robots: robots,
		})
	}
	if len(mechanics.robots) > 0 {
		groups = append(groups, mechanics)
	}
	for i := range groups {
		groups[i].title = fmt.Sprintf("%s · %d", groups[i].title,
			len(groups[i].robots))
	}
	return groups
}

func robotPanelLayoutFor(s *playScene) robotPanelLayout {
	box := robotPanelRect()
	layout := robotPanelLayout{
		box: box,
		close: golib.Rectangle{
			X: box.X + box.Width - 27, Y: box.Y + 7,
			Width: 18, Height: 18,
		},
		page: s.robotsPage,
	}
	groups := robotPanelGroups(s.state)
	entries := 0
	for _, group := range groups {
		entries += len(group.robots)
	}
	pageSize := robotPanelPageCapacity()
	layout.pages = max(1, (entries+pageSize-1)/pageSize)
	if layout.page >= layout.pages {
		layout.page = layout.pages - 1
	}
	layout.prev = golib.Rectangle{
		X: box.X + box.Width - 105, Y: box.Y + 7,
		Width: 18, Height: 18,
	}
	layout.next = golib.Rectangle{
		X: box.X + box.Width - 49, Y: box.Y + 7,
		Width: 18, Height: 18,
	}
	layout.assign = golib.Rectangle{
		X: box.X + 12, Y: box.Y + box.Height - 32,
		Width: 126, Height: robotPanelButtonH,
	}
	layout.recall = golib.Rectangle{
		X: box.X + 146, Y: box.Y + box.Height - 32,
		Width: 88, Height: robotPanelButtonH,
	}
	if robot, ok := s.state.Robots[s.robotsPicked]; ok {
		layout.selected, layout.hasSelected = robot, true
	}
	start := layout.page * pageSize
	end := min(start+pageSize, entries)
	entryIndex := 0
	y := box.Y + 38
	lastGroup := ""
	for _, group := range groups {
		for _, robot := range group.robots {
			if entryIndex >= start && entryIndex < end {
				if group.title != lastGroup {
					y += robotPanelGroupHeight
					lastGroup = group.title
				}
				layout.rows = append(layout.rows, robotPanelRow{
					robot: robot,
					area: golib.Rectangle{
						X: box.X + 8, Y: y,
						Width: box.Width - 16, Height: robotPanelRowHeight,
					},
				})
				y += robotPanelRowHeight
			}
			entryIndex++
		}
	}
	return layout
}

func robotPanelActivity(s *State, r Robot) string {
	task := r.taskNow(s).name
	switch task {
	case taskBuild:
		if work, ok := constructionFor(s, r); ok {
			switch work.kind {
			case constructionSite:
				return "building " + string(s.Jobs[work.index].Kind)
			case constructionDemolition:
				return "taking down " +
					string(s.Buildings[work.id].Kind)
			}
		}
		return "laying pipe"
	case taskCollect:
		return "collecting loose items"
	case taskPost:
		return "mining " + postWord(r)
	case taskRepair:
		if building, ok := damagedBuilding(s); ok {
			return "repairing " + string(building.Kind)
		}
		return "standing by"
	default:
		return robotCaption(s, r)
	}
}

func drawRobotPanel(s *playScene, screen *golib.Screen) {
	drawRobotPanelButton(s, screen)
	if !s.robotsOpen {
		return
	}
	layout := robotPanelLayoutFor(s)
	screen.DrawRectangle(layout.box, panelColor)
	screen.DrawRectangleOutline(layout.box, 1, panelEdgeColor)
	screen.DrawText("colony robots", layout.box.X+12,
		layout.box.Y+8, titleSize, panelTextColor, uiText)
	page := fmt.Sprintf("%d/%d", layout.page+1, layout.pages)
	screen.DrawText(page, layout.box.X+layout.box.Width-77,
		layout.box.Y+10, 10, panelDimColor,
		golib.TextOptions{Font: uiFont, Align: golib.AlignCenter})
	drawRobotPanelControl(screen, layout.prev, "<",
		layout.page > 0, s.mouse)
	drawRobotPanelControl(screen, layout.next, ">",
		layout.page+1 < layout.pages, s.mouse)
	drawRobotPanelClose(screen, layout.close, s.mouse)
	groups := robotPanelGroups(s.state)
	entries := 0
	for _, group := range groups {
		entries += len(group.robots)
	}
	if entries == 0 {
		screen.DrawText("no builders, workers or mechanics",
			layout.box.X+12, layout.box.Y+54, 11, panelDimColor, uiText)
	}
	lastGroup := ""
	for _, row := range layout.rows {
		group := robotPanelGroupFor(groups, row.robot)
		if group.title != lastGroup {
			y := row.area.Y - robotPanelGroupHeight + 3
			screen.DrawText(group.title, layout.box.X+13, y, 10,
				group.color, uiText)
			lastGroup = group.title
		}
		fill := buttonColor
		if row.robot.ID == s.robotsPicked {
			fill = buttonHoverColor
		} else if row.area.Contains(s.mouse.X, s.mouse.Y) {
			fill = buttonHoverColor
		}
		screen.DrawRectangle(row.area, fill)
		screen.DrawRectangleOutline(row.area, 1, buttonEdgeColor)
		drawRobotPanelIcon(screen, row.robot,
			row.area.X+18, row.area.Y+row.area.Height/2, 19.2)
		label := fmt.Sprintf("#%d", row.robot.ID)
		screen.DrawText(label, row.area.X+38, row.area.Y+3,
			11, panelTextColor, uiText)
		activity := robotPanelActivity(s.state, row.robot)
		screen.DrawText(activity, row.area.X+82, row.area.Y+3,
			10, groupColorForRobot(row.robot), uiText)
	}
	drawRobotPanelDetails(s, screen, layout)
}

func robotPanelGroupFor(groups []robotPanelGroup, robot Robot) robotPanelGroup {
	for _, group := range groups {
		for _, member := range group.robots {
			if member.ID == robot.ID {
				return group
			}
		}
	}
	return robotPanelGroup{color: panelDimColor}
}

func groupColorForRobot(robot Robot) golib.Color {
	switch robot.Kind {
	case RobotBuilder:
		return robotColor
	case RobotWorker:
		return factoryColor
	case RobotRepair:
		return chargerColor
	}
	return panelTextColor
}

func drawRobotPanelDetails(
	s *playScene,
	screen *golib.Screen,
	layout robotPanelLayout,
) {
	x := layout.box.X + 12
	y := layout.box.Y + layout.box.Height - 98
	screen.DrawLine(x, y-7, layout.box.X+layout.box.Width-12, y-7,
		1, panelEdgeColor)
	if !layout.hasSelected {
		screen.DrawText("select a robot to assign or recall it",
			x, y, 11, panelDimColor, uiText)
		return
	}
	r := layout.selected
	role := string(r.Kind)
	screen.DrawText(fmt.Sprintf("#%d · %s · %s", r.ID, role,
		robotPanelActivity(s.state, r)), x, y, 10, panelTextColor, uiText)
	post := "no deposit"
	if r.hasPost() {
		post = fmt.Sprintf("deposit tile %d,%d", r.PostCol, r.PostRow)
	}
	screen.DrawText(post, x, y+15, 10, panelDimColor, uiText)
	if r.tanked() {
		screen.DrawText(fmt.Sprintf("tank %s / %s",
			si(r.Tank, "L"), si(robotTankLiters, "L")), x,
			y+30, 10, oilColor, uiText)
	}
	if r.Kind == RobotBuilder || r.Kind == RobotWorker {
		label := "assign deposit"
		if s.assigningRobot == r.ID {
			label = "cancel assign"
		}
		drawRobotPanelButtonAt(screen, layout.assign, label,
			layout.assign.Contains(s.mouse.X, s.mouse.Y), true)
	}
	if r.hasPost() {
		drawRobotPanelButtonAt(screen, layout.recall, "recall",
			layout.recall.Contains(s.mouse.X, s.mouse.Y), true)
	}
}

func drawRobotPanelButton(s *playScene, screen *golib.Screen) {
	area := robotPanelButtonRect()
	fill := buttonColor
	edge := buttonEdgeColor
	if area.Contains(s.mouse.X, s.mouse.Y) {
		fill = buttonHoverColor
	}
	if s.robotsOpen {
		edge = coreGlowColor
	}
	screen.DrawRectangle(area, fill)
	screen.DrawRectangleOutline(area, 1, edge)
	drawRobotPanelIcon(screen, Robot{Kind: RobotBuilder},
		area.X+17, area.Y+area.Height/2, 14.4)
	drawRobotPanelIcon(screen, Robot{Kind: RobotWorker},
		area.X+43, area.Y+area.Height/2, 14.4)
	count := len(s.state.Robots)
	screen.DrawText(fmt.Sprintf("robots %d", count), area.X+58,
		area.Y+8, 12, panelTextColor, uiText)
}

// robotIconSpan is each chassis' drawn content inside the 128x192
// frames, in frame pixels, measured from the PNG sheets: the models are
// small in their frames and differ in height, so the icon scales by
// content and centers on it instead of on the frame's foot line.
var robotIconSpan = map[RobotKind][2]float32{
	RobotBuilder: {138, 166},
	RobotWorker:  {126, 169},
	RobotRepair:  {113, 168},
	RobotCombat:  {126, 170},
}

// drawRobotPanelIcon draws a model scaled to height screen pixels and
// centered on centerY, so every kind reads at the same size whatever its
// frame padding.
func drawRobotPanelIcon(
	screen *golib.Screen,
	robot Robot,
	x, centerY, height float32,
) {
	model := robotModel(robot.Kind)
	span := robotIconSpan[robot.Kind]
	if span == [2]float32{} {
		span = robotIconSpan[RobotWorker]
	}
	scale := height / (span[1] - span[0])
	screen.DrawSprite(model.sprite, int(robot.Facing%8), x, centerY,
		golib.DrawOptions{
			OriginX: modelFootX,
			OriginY: (span[0] + span[1]) / 2,
			Scale:   scale,
		})
}

func drawRobotPanelControl(
	screen *golib.Screen,
	area golib.Rectangle,
	label string,
	enabled bool,
	mouse golib.Vector2,
) {
	fill := buttonColor
	ink := panelTextColor
	if !enabled {
		fill, ink = panelColor, panelDimColor
	} else if area.Contains(mouse.X, mouse.Y) {
		fill = buttonHoverColor
	}
	screen.DrawRectangle(area, fill)
	screen.DrawRectangleOutline(area, 1, buttonEdgeColor)
	screen.DrawText(label, area.X+area.Width/2,
		area.Y+2, 11, ink,
		golib.TextOptions{Font: uiFont, Align: golib.AlignCenter})
}

func drawRobotPanelClose(
	screen *golib.Screen,
	area golib.Rectangle,
	mouse golib.Vector2,
) {
	color := panelDimColor
	if area.Contains(mouse.X, mouse.Y) {
		color = dangerColor
	}
	screen.DrawLine(area.X+4, area.Y+4, area.X+14, area.Y+14, 2, color)
	screen.DrawLine(area.X+14, area.Y+4, area.X+4, area.Y+14, 2, color)
}

func drawRobotPanelButtonAt(
	screen *golib.Screen,
	area golib.Rectangle,
	label string,
	hover, enabled bool,
) {
	fill, ink := buttonColor, panelTextColor
	if !enabled {
		fill, ink = panelColor, panelDimColor
	} else if hover {
		fill = buttonHoverColor
	}
	screen.DrawRectangle(area, fill)
	screen.DrawRectangleOutline(area, 1, buttonEdgeColor)
	screen.DrawText(label, area.X+7, area.Y+4,
		10, ink, uiText)
}
