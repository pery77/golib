package golib

import "testing"

func TestCaptureFreezesPixelsAndKeepsDrawing(t *testing.T) {
	screen, draw := openTestWindow(t, 16, 8)
	photo := NewImage(8, 4)
	result := draw(func() {
		screen.Clear(Red)
		screen.DrawRectangle(Rectangle{X: 8, Width: 8, Height: 8}, Blue)
		screen.DrawRectangle(Rectangle{Width: 2, Height: 2}, Gold)
		screen.Capture(photo)
		// Reading must leave the framebuffer bound and the saved pixels
		// unchanged by the drawing that follows it.
		screen.Clear(Green)
		screen.DrawImage(photo, 0, 0)
	})
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
	if photo.Pixel(0, 0) != Gold || photo.Pixel(0, 3) != Red || photo.Pixel(7, 3) != Blue {
		t.Fatalf("capture lost the colors: %v %v", photo.Pixel(0, 0), photo.Pixel(7, 3))
	}
	if c := result.NRGBAAt(12, 6); c.G != Green.G || c.R != Green.R {
		t.Errorf("drawing after capture went to another target: %v", c)
	}
	result = draw(func() { screen.Clear(Green); screen.DrawImage(photo, 0, 0) })
	if c := result.NRGBAAt(1, 1); c.R != Red.R || c.G != Red.G {
		t.Errorf("saved red pixels changed: %v", c)
	}
	if c := result.NRGBAAt(6, 2); c.B != Blue.B || c.R != Blue.R {
		t.Errorf("saved blue pixels changed: %v", c)
	}
	// A second capture updates the same reusable image and texture.
	draw(func() { screen.Clear(Gold); screen.Capture(photo); screen.Clear(Black); screen.DrawImage(photo, 0, 0) })
	if photo.Pixel(0, 0) != Gold {
		t.Error("recapture did not replace the old photo")
	}
}

func TestCaptureUsesTheRenderersAntialiasTarget(t *testing.T) {
	photo := NewImage(8, 4)
	drawFrame(t, Config{Width: 16, Height: 8, Antialias: true}, func(s *Screen) {
		s.Clear(Blue)
		s.DrawRectangle(Rectangle{Width: 8, Height: 4}, Gold)
		s.Capture(photo)
	})
	if photo.Pixel(1, 1) != Gold || photo.Pixel(6, 3) != Blue {
		t.Fatal("capture did not resize the actual antialias target or inverted its rows")
	}
}

func TestCaptureReportsMisuse(t *testing.T) {
	s := &Screen{}
	s.Capture(nil)
	wantError(t, "Screen.Capture got a nil picture")
	s.Capture(NewImage(0, 1))
	wantError(t, "1 to 4096 pixels")
	s.Capture(NewImage(1, 1))
	wantError(t, "Screen.Capture must be called from Draw")
}
