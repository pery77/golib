# Architecture

How GoLib keeps the framework apart from the games made with it, and where game content comes from.

> **Status:** the framework and game split exists since M1, with `framework/` and the example game, `games/platformer`. Tiled maps, Aseprite files, PNG images, fonts, sound files and jfxr sounds load since M4, and 3D models from Blender come in M7, after the 2D essentials of M6 (see [roadmap.md](roadmap.md)).

## Framework and games

GoLib has two sides, and they don't mix.

| | Framework | Game |
| --- | --- | --- |
| What it is | A Go library on top of raylib | An ordinary Go program that uses the framework |
| Lives in | `framework/` | `games/<name>/`, one folder per game |
| Who changes it | GoLib contributors, following [contributing.md](contributing.md) | Whoever makes the game, following [ai/making-a-game.md](ai/making-a-game.md) |
| Depends on | raylib | The framework, and raylib directly for anything the framework doesn't cover |

### Rules

1. **Dependencies point one way:** game, then framework, then raylib. The framework never imports a game, never refers to a game's files and never contains code written for one particular game.
2. **Making a game never requires editing the framework.** A game uses the framework's exported API. When that isn't enough, the game works around it in its own code and the gap is reported, so it can become a framework change.
3. **Framework features are generic.** A feature belongs in the framework only if it makes sense for games other than the one that asked for it. Whatever is specific to that game stays in the game.
4. **A game is self-contained.** Its code, its `DESIGN.md` and its content all live in its folder. Deleting the folder removes the game and nothing else.

### Layout

```text
framework/          The framework: Go module "golib"
  go.mod
  *.go              package golib
  README.md         The API guide: every exported name by task; apiguide_test.go keeps it in step
  internal/device/  The line to the machine: the contract, and one backend per platform
  internal/startup/ Checks, before raylib starts, that its libraries load (Windows); games can't import it
games/
  <name>/           One game; the folder name is the game's short name
    go.mod          Go module <name>, which uses the framework through a replace directive
    main.go         Entry point (package main)
    DESIGN.md       Design brief: the game's memory across sessions
    game.json       Title, version and author: the Windows executable's details, and the dist zip's version (optional)
    icon.png        The game's icon, a square PNG, for the executables golib builds on Windows (optional)
    assets/         Content: maps, sprites, models, sounds, fonts; read with golib.ReadAsset
    assets.go       Embeds assets/ in golib dist builds; needed only when assets/ exists
    sources/        Files the game doesn't load, such as .blend or .psd; committed, not shipped (optional)
    shaders/        GLSL post-processing shaders, embedded in the game's code with //go:embed (optional)
```

The framework folder can't be called `golib/`: that name is taken by the CLI entry point in the project root.

### Framework and machine

Package `golib` is plain Go: the game loop, the Aseprite and Tiled readers, the sound synthesizers, the camera, the vectors and the timers. Everything that touches the machine — drawing, sound, input, the window — goes through `framework/internal/device`, which has a contract and one backend per platform, chosen by a build tag:

```text
internal/device/
  device.go       The contract, and what a backend has to provide
  raylib*.go      //go:build !js   raylib on Windows, Linux and macOS
  web*.go         //go:build js    WebGL 2 in a browser, with web.js beside it
  web.js          The other half of the web backend, which golib web copies into the page
```

The rule: **nothing in `framework/*.go` imports raylib**, and `device_test.go` fails when something does. When the framework needs something the machine can do and the contract lacks, add it to the contract and to every backend, rather than reaching for raylib above the line.

The contract passes values, not raylib types: `device.Color`, `device.Rectangle` and the handles for a texture, a target, a shader, a font, a sound and a music. On the raylib backend those are raylib's own types, so nothing is converted and the desktop build is exactly what it was; another backend defines structs with the same fields. Only their `ID`, `Width` and `Height` fields are portable, and nothing above `device` reads anything else.

A game never sees this: it uses package `golib`, which is the same on every platform. Game code that imports raylib directly, which the framework allows for anything GoLib doesn't cover, only builds where raylib does.

A backend that can't do something does the harmless thing rather than stopping the game, and says so where the player can see it: a web build has no sound device yet, so it plays in silence the way `golib shot` does, and it has no post-processing shaders, so a game that asks for one stops with a message on the page instead of drawing the wrong picture.

### Go modules

The framework and every game are separate Go modules, so Go itself guards the boundary: a game can only use what the framework exports, and the framework can't import a game.

A game imports the framework as `"golib"`, and its `go.mod` points that name at the framework folder:

```text
module platformer

go 1.27.1

require golib v0.0.0

replace golib => ../../framework
```

`golib go -C games/<name> mod tidy` then adds the framework's own requirements (raylib-go and its dependencies) as indirect ones.

### Distribution

