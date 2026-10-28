package main

import (
	"golib"
	"math"
)

type star struct{ x, y, r, phase float32 }

func makeStars() []star {
	v := make([]star, 160)
	for i := range v {
		v[i] = star{golib.RandomFloat(0, 1280), golib.RandomFloat(0, 720), golib.RandomFloat(0.6, 1.8), golib.RandomFloat(0, 6.28)}
	}
	return v
}
func mixColor(a, b golib.Color, t float32) golib.Color {
	return rgb(uint8(golib.Lerp(float32(a.R), float32(b.R), t)), uint8(golib.Lerp(float32(a.G), float32(b.G), t)), uint8(golib.Lerp(float32(a.B), float32(b.B), t)))
}
func drawSky(s *golib.Screen, a *session, region int, parallax float32) {
	s.Clear(ink)
	palettes := [5]golib.Color{rgb(21, 48, 61), rgb(43, 34, 63), rgb(56, 35, 38), rgb(21, 50, 57), rgb(23, 33, 60)}
	for y := 0; y < 720; y += 6 {
		t := float32(y) / 720
		rect(s, 0, float32(y), 1280, 6, mixColor(ink, palettes[region], t*t))
	}
	for _, v := range a.stars {
		x := float32(math.Mod(float64(v.x-parallax*0.035+2560), 1280))
		alpha := 0.35 + 0.3*(sin(a.time*0.4+v.phase)+1)/2
		s.DrawCircle(x, v.y, v.r, fade(white, alpha))
		if v.r > 1.68 {
			s.DrawLine(x-4, v.y, x+4, v.y, 0.5, fade(cyan, alpha*0.4))
			s.DrawLine(x, v.y-4, x, v.y+4, 0.5, fade(cyan, alpha*0.4))
		}
	}
}

var planetSprites = [...]*golib.Sprite{
	golib.NewSprite("planets/0.png"), golib.NewSprite("planets/1.png"),
	golib.NewSprite("planets/2.png"), golib.NewSprite("planets/3.png"),
	golib.NewSprite("planets/4.png"),
}

