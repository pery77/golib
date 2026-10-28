package main

import (
	"fmt"
	"log"

	"golib"
)

const saveRowStep = 52

type loadScene struct {
	saves    []saveInfo
	buttons  []menuButton
	selected int
	first    int
	capacity int
	warning  string
}

func newLoadScene(saves []saveInfo) *loadScene {
	s := &loadScene{saves: saves}
	activeResize = s.resize
	s.resize(screenWidth, screenHeight)
	return s
}

func (s *loadScene) resize(width, height int) {
	s.capacity = max(1, (height-160)/saveRowStep)
	s.layout(width, height)
}

func (s *loadScene) layout(width, height int) {
	count := len(s.saves) + 1
	s.selected = moveSelection(s.selected, 0, count)
	s.first = max(0, min(s.first, count-s.capacity))
	if s.selected < s.first {
		s.first = s.selected
	}
	if s.selected >= s.first+s.capacity {
		s.first = s.selected - s.capacity + 1
	}
	rowWidth := float32(min(560, width-32))
	s.buttons = make([]menuButton, count)
	for i := s.first; i < min(count, s.first+s.capacity); i++ {
		label := "Back"
		if i < len(s.saves) {
			label = "Save " + s.saves[i].Slot
		}
		s.buttons[i] = menuButton{
			label: label,
			bounds: golib.Rectangle{
				X:     (float32(width) - rowWidth) / 2,
				Y:     96 + float32((i-s.first)*saveRowStep),
				Width: rowWidth, Height: 44,
			},
		}
	}
}

func (s *loadScene) Update(input *golib.Input, dt float32) {
	if menuBack(input) {
		golib.SwitchScene(newMenuScene())
		return
	}
	var confirm bool
	s.selected, confirm = updateMenuSelection(input, s.buttons, s.selected)
	s.layout(screenWidth, screenHeight)
	if !confirm {
		return
	}
	if s.selected == len(s.saves) {
		golib.SwitchScene(newMenuScene())
		return
	}
	slot := s.saves[s.selected].Slot
	if err := loadGame(slot); err != nil {
		log.Print(err)
		s.warning = "Could not load Save " + slot + "."
	}
}

func (s *loadScene) Draw(screen *golib.Screen) {
	screen.Clear(fogColor)
	drawCentered(screen, "Load game", 24, 32, textColor)
	if len(s.saves) == 0 {
		drawCentered(screen, "No saved games yet.", 68, 13, groundColor)
	} else {
		position := fmt.Sprintf("%d / %d", s.selected+1, len(s.saves))
		if s.selected == len(s.saves) {
			position = "Back to the menu"
		}
		drawCentered(screen, position, 68, 13, groundColor)
	}
	for i := s.first; i < min(len(s.buttons), s.first+s.capacity); i++ {
		button := s.buttons[i]
		if i == len(s.saves) {
			button.draw(screen, i == s.selected)
			continue
		}
		fill := buttonColor
		if i == s.selected {
			fill = buttonHoverColor
		}
		screen.DrawRectangle(button.bounds, fill)
		screen.DrawRectangleOutline(button.bounds, 2, buttonEdgeColor)
		x, y := button.bounds.X+12, button.bounds.Y+5
		screen.DrawText(button.label, x, y, 16, panelTextColor, uiText)
		saved := s.saves[i]
		details := saved.UpdatedAt.Local().Format("2006-01-02 15:04:05") +
			"   |   played " + savePlayTime(saved.Ticks)
		screen.DrawText(details, x, y+20, 12, panelDimColor, uiText)
	}
	if s.warning != "" {
		drawCentered(screen, s.warning, float32(screenHeight)-44,
			12, groundColor)
	}
	drawCentered(screen, "Enter / click: load   Arrows / wheel: choose   Esc: back",
		float32(screenHeight)-20, 12, groundColor)
}

func savePlayTime(ticks int64) string {
	seconds := max(0, ticks/60)
	if seconds >= 3600 {
		return fmt.Sprintf("%dh %02dm", seconds/3600, seconds/60%60)
	}
	if seconds >= 60 {
		return fmt.Sprintf("%dm %02ds", seconds/60, seconds%60)
	}
	return fmt.Sprintf("%ds", seconds)
}
