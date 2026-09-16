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

The command logic is moving from two twin scripts into one Go program, a command at a time (see [roadmap.md](roadmap.md#decisions)):

| Part | Commands |
| --- | --- |
| `tools/bootstrap/golib.ps1` (Windows PowerShell 5.1 and PowerShell 7) and `tools/bootstrap/golib.sh` (POSIX sh), twins with the same commands, options, output and exit codes | `help`, `setup`, `doctor`, `new`, `build`, `run`, `shot`, `test`, `go`, `clean`; they also build and start the Go program |
| `tools/cli/`, the Go program | `dist` |

The scripts will keep what has to work without Go, or while the Go program isn't running: downloading and checking Go in `setup`, building and starting the Go program, `clean` (on Windows a running program can't delete its own folder), and the checks of `doctor` that don't need Go.

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

The program finds the project from its own location, two folders above `build/golib/`, and sets the project environment for each `go` command it runs (`goEnv` in `tools/cli/project.go`), so the programs it starts otherwise get the user's environment. `golib test` vets and tests it; its tests replace the `go` command with a fake one that records its arguments.

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

raylib-go loads the raylib shared library when a program starts, and calls it through libffi, using the [ffi](https://github.com/jupiterrider/ffi) module. Both modules can embed their library in the executable and extract it into the user's cache folder at startup. Debug builds turn that off with the `raylib_no_embed` and `ffi_no_embed` tags and load the libraries from files golib provides; [dist builds](#dist-builds) keep the embedded copies.

raylib-go's module ships prebuilt raylib libraries for Windows, Linux and macOS, on amd64 and arm64, in its `libs/` folder. The ffi module ships libffi for Windows amd64 and macOS in its `assets/libffi/` folder. golib copies the ones for this machine into `.tools/raylib/` during `setup`, `build` and `test`, writes the versions they came from to `.tools/raylib/VERSION` (the raylib-go version on the first line, then `ffi <version>`), and makes sure debug builds find them:

| Platform | Libraries | `build`, `run` and `shot` | `test` |
| --- | --- | --- | --- |
| Windows | `raylib.dll`, `libffi-8.dll` | Copied next to the `.exe` | `.tools/raylib/` added to `PATH` |
| Linux | `libraylib.so.6.0.0`; `libffi.so.8` comes from the system | Copied next to the executable; `run` and `shot` set `LD_LIBRARY_PATH` | `LD_LIBRARY_PATH` |
| macOS | `libraylib.6.0.0.dylib`, `libffi.8.dylib` | Copied next to the executable; `run` and `shot` set `DYLD_LIBRARY_PATH` | `DYLD_LIBRARY_PATH` |

A debug program that can't find a library stops as soon as it starts, with `cannot load library ...` for raylib or `error loading library` for libffi. That includes test binaries started with `golib go test`: use `golib test`.

The prebuilt Linux library links against `libX11.so.6` and loads `libGL.so.1` when a window opens, and raylib-go needs the system's `libffi.so.8`. Desktop Linux systems usually have all three; `golib doctor` checks for them and prints the install command when they are missing.

To update raylib-go, run `golib go -C framework get github.com/gen2brain/raylib-go/raylib@<version>`, then `golib go -C <module folder> mod tidy` for the framework and every game, then `golib setup`.

## Screenshots

`golib shot [game] [frame...] [--input "<script>"]` builds a game and runs it with these environment variables set:

| Variable | Value |
| --- | --- |
| `GOLIB_SHOT_DIR` | `build/<game>/shots/`, emptied first |
| `GOLIB_SHOT_FRAMES` | The requested frames, ascending, separated by commas. Default: `60`, one second of game time. |
| `GOLIB_SHOT_INPUT` | The `--input` value, or unset |

`golib.Run` reads them, so games need no code for screenshots. It opens a hidden window, runs exactly one update per frame without waiting, draws each frame into an off-screen texture, runs the post-processing shaders over the requested frames and saves them as `frame-NNNNNN.png` (RGB, no alpha channel). Screenshots are always the screen's size, `Config.Width` by `Config.Height`, and ignore fullscreen. Then `Run` returns and the game exits. Frame N always shows the game after N updates, so the same code gives the same pictures on any machine, as long as the game bases its timing on `dt`.

The CLI stops a game that is still running after 120 seconds. It then prints an `[ok]` line for each saved file and a `[fail]` line for each missing one.

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

`golib dist [game]` builds a game for players: one executable to share, with nothing next to it. Every other command makes a debug build.

| | Debug build: `build`, `run`, `shot`, `test`, F5 | Dist build: `dist` |
| --- | --- | --- |
| Output | `build/<game>/<game>.exe`, next to the libraries | `build/<game>/dist/<game>.exe`, alone (no `.exe` on Linux and macOS) |
| Console window (Windows) | Yes: raylib's warnings and Go's errors appear there, and nothing else, so it stays empty while all is well. Started from Explorer, the window closes when the game ends, so `golib.Run` also shows its error, or a panic in the game, in a message box | No. `golib.Run` shows its error, or a panic in the game, in a message box |
| Debug symbols and paths from this machine | Kept, for Delve and readable stack traces | Removed |
| raylib and libffi | Loaded from next to the executable | Embedded; written to the player's cache folder the first time the game starts |
| The game's `assets/` folder | Read from disk, in the working directory, which `run`, `shot`, `test` and F5 set to `games/<game>/`. When the working directory has no `assets/` folder, as when the executable is started from Explorer in `build/<game>/`, from `games/<game>/assets/` instead | Embedded, through the game's `assets.go` |

It runs `go build -trimpath -tags=golib_dist -ldflags="-s -w -H=windowsgui"`, without `-H=windowsgui` outside Windows, and writes into `build/<game>/dist/`, emptied first. `-tags=golib_dist` replaces the tags in `GOFLAGS`, so raylib-go and ffi embed their libraries again. The same tag switches the framework to dist behavior and includes the game's `assets.go`, which embeds its assets folder:

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

Debug builds leave that file out, so they never embed assets. Before building, `dist` checks with `go list` that a game with an `assets/` folder embeds `assets` or `all:assets`, and stops with a `[fail]` line otherwise: without it the executable would build and then fail on the player's machine. `dist` builds for the machine it runs on; there is no cross-compiling yet.

When a dist build starts, raylib-go and ffi write their libraries to the user's cache folder, in folders they name: `%LOCALAPPDATA%\github.com\gen2brain\raylib-go\<raylib version>\` and `%LOCALAPPDATA%\github.com\jupiterrider\ffi\libffi\<libffi version>\` on Windows, under `~/.cache/` on Linux and `~/Library/Caches/` on macOS. They write each file only when it is missing and never check it afterwards, so a damaged copy stops the game from starting until that folder is deleted. The libraries load before `golib.Run` starts, so the player sees no message. This is why `dist` is going to put the libraries next to the executable instead (see [roadmap.md](roadmap.md#decisions)). On Linux, players also need `libX11.so.6`, `libGL.so.1` and `libffi.so.8`.

### Icon and version information (Windows)

On Windows, a dist build carries the game's icon, which Explorer, the title bar and the taskbar show, and the details that Explorer lists under Properties > Details and Task Manager uses as the program's name. They come from two optional files in the game's folder:

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

A mistake in either file stops `dist` with a `[fail]` line that says what to fix: invalid JSON (with its line), an unknown field, a version that isn't major.minor.patch, an icon that isn't a square PNG of at least 16 pixels. `dist` checks both files on Linux and macOS too, so a mistake shows up wherever the game is built.

How it works, in `tools/cli` (`dist.go`, `gameinfo.go`, `icon.go` and `winres.go`): `dist` resizes the icon to 16, 20, 24, 32, 40, 48, 64 and 256 pixels, averaging pixels to shrink and repeating them to grow, so pixel art stays sharp. It writes those images and the version information as Windows resources into a `.syso` file, the object file format the Go linker reads. The linker only picks up `.syso` files from the package's own folder, and `go build -overlay` doesn't cover them, so `dist` puts the file in the game's folder as `golib_dist_windows_<arch>.syso` while it builds, then deletes it, even when the build fails. `.gitignore` lists that name, in case a build is interrupted. The icon resource is named `GLFW_ICON`: GLFW, the library raylib opens windows with, gives an icon with that name to the game's window.

Debug builds don't carry the icon or the details, and dist builds on Linux and macOS don't use the two files yet.

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
| `build/<game>/dist/` | The latest `golib dist` build: the game as a single file |
| `build/golib/` | [The Go program](#the-go-program), `golib.exe` (`golib` on Linux and macOS), built by the scripts when a command needs it. `new` refuses `golib` as a game name, so no game's folder clashes with it. |

`.tools/` and `build/` are git-ignored. `golib setup` creates `.tools/` and `golib clean --all` removes it; `golib build`, `run`, `shot` and `dist` create `build/` and `golib clean` removes it. `.tools/downloads/` only exists while setup is downloading.

`.tools/` stays visible in the VS Code Explorer on purpose: nothing is hidden from the user. It is only excluded from search and file watching, for speed.

## Adding a command

1. Write it in [the Go program](#the-go-program), `tools/cli`, and add it to `commands` in `main.go`. Only what has to work without Go goes in the scripts.
2. Register it in both dispatchers, as `Invoke-Cli '<name>' $options` in `golib.ps1` and `run_cli <name> "$@"` in `golib.sh`, and in both help texts.
3. If people run it often, add a task to `.vscode/tasks.json` and a button to `$Actions` in `tools/ui/golib-ui.ps1`.
4. Document it in the Commands tables of `AGENTS.md` and `README.md`.
5. Test on Windows (PowerShell, cmd and Git Bash) and on Linux or macOS.

`.claude/settings.json` already allows every `golib` subcommand, so no permission change is needed.

## GoLib window

`tools/ui/golib-ui.ps1` is a window with a button for each command, for people who would rather click than type. `golib-ui.cmd` starts it. It is Windows only: it uses WPF and Windows PowerShell 5.1, which come with Windows 10 and 11, so it needs nothing installed. Linux and macOS use `./golib <command>` or the VS Code tasks.

- **No build logic.** Each command button starts `golib.ps1` with the same arguments as typing `.\golib <command>`, shows its output as it arrives, and reports the exit code. Stop ends the command's whole process tree, including a game started by Run.
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
