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

- Images from PNG files, including sprite sheets cut into a grid of frames.
- Sprites and animations from Aseprite's own `.aseprite` files, with their tags as animations.
- 2D maps from Tiled's TMX files and TSX tilesets: tile layers, tilesets and object layers.
- Sound effects loaded from files, for games that want more than the ones M2 makes in code. Music landed in M2.
- Fonts.

## M5: Shipping (in progress)

- `golib dist`. Done early, during M2, as a single executable with raylib, libffi and the game's assets inside; on Windows it opens no console window and shows errors in a message box. Since 2026-09-17 it is a folder and a zip (below). To do: a macOS app bundle, builds for other platforms than the current one.
- Done on 2026-09-17: the CLI in Go. The command logic moved from the twin scripts into one Go program, `tools/cli`, one command at a time (see [Decisions](#decisions) and [tooling.md](tooling.md#the-go-program)):
  - `dist`, with `tools/shipping` folded in. The scripts build the program into `build/golib/` and start it with the user's environment, and it sets GoLib's environment for the `go` commands it runs. It now checks `game.json` and `icon.png` on Linux and macOS too, and prints a game's title correctly when it isn't ASCII. On Windows, its executables match the ones the script made, byte for byte, apart from Go's build information.
  - `new`, `build`, `run`, `shot` and `test`. Their output is the same as before, their screenshots match the scripts' byte for byte, and games now start without GoLib's Go variables or any `GOLIB_SHOT_` variable from the user's environment. The scripts shrank from about 800 lines each to about 450, and keep `help`, `setup` up to installing Go, `doctor`, `clean` and `go`, which has to work when `tools/cli` doesn't compile.
  - `setup` after Go is installed: downloading the Go modules and filling `.tools/raylib/`, which now has one implementation. The scripts pass on how many warnings they printed, so the summary counts them. A setup from scratch, in a copy of the project with no `.tools/`, took 37 seconds, and `test`, `shot` and `dist` then worked there without writing to the user's Go telemetry folder.
- Done on 2026-09-17: dist builds as a folder, `build/<game>/dist/<game>/`, with the executable, raylib and libffi side by side, and a `THIRD-PARTY-LICENSES.txt` with the licenses of Go, of each Go module in the game but GoLib (purego, raylib-go and ffi), of raylib and libffi, and the game's `assets/ATTRIBUTION.md`; plus a zip of that folder to share, `<game>-<version>-<os>-<arch>.zip`, instead of one executable that writes the libraries into the player's cache folder (see [Decisions](#decisions) and [tooling.md](tooling.md#dist-builds)). The executable still carries the game's assets. Tried on Windows amd64: unzipped anywhere, the game runs from its folder and writes nothing into the player's cache folder. Built for Linux amd64 from Windows, the executable has `$ORIGIN` as its `DT_RUNPATH`, which should make Linux find `libraylib.so.6.0.0` next to it; not run on Linux yet. macOS dist builds still carry both libraries: macOS looks for a library given by its bare name, as raylib-go gives raylib's, only in `DYLD_LIBRARY_PATH`, the working directory and system folders.
- Done on 2026-09-17: the notices of the libraries inside raylib, which raylib's `LICENSE` doesn't cover. raylib-go's `config.h` and C files include cgltf, tinyobj_loader_c, vox_loader, m3d, par_shapes, QOI, QOA and glad, and, in the Windows library, dirent; their strings are in the prebuilt Windows, Linux and macOS libraries. Their notices, copied from their headers, are in `tools/cli/notices/raylib-6.0.txt`, which `dist` adds under raylib's heading, and `golib test` fails when raylib-go brings a raylib version without such a file (see [tooling.md](tooling.md#third-party-licenses)). The others (GLFW, miniaudio, dr_libs, stb, jar_xm, jar_mod, sinfl, sdefl, rprand, rl_gputex) are zlib, public domain, MIT-0 or a choice of public domain, which ask for nothing in programs. Not a legal review: whoever publishes a game should still check what they ship.
- To do: loading raylib and libffi from the executable's folder on macOS, probably with the app bundle. raylib-go and ffi would need a way to take a full path, such as `@executable_path/libraylib.6.0.0.dylib`; ffi reads its library's name from a package variable that `-ldflags=-X` can set, but raylib-go has none.
- Done: the Windows icon and version information. `golib dist` reads the game's `icon.png` and `game.json` (title, version, author, copyright), and `tools/shipping`, GoLib's first piece of tooling written in Go, turns them into Windows resources that the Go linker adds to the executable. Explorer, the title bar and the taskbar show the icon, and Explorer and Task Manager show the details. `golib new` writes a `game.json`, and both example games have both files; their icons are placeholders drawn in code, to be replaced with ones made in Aseprite.
- Web build: investigated on 2026-09-16, then parked by the owner; nothing was built. What was found, for when it resumes:
  - raylib-go has no web support, and its maintainer doesn't plan any ([issue #356](https://github.com/gen2brain/raylib-go/issues/356)). Raylib-Go-Wasm, a community fork that calls an Emscripten build of raylib from Go, has no license, so its code can't be reused.
  - GoLib doesn't compile for `GOOS=js`: raylib-go's purego mode and the ffi module don't, and the framework defines its colors and keys from raylib's constants. A web build first needs the framework split into a core that works everywhere and a raylib backend.
  - Go's side works, as tried in a throwaway program: `//go:wasmimport` is allowed with `GOOS=js` and costs about 20 ns a call (about 650 ns through `syscall/js`), JavaScript can read Go's memory (for audio samples), a game loop can wait for `requestAnimationFrame` on a channel, and a small program is about 2.5 MB (700 KB gzipped).
  - Checking web builds needs no installs on Windows: headless Edge ships with Windows and has WebGL 2, and a page can post its screenshots to a small local Go server. Linux and macOS would need Chrome or Chromium.
  - `games/asteroids`' shaders compile as GLSL ES 3.00 once `#version 330` becomes `#version 300 es`; shaders that mix integers into float math, such as `uv * 2`, don't.
  - Two designs. (A) A web backend of GoLib's own, WebGL 2 and Web Audio behind a small JavaScript file: no C toolchain anywhere, but browsers don't decode XM, MOD or QOA music, and every M4 feature would be built twice. (B) raylib built for the web once by GoLib's maintainers with Emscripten (a 654 MB SDK): behaves like desktop raylib, but needs published binaries, copies between two separate memories, and patches for audio and the main loop. TinyGo, and Emscripten on the user's machine, were ruled out.
  - If A is chosen, the owner prefers a first web version with sound effects only, and music in a later step.
  - itch.io takes a zip with `index.html` (up to 1,000 files and 500 MB). Browsers wait for a click or a key press before playing audio or going fullscreen.

Known gaps:

- Dist builds have only been tested on Windows amd64. On Linux, players also need the system's `libffi.so.8`, `libX11.so.6` and `libGL.so.1`.
- `tools/cli` has only run on Windows. For Linux and macOS, it passes `go vet` and its tests with those platforms' settings, and the part of `golib.sh` that builds and starts it ran with a simulated `uname` and `go`.
- On Windows, while a game started by `golib run` or `shot` is open, `build/golib/golib.exe` is running, so a change to `tools/cli` can't be built and `golib clean` can't delete it until the game ends.
- The icon and the details reach Windows dist builds only: debug builds show Windows' default icon, and Linux and macOS dist builds check `icon.png` and `game.json` but don't use them. For Windows on ARM, the resources have been linked and read back by Windows, but not seen in a running game.
- The ffi module ships libffi for Windows amd64 and macOS only. On Windows on ARM there is no libffi to load, so games would stop at startup, in debug and dist builds alike. Not tested.
- A dist build whose executable can't find its libraries, because a player moved it out of its folder, stops before `golib.Run` starts, and a Windows dist build has no console, so the player sees nothing happen. raylib-go and ffi load the libraries while Go initializes their packages, before any GoLib code runs.
- macOS dist builds still write raylib and libffi into the player's cache folder, with the problem the decision below describes.

## M6: 3D (planned, after M4)

Nothing built yet; the API is designed when the milestone starts (see [Decisions](#decisions)). First scope:

- 3D models exported from Blender as glTF (`.glb`), loaded through `golib.ReadAsset`.
- A 3D camera.
- Basic lighting.

## Open questions

None at the moment. Add new ones here, and move each one to Decisions once it is answered.

## Decisions

Open questions that have been answered, with the date and the reasons, so they aren't reopened by accident.

- **Framework distribution** (2026-09-16). GoLib ships as the whole template, with the framework's source inside, and games keep using it through `replace golib => ../../framework`. There is no published framework module, and the module path stays `golib`. Why: the CLI, `AGENTS.md` and the API guide describe one version of the framework, and a module updated on its own would get out of step with them; agents read the framework's code in `framework/`, not in the read-only module cache; and a published path, `github.com/pery77/golib/framework`, would be long and wouldn't match the package name. Releases, when there are any, will be tags of the whole repository. A game moves to a newer GoLib by copying its folder (see [architecture.md](architecture.md#distribution)). Reconsider if Go developers ask to use the framework outside the template; switching then means changing each game's `go.mod` and imports.
- **Libraries in shipped games** (2026-09-16). `golib dist` will put raylib and libffi next to the executable, with the third-party license notices, loaded the way debug builds load them, and zip that folder to share; a game will write nothing on the player's machine. Built on 2026-09-17, except on macOS (see M5). Why: raylib-go and ffi write their embedded libraries into cache folders named after their Go modules, shared with every other program built on them, and never check them again. A damaged copy, from an interrupted first start for example, stops the game from starting until someone deletes that folder, and the player sees no message: the libraries load before `main`, so `golib.Run` can't catch the error, and a Windows dist build has no console. Tried on 2026-09-16 with a truncated `raylib.dll` in the cache folder: the game exited with code 2, and the error went only to stderr. Programs that write DLLs into AppData and then load them also look like droppers to antivirus software. The cost: the executable no longer runs on its own, so players keep the folder together, which the zip takes care of. On Linux, the executable finds libraries in its own folder through an rpath, `$ORIGIN`; untested. macOS doesn't look there for a library given by its bare name, so macOS builds keep the embedded copies until raylib-go can load a full path.
- **2D and 3D** (2026-09-16). 2D first; 3D is its own milestone, M6, after M4, starting with glTF models from Blender, a 3D camera and basic lighting. Until then GoLib has no 3D. Why: 3D multiplies the API (cameras, models, lights, materials, 3D collisions), against "one obvious way" and "games, not engines"; agents reason about 3D space less reliably and can check it less reliably with `golib shot`; and 2D still lacks sprites, maps and fonts (M4) and a camera and vectors. Design those 2D pieces so that 3D can extend them later, `Vector2` next to a future `Vector3`, without breaking games.
- **Content formats** (2026-09-16). One format for each kind of content, the one the tool saves by default (see [architecture.md](architecture.md#content-established-tools-no-editors-of-our-own)):
  - Tiled: TMX maps and TSX tilesets only, not JSON. They are Tiled's defaults and what most tutorials and tile packs use, so saving in Tiled is enough; a second parser would double the code and the tests without letting agents do anything new, since they write TMX with CSV tile data as easily as JSON.
  - Aseprite: `.aseprite` files for animated sprites, with their tags as animations, and PNG for images and for sprite sheets on a grid, the usual form of free art packs. Not Aseprite's JSON sprite sheets: they need an export step that is easy to forget, and they would be a second way to do the same thing. The `.aseprite` format is documented and needs only `compress/zlib`; combining layers with their blend modes is the costly part.
  - Source files a game doesn't load, such as `.blend` or `.psd`, go in `games/<name>/sources/`, with the same paths as in `assets/`. They are committed, and `golib dist` leaves them out because it embeds only `assets/`.
- **CLI in Go** (2026-09-16). The command logic moves from the twin scripts into one Go program, `tools/cli`, one command at a time, starting with `dist`, before dist builds become a folder and a zip; `tools/shipping` becomes part of it. The scripts keep what must work without Go, or while the CLI isn't running: downloading and checking Go in `setup`, building and starting the CLI, `clean` (on Windows a running program can't delete its own folder), the checks of `doctor` that don't need Go, and a message to run `setup` first. Done on 2026-09-17, starting with `dist` (see M5). Added the same day: `go` stays in the scripts too, so it still works when `tools/cli` doesn't compile. Why: two scripts of about 800 lines each, which must change together, have already drifted apart (only `golib.ps1` adds the icon and version information, and `golib.sh` has never run on Linux or macOS). Go gives one implementation for every platform, tested with `go test`, with a standard library for what the next changes need, such as zip files: Windows PowerShell 5.1's `Compress-Archive` writes their paths with backslashes (checked on 2026-09-16), and Linux can't make them without installing `zip`. The cost, measured on Windows: today `golib` spends about 420 ms starting PowerShell, checking that the built CLI is up to date adds about 100 ms, and building it the first time takes about 5 s. The vision's "short script" rule becomes short scripts to start, plain Go for the rest.
- **License** (2026-09-16). GoLib is under the zlib license, in `LICENSE`, with the copyright held by pery77. It covers the framework, the tools, the docs and the example games, except files with terms of their own, such as the music in `games/asteroids/assets/`. The games people make in `games/` are theirs to license. Why: the framework is compiled into every game, so its license decides what GoLib asks of whoever publishes one, and zlib asks nothing in binaries: only that the notice stays in source copies and that changed sources say so. It is raylib's license too, and common for game libraries. MIT would ask each game to carry GoLib's notice, and Apache-2.0 would add the license text and change notices.
