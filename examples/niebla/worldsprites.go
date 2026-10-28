package main

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"sync"

	"golib"
)

const (
	modelFrameWidth  = 128
	modelFrameHeight = 192
	modelFootX       = 64
	modelFootY       = 160
	modelRenderZoom  = 32
)

type worldSprite struct {
	sprite        *golib.Sprite
	shadow        *golib.Sprite
	name          string
	geometry      *worldSpriteGeometry
	visibleAcross float32
	minPixels     float32
}

type worldSpriteGeometry struct {
	once   sync.Once
	bounds [8]image.Rectangle
	valid  [8]bool
}

func newWorldSprite(
	name, shadowName string, across, minimum float32,
) worldSprite {
	return worldSprite{
		sprite: golib.NewSpriteSheet(
			name, modelFrameWidth, modelFrameHeight,
		),
		shadow: golib.NewSpriteSheet(
			shadowName, modelFrameWidth, modelFrameHeight,
		),
		name:          name,
		geometry:      &worldSpriteGeometry{},
		visibleAcross: across, minPixels: minimum,
	}
}

var (
	coreWorkerModel = newWorldSprite(
		"sprites/worker-core.png",
		"sprites/shadows/worker-core.png", 4, 6,
	)
	carrierModel = newWorldSprite(
		"sprites/worker-carrier.png",
		"sprites/shadows/worker-carrier.png", 5, 6,
	)
	trooperModel = newWorldSprite(
		"sprites/worker-trooper.png",
		"sprites/shadows/worker-trooper.png", 5, 6,
	)
	mechanicModel = newWorldSprite(
		"sprites/worker-mechanic.png",
		"sprites/shadows/worker-mechanic.png", 5, 6,
	)
	rivalScoutModel = newWorldSprite(
		"sprites/rival-scout.png",
		"sprites/shadows/rival-scout.png", 3, 5,
	)
	rivalCrawlerModel = newWorldSprite(
		"sprites/rival-crawler.png",
		"sprites/shadows/rival-crawler.png", 8, 9,
	)
	rivalCityCrawlerModel = newWorldSprite(
		"sprites/rival-city-crawler.png",
		"sprites/shadows/rival-city-crawler.png", 8, 9,
	)
	rivalRaiderModel = newWorldSprite(
		"sprites/rival-raider.png",
		"sprites/shadows/rival-raider.png", 4, 5,
	)
	rivalArtilleryModel = newWorldSprite(
		"sprites/rival-artillery.png",
		"sprites/shadows/rival-artillery.png", 12, 8,
	)
)

func robotModel(kind RobotKind) worldSprite {
	switch kind {
	case RobotBuilder:
		return coreWorkerModel
	case RobotWorker:
		return carrierModel
	case RobotCombat:
		return trooperModel
	case RobotRepair:
		return mechanicModel
	default:
		return coreWorkerModel
	}
}

func modelScreenPosition(camera *golib.Camera, position golib.Vector2) golib.Vector2 {
	return camera.ToScreen(position)
}

func (m worldSprite) iconScale(zoom float32) float32 {
	return max(1, m.minPixels/(m.visibleAcross*unitW*zoom))
}

func (m worldSprite) opaqueBounds(facing uint8) (image.Rectangle, bool) {
	if m.geometry == nil {
		return image.Rectangle{}, false
	}
	m.geometry.once.Do(func() {
		data, err := golib.ReadAsset(m.name)
		if err != nil {
			return
		}
		picture, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return
		}
		columns := picture.Bounds().Dx() / modelFrameWidth
		if columns == 0 {
			return
		}
		rows := picture.Bounds().Dy() / modelFrameHeight
		if columns*rows < len(m.geometry.bounds) {
			return
		}
		for frame := range m.geometry.bounds {
			column, row := frame%columns, frame/columns
			left := picture.Bounds().Min.X + column*modelFrameWidth
			top := picture.Bounds().Min.Y + row*modelFrameHeight
			frameRect := image.Rect(
				left, top, left+modelFrameWidth, top+modelFrameHeight,
			)
			bounds := image.Rectangle{}
			for y := frameRect.Min.Y; y < frameRect.Max.Y; y++ {
				for x := frameRect.Min.X; x < frameRect.Max.X; x++ {
					_, _, _, alpha := picture.At(x, y).RGBA()
					if alpha == 0 {
						continue
					}
					pixel := image.Rect(x, y, x+1, y+1)
					if bounds.Empty() {
						bounds = pixel
					} else {
						bounds = bounds.Union(pixel)
					}
				}
			}
			if bounds.Empty() {
				continue
			}
			bounds = bounds.Sub(frameRect.Min)
			m.geometry.bounds[frame] = bounds
			m.geometry.valid[frame] = true
		}
	})
	frame := int(facing % uint8(len(m.geometry.bounds)))
	return m.geometry.bounds[frame], m.geometry.valid[frame]
}

func (m worldSprite) screenBounds(
	camera *golib.Camera,
	position golib.Vector2,
	facing uint8,
	zoom float32,
) (golib.Rectangle, bool) {
	bounds, ok := m.opaqueBounds(facing)
	if !ok {
		return golib.Rectangle{}, false
	}
	at := camera.ToScreen(position)
	at.X = float32(math.Floor(float64(at.X) + 0.5))
	at.Y = float32(math.Floor(float64(at.Y) + 0.5))
	scale := m.iconScale(zoom) * zoom / modelRenderZoom
	return golib.Rectangle{
		X:      at.X + (float32(bounds.Min.X)-modelFootX)*scale,
		Y:      at.Y + (float32(bounds.Min.Y)-modelFootY)*scale,
		Width:  float32(bounds.Dx()) * scale,
		Height: float32(bounds.Dy()) * scale,
	}, true
}

func (m worldSprite) draw(
	screen *golib.Screen, camera *golib.Camera,
	position golib.Vector2, facing uint8, zoom float32,
) {
	at := modelScreenPosition(camera, position)
	scale := m.iconScale(zoom)
	// DrawSprite rounds before applying a camera: draw at screen pixels so
	// one world-pixel step does not become a zoom-sized jump.
	screen.SetCamera(nil)
	screen.DrawSprite(m.sprite, int(facing%8), at.X, at.Y,
		golib.DrawOptions{
			OriginX: modelFootX,
			OriginY: modelFootY,
			Scale:   scale * zoom / modelRenderZoom,
		})
	screen.SetCamera(camera)
}

func (m worldSprite) drawShadow(
	screen *golib.Screen, camera *golib.Camera,
	position golib.Vector2, facing uint8, zoom float32,
) {
	at := modelScreenPosition(camera, position)
	scale := m.iconScale(zoom) * zoom / modelRenderZoom
	screen.SetCamera(nil)
	screen.DrawSprite(m.shadow, int(facing%8), at.X, at.Y,
		golib.DrawOptions{
			OriginX: modelFootX,
			OriginY: modelFootY,
			Scale:   scale,
			Tint:    unitShadowTint,
		})
	screen.SetCamera(camera)
}

func (m worldSprite) drawIcon(
	screen *golib.Screen, position golib.Vector2,
	facing uint8, tint golib.Color,
) {
	screen.DrawSprite(m.sprite, int(facing%8), position.X, position.Y,
		golib.DrawOptions{
			OriginX: modelFootX,
			OriginY: 147,
			Scale:   0.5,
			Tint:    tint,
		})
}
