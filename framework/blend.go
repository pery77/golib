package golib

import (
	"cmp"
	"image"
	"math"
	"slices"
)

// Blend modes, numbered as Aseprite files number them. The blending follows
// Aseprite 1.3 (src/doc/blend_funcs.cpp), with 8-bit integer math, so layers
// combine as they look in Aseprite.
const (
	blendNormal = iota
	blendMultiply
	blendScreen
	blendOverlay
	blendDarken
	blendLighten
	blendColorDodge
	blendColorBurn
	blendHardLight
	blendSoftLight
	blendDifference
	blendExclusion
	blendHue
	blendSaturation
	blendColor
	blendLuminosity
	blendAddition
	blendSubtract
	blendDivide
)

// rgba is a color with each channel from 0 to 255, not premultiplied, in
// ints for the math.
type rgba struct {
	r, g, b, a int
}

// blendImage blends src onto dst with its top-left corner at x, y, with
// opacity from 0 to 255 and the blend mode mode.
func blendImage(dst, src *image.NRGBA, x, y int, opacity uint8, mode uint16) {
	area := src.Rect.Add(image.Pt(x, y)).Intersect(dst.Rect)
	if area.Empty() || opacity == 0 {
		return
	}
	for py := area.Min.Y; py < area.Max.Y; py++ {
		for px := area.Min.X; px < area.Max.X; px++ {
			s := src.Pix[src.PixOffset(px-x, py-y):]
			if s[3] == 0 {
				continue
			}
			d := dst.Pix[dst.PixOffset(px, py):]
			out := blend(rgba{int(d[0]), int(d[1]), int(d[2]), int(d[3])}, rgba{int(s[0]), int(s[1]), int(s[2]), int(s[3])}, int(opacity), mode)
			d[0], d[1], d[2], d[3] = uint8(out.r), uint8(out.g), uint8(out.b), uint8(out.a)
		}
	}
}

// blend returns src blended over backdrop with the blend mode mode. Like
// Aseprite, a mode other than normal mixes its result with a normal blend as
// much as the backdrop is transparent, so a layer over nothing looks the same
// in every mode.
func blend(backdrop, src rgba, opacity int, mode uint16) rgba {
	if mode == blendNormal || mode > blendDivide || backdrop.a == 0 {
		return blendNormalColor(backdrop, src, opacity)
	}
	if src.a == 0 {
		return backdrop
	}
	normal := blendNormalColor(backdrop, src, opacity)
	blended := blendNormalColor(backdrop, blendModeColor(backdrop, src, mode), opacity)
	normalToBlend := blendMerge(normal, blended, backdrop.a)
	composite := mulUn8(backdrop.a, mulUn8(src.a, opacity))
	return blendMerge(normalToBlend, blended, composite)
}

// blendModeColor returns the color the blend mode makes of backdrop and src,
// with src's alpha.
func blendModeColor(backdrop, src rgba, mode uint16) rgba {
	channel := func(f func(b, s int) int) rgba {
		return rgba{f(backdrop.r, src.r), f(backdrop.g, src.g), f(backdrop.b, src.b), src.a}
	}
	switch mode {
	case blendMultiply:
		return channel(mulUn8)
	case blendScreen:
		return channel(blendScreenChannel)
	case blendOverlay:
		return channel(func(b, s int) int { return blendHardLightChannel(s, b) })
	case blendDarken:
		return channel(func(b, s int) int { return min(b, s) })
	case blendLighten:
		return channel(func(b, s int) int { return max(b, s) })
	case blendColorDodge:
		return channel(blendColorDodgeChannel)
	case blendColorBurn:
		return channel(blendColorBurnChannel)
	case blendHardLight:
		return channel(blendHardLightChannel)
	case blendSoftLight:
		return channel(blendSoftLightChannel)
	case blendDifference:
		return channel(func(b, s int) int { return abs(b - s) })
	case blendExclusion:
		return channel(func(b, s int) int { return b + s - 2*mulUn8(b, s) })
	case blendAddition:
		return channel(func(b, s int) int { return min(b+s, 255) })
	case blendSubtract:
		return channel(func(b, s int) int { return max(b-s, 0) })
	case blendDivide:
		return channel(blendDivideChannel)
	}
	// The hue, saturation, color and luminosity modes work on whole colors.
	br, bg, bb := float64(backdrop.r)/255, float64(backdrop.g)/255, float64(backdrop.b)/255
	sr, sg, sb := float64(src.r)/255, float64(src.g)/255, float64(src.b)/255
	var r, g, b float64
	switch mode {
	case blendHue:
		r, g, b = sr, sg, sb
		setSat(&r, &g, &b, sat(br, bg, bb))
		setLum(&r, &g, &b, lum(br, bg, bb))
	case blendSaturation:
		r, g, b = br, bg, bb
		setSat(&r, &g, &b, sat(sr, sg, sb))
		setLum(&r, &g, &b, lum(br, bg, bb))
	case blendColor:
		r, g, b = sr, sg, sb
		setLum(&r, &g, &b, lum(br, bg, bb))
	default: // luminosity
		r, g, b = br, bg, bb
		setLum(&r, &g, &b, lum(sr, sg, sb))
	}
	return rgba{int(255 * r), int(255 * g), int(255 * b), src.a}
}

