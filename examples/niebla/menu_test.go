package main

import (
	"fmt"
	"testing"
)

func TestSavedGameMenuAndLoadListFitSmallScreens(t *testing.T) {
	oldDB, oldPlayer, oldResize := db, player, activeResize
	st, _ := openTestStore(t)
	id, err := st.player("menu-test")
	if err != nil {
		t.Fatal(err)
	}
	db, player = st, id
	t.Cleanup(func() { db, player, activeResize = oldDB, oldPlayer, oldResize })
	menu := newMenuScene()
	if len(menu.buttons) != 2 || menu.buttons[0].action != playButton {
		t.Fatal("an empty store did not offer Play first")
	}
	for i := 1; i <= 12; i++ {
		if err := st.saveState(id, fmt.Sprint(i), newGame()); err != nil {
			t.Fatal(err)
		}
	}
	menu = newMenuScene()
	for i, saved := range menu.saves {
		if saved.Slot != fmt.Sprint(i+1) {
			t.Fatalf("slot %s appears at position %d", saved.Slot, i)
		}
	}
	want := []menuAction{continueButton, newButton, loadButton, quitButton}
	if len(menu.buttons) != len(want) {
		t.Fatalf("saved-game menu has %d buttons", len(menu.buttons))
	}
	for i, action := range want {
		if menu.buttons[i].action != action {
			t.Fatalf("button %d has action %d, want %d",
				i, menu.buttons[i].action, action)
		}
	}
	load := newLoadScene(menu.saves)
	for _, size := range [][2]int{{1280, 720}, {640, 360}, {512, 300}} {
		menu.resize(size[0], size[1])
		for _, button := range menu.buttons {
			assertMenuButtonFits(t, button, size[0], size[1])
		}
		load.resize(size[0], size[1])
		for selected := range load.buttons {
			load.selected = selected
			load.layout(size[0], size[1])
			button := load.buttons[selected]
			if button.bounds.Height == 0 {
				t.Fatalf("selected row %d is hidden at %v", selected, size)
			}
			assertMenuButtonFits(t, button, size[0], size[1])
			center := button.bounds.Center()
			if buttonAt(load.buttons, center.X, center.Y) != selected {
				t.Fatalf("selected row %d cannot be clicked at %v", selected, size)
			}
		}
	}
}

func assertMenuButtonFits(t *testing.T, button menuButton, width, height int) {
	t.Helper()
	b := button.bounds
	if b.X < 0 || b.Y < 0 || b.X+b.Width > float32(width) ||
		b.Y+b.Height > float32(height-24) {
		t.Fatalf("button %q does not fit %dx%d: %+v",
			button.label, width, height, b)
	}
}
