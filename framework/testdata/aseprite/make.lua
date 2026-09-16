-- Makes the Aseprite files that aseprite_test.go reads, and a PNG of every
-- frame as Aseprite draws it, for the tests to compare with. Run it from the
-- framework folder with Aseprite 1.3:
--
--   Aseprite -b --script-param dir=testdata/aseprite --script testdata/aseprite/make.lua

local dir = app.params.dir or "."

local rgba = app.pixelColor.rgba
local graya = app.pixelColor.graya

-- Saves the sprite, then each frame as Aseprite draws it, in RGBA:
-- <name>.aseprite and <name>-<frame>.png, with frames counted from 0.
local function save(sprite, name)
  sprite:saveAs(app.fs.joinPath(dir, name .. ".aseprite"))
  local copy = Sprite(sprite)
  if copy.colorMode ~= ColorMode.RGB then
    app.command.ChangePixelFormat{ format = "rgb" }
  end
  for i, frame in ipairs(copy.frames) do
    local picture = Image(copy.width, copy.height, ColorMode.RGB)
    picture:drawSprite(copy, frame)
    picture:saveAs(app.fs.joinPath(dir, name .. "-" .. (i - 1) .. ".png"))
  end
  copy:close()
  sprite:close()
end

-- An image whose pixels come from f(x, y).
local function paint(width, height, mode, f)
  local image = Image(width, height, mode or ColorMode.RGB)
  for y = 0, height - 1 do
    for x = 0, width - 1 do
      image:drawPixel(x, y, f(x, y))
    end
  end
  return image
end

-- blend: one frame for each blend mode. Every frame has the same backdrop,
-- opaque at the top, half see-through below and empty at the bottom, and a
-- layer in that frame's mode over it, with pixels of several opacities.
do
  local modes = {
    BlendMode.NORMAL, BlendMode.MULTIPLY, BlendMode.SCREEN, BlendMode.OVERLAY,
    BlendMode.DARKEN, BlendMode.LIGHTEN, BlendMode.COLOR_DODGE, BlendMode.COLOR_BURN,
    BlendMode.HARD_LIGHT, BlendMode.SOFT_LIGHT, BlendMode.DIFFERENCE, BlendMode.EXCLUSION,
    BlendMode.HSL_HUE, BlendMode.HSL_SATURATION, BlendMode.HSL_COLOR, BlendMode.HSL_LUMINOSITY,
    BlendMode.ADDITION, BlendMode.SUBTRACT, BlendMode.DIVIDE,
  }
  local sprite = Sprite(20, 12)
  for _ = 2, #modes do
    sprite:newEmptyFrame()
  end
  local backdropLayer = sprite.layers[1]
  backdropLayer.name = "backdrop"
  local backdrop = paint(20, 12, nil, function(x, y)
    local alpha = 255
    if y >= 6 then alpha = 128 end
    if y >= 10 then alpha = 0 end
    return rgba(x * 13, 20 + y * 19, 255 - x * 11, alpha)
  end)
  for i = 1, #modes do
    sprite:newCel(backdropLayer, i, backdrop, Point(0, 0))
  end
  for i, mode in ipairs(modes) do
    local layer = sprite:newLayer()
    layer.name = "mode " .. (i - 1)
    layer.blendMode = mode
    if i % 2 == 0 then layer.opacity = 200 end
    local source = paint(20, 12, nil, function(x, y)
      local alpha = 255
      if x >= 10 then alpha = 180 end
      if x >= 15 then alpha = 60 end
      return rgba(255 - y * 21, (x * 29 + y * 7) % 256, (x * y * 17 + 40) % 256, alpha)
    end)
    local cel = sprite:newCel(layer, i, source, Point(0, 0))
    if i % 3 == 0 then cel.opacity = 230 end
  end
  save(sprite, "blend")
end

