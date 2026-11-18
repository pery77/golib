package golib

import (
	"fmt"
	"golib/internal/device"
)

// Capture copies everything drawn on the screen so far into picture, a
// reusable Image made with NewImage. Call it only from Draw, after End3D if
// drawing in 3D. Later drawing and later updates leave the picture unchanged.
// Draw it with Screen.DrawImage, for example as a paused monitor or a save
// slot's thumbnail. Capture fills the picture's size with nearest-pixel
// sampling; choose the screen's aspect ratio to avoid stretching the copy.
//
// The copy is before post-processing and excludes FullResolution drawing
// and the mouse sprite, which are presented later. It stays in memory; it
// never writes a screenshot to disk. Reading back from the graphics card
// costs time, so capture on demand rather than in every frame.
//
//	// In Draw, after drawing the game, before drawing the pause menu:
//	if takePhoto {
//	    screen.Capture(photo) // photo := golib.NewImage(640, 360), made once
//	}
func (s *Screen) Capture(picture *Image) {
	if picture == nil {
		reportError(fmt.Errorf("golib: Screen.Capture got a nil picture: make one with golib.NewImage"))
		return
	}
	if err := picture.sizeError(); err != nil {
		reportError(err)
		return
	}
	if s.captureTarget.ID == 0 {
		reportError(fmt.Errorf("golib: Screen.Capture must be called from Draw, after drawing the picture to capture"))
		return
	}
	if s.pass.open {
		reportError(fmt.Errorf("golib: Screen.Capture needs screen.End3D() before capturing a 3D pass"))
		return
	}
	device.FlushDrawing()
	source := device.ReadTarget(s.captureTarget)
	if source == nil || source.Bounds().Empty() {
		reportError(fmt.Errorf("golib: Screen.Capture could not read the drawing surface"))
		return
	}
	picture.mu.Lock()
	defer picture.mu.Unlock()
	w, h := source.Bounds().Dx(), source.Bounds().Dy()
	for y := range picture.height {
		for x := range picture.width {
			from := source.PixOffset(x*w/picture.width, y*h/picture.height)
			to := (y*picture.width + x) * 4
			copy(picture.pixels[to:to+4], source.Pix[from:from+4])
		}
	}
	picture.changed = true
}
