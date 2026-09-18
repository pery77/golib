# AGENTS.md

Instructions for AI coding agents working in this repository (Claude Code, OpenAI Codex, GitHub Copilot, Cursor and others). Humans should start with [README.md](README.md).

## What GoLib is

GoLib is a small game framework for **Go**, built on **raylib** (windowing, input, graphics, audio), distributed as a ready-to-use **VS Code template**.

The promise: someone downloads the template, runs a couple of commands, and builds a complete game with an AI agent from one prompt and a few iterations. That makes you a primary user of this project. Docs, CLI output and (soon) the API are written so you can work without guessing, and you are expected to keep them that way.

## Project status

Keep this section true: update it in the same change that lands or removes a feature. Never describe planned work as if it existed.

Last updated: 2026-09-18 (milestones M5, Shipping, and M6, 2D essentials, done; Linux and macOS ran GoLib for the first time; the web build started with its first stage, the seam between the framework and the machine; M7, 3D, comes after it).

| Area | State |
| --- | --- |
| `golib` CLI: `setup`, `doctor`, `clean`, `help` | Done |
| The CLI in Go, `tools/cli`: one program for every platform with the commands' logic, started by the twin scripts | Done (M5): `new`, `build`, `run`, `shot`, `test`, `dist` and most of `setup`; `help`, `doctor`, `go` and `clean` stay in the scripts. Tested on Windows, and it set up, built and ran games on Linux and macOS on 2026-09-18 |
| VS Code workspace: extensions, settings, tasks | Done |
| AI instructions: this file, `CLAUDE.md`, Claude Code settings, `/make-game` skill | Done |
| Go 1.27.1 downloaded into `.tools/` by `golib setup` | Done |
| raylib 6.0 through raylib-go, with no C compiler | Done |
| `framework/` (package `golib`) and the example game, `games/platformer` | Done |
| `framework/internal/device`: the line between package `golib` and the machine, with the raylib backend behind a build tag | Done (2026-09-18, stage 0 of the web build). Package `golib` imports no raylib: add to the contract instead, and `device_test.go` fails when a file of package `golib` reaches for raylib. See [docs/architecture.md](docs/architecture.md#framework-and-machine) |
| `golib run`, `golib build`, `golib test`, `golib go` | Done |
| VS Code: Go extension on the local toolchain | Done |
| VS Code debug configuration: "GoLib: debug game" (F5) | Done |
| GoLib window (`golib-ui.cmd`): a button for each `golib` command, with its output | Done (Windows only) |
| Linux and macOS (`golib.sh`) | Works: on 2026-09-18 two other people set up GoLib on their own Linux and macOS machines, built and played a game, and ran an unzipped `golib dist` build. Windows still comes first, and `doctor`, `test`, `shot`, the icon and the version information there are unreported (see "Other platforms" in [docs/roadmap.md](docs/roadmap.md)) |
| Fixed-step game loop: 60 updates per second at any frame rate | Done (M2) |
| `golib shot`: screenshots of chosen frames, rendered in a hidden window, with scripted keyboard, mouse and gamepad input (`--input`) and random numbers from a fixed seed | Done (brought forward from M3); `--save` starts the game from saved data and `--scale` enlarges the pictures (M6) |
| `golib dist`: the game to share, as a folder with the executable (assets inside), the raylib libraries and `THIRD-PARTY-LICENSES.txt`, and a zip of it | Done (M5; tried on Windows, and on Linux and macOS on 2026-09-18; on macOS the executable still carries the libraries) |
| Windows icon and version information, from the game's `icon.png` and `game.json`, in dist builds and in the debug builds of `build`, `run` and `shot` | Done (M5; not yet on Linux and macOS) |
| A Windows game that can't load raylib or libffi says why, in a message box when nobody sees its console, instead of ending silently | Done (M5) |
| On Windows, `build` and `shot` work while `run` has the game open, and `golib` works while one of its copies is running | Done (M5) |
| Keyboard, mouse (pointer, buttons, wheel) and gamepad (buttons, sticks) input; rectangles, circles, lines and triangles; `Rectangle` overlap and point checks | Done (M2) |
| Screen scaled to any window size, fullscreen (`golib.SetFullscreen`), post-processing shaders (`golib.NewShader`, `golib.SetPostProcess`) | Done (M2) |
| Random numbers: `golib.RandomInt`, `golib.RandomFloat`, `golib.SetRandomSeed` | Done (M2) |
| Sound effects made in code, with no sound files: `golib.NewSound`, `golib.SoundSpec`, the ready-made recipes and `golib.SetVolume` | Done (M2) |
| Music streamed from the game's `assets/` folder: `golib.NewMusic` (OGG, MP3, WAV, QOA, XM, MOD; not IT) | Done (M2) |
| `golib.Quit`; no key quits a game on its own, not even Esc | Done (M2) |
| Scenes: `golib.SwitchScene` moves between title, play, pause and other screens | Done (M2) |
| Reading files from the game's `assets/` folder (`golib.ReadAsset`), embedded in dist builds | Done (M2) |
| `golib new <name>`: a new game, ready to run, from `tools/template/game/` | Done (M2) |
| API guide for agents, `framework/README.md`: every exported name by task, checked against the code by `golib test` | Done (M2) |
| Sprites from PNG images, PNG sprite sheets and Aseprite files, with animations: `golib.NewSprite`, `golib.NewSpriteSheet`, `Screen.DrawSprite`, `golib.Animation` | Done (M4) |
| Tiled maps: `golib.NewMap`, `Screen.DrawMap`, `Screen.DrawMapLayer`, tiles and objects by layer, custom properties | Done (M4) |
| Sound effects from `.wav`, `.ogg`, `.mp3` and `.qoa` files: `golib.NewSoundFile`, and `Sound.SetVolume` | Done (M4) |
| Sound effects designed in jfxr, the sound effect maker: `.jfxr` files through `golib.NewSoundFile`, made by a Go version of jfxr's synthesizer that matches jfxr's samples | Done (M4) |
| Fonts from `.ttf` and `.otf` files, in any language: `golib.NewFont`, `golib.TextOptions` | Done (M4) |
| Vectors and a 2D camera: `Vector2` methods, `golib.NewCamera`, `Screen.SetCamera` | Done (M6) |
| Saving high scores, settings and progress: `golib.SaveData`, `golib.LoadData`, `golib.DeleteData` | Done (M6) |
| Looping sounds and stopping them: `Sound.Loop`, `Sound.Stop` | Done (M6) |
| Outlines, polygons, aligned text, a color's opacity, additive blending; hiding the mouse pointer | Done (M6) |
| `games/skyraid` and `games/crates`: the test games, on the framework's camera, saving, tune and asset listing | Done (M6) |
| Music from notes, with no music file: `golib.NewTune` | Done (M6) |
| `Map.Err` and `golib.ListAssets`: whether a map loaded, and what is in the assets folder | Done (M6) |
| `golib.Timer` for cooldowns and intervals; `golib.Lerp`, `golib.Clamp` and the easings; `Sound.PlayWith` for one play's volume and pitch; `Config.PauseUnfocused` and `golib.WindowFocused` | Done (M6) |
| Games in the browser: a web backend of GoLib's own, on WebGL 2 and Web Audio, `golib web` and `golib dist --web` | Done (2026-09-18, stages 0 to 3): the picture, the input, the sound, post-processing shaders, fonts from files, saved data in the browser's store, and a zip for itch.io. Not there: `.xm`, `.mod` and `.qoa`, which browsers can't decode, text from a font file pixel for pixel as on the desktop, and `golib shot --web`. Nobody has heard or played a web build by hand; the stages are in [docs/roadmap.md](docs/roadmap.md#web-build-started-2026-09-18) |
| 3D: glTF models from Blender, a 3D camera, basic lighting | Planned (M7, after the web build); nothing built |

**The framework is still small.** It opens a window, runs a fixed-step game loop, reads the keyboard, the mouse and gamepads, draws rectangles, circles, lines, triangles and polygons, filled or outlined, text in its built-in font or in fonts from files, aligned as the game wants, sprites from PNG and Aseprite files with their animations, and Tiled maps, whose tiles and objects a game can look up, shows worlds larger than the screen through a camera, does vector math, scales the screen to any window or fullscreen, runs post-processing shaders, makes random numbers, counts time down with timers and softens movement with easings, makes and plays sound effects, once, in a loop or at another volume and pitch, from code, from jfxr's `.jfxr` files or from sound files, streams music from a file or from notes, switches between scenes, reads files from the game's `assets/` folder, saves high scores, settings and progress, waits while the player is in another program, quits when the game asks, and takes screenshots for `golib shot`. [framework/README.md](framework/README.md) is its API guide: every exported name, grouped by task, with the rules the names don't tell you and what is still missing. Read it before writing game code; the doc comments in `framework/*.go` have the details. `games/platformer` is the reference for using it: read it before writing a game. It shows sprites, animations, a Tiled map seen through `golib.Camera`, pixel art, and a sound designed in jfxr. `games/asteroids` shows post-processing shaders, fullscreen, sound and music. Sound effects are made in code, from `.jfxr` files or from sound files, and music, sprites, maps and fonts are files in the game's `assets/` folder. There is no 3D yet (M7, after M6). If someone asks for a game that needs it, or anything else the API guide lists as missing, say what is missing and point to [docs/roadmap.md](docs/roadmap.md). Do not improvise a stand-alone engine to fill the gap.

## Golden rules

1. **English in the repository, always.** Code, identifiers, comments, docs, CLI output and commit messages are in English, whatever language the user speaks. Reply to the user in their language; write files in English. Only exception: in-game text, when the user explicitly wants their game in another language.
2. **Keep the zero-install promise.** Everything is provisioned by `golib setup` inside the project, in `.tools/`. Never ask the user to install software globally, edit PATH, set environment variables or use admin rights. If a platform truly needs a system package, `golib doctor` must detect it and print the exact fix: extend the tooling instead of giving ad-hoc instructions.
3. **Go through the `golib` CLI** for setup, building, running and testing. Run any other Go command as `golib go <args>`, never with a global `go` or `.tools/go/bin/go` directly: only `golib` sets the environment that keeps caches and telemetry inside `.tools/`. If you keep needing something the CLI lacks, add it to the CLI (both implementations, see [docs/tooling.md](docs/tooling.md)) instead of working around it.
4. **No new dependencies without explicit approval.** The budget is the Go standard library plus the raylib binding. Ask before adding Go modules, system packages or tools in other languages.
5. **Verify before saying "done".** Run the relevant commands and read their output. Report failures as they are, with the output. Compiling is not the same as working.
6. **Docs are part of the change.** When you change the CLI, the framework API or the layout, update this file and the affected docs in the same change. Stale AI docs are bugs.
7. **Transparent over clever.** Small readable scripts, plain Go, visible folders. No hidden state, no generated code the user can't see, nothing written outside the project folder.
8. **Nothing sensitive in the repo.** No secrets, tokens, personal data or machine-specific absolute paths. Assume every commit is public.
9. **Framework and games stay apart.** A game lives in its own folder under `games/` and uses only the framework's exported API. The framework never contains code for one particular game. Rules in [docs/architecture.md](docs/architecture.md).
10. **No editors of our own.** Content comes from established tools: Tiled for 2D maps, Aseprite for sprites, jfxr for sound effects, Blender for 3D models. Never build a level editor, sprite editor or asset GUI, not even inside a game; load those tools' files instead.

## Commands

Run from the project root. The command name is the same everywhere; only the prefix changes.

| Shell | Invocation |
| --- | --- |
| PowerShell or cmd (Windows) | `.\golib <command>` |
| bash, zsh, sh (Linux, macOS, Git Bash on Windows) | `./golib <command>` |

| Command | Effect |
| --- | --- |
| `setup` | Checks the environment, then installs Go, the Go modules and the raylib libraries into `.tools/`. Safe to run repeatedly. |
| `doctor` | Read-only diagnosis of the environment and the project. |
| `new <name>` | Creates `games/<name>/` from `tools/template/game/`: a small game that runs straight away, laid out like `games/platformer`. Names are lowercase letters, digits, `-` and `_`. |
| `build [game]` | Debug build: builds `games/<game>` into `build/<game>/`, next to copies of the raylib libraries, with a console window for errors. On Windows it carries the game's icon and details from `icon.png` and `game.json`, as a dist build does. `run`, `shot`, `test` and F5 build the same way, and read `assets/` from disk. Started from Explorer, the executable still reads `games/<game>/assets/` and shows errors in a message box, because its console window closes when the game ends. |
| `dist [game] [--web]` | Dist build, to share. With `--web`, builds for the browser instead into `build/<game>/dist/web/` and zips what is in it, with `index.html` at the top, ready for itch.io; its `THIRD-PARTY-LICENSES.txt` names Go and jfxr only, since a web build carries no raylib. Without it: builds `build/<game>/dist/<game>/`, with `<game>.exe` (no `.exe` on Linux and macOS, and no console window on Windows), the raylib libraries it loads and `THIRD-PARTY-LICENSES.txt`, and zips that folder into `build/<game>/dist/<game>-<version>-<os>-<arch>.zip`, the file to share. The executable carries the game's `assets/` folder, and players need the files next to it. `THIRD-PARTY-LICENSES.txt` includes the game's `assets/ATTRIBUTION.md`. A game with an `assets/` folder needs an `assets.go` file: see `golib.EmbedAssets`. On Windows the executable also carries the game's icon, from `icon.png`, and its title, version and author, from `game.json`, which also gives the zip its version (see [docs/tooling.md](docs/tooling.md#icon-and-version-information-windows)). The game writes nothing on the player's machine but what it saves with `golib.SaveData`, in `GoLib games/<game>` in their settings folder. |
| `run [game] [--dist]` | Builds the game, then runs it with `games/<game>/` as the working directory. With `--dist`, makes the dist build instead and runs it from its own folder, with the player's environment: the way to see what players get, assets and all. |
| `shot [game] [frame...] [--input "<script>"] [--save <file>] [--scale <n>]` | Builds the game, runs it in a hidden window and saves screenshots of the given frames (default: 60) as `build/<game>/shots/frame-NNNNNN.png`. Frame N shows the game after N updates. `--input "Enter@1 Right@30-90 Mouse@100:640,360 MouseLeft@101"` presses Enter in update 1, holds Right from update 30 to 90, moves the mouse pointer to 640, 360 and clicks. `--save <file.json>` starts the game with that data saved, so a shot opens on level 8 instead of playing there, and `--scale <1-8>` enlarges the pictures, for pixel art too small to read (see [docs/tooling.md](docs/tooling.md#screenshots)). Random numbers start from the same seed, so shots repeat. Open the files to see the game. |
| `test [game]` | Runs `go vet` and `go test` for the framework, every game and `tools/cli`. Given a game, only for `games/<game>`, so another game in progress doesn't get in the way. |
| `web [game] [--port <n>] [--no-open]` | Builds the game for the browser into `build/<game>/web/` and serves it at a `http://localhost` address, opening it unless `--no-open`. A web build draws, sounds, reads input, runs shaders, reads font files and saves in the browser's store; it cannot play `.xm`, `.mod` or `.qoa` (see [docs/tooling.md](docs/tooling.md#web-builds)). |
| `go <args>` | Runs the project's Go toolchain with GoLib's environment, for example `go -C games/platformer mod tidy`. It also puts `.tools/raylib/` on the library search path, so `golib go run` and `golib go test` can start programs that load raylib. |
| `clean` | Deletes `build/`. |
| `clean --all` | Also deletes `.tools/`. Run `setup` again afterwards. |
| `help` | Lists commands. |

`[game]` is a folder name in `games/`; leave it out when there is only one game. Check lines start with `[ok]`, `[info]`, `[warn]` or `[fail]`, followed by a summary line. Exit codes: `0` success, `1` failure, `2` usage error. When anything behaves unexpectedly, run `doctor` and read its output before trying fixes.

People who would rather click can double-click `golib-ui.cmd` (Windows) for a window with a button per command; it runs this same CLI and shows its output. Agents use the CLI.

In PowerShell, quote arguments that start with `-` and contain a dot, such as `'-replace=golib=../../framework'`: Windows PowerShell 5.1 splits them before `golib` receives them.

## Repository layout

```text
AGENTS.md            Canonical instructions for AI agents (this file)
CLAUDE.md            Claude Code entry point; imports this file
README.md            Human quick start
LICENSE              zlib license; games made in games/ are their authors'
golib, golib.cmd     CLI entry points for POSIX shells and Windows; thin shims, no logic
golib-ui.cmd         Double-click to open the GoLib window (Windows); a thin shim too
tools/bootstrap/     CLI scripts: golib.sh (Linux, macOS), golib.ps1 (Windows); they install Go, run help, doctor, go and clean, and build and start tools/cli for the rest
tools/cli/           The CLI in Go, built into build/golib/: new, build, run, shot, test, dist and most of setup
tools/ui/            The GoLib window: golib-ui.ps1, buttons that run the CLI
tools/template/game/ The files golib new copies into games/<name>/
framework/           The framework: Go module and package "golib"; README.md is its API guide
  internal/device/   The line to the machine: the contract, the raylib backend and the web one (web.js included)
games/               One folder per game, each its own Go module
  platformer/        The example game: tests each framework feature and shows how to use it
  asteroids/         A second example: post-processing shaders, fullscreen, sound and music
docs/                Vision, roadmap, architecture, tooling, contributing, AI playbooks
.claude/             Claude Code project settings and skills
.vscode/             Recommended extensions, editor settings, tasks
.tools/              Git-ignored. Go, Go modules and the raylib libraries, created by setup
build/               Git-ignored. Build outputs, created by build, run, shot and dist
```

A game imports the framework as `"golib"`, and its `go.mod` points that name at `../../framework` with a `replace` directive. How to create a game and the rules between framework and games are in [docs/ai/making-a-game.md](docs/ai/making-a-game.md) and [docs/architecture.md](docs/architecture.md).

## Two kinds of work

Decide which one you're doing before you start.

- **Making a game with GoLib.** The user describes a game; you build it in its own folder under `games/`, without modifying the framework. Follow [docs/ai/making-a-game.md](docs/ai/making-a-game.md).
- **Developing GoLib itself.** Changing the CLI, the framework, the template or these docs. Follow [docs/contributing.md](docs/contributing.md) and keep [docs/vision.md](docs/vision.md) in mind.

The test game developed alongside the framework is still a game: it follows the game rules, and any framework feature it needs is a separate framework change.

## Documentation map

| Document | Read it when |
| --- | --- |
| [docs/vision.md](docs/vision.md) | Judging whether an idea fits GoLib |
| [docs/roadmap.md](docs/roadmap.md) | Checking what's planned, or planning work |
| [docs/architecture.md](docs/architecture.md) | Deciding whether code belongs in the framework or in a game, how maps, sprites and models get into a game, or how a game moves to a newer GoLib |
| [docs/tooling.md](docs/tooling.md) | Changing the CLI, the VS Code config, line endings or `.tools/` |
| [docs/contributing.md](docs/contributing.md) | Changing GoLib itself: definition of done, API design for agents, commits |
| [docs/ai/making-a-game.md](docs/ai/making-a-game.md) | Building or iterating on a game |
| [framework/README.md](framework/README.md) | Writing game code: the framework's API by task, the rules it doesn't show, and what it doesn't have yet |

## Platform notes

- Windows baseline: Windows 10 or later with the built-in Windows PowerShell 5.1. PowerShell 7 is not required.
- Always type the `.\` or `./` prefix. Some environments, including agent sandboxes, stop Windows from running programs from the current folder by bare name; an explicit relative path always works.
- On Windows, `./golib` from Git Bash and `.\golib` from PowerShell run the same implementation (`golib.ps1`), so their results match.
- A debug build loads the raylib library, and libffi on Windows and macOS, when it starts, and stops with `cannot load library ...` if it can't find them (on Windows, with exit code 1 and a line that says where they go, and in a message box when nobody sees the console). `golib build`, `run`, `shot`, `test` and `go` take care of that; running a debug executable from outside `build/<game>/` doesn't. A `golib dist` build loads them from its own folder, where `dist` copies them, except on macOS, where it carries both inside; on Windows, moved away from them, it says so in a message box.
- `.gitattributes` enforces line endings: LF everywhere, CRLF only for `*.cmd` and `*.bat`. Don't change files to work around it.
