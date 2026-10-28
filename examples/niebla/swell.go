package main

import (
	"math"

	"golib"
)

// Waves of shade roll in across the fog toward the line, like wind over
// snow. Static crawls over exposed air even in calm, faint in the haze and
// darker in thick fog. Both are drawn from the state's tick alone: no
// randomness, nothing kept between frames, so a save redraws them.

const (
	swellWaves       = 7    // crests between the rim and the line
	swellWaveRim     = 17.0 // tiles from the core where the crests are born
	swellWaveSpeed   = 0.6  // tiles per second, toward the core
	swellWaveWidth   = 0.22 // tiles, a crest's shade from edge to edge
	swellWaveOpacity = 0.16 // a crest at its darkest, the swell pressed in whole
	swellWaveArcs    = 96   // pieces a crest is drawn in

	swellStaticSpecks  = 900 // specks on the screen at full exposure
	swellStaticTicks   = 3   // updates a speck lasts
	swellStaticOpacity = 0.3 // opacity at full exposure
)

// drawSwellWaves paints the crests over the fog's cover, in the world.
func drawSwellWaves(s *State, screen *golib.Screen) {
	pressure := float32(s.Fog.Pressure)
	if pressure <= 0 {
		return
	}
	cx, cy := projectCore()
	clear := clearDiscs(s)
	line := fogLineNow(s)
	span := swellWaveRim - line
	seconds := float32(s.Ticks) / 60
	for wave := 0; wave < swellWaves; wave++ {
		// 0 at the rim, 1 at the line: a crest is born faint, darkens
		// on its way in and breaks on the line.
		along := float32(math.Mod(
			float64(seconds*swellWaveSpeed/span+float32(wave)/swellWaves), 1))
		radius := swellWaveRim - along*span
		strength := pressure * swellWaveOpacity *
			float32(math.Sin(float64(along)*math.Pi*0.5))
		drawSwellCrest(screen, clear, cx, cy, radius, strength, wave, seconds)
	}
}

// drawSwellCrest paints one crest as a ring of quads, a wide faint one
// under a narrow one, so its shade has no hard edge. The ring wobbles
// and breaks into arcs, each crest its own way, so no two read as the
// same circle. A crest is the mist's, so it stops at the clear circles.
func drawSwellCrest(
	screen *golib.Screen,
	clear []disc,
	cx, cy, radius, strength float32,
	wave int,
	seconds float32,
) {
	phase := float64(wave) * 2.399
	at := func(i int, r float32) golib.Vector2 {
		angle := float64(i) / swellWaveArcs * 2 * math.Pi
		wobble := 0.25*math.Sin(3*angle+phase) + 0.12*math.Sin(7*angle-phase*1.7)
		halfW, halfH := ellipseSemiAxes(r + float32(wobble))
		return golib.Vector2{
			X: cx + halfW*float32(math.Cos(angle)),
			Y: cy + halfH*float32(math.Sin(angle)),
		}
	}
	for i := 0; i < swellWaveArcs; i++ {
		angle := float64(i) / swellWaveArcs * 2 * math.Pi
		broken := 0.5 + 0.5*math.Sin(5*angle+phase*3+float64(seconds)*0.3)
		opacity := strength * float32(broken*broken)
		if opacity < 0.004 {
			continue
		}
		if a, b := at(i, radius), at(i+1, radius); inDiscs(clear, (a.X+b.X)/2, (a.Y+b.Y)/2) {
			continue
		}
		for _, layer := range []struct{ width, share float32 }{
			{swellWaveWidth, 0.4}, {swellWaveWidth * 0.4, 0.6},
		} {
			in, out := radius-layer.width/2, radius+layer.width/2
			screen.DrawPolygon([]golib.Vector2{
				at(i, in), at(i+1, in), at(i+1, out), at(i, out),
			}, golib.WithOpacity(golib.Black, opacity*layer.share))
		}
	}
}

// drawSwellStatic paints the crawl, in screen pixels: specks of shade
// that jump every few ticks, only where the mist stands and thicker
// where it is thicker. Call it with no camera set.
func drawSwellStatic(s *State, screen *golib.Screen, camera *golib.Camera) {
	beat := uint32(s.Ticks / swellStaticTicks)
	clear := clearDiscs(s)
	for i := 0; i < swellStaticSpecks; i++ {
		x := hashUnit(beat, uint32(i), 1) * float32(screenWidth)
		y := hashUnit(beat, uint32(i), 2) * float32(screenHeight)
		world := camera.ToWorld(x, y)
		if inDiscs(clear, world.X, world.Y) {
			continue
		}
		ux, uy := unitsAtWorld(float64(world.X), float64(world.Y))
		fog := float32(fogExposureAt(s, ux, uy))
		if fog <= 0 {
			continue
		}
		screen.DrawRectangle(
			golib.Rectangle{X: float32(int(x)), Y: float32(int(y)), Width: 1, Height: 1},
			golib.WithOpacity(golib.Black, swellStaticOpacity*fog),
		)
	}
}

// hashUnit returns a number from 0 up to 1 that depends on nothing but
// its three arguments.
func hashUnit(a, b, c uint32) float32 {
	h := a*0x9E3779B1 ^ b*0x85EBCA77 ^ c*0xC2B2AE3D
	h ^= h >> 15
	h *= 0x2C1B3C6D
	h ^= h >> 12
	h *= 0x297A2D39
	h ^= h >> 15
	return float32(h>>8) / (1 << 24)
}
