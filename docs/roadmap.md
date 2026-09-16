# Roadmap

Milestones land in order; what goes into each one can still move as we learn. The "Project status" table in [AGENTS.md](../AGENTS.md) mirrors this file, so update both together.

## M0: Foundation (done, 2026-09-14)

- Cross-platform `golib` CLI entry points with `setup`, `doctor`, `clean` and `help`.
- VS Code workspace: recommended extensions, editor settings, tasks.
- AI instructions: `AGENTS.md`, `CLAUDE.md`, Claude Code settings and the `/make-game` skill.
- Docs: vision, roadmap, architecture, tooling, contributing, game-making playbook.

## M1: Toolchain and first window (done, 2026-09-14)

- `golib setup` downloads Go 1.27.1 into `.tools/`, verifying its checksum.
- raylib 6.0 through [raylib-go](https://github.com/gen2brain/raylib-go) in its purego mode, so no platform needs a C compiler. The prebuilt raylib library ships inside the binding's module; `golib` extracts it into `.tools/raylib/` and copies it next to each build (see [tooling.md](tooling.md)).
- The framework and game split (see [architecture.md](architecture.md)). `framework/` is the Go module `golib`, and `games/hello` opens a window and draws text through it. In M2, `games/platformer` replaced it as the test game.
- `golib run`, `golib build` and `golib test`, with the game picked by its folder name, plus `golib go` to run the project's Go toolchain.
- VS Code: the Go extension uses the local toolchain, and "GoLib: debug game" (F5) debugs a game with Delve.

Known gaps, written down as the [definition of done](contributing.md#definition-of-done) requires:

- On hold since 2026-09-15, while Windows comes first: `golib.sh` has not run on a real Linux or macOS machine, and framework features since M1 have only been tried on Windows. From Windows, its help, usage errors and doctor ran in Git Bash, and its Go download, checksum check and extraction ran on a simulated Linux. The first time GoLib is on Linux or macOS, run `golib setup`, `golib test` and `golib run` there; continuous integration on Linux and macOS runners would keep it working.
- VS Code's Go extension runs `go version -m` without the project environment each time it starts, so Go updates its local telemetry counters in the user's config folder. No setting redirects those calls; see [tooling.md](tooling.md#vs-code-integration).

## M2: Framework basics (done, 2026-09-16)

- Done: a fixed-step game loop. `Update` runs 60 times per second of game time with a constant `dt`, whatever the frame rate; after a long pause the lost time is skipped.
- Done: scenes. Each scene is a `golib.Game`, and `golib.SwitchScene` moves between them; `games/platformer` has a title, play, pause and won scene.
- Done: input from the keyboard, the mouse (pointer, buttons, wheel) and up to four gamepads (buttons, and sticks with a dead zone), with each press, click and wheel turn delivered to exactly one update. Not covered: analog trigger pressure, vibration.
- Done: random numbers with `golib.RandomInt` and `golib.RandomFloat`, and `golib.SetRandomSeed` to repeat them.
- Done: the GoLib window, `golib-ui.cmd`: a button for each `golib` command, with its output, for people who prefer clicking. Windows only; on Linux and macOS, use the CLI or the VS Code tasks.
- Done: `golib.Quit` ends a game. No key quits on its own: raylib's default of closing on Esc is turned off.
- Done: 2D drawing with rectangles, circles, lines, triangles, text, and `Rectangle` overlap and point checks. Textures and sprites are part of M4.
- Done: the screen scales to any window size and to fullscreen (`golib.SetFullscreen`), with `Config.PixelArt` for sharp pixels, and post-processing shaders run over the whole picture (`golib.NewShader`, `golib.SetPostProcess`). `games/asteroids` tries them out.
- Done: sound effects made in code, so a game ships no sound files. `golib.NewSound` turns a `golib.SoundSpec` (waveform, frequency, pitch slide, duration, attack, release, volume, duty, vibrato) into a sound raylib plays, with ready-made recipes (`golib.Laser`, `golib.Explosion`, `golib.Pickup`, `golib.Jump`, `golib.Hurt`, `golib.PowerUp`) and `golib.SetVolume`. Up to four copies of a sound play at once. `games/asteroids` uses them. Not covered: looping or positional sounds, and sound effects loaded from files, which are part of M4.
- Done: music streamed from the game's `assets/` folder with `golib.NewMusic`: OGG, MP3, WAV, QOA and the tracker formats XM and MOD, but not IT, which raylib doesn't read. It loops, and `golib.Music` has `Play`, `Pause`, `Stop` and `SetVolume`. `games/asteroids` plays a module. Not covered: crossfading and playlists.
- Done: reading files from the game's `assets/` folder with `golib.ReadAsset`, from disk in debug builds and from the copy embedded in dist builds.
- Done: `golib new <name>` creates a game folder in `games/` from `tools/template/game/`, ready to run, without touching the framework.
- Done: example games that double as documentation. `games/platformer` tries each feature as it lands, and `games/asteroids` shows screen effects, fullscreen, sound and music; both keep growing with the framework.
- Done: an API guide for agents, `framework/README.md`: every exported name grouped by task, the rules the names don't tell you, and what GoLib doesn't have yet. Two framework tests fail when it misses an exported name or names one that doesn't exist. They embed the guide, so `go test` runs them again when only the guide changes.

## M3: Agent verification loop (done early, during M2, 2026-09-15)

- `golib shot` runs a game in a hidden window for a number of frames and saves screenshots an agent can inspect, and `--input` plays keys, mouse buttons and mouse moves in chosen updates, so shots reach every scene.
- Deterministic runs, so results are reproducible: the fixed timestep, with exactly one update per frame in shots, and random numbers that start from the same seed in every shot.

## M4: Content (postponed)

Postponed on 2026-09-15, until the owner picks it up again. Loading what Aseprite, Tiled and audio tools make, through `golib.ReadAsset`:

- Textures, and sprites and animations loaded from Aseprite files.
- 2D maps loaded from Tiled: tile layers, tilesets and object layers.
- Sound effects loaded from files, for games that want more than the ones M2 makes in code. Music landed in M2.
- Fonts.

## M5: Shipping (in progress)

- `golib dist`. Done early, during M2: a single executable with raylib, libffi and the game's assets inside; on Windows it opens no console window and shows errors in a message box. To do: a macOS app bundle, builds for other platforms than the current one.
- Done: the Windows icon and version information. `golib dist` reads the game's `icon.png` and `game.json` (title, version, author, copyright), and `tools/shipping`, GoLib's first piece of tooling written in Go, turns them into Windows resources that the Go linker adds to the executable. Explorer, the title bar and the taskbar show the icon, and Explorer and Task Manager show the details. `golib new` writes a `game.json`, and both example games have both files; their icons are placeholders drawn in code, to be replaced with ones made in Aseprite.
- Investigate a web build.

Known gaps:

- Dist builds have only been tested on Windows amd64. On Linux, players also need the system's `libffi.so.8`, `libX11.so.6` and `libGL.so.1`.
- The icon and the details reach Windows dist builds only: debug builds show Windows' default icon, and Linux and macOS dist builds ignore `icon.png` and `game.json`. For Windows on ARM, the resources have been linked and read back by Windows, but not seen in a running game.
- The ffi module ships libffi for Windows amd64 and macOS only. On Windows on ARM there is no libffi to load, so games would stop at startup, in debug and dist builds alike. Not tested.

## Open questions

- **Framework distribution.** Today the template ships the framework's source, and each game points at it with a `replace` directive in its `go.mod`. Should games later depend on a published, versioned framework module instead? That needs the final repository URL, which would also become the framework's module path (today just `golib`).
- **Libraries in shipped games.** A dist build carries raylib and libffi inside, and raylib-go and ffi write them to the player's cache folder on first start, in folders named after their Go modules (`github.com\gen2brain\...`) that GoLib can't choose. Is that acceptable for published games, or should `dist` offer the libraries next to the executable instead, as debug builds do?
- **2D and 3D.** 2D only at first, or both from the start? Either way, 3D models come from Blender as glTF.
- **Content formats.** Tiled saves maps as TMX (XML) or JSON: support one or both? Aseprite: load `.aseprite` files only (no export step), or also PNG sprite sheets with Aseprite's JSON, which other pixel-art tools can produce? Where do source files the game doesn't load, like `.blend`, live inside the game folder?
- **CLI in Go.** Now that Go is local, move command logic from the twin shell scripts into one Go program, keeping the scripts as tiny bootstrappers (see [tooling.md](tooling.md)).
- **License.**