GoLib ships as the whole template: the framework's source, the CLI, the docs that describe them and the example games, versioned together. The framework isn't published as a Go module of its own, so `golib` is never downloaded and its module path needs no domain; the reasons are in [roadmap.md](roadmap.md#decisions). No versions have been released yet.

Because a game depends only on `../../framework`, it moves to a newer GoLib as a folder:

1. Run `golib setup` in the newer template.
2. Copy `games/<name>/` into its `games/` folder.
3. Run `golib go -C games/<name> mod tidy`, which brings the game's indirect requirements up to the new framework's.
4. Run `golib test` and `golib run <name>`, and fix whatever the new framework changed.

### Developing the framework with a test game

The framework is built together with a real game that tries out each feature as it lands; today that is `games/platformer`. It follows the same rules as any other game: it is the framework's first user, not part of it. When it needs something the framework lacks:

1. Add the general capability to the framework, with its docs and tests, as its own change.
2. Use it from the game in a separate change.

If the game can't be written without reaching into framework internals, the framework's API is missing something: fix the API, don't bypass it.

## Content: established tools, no editors of our own

GoLib has no visual editor, scene designer, level editor or asset GUI, and won't get one. Content is made in mature tools that many people already know, and GoLib loads the files they save.

| Content | Made with | What the game loads | Planned |
| --- | --- | --- | --- |
| 2D maps and levels | [Tiled](https://www.mapeditor.org) | TMX maps, TSX tilesets and TX templates, Tiled's default formats; orthogonal maps only | Done (M4): `golib.NewMap` |
| Sprites and animations | [Aseprite](https://www.aseprite.org) | `.aseprite` and `.ase` files, with their layers combined as Aseprite shows them and their tags as animations | Done (M4): `golib.NewSprite` |
| Images and sprite sheets | Any image tool | PNG; a sprite sheet is cut into a grid of frames of one size | Done (M4): `golib.NewSprite`, `golib.NewSpriteSheet` |
| 3D models | [Blender](https://www.blender.org) | glTF (`.glb`) exported from Blender | M7, with 3D support |
| Music | Any music tool, or a tracker such as [MilkyTracker](https://milkytracker.org) | `.ogg`, `.mp3`, `.wav`, `.qoa`, and the tracker modules `.xm` and `.mod` | Done (M2): `golib.NewMusic` |
| Sound effects | [jfxr](https://jfxr.frozenfractal.com), a sound effect maker in the browser | `.jfxr` files, the settings jfxr saves, from which GoLib makes the sound as jfxr does | Done (M4): `golib.NewSoundFile` |
| Recorded sound effects | Any audio tool | `.wav`, `.ogg`, `.mp3`, `.qoa` | Done (M4): `golib.NewSoundFile`. GoLib also makes sound effects in code: `golib.NewSound` |
| Fonts | Existing fonts whose license allows it | `.ttf`, `.otf` | Done (M4): `golib.NewFont` |

Why:

- **Proven and documented.** These tools are mature and their file formats are documented. GoLib doesn't have to build, maintain or teach an editor.
- **Good for agents too.** Tiled maps are plain XML, and jfxr sounds plain JSON: an agent can write a level or a sound as easily as code, and a human can open it in Tiled or jfxr to adjust it.
- **Clear split.** Tools decide what content looks like; game code decides what it means.

### Rules

1. **The tools are for editing, not for building.** A game builds and runs on a machine that has none of them, so the zero-install promise holds. Builds never call these tools, which means every file a game loads is committed in its folder.
2. **Load what the tool saves.** Prefer the tool's own file format, so saving in the tool and running the game is all it takes, with no export step to forget. Export only when the tool's format can't be loaded at runtime (Blender's `.blend`), and then commit the source file too, in `sources/`. One format for each kind of content: GoLib doesn't read Tiled's JSON maps or Aseprite's JSON sprite sheets.
3. **No editors inside games either.** Don't build an in-game level editor, sprite editor or any other tool for making content. If a game has levels, they are Tiled maps.
4. **No dependencies for formats.** Tiled and Aseprite files are parsed with the Go standard library (`encoding/xml`, `encoding/base64`, `compress/zlib`, `compress/gzip`). The standard library has no Zstandard, so a Tiled map compressed with it will fail with a message that says to pick another compression in Tiled. jfxr files are read with `encoding/json`, and GoLib makes their sound itself, with a Go version of jfxr's synthesizer (`framework/jfxr.go`, under jfxr's BSD license, which `golib dist` adds to every game's notices). 3D models and recorded audio go through raylib.
5. **Everything in `assets/` ships.** `golib dist` embeds the whole folder in the executable, so keep only files the game loads there. Source files the game doesn't load go in `sources/`, with the same paths: `sources/models/ship.blend` for `assets/models/ship.glb`. They are committed, but not shipped.

The formats were chosen on 2026-09-16, and jfxr for sound effects on 2026-09-17; the reasons are in [roadmap.md](roadmap.md#decisions).