-- layers: a background layer, hidden layers and groups, linked cels, cels
-- partly outside the canvas, a cel moved up with its z-index, frame durations
-- and tags in every direction.
do
  local sprite = Sprite(16, 16)
  for _ = 2, 6 do
    sprite:newEmptyFrame()
  end
  local durations = { 0.1, 0.25, 0.05, 0.1, 0.3, 0.1 }
  for i, frame in ipairs(sprite.frames) do
    frame.duration = durations[i]
  end

  local background = sprite.layers[1]
  background.name = "sky"
  for i = 1, 6 do
    sprite:newCel(background, i, paint(16, 16, nil, function(x, y)
      return rgba(40 + i * 20, 60 + y * 8, 120 + x * 6, 255)
    end), Point(0, 0))
  end
  app.activeLayer = background
  app.command.BackgroundFromLayer()

  local hidden = sprite:newLayer()
  hidden.name = "hidden"
  hidden.isVisible = false
  sprite:newCel(hidden, 1, paint(16, 16, nil, function() return rgba(255, 0, 0, 255) end), Point(0, 0))

  local walker = sprite:newLayer()
  walker.name = "walker"
  local block = paint(6, 6, nil, function(x, y) return rgba(250, 220, 40 + x * 30, 200 + y * 10) end)
  for i = 1, 6 do
    sprite:newCel(walker, i, block, Point(i * 3 - 5, 5))
  end

  local group = sprite:newGroup()
  group.name = "group"
  group.opacity = 160
  group.blendMode = BlendMode.MULTIPLY
  local inside = sprite:newLayer()
  inside.name = "inside"
  inside.parent = group
  inside.blendMode = BlendMode.SCREEN
  local stripe = paint(16, 4, nil, function(x) return rgba(x * 16, 90, 200, 220) end)
  for i = 1, 6 do
    sprite:newCel(inside, i, stripe, Point(0, 10))
  end

  local hiddenGroup = sprite:newGroup()
  hiddenGroup.name = "hidden group"
  hiddenGroup.isVisible = false
  local lost = sprite:newLayer()
  lost.name = "lost"
  lost.parent = hiddenGroup
  sprite:newCel(lost, 2, paint(16, 16, nil, function() return rgba(0, 255, 0, 255) end), Point(0, 0))

  -- linked: frame 1 has a cel, and frames 2 and 3 link to it.
  local linked = sprite:newLayer()
  linked.name = "linked"
  sprite:newCel(linked, 1, paint(4, 4, nil, function(x, y) return rgba(10, 10 + x * 60, 10 + y * 60, 255) end), Point(1, 1))
  app.activeLayer = linked
  app.range.frames = { sprite.frames[1], sprite.frames[2], sprite.frames[3] }
  app.range.layers = { linked }
  app.command.LinkCels()

  -- top: a cel in frame 4 that its z-index puts under the walker.
  local under = sprite:newLayer()
  under.name = "under"
  local cel = sprite:newCel(under, 4, paint(8, 8, nil, function() return rgba(120, 40, 160, 255) end), Point(0, 3))
  cel.zIndex = -3

  local function tag(from, to, name, direction, repeats)
    local t = sprite:newTag(from, to)
    t.name = name
    t.aniDir = direction
    t.repeats = repeats
  end
  tag(1, 3, "walk", AniDir.FORWARD, 0)
  tag(2, 4, "back", AniDir.REVERSE, 2)
  tag(1, 4, "bounce", AniDir.PING_PONG, 0)
  tag(3, 6, "bounce back", AniDir.PING_PONG_REVERSE, 3)
  tag(5, 5, "still", AniDir.PING_PONG, 0)
  tag(5, 6, "walk", AniDir.FORWARD, 1)
  save(sprite, "layers")
end

-- indexed: palette colors, one of them see-through, and the transparent
-- index, which a background layer shows as its color.
do
  local sprite = Sprite(8, 8, ColorMode.INDEXED)
  local palette = Palette(6)
  palette:setColor(0, Color{ r = 200, g = 30, b = 30, a = 255 })
  palette:setColor(1, Color{ r = 30, g = 200, b = 30, a = 255 })
  palette:setColor(2, Color{ r = 30, g = 30, b = 200, a = 255 })
  palette:setColor(3, Color{ r = 250, g = 250, b = 250, a = 120 })
  palette:setColor(4, Color{ r = 0, g = 0, b = 0, a = 255 })
  palette:setColor(5, Color{ r = 255, g = 200, b = 0, a = 255 })
  sprite:setPalette(palette)
  sprite.transparentColor = 0

  local background = sprite.layers[1]
  background.name = "floor"
  sprite:newCel(background, 1, paint(8, 8, ColorMode.INDEXED, function(x, y) return (x + y) % 3 end), Point(0, 0))
  app.activeLayer = background
  app.command.BackgroundFromLayer()

  local top = sprite:newLayer()
  top.name = "top"
  sprite:newCel(top, 1, paint(8, 8, ColorMode.INDEXED, function(x, y) return (x * y) % 6 end), Point(0, 0))
  save(sprite, "indexed")
end

-- gray: grayscale with see-through pixels and a second layer in multiply.
do
  local sprite = Sprite(8, 8, ColorMode.GRAY)
  local base = sprite.layers[1]
  base.name = "base"
  sprite:newCel(base, 1, paint(8, 8, ColorMode.GRAY, function(x, y) return graya(x * 36, 255 - y * 20) end), Point(0, 0))
  local shade = sprite:newLayer()
  shade.name = "shade"
  shade.blendMode = BlendMode.MULTIPLY
  sprite:newCel(shade, 1, paint(8, 8, ColorMode.GRAY, function(x, y) return graya(255 - y * 30, 200) end), Point(0, 0))
  save(sprite, "gray")
end