// blendNormalColor puts src, made opacity times as opaque, over backdrop.
func blendNormalColor(backdrop, src rgba, opacity int) rgba {
	if backdrop.a == 0 {
		return rgba{src.r, src.g, src.b, mulUn8(src.a, opacity)}
	}
	if src.a == 0 {
		return backdrop
	}
	sa := mulUn8(src.a, opacity)
	ra := sa + backdrop.a - mulUn8(backdrop.a, sa)
	if ra == 0 {
		return rgba{}
	}
	return rgba{
		backdrop.r + (src.r-backdrop.r)*sa/ra,
		backdrop.g + (src.g-backdrop.g)*sa/ra,
		backdrop.b + (src.b-backdrop.b)*sa/ra,
		ra,
	}
}

// blendMerge moves backdrop toward src by amount, from 0 to 255, alpha
// included.
func blendMerge(backdrop, src rgba, amount int) rgba {
	var out rgba
	switch {
	case backdrop.a == 0:
		out.r, out.g, out.b = src.r, src.g, src.b
	case src.a == 0:
		out.r, out.g, out.b = backdrop.r, backdrop.g, backdrop.b
	default:
		out.r = backdrop.r + mulUn8(src.r-backdrop.r, amount)
		out.g = backdrop.g + mulUn8(src.g-backdrop.g, amount)
		out.b = backdrop.b + mulUn8(src.b-backdrop.b, amount)
	}
	out.a = backdrop.a + mulUn8(src.a-backdrop.a, amount)
	if out.a == 0 {
		out.r, out.g, out.b = 0, 0, 0
	}
	return out
}

// mulUn8 returns a times b divided by 255, rounded, as Aseprite's MUL_UN8.
func mulUn8(a, b int) int {
	t := a*b + 0x80
	return ((t >> 8) + t) >> 8
}

// divUn8 returns a times 255 divided by b, rounded, as Aseprite's DIV_UN8.
func divUn8(a, b int) int {
	return (a*0xff + b/2) / b
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func blendScreenChannel(b, s int) int {
	return b + s - mulUn8(b, s)
}

func blendHardLightChannel(b, s int) int {
	if s < 128 {
		return mulUn8(b, s<<1)
	}
	return blendScreenChannel(b, (s<<1)-255)
}

func blendColorDodgeChannel(b, s int) int {
	if b == 0 {
		return 0
	}
	s = 255 - s
	if b >= s {
		return 255
	}
	return divUn8(b, s)
}

func blendColorBurnChannel(b, s int) int {
	if b == 255 {
		return 255
	}
	b = 255 - b
	if b >= s {
		return 0
	}
	return 255 - divUn8(b, s)
}

func blendSoftLightChannel(b8, s8 int) int {
	b, s := float64(b8)/255, float64(s8)/255
	var d, r float64
	if b <= 0.25 {
		d = ((16*b-12)*b + 4) * b
	} else {
		d = math.Sqrt(b)
	}
	if s <= 0.5 {
		r = b - (1-2*s)*b*(1-b)
	} else {
		r = b + (2*s-1)*(d-b)
	}
	return int(r*255 + 0.5)
}

func blendDivideChannel(b, s int) int {
	switch {
	case b == 0:
		return 0
	case b >= s:
		return 255
	default:
		return divUn8(b, s)
	}
}

func lum(r, g, b float64) float64 {
	return 0.3*r + 0.59*g + 0.11*b
}

func sat(r, g, b float64) float64 {
	return max(r, g, b) - min(r, g, b)
}

func clipColor(r, g, b *float64) {
	l := lum(*r, *g, *b)
	n := min(*r, *g, *b)
	x := max(*r, *g, *b)
	if n < 0 {
		*r = l + (*r-l)*l/(l-n)
		*g = l + (*g-l)*l/(l-n)
		*b = l + (*b-l)*l/(l-n)
	}
	if x > 1 {
		*r = l + (*r-l)*(1-l)/(x-l)
		*g = l + (*g-l)*(1-l)/(x-l)
		*b = l + (*b-l)*(1-l)/(x-l)
	}
}

func setLum(r, g, b *float64, l float64) {
	d := l - lum(*r, *g, *b)
	*r += d
	*g += d
	*b += d
	clipColor(r, g, b)
}

// setSat gives the color saturation s: the largest channel becomes s, the
// smallest 0, and the middle one keeps its place between them.
func setSat(r, g, b *float64, s float64) {
	channels := []*float64{r, g, b}
	slices.SortStableFunc(channels, func(x, y *float64) int { return cmp.Compare(*x, *y) })
	low, mid, high := channels[0], channels[1], channels[2]
	if *high > *low {
		*mid = (*mid - *low) * s / (*high - *low)
		*high = s
	} else {
		*mid, *high = 0, 0
	}
	*low = 0
}
