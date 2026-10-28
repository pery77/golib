package main

import "testing"

func TestResizeScenes(t *testing.T) {
	oldWidth, oldHeight, oldResize := screenWidth, screenHeight, activeResize
	t.Cleanup(func() {
		screenWidth, screenHeight, activeResize = oldWidth, oldHeight, oldResize
	})

	menu := newMenuScene()
	resizeScreen(640, 360)
	for _, button := range menu.buttons {
		if button.bounds.X < 0 ||
			button.bounds.X+button.bounds.Width > float32(screenWidth) ||
			button.bounds.Y+button.bounds.Height > float32(screenHeight) {
			t.Errorf("menu button outside 640x360 screen: %+v", button.bounds)
		}
	}
	resizeScreen(512, 300)
	for _, button := range menu.buttons {
		if button.bounds.Y+button.bounds.Height > float32(screenHeight) {
			t.Errorf("menu button outside 512x300 screen: %+v", button.bounds)
		}
	}
	play := newPlayScene(nil)
	for _, size := range [][2]int{{640, 360}, {512, 300}, {960, 540}} {
		resizeScreen(size[0], size[1])
		center := play.camera.ToScreen(play.camera.Target)
		if center.X != float32(size[0])/2 ||
			center.Y != float32(size[1])/2 {
			t.Errorf("screen %dx%d: camera target at %+v",
				size[0], size[1], center)
		}
		panel := robotPanelRect()
		if panel.X < 0 || panel.Y < 0 ||
			panel.X+panel.Width > float32(screenWidth) ||
			panel.Y+panel.Height > float32(screenHeight) {
			t.Errorf("screen %dx%d: roster outside: %+v",
				size[0], size[1], panel)
		}
		layout := robotPanelLayoutFor(play)
		if layout.assign.Y+layout.assign.Height > panel.Y+panel.Height {
			t.Errorf("screen %dx%d: roster controls outside", size[0], size[1])
		}
	}
}
