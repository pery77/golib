package main

import (
	"math"

	"golib"
)

// One place for the marks the build menu wears. A blueprint's icon is
// the very body the world draws (drawBuilding), scaled down into the
// menu's circle, so a change to a building's look is a change to its
// icon too; the groups, which stand for no one building, and the pipes,
// which the region lifts on posts along the ground, carry marks of
// their own drawn here and nowhere else.

// Where a gun points in an icon: up and to the right, the way a barrel
// aims away from the core out in the region. Icon coordinates are screen
// pixels - the menu draws with the camera off.
const (
	iconGunX = 0.7
	iconGunY = -0.7
)

func drawBlueprintIcon(
	screen *golib.Screen,
	kind BuildingKind,
	cx, cy, box float32,
) {
	const rim = 0.8
	box *= rim
	across, height := buildingSize(kind)
	// An isometric body draws across*unitW wide and (height+across)*unitH
	// tall, ground diamond included: fit that into the box.
	w, h := across*unitW, (height+across)*unitH
	k := box / w
	if h*k > box {
		k = box / h
	}
	across, height = across*k, height*k
	// The body rises from its ground point, so drop that point to have
	// the body centered on cy rather than standing on it.
	drawBuilding(screen, kind, cx, cy+height*unitH/2, across, height,
		iconGunX, iconGunY)
}

// drawPipeIcon paints a pipe in miniature: the tube the region lifts on
// posts. The square it stands on is dark, so the posts wear the pipe's
// light metal too and the shadow the region draws is left to the world.
func drawPipeIcon(screen *golib.Screen, cx, cy, box float32) {
	w := box * 0.92 // the tube's length
	tube := box * 0.2
	lift := box * 0.5
	ground := cy + lift*0.45
	for _, px := range []float32{cx - w/2 + tube, cx + w/2 - tube} {
		screen.DrawLine(
			px, ground, px, ground-lift, tube*0.55, pipeColor,
		)
	}
	screen.DrawLine(
		cx-w/2, ground-lift, cx+w/2, ground-lift, tube, pipeDarkColor,
	)
	screen.DrawLine(
		cx-w/2, ground-lift, cx+w/2, ground-lift, tube*0.62, pipeColor,
	)
}

func drawGroupGlyph(
	screen *golib.Screen,
	group buildGroup,
	cx, cy, box float32,
	ink golib.Color,
) {
	// Every mark fits inside the circle of radius box/2 around cx, cy,
	// so none of them spills over the menu's ring.
	t := float32(1.8)
	r := box * 0.5
	switch group {
	case groupIndustry:
		ring := r * 0.62
		screen.DrawCircleOutline(cx, cy, ring, t, ink)
		for i := 0; i < 4; i++ {
			a := float64(i) * math.Pi / 2
			c, s := float32(math.Cos(a)), float32(math.Sin(a))
			screen.DrawLine(cx+c*ring, cy+s*ring, cx+c*r, cy+s*r, t, ink)
		}
		screen.DrawCircle(cx, cy, ring*0.38, ink)
	case groupMilitary:
		w, h := r*0.62, r*0.9
		screen.DrawPolygonOutline([]golib.Vector2{
			{X: cx - w, Y: cy - h},
			{X: cx + w, Y: cy - h},
			{X: cx + w, Y: cy + h*0.3},
			{X: cx, Y: cy + h},
			{X: cx - w, Y: cy + h*0.3},
		}, t, ink)
		screen.DrawLine(cx-w, cy-h*0.15, cx+w, cy-h*0.15, t, ink)
	case groupLogistics:
		w, h := r*0.85, r*0.65
		screen.DrawRectangleOutline(golib.Rectangle{
			X: cx - w, Y: cy - h, Width: 2 * w, Height: 2 * h,
		}, t, ink)
		screen.DrawLine(cx, cy-h, cx, cy+h, t, ink)
	}
}
