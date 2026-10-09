# AGENTS.md

Instructions for AI coding agents working in this repository (Claude Code, OpenAI Codex, GitHub Copilot, Cursor and others). Humans should start with [README.md](README.md).

## What GoLib is

GoLib is a small game framework for **Go**, built on **raylib** (windowing, input, graphics, audio), distributed as a ready-to-use **VS Code template**.

The promise: someone downloads the template, runs a couple of commands, and builds a complete game with an AI agent from one prompt and a few iterations. That makes you a primary user of this project. Docs, CLI output and (soon) the API are written so you can work without guessing, and you are expected to keep them that way.

## Project status

Keep this section true: update it in the same change that lands or removes a feature. Never describe planned work as if it existed.

Last updated: 2026-10-09 (`game.json`'s `buildTags`: a game's build tags, such as a demo's, kept to dist builds if it asks, which `build` and `run` without `--dist` then refuse, and a checkbox for each in the GoLib window); before it, 2026-10-07 (`Config.FixedWindowSize`, a window the player can't resize by its edges, for a game that offers window sizes in its settings); before it, 2026-10-04 (`--tags` for `build`, `run`, `shot`, `test`, `dist` and `web`: Go build tags, so one game builds in more than one way, such as a demo and the full game; `golib.Now`, the wall clock for a game whose world follows it, which `golib shot` moves 1/60 of a second per update, so its shots repeat); before it, 2026-10-03 (`game.json`'s `besideExecutable`: files of the game that `golib dist` copies next to the executable, such as translations for players to read and change; `golib.WindowSize`, the window's size out of fullscreen, for a game to open it as the player left it); before it, 2026-10-02 (the languages the player's system speaks, `golib.SystemLanguages` and `golib.ChooseLanguage`, for a translated game to start in the player's; sprites drawn at the window's resolution, `DrawOptions.FullResolution`, so a high-resolution logo stays sharp on a small screen, and a mouse pointer of the game's own, `golib.SetMouseSprite`; on the desktop, frames wait for the monitor's refresh, V-Sync, so a moving picture doesn't tear; tunes from notes get instruments' sounds and a room, and `Music.Preload` makes music before it plays).

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
| GoLib window (`golib-ui.cmd`): a button for each `golib` command, with its output | Done (Windows only); a checkbox for each of the game's build tags from `game.json` (2026-10-09) |
| Linux and macOS (`golib.sh`) | Works: on 2026-09-18 two other people set up GoLib on their own Linux and macOS machines, built and played a game, and ran an unzipped `golib dist` build. Windows still comes first, and `doctor`, `test`, `shot`, the icon and the version information there are unreported (see "Other platforms" in [docs/roadmap.md](docs/roadmap.md)) |
| Fixed-step game loop: 60 updates per second at any frame rate | Done (M2) |
| `golib shot`: screenshots of chosen frames, rendered in a hidden window, with scripted keyboard, mouse and gamepad input (`--input`) and random numbers from a fixed seed | Done (brought forward from M3); `--save` starts the game from saved data and `--scale` enlarges the pictures (M6); `golib.Now`, the wall clock for a world that follows it, which moves 1/60 of a second per update under `golib shot`, so such a game's shots repeat too (2026-10-04) |
| `golib dist`: the game to share, as a folder with the executable (assets inside), the raylib libraries and `THIRD-PARTY-LICENSES.txt`, and a zip of it | Done (M5; tried on Windows, and on Linux and macOS on 2026-09-18; on macOS the executable still carries the libraries) |
| Windows icon and version information, from the game's `icon.png` and `game.json`, in dist builds and in the debug builds of `build`, `run` and `shot` | Done (M5; not yet on Linux) |
| Files beside the executable: `game.json`'s `besideExecutable` lists files and folders of the game that `golib dist` copies next to the executable (next to the app on macOS), such as `assets/locale`, for players to see and change | Done (2026-10-03), tried on Windows |
| macOS app: `golib dist` wraps the executable in `<title>.app`, with the icon from `icon.png` and the details from `game.json`, signed ad hoc; debug builds show `icon.png` in the Dock | Done (2026-09-26), tried on an Intel Mac |
| A Windows game that can't load raylib or libffi says why, in a message box when nobody sees its console, instead of ending silently | Done (M5) |
| On Windows, `build` and `shot` work while `run` has the game open, and `golib` works while one of its copies is running | Done (M5); on Linux and macOS, `build` and `shot` too since 2026-09-30, by linking the new executable beside the running one and renaming it over it (see [docs/tooling.md](docs/tooling.md#the-go-program)) |
| Keyboard, mouse (pointer, buttons, wheel) and gamepad (buttons, sticks) input; rectangles, circles, lines and triangles; `Rectangle` overlap and point checks | Done (M2) |
| Prompts that show the button the player sees: whether they play with a gamepad now (`golib.PlayingWithGamepad`) and whether it is an Xbox, PlayStation or Nintendo one, from its name (`Input.GamepadType`) | Done (2026-09-29); the type is guessed from the names GLFW and the browsers give, and hasn't been tried with a PlayStation or Nintendo gamepad in hand yet |
| A game's graphics settings: the monitors connected and moving the window to one (`golib.Monitors`, `golib.CurrentMonitor`, `golib.SetMonitor`), how many of the screen's pixels a point is on a Retina Mac (`golib.DisplayScale`), the window's size (`golib.SetWindowSize`) and reading it back, out of fullscreen, so a game opens its window as the player left it (`golib.WindowSize`, 2026-10-03, used by a private game), a window the player can't resize by its edges, for a game that offers its own sizes (`Config.FixedWindowSize`, 2026-10-07, tried on Windows, used by a private game), a frame rate cap (`golib.SetFrameRate`, 60 by default) over frames that wait for the monitor's refresh, so a moving picture doesn't tear (V-Sync, on the desktop since 2026-10-02; a browser always does), the frames drawn a second (`golib.FPS`), and opening a web page or a mail to write (`golib.OpenURL`) | Done (2026-09-30), tried on macOS with one monitor; untried with several monitors and on Windows and Linux. In a browser, the frame rate cap works, the monitor and the window's size don't apply, and a link opens in a new tab |
| Typed text, as the player's keyboard layout makes it (`Input.TypedText`, and `Type@` in `golib shot --input`), and the keys Delete, Home, End, Page Up and Page Down; the punctuation keys, such as `KeyComma`, by their place on a US keyboard | Done (2026-09-27; punctuation 2026-10-01, for controls laid out by hand, such as a piano on the keyboard's bottom rows), for text a game lets the player type: names, code |
| Screen scaled to any window size, fullscreen (`golib.SetFullscreen`), post-processing shaders (`golib.NewShader`, `golib.SetPostProcess`) | Done (M2) |
| A screen that takes the window's shape, with no black bars in a window or in fullscreen, for tools and games whose layout stretches (`Config.FillWindow`) | Done (2026-09-30), tried on Windows |
| A screen that follows the window's size at a fraction of it, 2 for half its width and height, at the window's full resolution below a width of the game's choosing, and a call when it changes, for a game's layout and camera (`Config.WindowScale`, `Config.WindowScaleMinWidth`, `Config.OnScreenResize`) | Done (2026-10-01), used by `games/niebla` |
| Random numbers: `golib.RandomInt`, `golib.RandomFloat`, `golib.SetRandomSeed` | Done (M2) |
| Sound effects made in code, with no sound files: `golib.NewSound`, `golib.SoundSpec`, the ready-made recipes and `golib.SetVolume` | Done (M2) |
| Music streamed from the game's `assets/` folder: `golib.NewMusic` (OGG, MP3, WAV, QOA, XM, MOD; not IT) | Done (M2) |
| `golib.Quit`; no key quits a game on its own, not even Esc | Done (M2) |
| Scenes: `golib.SwitchScene` moves between title, play, pause and other screens | Done (M2) |
| Reading files from the game's `assets/` folder (`golib.ReadAsset`), embedded in dist builds | Done (M2) |
| `golib new <name>`: a new game, ready to run, from `tools/template/game/` | Done (M2) |
| Build tags: `--tags` for `build`, `run`, `shot`, `test`, `dist` and `web`, added to GoLib's own, so one game builds in more than one way, such as a demo beside the full game; a zip made with them carries them in its name | Done (2026-10-04), tried on macOS, where every command built with the tags; on Windows since 2026-10-09, by a private game's demo. `game.json`'s `buildTags` lists a game's tags, and `"dist"` keeps one to dist builds: `build` and `run` without `--dist` refuse it, and the GoLib window shows a checkbox per tag (2026-10-09) |
| Private games: `games/_<name>/`, which `.gitignore` keeps out of this repository, so a game can live in one of its own | Done (2026-09-19); every command treats it as any other game |
| API guide for agents, `framework/README.md`: every exported name by task, checked against the code by `golib test` | Done (M2) |
| Sprites from PNG images, PNG sprite sheets and Aseprite files, with animations: `golib.NewSprite`, `golib.NewSpriteSheet`, `Screen.DrawSprite`, `golib.Animation` | Done (M4); drawn at the window's resolution, smoothed, with `DrawOptions.FullResolution`, on 2026-10-02, for pictures with more detail than a small screen has room for, tried on Windows and in a browser |
| Tiled maps: `golib.NewMap`, `Screen.DrawMap`, `Screen.DrawMapLayer`, tiles and objects by layer, custom properties | Done (M4) |
| Pictures made pixel by pixel as a game runs, such as an emulator's screen: `golib.NewImage`, `Image.SetPixel`, `Screen.DrawImage`, on a raylib texture updated when it changes | Done (2026-09-26) |
| Sound effects from `.wav`, `.ogg`, `.mp3` and `.qoa` files: `golib.NewSoundFile`, and `Sound.SetVolume` | Done (M4) |
| Sound effects designed in jfxr, the sound effect maker: `.jfxr` files through `golib.NewSoundFile`, made by a Go version of jfxr's synthesizer that matches jfxr's samples; the same settings in code through `golib.NewSoundJfxr`, and `Sound.Unload` for sounds a game makes as it runs | Done (M4; settings in code and `Unload` 2026-09-26) |
| Fonts from `.ttf` and `.otf` files, in any language: `golib.NewFont`, `golib.TextOptions` | Done (M4) |
| Vectors and a 2D camera: `Vector2` methods, `golib.NewCamera`, `Screen.SetCamera` | Done (M6) |
| Saving high scores, settings and progress: `golib.SaveData`, `golib.LoadData`, `golib.DeleteData` | Done (M6) |
| The player's language, for a translated game: the languages the system shows its text in (`golib.SystemLanguages`), from Windows, macOS, Linux's environment or the browser, and which of the game's to start in (`golib.ChooseLanguage`) | Done (2026-10-02), tried on Windows; untried on macOS and Linux. The text and its translations are the game's |
| Looping sounds and stopping them: `Sound.Loop`, `Sound.Stop` | Done (M6) |
| Outlines, polygons, aligned text, a color's opacity, additive blending; hiding the mouse pointer | Done (M6); a pointer of the game's own, a sprite drawn at the window's resolution in place of the system's (`golib.SetMouseSprite`), on 2026-10-02, tried on Windows and in a browser |
| `games/skyraid` and `games/crates`: the test games, on the framework's camera, saving, tune and asset listing | Done (M6) |
| Music from notes, with no music file: `golib.NewTune`, with an instrument's sound for each voice (`Voice.Attack`, `Voice.Decay`, `Voice.Ring`, `Voice.LowPass`, `Voice.Detune`) and a room for the tune (`TuneSpec.Reverb`), and `Music.Preload` to make it before it plays | Done (M6); the instruments and `Preload` on 2026-10-02, measured and heard in a private game, whose author kept them, and a tune without them sounds as before, sample for sample |
| `Map.Err` and `golib.ListAssets`: whether a map loaded, and what is in the assets folder | Done (M6) |
| `golib.Timer` for cooldowns and intervals; `golib.Lerp`, `golib.Clamp` and the easings; `Sound.PlayWith` for one play's volume and pitch; `Config.PauseUnfocused` and `golib.WindowFocused` | Done (M6) |
| Games in the browser: a web backend of GoLib's own, on WebGL 2 and Web Audio, with `golib web`, `golib dist --web` and `golib shot --web` | Done (2026-09-18, stages 0 to 3): the picture, the input, the sound, post-processing shaders, fonts from files, saved data in the browser's store, a zip for itch.io, and screenshots. `games/platformer`, `tetris` and `crates` draw byte for byte what they draw on the desktop. Not there: `.xm`, `.mod` and `.qoa`, which browsers can't decode, and text from a font file pixel for pixel as on the desktop. Music in `.xm` or `.mod` no longer stops a web build (2026-09-19): GoLib plays a file of the same name in `.ogg`, `.mp3` or `.wav` beside it, and without one the game runs on in silence and says so. A build was uploaded to itch.io and played by hand on 2026-09-19: it drew and sounded, and the keyboard reached it once the backend learned to take the focus inside itch.io's `<iframe>`. The stages are in [docs/roadmap.md](docs/roadmap.md#web-build-started-2026-09-18) |
| Games on a phone: one web build played with fingers there and with the keyboard and mouse on a computer (`golib.PlayingWithTouch`, `Input.Touches`, `Input.TouchDownIn`, `Input.TouchPressedIn`, `Touch@` in `golib shot --input`, `golib web --lan`) | Done (2026-09-19, stage 4 of the web build): the page goes fullscreen instead of the canvas, and the canvas is measured against the screen while the game is fullscreen, so a phone turned from portrait to landscape fills it; touch events become fingers, and the oldest finger moves the mouse pointer and holds its left button, so a tap works a game written for a mouse. Whether a game shows its on-screen controls follows what the player last used, so the same build suits both: fingers turn them on, the keyboard, the mouse or a gamepad turns them off, and a computer with a touch screen starts without them. `games/asteroids` is the reference: it plays with fingers alone, and F4 chooses instead of the automatic answer. Checked in a browser with no window by `webscreen_test.go` and `webtouch_test.go`; played on a phone on 2026-09-19 (the fullscreen and the pads work; the earlier report of a game sitting in half the screen was that phone's rotation lock) |
| 3D: glTF models from Blender, a 3D camera, basic lighting | Planned (M7, after the web build); nothing built |

**The framework is still small.** It opens a window, runs a fixed-step game loop, reads the keyboard, the mouse, gamepads and the fingers on a touch screen, draws rectangles, circles, lines, triangles and polygons, filled or outlined, text in its built-in font or in fonts from files, aligned as the game wants, sprites from PNG and Aseprite files with their animations, in the screen's pixels or the window's, a mouse pointer of the game's own, pictures made pixel by pixel, and Tiled maps, whose tiles and objects a game can look up, shows worlds larger than the screen through a camera, does vector math, scales the screen to any window or fullscreen, moves the window to another monitor, sizes it and caps its frame rate, counts its frames, opens links, runs post-processing shaders, makes random numbers, counts time down with timers and softens movement with easings, makes and plays sound effects, once, in a loop or at another volume and pitch, from code, from jfxr's `.jfxr` files or from sound files, streams music from a file or from notes, switches between scenes, reads files from the game's `assets/` folder, saves high scores, settings and progress, says which languages the player's system speaks, waits while the player is in another program, quits when the game asks, and takes screenshots for `golib shot`, where `golib.Now`, the wall clock for a world that follows it, moves with the updates. [framework/README.md](framework/README.md) is its API guide: every exported name, grouped by task, with the rules the names don't tell you and what is still missing. Read it before writing game code; the doc comments in `framework/*.go` have the details. `games/platformer` is the reference for using it: read it before writing a game. It shows sprites, animations, a Tiled map seen through `golib.Camera`, pixel art, and a sound designed in jfxr. `games/asteroids` shows post-processing shaders, fullscreen, sound and music. Sound effects are made in code, from `.jfxr` files or from sound files, and music, sprites, maps and fonts are files in the game's `assets/` folder. There is no 3D yet (M7, after M6). If someone asks for a game that needs it, or anything else the API guide lists as missing, say what is missing and point to [docs/roadmap.md](docs/roadmap.md). Do not improvise a stand-alone engine to fill the gap.

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
| `new <name>` | Creates `games/<name>/` from `tools/template/game/`: a small game that runs straight away, laid out like `games/platformer`. Names are lowercase letters, digits, `-` and `_`. A name starting with `_`, such as `_moonshot`, makes a private game, which git ignores here (see [docs/architecture.md](docs/architecture.md#private-games)). |
| `build [game]` | Debug build: builds `games/<game>` into `build/<game>/`, next to copies of the raylib libraries, with a console window for errors. On Windows it carries the game's icon and details from `icon.png` and `game.json`, as a dist build does; on macOS the game shows `icon.png` in the Dock while it runs. `run`, `shot`, `test` and F5 build the same way, and read `assets/` from disk. Started from Explorer, the executable still reads `games/<game>/assets/` and shows errors in a message box, because its console window closes when the game ends. |
| `dist [game] [--web]` | Dist build, to share. With `--web`, builds for the browser instead into `build/<game>/dist/web/` and zips what is in it, with `index.html` at the top, ready for itch.io; its `THIRD-PARTY-LICENSES.txt` names Go and jfxr only, since a web build carries no raylib. Without it: builds `build/<game>/dist/<game>/`, with `<game>.exe` (no `.exe` on Linux, no console window on Windows, and on macOS the app `<title>.app` instead, which holds the executable), the raylib libraries it loads and `THIRD-PARTY-LICENSES.txt`, and zips that folder into `build/<game>/dist/<game>-<version>-<os>-<arch>.zip`, the file to share. The executable carries the game's `assets/` folder, and players need the files next to it. `THIRD-PARTY-LICENSES.txt` includes the game's `assets/ATTRIBUTION.md`. What `game.json`'s `besideExecutable` lists, such as `["assets/locale"]`, is copied next to the executable too, for players to see and change (see [docs/tooling.md](docs/tooling.md#icon-and-version-information-windows-and-macos)). A game with an `assets/` folder needs an `assets.go` file: see `golib.EmbedAssets`. On Windows the executable also carries the game's icon, from `icon.png`, and its title, version and author, from `game.json`, which also gives the zip its version; on macOS the app carries them (see [docs/tooling.md](docs/tooling.md#icon-and-version-information-windows-and-macos)). The game writes nothing on the player's machine but what it saves with `golib.SaveData`, in `GoLib games/<game>` in their settings folder. |
| `run [game] [--dist]` | Builds the game, then runs it with `games/<game>/` as the working directory. With `--dist`, makes the dist build instead and runs it from its own folder, with the player's environment: the way to see what players get, assets and all. |
| `shot [game] [frame...] [--input "<script>"] [--save <file>] [--scale <n>] [--web]` | Builds the game, runs it in a hidden window and saves screenshots of the given frames (default: 60) as `build/<game>/shots/frame-NNNNNN.png`. Frame N shows the game after N updates. `--input "Enter@1 Right@30-90 Mouse@100:640,360 MouseLeft@101"` presses Enter in update 1, holds Right from update 30 to 90, moves the mouse pointer to 640, 360 and clicks; `Touch@40-90:200,600` puts a finger on the touch screen, for a game's on-screen controls. `--save <file.json>` starts the game with that data saved, so a shot opens on level 8 instead of playing there, and `--scale <1-8>` enlarges the pictures, for pixel art too small to read (see [docs/tooling.md](docs/tooling.md#screenshots)). Random numbers start from the same seed, so shots repeat. Open the files to see the game. With `--web`, takes the same shots of the web build instead, in a browser with no window, into `build/<game>/shots-web/`, so a web build can be checked against the desktop one; `--save` doesn't work there. |
| `test [game]` | Runs `go vet` and `go test` for the framework, every game and `tools/cli`. Given a game, only for `games/<game>`, so another game in progress doesn't get in the way. |
| `web [game] [--port <n>] [--no-open] [--lan]` | Builds the game for the browser into `build/<game>/web/` and serves it at a `http://localhost` address, opening it unless `--no-open`. With `--lan` it serves to the whole network as well and prints the address to open on a phone or a tablet on the same Wi-Fi, which is how a touch screen is tried. A web build draws, sounds, reads input, runs shaders, reads font files and saves in the browser's store; it cannot play `.xm`, `.mod` or `.qoa` (see [docs/tooling.md](docs/tooling.md#web-builds)). |
| `go <args>` | Runs the project's Go toolchain with GoLib's environment, for example `go -C games/platformer mod tidy`. It also puts `.tools/raylib/` on the library search path, so `golib go run` and `golib go test` can start programs that load raylib. |
| `clean` | Deletes `build/`. |
| `clean --all` | Also deletes `.tools/`. Run `setup` again afterwards. |
| `help` | Lists commands. |

`build`, `run`, `shot`, `test`, `dist` and `web` also take `--tags <tags>`: Go build tags, separated by commas, such as `--tags demo`, for the game's files that start with `//go:build demo`. They go after GoLib's own, and a zip made with them carries them in its name, `<game>-<version>-demo-<os>-<arch>.zip` (see [docs/tooling.md](docs/tooling.md#build-tags)). A tag the game's `game.json` keeps to dist builds, `"buildTags": {"demo": "dist"}`, is refused by `build` and by `run` without `--dist`: use `dist`, `run --dist` or `web`; `shot` and `test` still take it, to check that build.

`[game]` is a folder name in `games/`; leave it out when there is only one game. Check lines start with `[ok]`, `[info]`, `[warn]` or `[fail]`, followed by a summary line. Exit codes: `0` success, `1` failure, `2` usage error. When anything behaves unexpectedly, run `doctor` and read its output before trying fixes.

People who would rather click can double-click `golib-ui.cmd` (Windows) for a window with a button per command; it runs this same CLI and shows its output. Agents use the CLI.

In PowerShell, quote arguments that start with `-` and contain a dot, such as `'-replace=golib=../../framework'`: Windows PowerShell 5.1 splits them before `golib` receives them. Quote a list of build tags too, `--tags 'demo,steam'`: PowerShell reads words joined by commas as a list.

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
  _<name>/           A private game: git ignores games/_*/, so it stays out of this repository
  platformer/        The example game: tests each framework feature and shows how to use it
  asteroids/         A second example: post-processing shaders, fullscreen, sound and music
docs/                Vision, roadmap, architecture, tooling, contributing, AI playbooks
.claude/             Claude Code project settings and skills
.vscode/             Recommended extensions, editor settings, tasks
.tools/              Git-ignored. Go, Go modules and the raylib libraries, created by setup
build/               Git-ignored. Build outputs, created by build, run, shot and dist
```

A game imports the framework as `"golib"`, and its `go.mod` points that name at `../../framework` with a `replace` directive. How to create a game and the rules between framework and games are in [docs/ai/making-a-game.md](docs/ai/making-a-game.md) and [docs/architecture.md](docs/architecture.md).

A game whose folder name starts with `_` is **private**: `.gitignore` has `/games/_*/`, so it never enters this repository and can have a git repository of its own. It is an ordinary game everywhere else, and the same rules apply to it: it uses the framework's exported API, and anything it needs from GoLib is a separate, public framework change. Never commit a private game's files here, and never move its code into `framework/`. See [docs/architecture.md](docs/architecture.md#private-games).

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