func drawPlanet(s *golib.Screen, x, y, r float32, region int) {
	scale := r / 258
	s.DrawSprite(planetSprites[region], 0, x, y, golib.DrawOptions{Scale: scale, OriginX: 300, OriginY: 300})
	s.DrawCircleOutline(x, y, r+2, 0.7, fade(cyan, 0.2))
}
func shipPoint(p golib.Vector2, angle, scale, x, y float32) golib.Vector2 {
	return golib.Vector2{X: x * scale, Y: y * scale}.Rotate(angle).Add(p)
}
func drawShip(s *golib.Screen, p golib.Vector2, angle, scale float32, model int, thrust, t float32) {
	accent := [3]golib.Color{cyan, orange, rgb(180, 169, 255)}[model]
	poly := func(c golib.Color, values ...float32) {
		pts := make([]golib.Vector2, 0, len(values)/2)
		for i := 0; i < len(values); i += 2 {
			pts = append(pts, shipPoint(p, angle, scale, values[i], values[i+1]))
		}
		s.DrawPolygon(pts, c)
	}
	line := func(x1, y1, x2, y2, w float32, c golib.Color) {
		a, b := shipPoint(p, angle, scale, x1, y1), shipPoint(p, angle, scale, x2, y2)
		s.DrawLine(a.X, a.Y, b.X, b.Y, w*scale, c)
	}
	if thrust > 0 {
		length := (28 + 12*sin(t*61)) * thrust
		s.SetBlendMode(golib.BlendAdd)
		for i := 5; i > 0; i-- {
			q := shipPoint(p, angle, scale, 0, 20+length*0.35)
			s.DrawCircle(q.X, q.Y, float32(i)*scale*5, fade(orange, 0.024))
		}
		poly(fade(orange, 0.5), -8, 14, 0, 20+length*1.4, 8, 14)
		poly(rgb(255, 200, 122), -5, 15, 0, 19+length, 5, 15)
		poly(white, -2.5, 15, 0, 20+length*0.52, 2.5, 15)
		s.SetBlendMode(golib.BlendNormal)
	}
	// Landing struts, feet and thermal skirt.
	line(-9, 8, -19, 23, 3, rgb(113, 133, 143))
	line(9, 8, 19, 23, 3, rgb(113, 133, 143))
	line(-24, 23, -14, 23, 3, white)
	line(14, 23, 24, 23, 3, white)
	poly(rgb(44, 62, 73), -8, 9, -7, 17, 7, 17, 8, 9)
	if model == 1 {
		poly(rgb(130, 146, 156), -10, -5, -31, 11, -24, 15, -8, 10)
		poly(rgb(86, 111, 127), 10, -5, 31, 11, 24, 15, 8, 10)
		line(-26, 10, -13, 0, 1, accent)
		line(26, 10, 13, 0, 1, accent)
	}
	if model == 2 {
		poly(rgb(120, 131, 157), -23, -9, -14, -15, -10, 13, -23, 11)
		poly(rgb(73, 88, 122), 23, -9, 14, -15, 10, 13, 23, 11)
		line(-21, -5, -21, 6, 2, accent)
		line(21, -5, 21, 6, 2, accent)
	}
	poly(rgb(169, 184, 190), 0, -25, -12, -12, -14, 6, -8, 12, 8, 12, 14, 6, 12, -12)
	poly(rgb(100, 126, 141), 0, -25, 12, -12, 14, 6, 8, 12, 0, 12)
	poly(rgb(223, 231, 224), 0, -25, -12, -12, 0, -10)
	poly(rgb(10, 30, 43), -8, -12, -7, -2, 7, -2, 8, -12, 0, -18)
	poly(accent, -6, -11, -5, -5, 5, -5, 6, -11, 0, -15)
	poly(fade(white, 0.65), -5, -11, -4, -8, 4, -8, 4, -11)
	line(-10, 3, 10, 3, 1, rgb(46, 68, 78))
	line(-8, 7, -3, 7, 2, accent)
	line(3, 7, 8, 7, 2, accent)
	line(8, -15, 14, -23, 1, white)
	q := shipPoint(p, angle, scale, 14, -23)
	s.DrawCircle(q.X, q.Y, 1.3*scale, accent)
}
func drawTerrain(s *golib.Screen, w *world, t float32, bases [5]int) {
	pts := w.land.terrain
	base := [5]golib.Color{rgb(39, 53, 64), rgb(49, 44, 66), rgb(65, 44, 46), rgb(29, 56, 61), rgb(36, 43, 63)}[w.contract.Region]
	// Each strip is a faceted rock volume, with a brightly lit upper lip.
	for i := 1; i < len(pts); i++ {
		a, b := pts[i-1], pts[i]
		mid := golib.Vector2{X: golib.Lerp(a.X, b.X, 0.48), Y: max(a.Y, b.Y) + 95 + float32(i%4)*26}
		s.DrawTriangle(a.X, a.Y, b.X, b.Y, mid.X, mid.Y, mixColor(base, white, 0.045*float32(i%3)))
		s.DrawTriangle(a.X, a.Y, mid.X, mid.Y, a.X, 1700, mixColor(base, ink, 0.22))
		s.DrawTriangle(b.X, b.Y, b.X, 1700, mid.X, mid.Y, mixColor(base, ink, 0.42))
		s.DrawTriangle(a.X, 1700, mid.X, mid.Y, b.X, 1700, mixColor(base, ink, 0.5))
		s.DrawLine(a.X, a.Y, b.X, b.Y, 2, mixColor(base, muted, 0.38))
		s.DrawLine(a.X, a.Y, mid.X, mid.Y, 0.7, fade(muted, 0.11))
		for k := 1; k < 9; k++ {
			xx := golib.Lerp(a.X, b.X, float32(k)/9)
			yy := golib.Lerp(a.Y, b.Y, float32(k)/9)
			rr := 2 + float32((i*k)%5)
			s.DrawTriangle(xx-rr, yy+18, xx+rr, yy+20, xx, yy+13-rr, fade(muted, 0.15))
		}
	}
	for i, pad := range w.land.pads {
		active := i == w.contract.Kind
		c := muted
		if active {
			c = cyan
		}
		// Solar array, communications mast and modular habitat behind the pad.
		level := bases[w.contract.Region]
		drawBase(s, pad.X-108, pad.Y, 1, level, c, t)
		rect(s, pad.X, pad.Y, pad.Width, 12, rgb(35, 53, 62))
		rect(s, pad.X, pad.Y, pad.Width, 3, c)
		for x := pad.X + 8; x < pad.X+pad.Width; x += 22 {
			s.DrawLine(x, pad.Y+6, x+7, pad.Y+11, 2, orange)
		}
		for _, x := range []float32{pad.X + 4, pad.X + pad.Width - 4} {
			s.DrawLine(x, pad.Y-19, x, pad.Y, 2, c)
			s.DrawCircle(x, pad.Y-20, 3, c)
			if active {
				s.SetBlendMode(golib.BlendAdd)
				for j := 4; j > 0; j-- {
					s.DrawCircle(x, pad.Y-20, float32(j)*6, fade(c, 0.025))
				}
				s.SetBlendMode(golib.BlendNormal)
			}
		}
		if active {
			s.DrawLine(pad.Center().X, pad.Y-95, pad.Center().X, pad.Y-30, 1, fade(cyan, 0.25))
			text(s, "LANDING ZONE", pad.X+8, pad.Y+23, 12, c)
			for j := 0; j < 3; j++ {
				yy := pad.Y - 54 - float32(j)*17 - float32(math.Mod(float64(t*18), 17))
				s.DrawLine(pad.Center().X-7, yy, pad.Center().X, yy+5, 1.5, fade(cyan, 0.65-float32(j)*0.18))
				s.DrawLine(pad.Center().X, yy+5, pad.Center().X+7, yy, 1.5, fade(cyan, 0.65-float32(j)*0.18))
			}
		} else {
			text(s, "INACTIVE", pad.X+15, pad.Y+23, 12, muted)
		}
	}
}
func drawBase(s *golib.Screen, x, y, scale float32, level int, c golib.Color, t float32) {
	// Dome, airlock, antenna and photovoltaic panels.
	s.DrawCircle(x+30*scale, y-16*scale, 26*scale, rgb(56, 76, 88))
	s.DrawCircle(x+30*scale, y-16*scale, 22*scale, rgb(20, 41, 54))
	rect(s, x, y-16*scale, 65*scale, 16*scale, rgb(68, 83, 89))
	rect(s, x+11*scale, y-19*scale, 7*scale, 15*scale, c)
	rect(s, x+26*scale, y-19*scale, 20*scale, 3*scale, fade(c, 0.4))
	s.DrawLine(x+62*scale, y, x+62*scale, y-65*scale, 1*scale, muted)
	s.DrawLine(x+62*scale, y-65*scale, x+77*scale, y-74*scale, 2*scale, white)
	s.DrawCircle(x+62*scale, y-65*scale, 2*scale, fade(c, 0.5+sin(t*3)*0.4))
	if level > 0 {
		for j := 0; j < level; j++ {
			xx := x - 35*float32(j+1)*scale
			rect(s, xx, y-27*scale, 28*scale, 17*scale, rgb(34, 67, 86))
			s.DrawRectangleOutline(golib.Rectangle{X: xx, Y: y - 27*scale, Width: 28 * scale, Height: 17 * scale}, scale, c)
			s.DrawLine(xx+14*scale, y-10*scale, xx+14*scale, y, 2*scale, muted)
			s.DrawLine(xx, y-18*scale, xx+28*scale, y-18*scale, scale, lineColor)
			s.DrawLine(xx+14*scale, y-27*scale, xx+14*scale, y-10*scale, scale, lineColor)
		}
	}
}

