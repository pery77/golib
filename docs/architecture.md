# Architecture

How GoLib keeps the framework apart from the games made with it, and where game content comes from.

> **Status:** the framework and game split exists since M1, with `framework/` and the example game, `games/platformer`. Loading content from Tiled, Aseprite and Blender is planned for M4, which is postponed (see [roadmap.md](roadmap.md)).

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
games/
  <name>/           One game; the folder name is the game's short name
    go.mod          Go module <name>, which uses the framework through a replace directive
    main.go         Entry point (package main)
    DESIGN.md       Design brief: the game's memory across sessions
    assets/         Content: maps, sprites, models, sounds, fonts; read with golib.ReadAsset
    assets.go       Embeds assets/ in golib dist builds; needed only when assets/ exists
    shaders/        GLSL post-processing shaders, embedded in the game's code with //go:embed (optional)
```

The framework folder can't be called `golib/`: that name is taken by the CLI entry point in the project root.

### Go modules

The framework and every game are separate Go modules, so Go itself guards the boundary: a game can only use what the framework exports, and the framework can't import a game.

A game imports the framework as `"golib"`, and its `go.mod` points that name at the framework folder:

```text
module platformer

go 1.27.1

require golib v0.0.0

replace golib => ../../framework
```

`golib go -C games/<name> mod tidy` then adds the framework's own requirements (raylib-go and its dependencies) as indirect ones. Whether games should later depend on a published framework version instead is an open question in [roadmap.md](roadmap.md).

### Developing the framework with a test game

The framework is built together with a real game that tries out each feature as it lands; today that is `games/platformer`. It follows the same rules as any other game: it is the framework's first user, not part of it. When it needs something the framework lacks:

1. Add the general capability to the framework, with its docs and tests, as its own change.
2. Use it from the game in a separate change.

If the game can't be written without reaching into framework internals, the framework's API is missing something: fix the API, don't bypass it.

## Content: established tools, no editors of our own

GoLib has no visual editor, scene designer, level editor or asset GUI, and won't get one. Content is made in mature tools that many people already know, and GoLib loads the files they save.

| Content | Made with | What the game loads | Planned |
| --- | --- | --- | --- |
| 2D maps and levels | [Tiled](https://www.mapeditor.org) | Tiled maps and tilesets | M4 |
| Sprites and animations | [Aseprite](https://www.aseprite.org) | Aseprite sprites, with their animation tags | M4 |
| 3D models | [Blender](https://www.blender.org) | glTF (`.glb`) exported from Blender | With 3D support |
| Music | Any music tool, or a tracker such as [MilkyTracker](https://milkytracker.org) | `.ogg`, `.mp3`, `.wav`, `.qoa`, and the tracker modules `.xm` and `.mod` | Done (M2): `golib.NewMusic` |
| Sound effects | Any audio tool | `.wav`, `.ogg` | M4. Until then, GoLib makes sound effects in code: `golib.NewSound` |
| Fonts | Existing fonts whose license allows it | `.ttf` | M4 |

Why:

- **Proven and documented.** These tools are mature and their file formats are documented. GoLib doesn't have to build, maintain or teach an editor.
- **Good for agents too.** Tiled maps are plain XML or JSON: an agent can write a level as easily as code, and a human can open it in Tiled to adjust it.
- **Clear split.** Tools decide what content looks like; game code decides what it means.

### Rules

1. **The tools are for editing, not for building.** A game builds and runs on a machine that has none of them, so the zero-install promise holds. Builds never call these tools, which means every file a game loads is committed in its folder.
2. **Load what the tool saves.** Prefer the tool's own file format, so saving in the tool and running the game is all it takes, with no export step to forget. Export only when the tool's format can't be loaded at runtime (Blender's `.blend`), and then commit the source file too.
3. **No editors inside games either.** Don't build an in-game level editor, sprite editor or any other tool for making content. If a game has levels, they are Tiled maps.
4. **No dependencies for formats.** Tiled and Aseprite files are parsed with the Go standard library (`encoding/xml`, `encoding/json`, `compress/zlib`). 3D models and audio go through raylib.
5. **Everything in `assets/` ships.** `golib dist` embeds the whole folder in the executable, so keep only files the game loads there.

Exact formats (TMX or JSON for Tiled; `.aseprite` files only, or exported sprite sheets as well) are open questions in [roadmap.md](roadmap.md).
