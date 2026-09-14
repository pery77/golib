# Roadmap

Milestones land in order; what goes into each one can still move as we learn. The "Project status" table in [AGENTS.md](../AGENTS.md) mirrors this file, so update both together.

## M0: Foundation (done, 2026-09-14)

- Cross-platform `golib` CLI entry points with `setup`, `doctor`, `clean` and `help`.
- VS Code workspace: recommended extensions, editor settings, tasks.
- AI instructions: `AGENTS.md`, `CLAUDE.md`, Claude Code settings and the `/make-game` skill.
- Docs: vision, roadmap, architecture, tooling, contributing, game-making playbook.

## M1: Toolchain and first window (in progress)

- Done: `golib setup` downloads Go 1.27.1 into `.tools/`, verifying its checksum.
- Done: raylib 6.0 through [raylib-go](https://github.com/gen2brain/raylib-go) in its purego mode, so no platform needs a C compiler. The prebuilt raylib library ships inside the binding's module; `golib` extracts it into `.tools/` and copies it next to each build (see [tooling.md](tooling.md)).
- Done: the framework and game split (see [architecture.md](architecture.md)). `framework/` is the Go module `golib`, and `games/hello` opens a window and draws text through it. `hello` keeps growing as the framework's test game.
- Done: `golib run`, `golib build` and `golib test`, with the game picked by its folder name, plus `golib go` to run the project's Go toolchain.
- Done, not yet checked in the editor: VS Code's Go extension pointed at the local toolchain.
- To do: a VS Code debug configuration.
- To do: run `golib.sh` on real Linux and macOS machines. So far it has only been exercised from Windows: help, usage errors and doctor in Git Bash, and the Go download, checksum and extraction on a simulated Linux.

## M2: Framework basics

- Game loop with frame-rate independent updates.
- Scenes (title, play, pause, game over) and switching between them.
- Input: keyboard, mouse, gamepad.
- 2D drawing: shapes, textures, sprites, text.
- Sprites and animations loaded from Aseprite files.
- 2D maps loaded from Tiled: tile layers, tilesets and object layers.
- Audio: sound effects and music.
- Loading assets from the game's `assets/` folder.
- `golib new <name>`: create a game folder in `games/`, ready to run, without touching the framework.
- An example game that doubles as documentation (the test game is the natural candidate), and an API guide for agents.

## M3: Agent verification loop

- `golib shot`: run the game for a number of frames, optionally with scripted input, and save screenshots an agent can inspect.
- Deterministic runs (fixed seed and timestep) so results are reproducible.

## M4: Shipping

- `golib dist`: package a game for Windows, Linux and macOS.
- Investigate a web build.

## Open questions

- **Framework distribution.** Today the template ships the framework's source, and each game points at it with a `replace` directive in its `go.mod`. Should games later depend on a published, versioned framework module instead? That needs the final repository URL, which would also become the framework's module path (today just `golib`).
- **Finding raylib in shipped games.** Programs load the raylib library by name when they start. On Windows the copy next to the `.exe` is enough; on Linux and macOS, `golib run` and `golib test` set the library search path. A packaged game (M4) needs its own way to find the library there.
- **2D and 3D.** 2D only at first, or both from the start? Either way, 3D models come from Blender as glTF.
- **Content formats.** Tiled saves maps as TMX (XML) or JSON: support one or both? Aseprite: load `.aseprite` files only (no export step), or also PNG sprite sheets with Aseprite's JSON, which other pixel-art tools can produce? Where do source files the game doesn't load, like `.blend`, live inside the game folder?
- **CLI in Go.** Now that Go is local, move command logic from the twin shell scripts into one Go program, keeping the scripts as tiny bootstrappers (see [tooling.md](tooling.md)).
- **License.**