type particle struct {
	p, v                golib.Vector2
	life, maxLife, size float32
	c                   golib.Color
}

func (p *playScene) emit(position, velocity golib.Vector2, life, size float32, c golib.Color) {
	if len(p.particles) < 650 {
		p.particles = append(p.particles, particle{position, velocity, life, life, size, c})
	}
}
func (p *playScene) updateParticles(dt float32) {
	out := p.particles[:0]
	for _, v := range p.particles {
		v.life -= dt
		if v.life <= 0 {
			continue
		}
		v.p = v.p.Add(v.v.Scale(dt))
		v.v = v.v.Scale(1 - dt*0.3)
		out = append(out, v)
	}
	p.particles = out
	w := p.world
	if w.thrust > 0 && w.state == flying {
		for range 3 {
			origin := shipPoint(w.pos, w.angle, 1, golib.RandomFloat(-5, 5), 18)
			velocity := golib.Vector2{X: golib.RandomFloat(-22, 22), Y: golib.RandomFloat(60, 155)}.Rotate(w.angle).Add(w.vel.Scale(0.3))
			p.emit(origin, velocity, golib.RandomFloat(0.25, 0.7), golib.RandomFloat(1, 4), orange)
		}
		if w.altitude() < 35 {
			for range 2 {
				origin := golib.Vector2{X: w.pos.X + golib.RandomFloat(-24, 24), Y: w.land.height(w.pos.X) - 3}
				p.emit(origin, golib.Vector2{X: golib.RandomFloat(-100, 100), Y: golib.RandomFloat(-35, -5)}, 0.8, 4, muted)
			}
		}
	}
}
func (p *playScene) burst(success bool) {
	color := orange
	if success {
		color = cyan
	}
	for range 130 {
		v := golib.Vector2{X: golib.RandomFloat(-150, 150), Y: golib.RandomFloat(-130, 60)}
		p.emit(p.world.pos, v, golib.RandomFloat(0.3, 1.8), golib.RandomFloat(1, 5), color)
	}
}
func (p *playScene) drawParticles(s *golib.Screen) {
	s.SetBlendMode(golib.BlendAdd)
	for _, v := range p.particles {
		f := v.life / v.maxLife
		s.DrawCircle(v.p.X, v.p.Y, v.size*2.5, fade(v.c, f*0.06))
		s.DrawCircle(v.p.X, v.p.Y, v.size*f, fade(v.c, f*0.85))
	}
	s.SetBlendMode(golib.BlendNormal)
}
