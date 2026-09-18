# Tooling

How the `golib` command works, where its files live, and how to change it.

## Entry points

| File | Started from | What it does |
| --- | --- | --- |
| `golib.cmd` | cmd, PowerShell | Runs `tools/bootstrap/golib.ps1`. `-ExecutionPolicy Bypass` applies to that single run; no system setting changes. |
| `golib` | sh, bash, zsh on Linux and macOS; Git Bash and MSYS2 on Windows | Runs `tools/bootstrap/golib.sh`. On Windows POSIX shells it delegates to `golib.ps1`, so Windows always uses one implementation. |
| `golib-ui.cmd` | Double-click in Explorer, or `.\golib-ui` | Starts the [GoLib window](#golib-window), `tools/ui/golib-ui.ps1`, in a hidden Windows PowerShell, and returns at once. |

PowerShell and cmd both resolve `.\golib` to `golib.cmd`, so the same word works in every shell; only the prefix changes (`.\` on Windows, `./` elsewhere). Gradle's `gradlew` and `gradlew.bat` use the same pattern.

Always write the explicit prefix, in docs, tasks and scripts. Windows can be configured not to run programs from the current folder by bare name (the `NoDefaultCurrentDirectoryInExePath` environment variable, which some agent environments set). An explicit relative path always works.

Both shims find the project root from their own location, so they also work when called with a full path from another folder.

## Implementations

The command logic lives in one Go program, and two twin scripts start it (see [roadmap.md](roadmap.md#decisions)):

| Part | Commands |
| --- | --- |
| `tools/bootstrap/golib.ps1` (Windows PowerShell 5.1 and PowerShell 7) and `tools/bootstrap/golib.sh` (POSIX sh), twins with the same commands, options, output and exit codes | `help`, `setup` up to installing Go, `doctor`, `go`, `clean`; they also build and start the Go program for everything else |
| `tools/cli/`, the Go program | `new`, `build`, `run`, `shot`, `test`, `dist`, and the rest of `setup`: downloading the Go modules and filling `.tools/raylib/` |

The scripts keep what has to work without Go, or while the Go program isn't running or doesn't build: checking the machine and downloading Go in `setup`, building and starting the Go program, `clean` (on Windows a running program can't delete its own folder), `doctor`, which never starts Go, and `go`, which stays usable to fix `tools/cli` when it doesn't compile. After installing Go, `setup` hands over to the Go program's `setup`, passing `--warnings=N`, the number of warnings it printed, so the summary line counts them; the scripts refuse options to `setup`, so people never pass it.

Rules for everything golib prints, in the scripts and the Go program:

- Check output: one fact per line, prefixed `[ok]`, `[info]`, `[warn]` or `[fail]`, then a summary line such as `doctor: 0 failed, 1 warning(s)`. Plain ASCII, no colors: the reader is often an agent parsing logs. Text quoted from the user's files, such as a game's title, is printed as it is.
- Exit codes: `0` success, `1` failure, `2` usage error. Usage errors go to stderr, followed by `Run "golib help" for usage.`
- Never read or write anything outside the project folder, except downloading from pinned URLs. Every `go` command runs with the project environment described below.
- `setup` must be idempotent: running it twice is safe and fast.
- `doctor` only reads: it inspects files and never starts `go`.

### The Go program

`tools/cli` is a Go module of its own, `cli`, that uses only the standard library. For each command it implements, both scripts:

1. check that the pinned Go is installed, and stop with `[fail] ... (run: golib setup)` if it isn't;
2. build it with the project environment into `build/golib/golib.exe` (`build/golib/golib` on Linux and macOS). `go build` leaves an up-to-date executable alone, so this adds about 0.1 seconds; after a change to `tools/cli`, or after `golib clean`, it takes a few seconds. A build error stops with a `[fail]` line;
3. start it with the command and its options, and with the user's own environment, and exit with its exit code: `Invoke-Cli` in `golib.ps1`, `run_cli` in `golib.sh`.

The program finds the project from its own location, two folders above `build/golib/`, and sets the project environment for each `go` command it runs (`goEnv` in `tools/cli/project.go`), so the games it starts get the user's environment. `golib test` vets and tests it; its tests replace the `go` command and the games with fakes that record how they were started (`runGo` and `runGame` in `project.go`).

While a game started by `run` or `shot` is open, the program is running too, and so is the game. On Windows, a running program's file can't be replaced or deleted, so golib works around both:

- When `build/golib/golib.exe` is running, `golib.ps1` builds and starts `golib-2.exe` instead, or `golib-3.exe` and so on, the first one that isn't running (`Get-CliExe`), so other commands work, and a change to `tools/cli` still takes effect.
- A debug build copies `raylib.dll` and `libffi-8.dll` next to the game only when they differ from the ones there, and `go build` moves a running executable aside (as `<game>.exe~`) to write the new one. So `build` and `shot` work while `run` has the game open; the build fails only if the game was built again and both older copies are still open.
- `golib clean` can't delete a running program's files: it stops with `cannot delete build/ completely` and exit code 1. Close the games golib started, then run it again.

Linux and macOS replace and delete running programs' files, so `golib.sh` needs only the `clean` message, which it prints when `rm` fails for any other reason.

To move a command into it:

1. Write the command in `tools/cli`, add it to `commands` in `main.go`, and test it.
2. In both scripts, send the command to the Go program in the dispatcher, and delete the code that only that command used.
3. Check that it prints what the scripts printed, on Windows and on Linux or macOS, then update the table above and [roadmap.md](roadmap.md).

### Rules for the scripts

Change both in the same commit, and test both.

Rules for `golib.ps1`:

- ASCII only. Windows PowerShell 5.1 reads files without a BOM as ANSI.
- No PowerShell 7-only syntax: no `&&`, `||`, `??` or ternary operators.
- Keep `$ErrorActionPreference = 'Stop'` and `Set-StrictMode -Version 3.0`.
- Inside functions that return a value, pipe native commands to `Out-Host`, or their output becomes part of the return value.

Rules for `golib.sh`:

- POSIX sh only, so it runs under dash: no arrays, no `[[ ]]`, no `local`, no `function` keyword. Functions share one variable namespace, so prefix their variables.
- Keep `set -eu`. Remember that a failing redirection on a special built-in such as `:` exits the shell; wrap probes in a subshell.

## Go toolchain and environment

`golib setup` downloads the pinned Go release from `https://go.dev/dl/`, checks its SHA-256 and unpacks it into `.tools/go/`. To move to another Go version, change the version and all checksums in both scripts together (`$GoVersion` and `$GoSha256` in `golib.ps1`, `go_version` and `go_sha256_*` in `golib.sh`). The checksums are listed at `https://go.dev/dl/?mode=json`.

Every `go` command that golib starts gets this environment, and so do the programs those commands run:

| Variable | Value | Why |
| --- | --- | --- |
| `GOROOT` | `.tools/go` | The pinned toolchain |
| `GOPATH`, `GOMODCACHE` | `.tools/gopath`, `.tools/gopath/pkg/mod` | Downloaded modules stay in the project |
| `GOCACHE` | `.tools/gocache` | The build cache stays in the project |
| `GOENV` | `off` | Ignore settings saved globally with `go env -w` |
| `GOTOOLCHAIN` | `local` | Never switch to another Go version |
| `CGO_ENABLED` | `0` | raylib-go in purego mode: no C compiler |
| `GOFLAGS` | `-tags=raylib_no_embed,ffi_no_embed` | Debug builds load raylib and libffi from files golib provides, instead of extracting embedded copies into the user's cache folder when a game starts. `golib dist` replaces these tags: see [Dist builds](#dist-builds) |
| `APPDATA` (Windows), `XDG_CONFIG_HOME` (Linux), `HOME` (macOS) | `.tools/config` (`.tools/home` on macOS) | Go writes telemetry counters to the user's config folder; this keeps them in the project. `golib run` restores the real value before starting the game. |

The scripts set these variables for themselves and the programs they start, except the Go program, which starts with the user's environment and sets them for each `go` command it runs. The three lists must stay the same: `Set-GoEnvironment` in `golib.ps1`, `set_go_environment` in `golib.sh` and `goEnv` in `tools/cli/project.go`.

The variables exist only while golib runs; nothing is saved. Running `.tools/go/bin/go` or `gofmt` directly skips them, and Go then writes caches and telemetry into your user folders. Use `golib go <args>` instead.

## raylib libraries

raylib-go loads the raylib shared library when a program starts, and calls it through libffi, using the [ffi](https://github.com/jupiterrider/ffi) module. Both modules can embed their library in the executable and extract it into the user's cache folder at startup. Debug builds turn that off with the `raylib_no_embed` and `ffi_no_embed` tags and load the libraries from files golib provides, and so do [dist builds](#dist-builds), except on macOS.

raylib-go's module ships prebuilt raylib libraries for Windows, Linux and macOS, on amd64 and arm64, in its `libs/` folder. The ffi module ships libffi for Windows amd64 and macOS in its `assets/libffi/` folder. golib copies the ones for this machine into `.tools/raylib/` during `setup`, `build` and `test`, writes the versions they came from to `.tools/raylib/VERSION` (the raylib-go version on the first line, then `ffi <version>`), and makes sure debug builds find them:

| Platform | Libraries | `build`, `run` and `shot` | `test` |
| --- | --- | --- | --- |
| Windows | `raylib.dll`, `libffi-8.dll` | Copied next to the `.exe` | `.tools/raylib/` added to `PATH` |
| Linux | `libraylib.so.6.0.0`; `libffi.so.8` comes from the system | Copied next to the executable; `run` and `shot` set `LD_LIBRARY_PATH` | `LD_LIBRARY_PATH` |
| macOS | `libraylib.6.0.0.dylib`, `libffi.8.dylib` | Copied next to the executable; `run` and `shot` set `DYLD_LIBRARY_PATH` | `DYLD_LIBRARY_PATH` |

A debug program that can't find a library stops as soon as it starts. On Windows it prints `cannot load library <name>: <reason>` for each library, says where they go, and exits with code 1 (see [Libraries that don't load](#libraries-that-dont-load)); on Linux and macOS raylib-go or ffi panic, with `cannot load library ...` for raylib or `error loading library` for libffi. That includes test binaries started with `golib go test`: use `golib test`.

The prebuilt Linux library links against `libX11.so.6` and loads `libGL.so.1` when a window opens, and raylib-go needs the system's `libffi.so.8`. Desktop Linux systems usually have all three; `golib doctor` checks for them and prints the install command when they are missing.

To update raylib-go, run `golib go -C framework get github.com/gen2brain/raylib-go/raylib@<version>`, then `golib go -C <module folder> mod tidy` for the framework and every game, then `golib setup` and `golib test`. If the new version brings a new raylib version, `golib test` asks for its [third-party notices](#third-party-licenses).

## Screenshots

`golib shot [game] [frame...] [--input "<script>"] [--save <file>] [--scale <n>]` builds a game and runs it with these environment variables set:

| Variable | Value |
| --- | --- |
| `GOLIB_SHOT_DIR` | `build/<game>/shots/`, emptied first |
| `GOLIB_SHOT_FRAMES` | The requested frames, ascending, separated by commas. Default: `60`, one second of game time. |
| `GOLIB_SHOT_INPUT` | The `--input` value, or unset |
| `GOLIB_SHOT_SAVE` | The whole path of the `--save` file, or unset |
| `GOLIB_SHOT_SCALE` | The `--scale` value, or unset for pictures at the screen's size |

`golib.Run` reads them, so games need no code for screenshots. It opens a hidden window, runs exactly one update per frame without waiting, draws each frame into an off-screen texture, runs the post-processing shaders over the requested frames and saves them as `frame-NNNNNN.png` (RGB, no alpha channel). Screenshots are always the screen's size, `Config.Width` by `Config.Height`, and ignore fullscreen. Then `Run` returns and the game exits. Frame N always shows the game after N updates, so the same code gives the same pictures on any machine, as long as the game bases its timing on `dt`.

The CLI stops a game that is still running after 120 seconds. It then prints an `[ok]` line for each saved file and a `[fail]` line for each missing one.

`--save <file.json>` gives the game data to start from, so a shot can open on level 8, with the music off, instead of playing all the way there. The file holds one saved value per name, exactly what `golib.SaveData` stores:

    {
      "progress": { "best": { "maps/level01.tmx": 14, "maps/level08.tmx": 88 }, "musicOff": true },
      "settings": { "Volume": 0.5 }
    }

The framework puts those values in memory before `main` runs, so a scene built in a package variable already finds them through `golib.LoadData`. The game still writes nothing to disk, and `golib.SaveData` replaces the seeded values in memory as usual. The path is relative to where you run `golib`, the project root, and names are the same lowercase letters, digits, `-` and `_` that `SaveData` takes. A file that isn't there, isn't a JSON object, or holds a name `SaveData` wouldn't take, stops the command. Write one by hand, or copy `build/<game>/save/<name>.json` from a debug build after playing to the state you want to see.

`--scale <n>`, from 1 to 8, enlarges every picture by whole numbers with nearest-neighbour sampling, the way the window enlarges pixel art, so a 320 by 180 game with `--scale 3` saves 960 by 540 pictures of exactly the pixels the game drew. Use it when a pixel art game is too small to judge: the game still runs at the screen's size, so the shots show the same thing, only bigger.

`--input` plays keyboard and mouse input, so shots can reach every scene. It takes items separated by spaces:

| Item | Effect |
| --- | --- |
| `Name@N` | Holds a key or mouse button down in update N only: one press or click |
| `Name@A-B` | Holds it down from update A to update B, both included |
| `Mouse@N:X,Y` | Moves the mouse pointer to pixel X, Y in update N; it stays there until the next move. Before the first move it is at 0, 0. |
| `MouseWheel@N:A` | Turns the mouse wheel by A notches in update N: up when positive, down when negative |
| `GamepadLeftStick@N:X,Y`, `GamepadRightStick@N:X,Y` | Tilts a stick of gamepad 0 to X, Y, each from -1 to 1, in update N; it stays there until the next tilt |

Names are the `golib.Key` constants without `Key` (`Enter`, `Escape`, `Space`, `Left`, `A`, `Zero`), the `golib.MouseButton` constants (`MouseLeft`, `MouseRight`, `MouseMiddle`) and the `golib.GamepadButton` constants (`GamepadA`, `GamepadStart`, `GamepadUp`), in any letter case. The game sees a press in the first update of each hold, exactly as `Input.KeyPressed`, `Input.MousePressed` and `Input.GamepadPressed` report real ones. Gamepad items act on gamepad 0, which is connected, with the name `golib shot`, whenever the script has a gamepad item; real devices are ignored. Frame N is drawn right after update N, so `golib shot 90 --input "Enter@1 Escape@60"` shows the game 30 updates after Escape went down, and `--input "Mouse@10:640,500 MouseLeft@11"` clicks at 640, 500. An invalid item makes the game exit with an error that lists the names. Quote the script in every shell.

Random numbers from `golib.RandomInt` and `golib.RandomFloat` start from the same seed in every shot, so the same command gives the same pictures. The framework picks that seed when the program starts, before `main`, by checking `GOLIB_SHOT_FRAMES`.

## Dist builds

`golib dist [game]` builds a game for players: a folder with the executable and the files it needs, and a zip of that folder to share. Every other command makes a debug build, except `golib run [game] --dist`, which makes this build and then runs it from its own folder, with the player's environment, so that what players get can be played before it is shared.

```text
build/<game>/dist/                  emptied first
  <game>/                           the folder players get
    <game>.exe                      the game, with its assets inside (no .exe on Linux and macOS)
    raylib.dll, libffi-8.dll        the libraries it loads when it starts (Windows; see below)
    THIRD-PARTY-LICENSES.txt        the licenses of the software and files in the game made by others
  <game>-<version>-<os>-<arch>.zip  the folder, zipped: the file to share
```

`<version>` comes from the game's `game.json`, and is `0.0.0` without one; `<os>` is `windows`, `linux` or `macos`, and `<arch>` is `amd64` or `arm64`. For example, `rocks-1.2.0-windows-amd64.zip` holds the `rocks/` folder, so unzipping it gives players one folder with everything in it. `dist` builds for the machine it runs on; there is no cross-compiling yet.

| | Debug build: `build`, `run`, `shot`, `test`, F5 | Dist build: `dist`, `run --dist` |
| --- | --- | --- |
| Output | `build/<game>/<game>.exe`, next to the libraries | `build/<game>/dist/<game>/<game>.exe`, next to the libraries and `THIRD-PARTY-LICENSES.txt`, and a zip of that folder |
| Console window (Windows) | Yes: raylib's warnings and Go's errors appear there, and nothing else, so it stays empty while all is well. Started from Explorer, the window closes when the game ends, so `golib.Run` also shows its error, or a panic in the game, in a message box | No. `golib.Run` shows its error, or a panic in the game, in a message box |
| Debug symbols and paths from this machine | Kept, for Delve and readable stack traces | Removed |
| raylib and libffi | Loaded from next to the executable | Loaded from next to the executable, except on macOS (below) |
| The game's `assets/` folder | Read from disk, in the working directory, which `run`, `shot`, `test` and F5 set to `games/<game>/`. When the working directory has no `assets/` folder, as when the executable is started from Explorer in `build/<game>/`, from `games/<game>/assets/` instead | Embedded, through the game's `assets.go` |
| `golib.SaveData` | Saves in `build/<game>/save/`, next to the executable; in memory under `shot` and `test` | Saves in the player's settings folder, in `GoLib games/<game>`: `%AppData%GoLib games<game>` on Windows, `~/.config/GoLib games/<game>` on Linux, `~/Library/Application Support/GoLib games/<game>` on macOS |

It runs `go build -trimpath` with these build tags and linker flags:

| Platform | Next to the executable | `-tags` | `-ldflags` |
| --- | --- | --- | --- |
| Windows | `raylib.dll`, and `libffi-8.dll` on amd64 | `golib_dist,raylib_no_embed,ffi_no_embed` | `-s -w -X golib.saveName=<game> -H=windowsgui` |
| Linux | `libraylib.so.6.0.0`. Players' systems provide `libffi.so.8`, `libX11.so.6` and `libGL.so.1` | `golib_dist,raylib_no_embed,ffi_no_embed` | `-s -w -X golib.saveName=<game> -r $ORIGIN` |
| macOS | Nothing: the executable carries both libraries | `golib_dist` | `-s -w -X golib.saveName=<game>` |

`-X golib.saveName=<game>` names the folder `golib.SaveData` saves in after the game's folder in `games/`, so it stays the same when a player renames the executable.

`-tags` replaces the tags in `GOFLAGS`. `golib_dist` switches the framework to dist behavior and includes the game's `assets.go`, which embeds its assets folder:

```go
//go:build golib_dist

package main

import (
	"embed"

	"golib"
)

//go:embed all:assets
var assets embed.FS

func init() { golib.EmbedAssets(assets) }
```

Debug builds leave that file out, so they never embed assets. Before building, `dist` checks with `go list` that a game with an `assets/` folder embeds `assets` or `all:assets`, and stops with a `[fail]` line otherwise: without it the executable would build and then fail on the player's machine.

The libraries are the ones debug builds use: raylib from the archive for this platform in raylib-go's `libs/` folder, and libffi from the ffi module's `assets/libffi/` folder. `raylib_no_embed` and `ffi_no_embed` keep the executable from carrying its own copies, which raylib-go and ffi would otherwise write into the player's cache folder when the game first starts, and never check again: a damaged copy there would stop the game until someone deleted that folder, with no message (see [roadmap.md](roadmap.md#decisions)). Instead, the game loads the files next to it, and writes nothing on the player's machine but what it saves with `golib.SaveData`:

- Windows looks for a library in the executable's folder first.
- Linux's dynamic linker looks there because `-r $ORIGIN` writes that folder into the executable's `DT_RUNPATH`. Not tried on Linux yet.
- macOS looks for a library that is given by its bare name, as raylib-go gives raylib's, only in `DYLD_LIBRARY_PATH`, the working directory and system folders, never in the executable's folder. So macOS dist builds still carry both libraries, and write them to `~/Library/Caches/github.com/` when the game first starts.

Players have to keep the folder together, which unzipping it does. Windows also looks for libraries in the folders on `PATH`, so a `libffi-8.dll` from another program, such as MSYS2's, can hide a missing one on your machine: try the game from the unzipped folder.

#### Libraries that don't load

raylib-go and ffi load their libraries while Go initializes their packages, before `main` and `golib.Run`, and panic when they can't. A Windows dist build has no console, so a player who moved the executable out of its folder would see nothing happen. So on Windows, the framework's `golib/internal/startup` package loads `libffi-8.dll` and `raylib.dll` first, the same way. When one doesn't load, it writes `cannot load library <name>: <reason>` for each to stderr, with a line that says where the libraries go, shows the same in a message box when nobody would see the console (a dist build, or a debug build started from Explorer), and exits with code 1.

It can only run first because Go initializes packages in the order of their import paths, each as soon as the packages it imports are ready: `golib/internal/startup` comes right after `syscall`, before `os`, and raylib-go and ffi need `os`. So the package imports only `syscall` and `unsafe` (even `strings` would hold it back, through `unicode`), which its test checks; another test in the framework starts a test binary where the libraries can't be found and checks the message. The check runs only in builds with the `raylib_no_embed` and `ffi_no_embed` tags, which all of golib's Windows builds have: without them, the libraries come from inside the executable. A library that loads but isn't the one the game was built for, such as a `raylib.dll` of another raylib version, still makes raylib-go panic with no message in a dist build.

### Third-party licenses

The licenses of Go, jfxr, purego (Apache-2.0), ffi and libffi ask for their notices to go with the programs built from them. `THIRD-PARTY-LICENSES.txt` holds them, each under a heading that says what it is and where it is in the game:

| Heading | License text from |
| --- | --- |
| Go | `.tools/go/LICENSE`: the Go runtime and standard library are in every executable |
| jfxr | `framework/LICENSE-jfxr.txt` (BSD-3-Clause): GoLib's framework carries a Go version of jfxr's synthesizer, `framework/jfxr.go`, in every executable. Without the file, `dist` prints a `[warn]` line and leaves the heading empty |
| Each Go module the game is built from, as `go list -deps` reports with the dist build tags, except the game's own and GoLib's | The files in the module's folder whose names start with `LICENSE`, `LICENCE`, `COPYING`, `COPYRIGHT` or `NOTICE`. A module without one gets a `[warn]` line: find its license and add its notice by hand |
| raylib, and libffi when the platform has it | `libs/LICENSE` in raylib-go, `assets/libffi/LICENSE` in ffi. For raylib, also `tools/cli/notices/raylib-<version>.txt` (below) |
| `assets/ATTRIBUTION.md`, when the game has one | The file itself: where the files in the assets folder that weren't made for the game come from, and their licenses (see [framework/README.md](../framework/README.md)) |

GoLib is left out because its license, zlib, asks for nothing in games (see [roadmap.md](roadmap.md#decisions)); jfxr's code inside it is the exception. The file starts with the game's title from `game.json`. `dist` writes it again on every build, so don't edit it: put what it should say in `assets/ATTRIBUTION.md`.

raylib's library includes other libraries, and raylib's `LICENSE` covers none of them. Most of them (GLFW, miniaudio, dr_libs, stb, jar_xm, jar_mod, sinfl, sdefl, rprand, rl_gputex) are under zlib, public domain, MIT-0 or a choice of public domain, and ask for no notice in programs. The others ask for one: cgltf, tinyobj_loader_c, vox_loader, m3d, par_shapes, QOI and QOA (MIT), glad's `khrplatform.h` (Khronos), and dirent, in the Windows library only. Their notices are written out in `tools/cli/notices/raylib-<version>.txt`, which `dist` embeds and adds under raylib's heading. The texts sit in comments in each library's header, in different forms, so they are copied by hand rather than extracted.

When raylib-go moves to a new raylib version, `golib test` fails until that file exists for it. To write it, copy the previous one, then check it against raylib-go's `external/` folder and `config.h`: which headers the C files include with the default settings, what each header's license says, and whether the prebuilt libraries contain them (search them for strings such as `KHR_materials_emissive_strength` for cgltf or `mtllib` for tinyobj_loader_c). `dist` prints a `[warn]` line, and leaves those notices out, while the file is missing.

### Icon and version information (Windows)

On Windows, the executable carries the game's icon, which Explorer, the title bar and the taskbar show, and the details that Explorer lists under Properties > Details and Task Manager uses as the program's name, in dist builds and in the debug builds of `build`, `run` and `shot` (not in F5's, which the Go extension builds). They come from two optional files in the game's folder:

| File | Holds | Without it |
| --- | --- | --- |
| `icon.png` | The icon: a square PNG, ideally 256 by 256 pixels, transparent around the shape. Pixel art can be smaller, down to 16 by 16. | Windows' default program icon |
| `game.json` | The title, version and author, below. `golib new` writes one. | The folder name and version 0.0.0 |

Every field of `game.json` is optional. Save it as UTF-8; the byte order mark that Windows PowerShell 5.1 writes is fine.

| Field | Example | Becomes |
| --- | --- | --- |
| `title` | `"Rocks in Space"` | The file description, which Task Manager shows, and the product name. Default: the folder name. Keep it the same as `Config.Title`. |
| `version` | `"1.2.0"`, `"1.2.0-beta"` | The file and product version: major.minor.patch, each from 0 to 65535, optionally followed by a label after `-`, which marks a pre-release. Default: `0.0.0`. |
| `author` | `"Ada Lovelace"` | The company name |
| `copyright` | `"Copyright 2026 Ada Lovelace"` | The copyright |

```json
{
  "title": "Rocks in Space",
  "version": "1.2.0",
  "author": "Ada Lovelace"
}
```

A mistake in either file stops the build with a `[fail]` line that says what to fix: invalid JSON (with its line), an unknown field, a version that isn't major.minor.patch, an icon that isn't a square PNG of at least 16 pixels. `dist` reports what it found, and checks both files on Linux and macOS too, so a mistake shows up wherever the game is built; debug builds say nothing while both files are fine, and don't check them on Linux and macOS.

How it works, in `tools/cli` (`dist.go`, `gameinfo.go`, `icon.go` and `winres.go`): golib resizes the icon to 16, 20, 24, 32, 40, 48, 64 and 256 pixels, averaging pixels to shrink and repeating them to grow, so pixel art stays sharp. It writes those images and the version information as Windows resources into a `.syso` file, the object file format the Go linker reads. The linker only picks up `.syso` files from the package's own folder, and `go build -overlay` doesn't cover them, so golib puts the file in the game's folder as `golib_windows_<arch>.syso` while it builds, then deletes it, even when the build fails. Debug and dist builds use the same name, so a file left by an interrupted build is replaced rather than linked twice, and `.gitignore` lists it. The same resources give the same file, so an unchanged game isn't linked again. The icon resource is named `GLFW_ICON`: GLFW, the library raylib opens windows with, gives an icon with that name to the game's window.

Dist builds on Linux and macOS don't use the two files yet.

## New games

`golib new <name>` creates `games/<name>/` from the files in `tools/template/game/`:

1. It checks the name: 1 to 32 lowercase letters, digits, `-` and `_`, starting with a letter. It refuses `golib`, which would clash with the framework's import path, names Windows reserves for devices, such as `con`, and folders that already exist in `games/`.
2. It copies every `*.tmpl` file without its `.tmpl` suffix, replacing `{{name}}` with the name, `{{go}}` with the pinned Go version and `{{date}}` with today's date. The suffix stops Go and gopls from treating the templates as a module of their own.
3. It copies `framework/go.sum`, so `go mod tidy` finds the checksums it needs, then runs `go mod tidy` in the new folder.

If `go mod tidy` fails, `new` deletes the folder again, so it can simply run again. To change what new games start with, edit the templates, then try them with `golib new` on a throwaway name and `golib test`.

## PowerShell argument splitting

Windows PowerShell 5.1 splits arguments that start with `-` and contain a dot before they reach a native program. `.\golib go -C games/platformer mod edit -replace=golib=../../framework` arrives with `-replace=golib=` and `../../framework` as separate arguments. Quote such arguments: `'-replace=golib=../../framework'`. cmd and Git Bash pass them unchanged.

## Folders owned by the tooling

| Folder | Contents |
| --- | --- |
| `.tools/go/` | The Go toolchain |
| `.tools/gopath/` | Downloaded Go modules. Go makes them read-only; `clean --all` handles that. |
| `.tools/gocache/` | Go's build cache |
| `.tools/raylib/` | The raylib library, extracted from raylib-go; libffi, copied from the ffi module on Windows amd64 and macOS; and a `VERSION` file naming both versions |
| `.tools/config/`, `.tools/home/` | Go's telemetry counters (Windows and Linux; macOS) |
| `.tools/gotools/` | Tools the VS Code Go extension installs, such as gopls |
| `build/<game>/` | A game's debug executable, next to its copies of the raylib libraries |
| `build/<game>/shots/` | Screenshots from the latest `golib shot` |
| `build/<game>/dist/` | The latest `golib dist` build: the folder to share, `<game>/`, and its zip |
| `build/golib/` | [The Go program](#the-go-program), `golib.exe` (`golib` on Linux and macOS), built by the scripts when a command needs it, and on Windows `golib-2.exe` and so on, built while `golib.exe` is running. `new` refuses `golib` as a game name, so no game's folder clashes with it. |

`.tools/` and `build/` are git-ignored. `golib setup` creates `.tools/` and `golib clean --all` removes it; `golib build`, `run`, `shot` and `dist` create `build/` and `golib clean` removes it. `.tools/downloads/` only exists while setup is downloading.

`.tools/` stays visible in the VS Code Explorer on purpose: nothing is hidden from the user. It is only excluded from search and file watching, for speed.

## Web builds

`golib web [game]` builds a game for the browser and serves it on this machine, so it can be played at a `http://localhost` address. A web build draws, sounds, reads input, runs post-processing shaders, reads fonts from files and saves in the browser's store. It cannot play `.xm`, `.mod` or `.qoa`, which browsers do not decode, and text from a font file lands within a few pixels of where the desktop puts it, not on it (see [roadmap.md](roadmap.md#web-build-started-2026-09-18)).

```text
build/<game>/web/
  <game>.wasm     the game, built with GOOS=js GOARCH=wasm, with its assets inside
  index.html      the page: a canvas that fills the window, and the three files below
  wasm_exec.js    Go's own loader, copied from .tools/go/lib/wasm/
  golib.js        the other half of the web backend, copied from framework/internal/device/web.js
```

`--port <n>` chooses the port (8080 by default, and 0 picks a free one), and `--no-open` leaves the browser closed, for a machine with none. The server sends `.wasm` files as `application/wasm`, which browsers insist on, and asks for nothing to be cached, so a rebuild shows on the next reload.

A web build carries its assets inside, as a dist build does, so a game needs `assets.go` (see `golib.EmbedAssets`) and `ReadAsset` works unchanged, with no loading screen. It is built with the `golib_dist` tag for that reason.

The browser decides when to draw: the game waits for `requestAnimationFrame` in `device.EndFrame`, which is what paces it, and the fixed-step loop is the same one the desktop runs. Drawing doesn't cross into JavaScript one shape at a time: package golib writes its shapes into a buffer of numbers that `web.js` reads once a frame, and the keyboard, the mouse and the gamepads come back the same way, because a call per key would cost more than the game.

Sound goes through Web Audio. The synthesizers above the backend already turn a `SoundSpec`, a `.jfxr` file or a tune into the bytes of a WAV file, so the backend only hands those bytes to the browser and plays what comes back. The browser decodes them asynchronously while the contract is synchronous, so the first play of a sound waits for it: when every goroutine waits, Go hands the thread back to the page, which finishes the decoding and wakes the game. Browsers refuse to make a sound before the player has pressed a key or clicked, so the first of either starts the sound device.

Post-processing shaders are compiled for OpenGL ES, which is what browsers have: `device.ESShader` replaces the first lines of a game's `#version 330` shader and leaves the rest alone, so a shader that mixes whole numbers into float arithmetic, which ES refuses, still fails and says so on the page. `golib.SaveData` writes into the browser's own store for the address the game is served from, which survives the page being closed, and which a player clears with their browsing data.

`golib shot [game] [frame...] --web` checks a web build the way `golib shot` checks a desktop one. It serves the game, opens it in Microsoft Edge, Google Chrome or Chromium with no window and a profile of its own, and writes into `build/<game>/shots-web/` the pictures the page posts back to it, because a page cannot write files. The frames, the input script and the scale travel in the address, and the page puts them where a game on the desktop finds them in its environment, so the framework reads them with `os.Getenv` either way. A game that stops says why back to the terminal. `--save` doesn't work there yet: a page cannot read a file from this machine.

A browser's own `--screenshot` is not used, and shouldn't be: it needs a fresh profile for every run and gives up on a page that takes more than about four seconds, which a game loading its assets often does.

The two backends draw the same picture. Measured by taking the same frames both ways: `games/platformer`, `games/tetris` and `games/crates` come out byte for byte identical, sprites, tilemaps, cameras, text and blending included. Games with post-processing shaders differ by one or two levels of 255 on many pixels, and by more than eight on 0.01% of them, because GLSL arithmetic is not required to give the same answer on two graphics stacks.

Nothing is published by `golib web`: it serves on `127.0.0.1` for the person running it. To share a game, `golib dist [game] --web` builds it into `build/<game>/dist/web/` and zips what is in that folder:

```text
build/<game>/dist/
  web/                          the folder to serve
    index.html                  at the top of the zip, where a page host looks for it
    <game>.wasm, wasm_exec.js, golib.js
    THIRD-PARTY-LICENSES.txt    Go's license and jfxr's; a web build has no raylib in it
  <game>-<version>-web.zip      the file to upload to itch.io
```

## Adding a command

1. Write it in [the Go program](#the-go-program), `tools/cli`, and add it to `commands` in `main.go`. Only what has to work without Go goes in the scripts.
2. Register it in both dispatchers, as `Invoke-Cli '<name>' $options` in `golib.ps1` and `run_cli <name> "$@"` in `golib.sh`, and in both help texts.
3. If people run it often, add a task to `.vscode/tasks.json` and a button to `$Actions` in `tools/ui/golib-ui.ps1`.
4. Document it in the Commands tables of `AGENTS.md` and `README.md`.
5. Test on Windows (PowerShell, cmd and Git Bash) and on Linux or macOS.

`.claude/settings.json` already allows every `golib` subcommand, so no permission change is needed.

## GoLib window

`tools/ui/golib-ui.ps1` is a window with a button for each command, for people who would rather click than type. `golib-ui.cmd` starts it. It is Windows only: it uses WPF and Windows PowerShell 5.1, which come with Windows 10 and 11, so it needs nothing installed. Linux and macOS use `./golib <command>` or the VS Code tasks.

- **No build logic.** Each command button starts `golib.ps1` with the same arguments as typing `.\golib <command>`, shows its output as it arrives, and reports the exit code. Stop ends the command's whole process tree, including a game started by Run debug or Run dist.
- **Buttons come from the `$Actions` table** at the top of the script. `Command` is golib's arguments, where `{game}` is the game picked in the list and `{frames}` the frame numbers in the box; `Folder` opens a folder in Explorer instead; `Confirm` asks before running. A new CLI command usually needs one line there.
- **One command at a time.** While a command runs, the other command buttons are disabled, so two commands never write to `.tools/raylib/` or `build/` together.
- **Same rules as `golib.ps1`:** ASCII only, no PowerShell 7-only syntax, `Set-StrictMode -Version 3.0`.
- **Output without threads.** PowerShell script blocks can't run as callbacks on other threads, so the window reads the command's output with `ReadLineAsync` and collects the lines from a `DispatcherTimer` on its own thread.

## Line endings and file modes

- `.gitattributes` stores every text file with LF, except `*.cmd` and `*.bat`, which use CRLF because cmd.exe misparses LF batch files.
- `golib` must be committed as executable. From Windows, set the bit once with `git update-index --chmod=+x golib`.
- If the execute bit gets lost, for example after extracting a ZIP download, `sh golib <command>` works too.

## VS Code integration

| File | Provides |
| --- | --- |
| `.vscode/tasks.json` | "GoLib: ..." tasks that call the CLI (Terminal > Run Task...). "GoLib: run" is the default build task (Ctrl+Shift+B) and "GoLib: test" the default test task. Windows tasks run `cmd.exe /d /c .\golib.cmd`, so they work whatever the default terminal shell is. |
| `.vscode/settings.json` | Line endings and formatting defaults; `.tools/` and `build/` excluded from search and file watching. The Go extension uses `.tools/go` (`go.goroot`) with the CLI's environment (`go.toolsEnvVars`) and installs its tools, gopls and Delve, into `.tools/gotools/`. Tool update checks are off (`go.toolsManagement.checkForUpdates`), because they run `go` outside that environment on every debug session. |
| `.vscode/launch.json` | "GoLib: debug game" (F5). It asks for a game folder name, has Delve build the game into `build/<game>/`, and runs it from `games/<game>/` with `.tools/raylib/` on the library search path. The `raylib_no_embed` and `ffi_no_embed` tags come from `GOFLAGS` in `go.toolsEnvVars`: the configuration sets no `buildFlags`, because whenever `buildFlags` is set the extension runs `go` outside the project environment to inspect Delve. The game sees the same redirected config folder as the tools. The Go extension offers to install Delve the first time. |
| `.vscode/extensions.json` | Recommended extensions: Go, EditorConfig, Claude Code. |

The Go extension looks for Go only when it starts. If it started before `golib setup` installed Go, for example because a `.go` file was open, it fails to activate and stays that way: run **Developer: Reload Window** after setup.

Known limitation: every time the Go extension starts, and when you run **Go: Locate Configured Go Tools**, it runs `go version -m` on its installed tools without `go.toolsEnvVars`. Go then updates its local telemetry counters in the user's config folder (`%APPDATA%\go\telemetry` on Windows). No setting redirects those calls. The counters stay on the machine unless the user opts in to uploading them with `go telemetry on`. On macOS every tool the extension starts writes there, because the redirect relies on `HOME`, which the settings leave alone.

## Planned architecture

The shell scripts are shrinking to bootstrappers: provision the pinned Go toolchain into `.tools/`, then hand over to [the Go program](#the-go-program), which implements every command once, for every platform. So far it has `dist`; the other commands live in the twin scripts until they move.
