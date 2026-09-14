# Tooling

How the `golib` command works, where its files live, and how to change it.

## Entry points

| File | Started from | What it does |
| --- | --- | --- |
| `golib.cmd` | cmd, PowerShell | Runs `tools/bootstrap/golib.ps1`. `-ExecutionPolicy Bypass` applies to that single run; no system setting changes. |
| `golib` | sh, bash, zsh on Linux and macOS; Git Bash and MSYS2 on Windows | Runs `tools/bootstrap/golib.sh`. On Windows POSIX shells it delegates to `golib.ps1`, so Windows always uses one implementation. |

PowerShell and cmd both resolve `.\golib` to `golib.cmd`, so the same word works in every shell; only the prefix changes (`.\` on Windows, `./` elsewhere). Gradle's `gradlew` and `gradlew.bat` use the same pattern.

Always write the explicit prefix, in docs, tasks and scripts. Windows can be configured not to run programs from the current folder by bare name (the `NoDefaultCurrentDirectoryInExePath` environment variable, which some agent environments set). An explicit relative path always works.

Both shims find the project root from their own location, so they also work when called with a full path from another folder.

## Implementations

`tools/bootstrap/golib.ps1` (Windows PowerShell 5.1 and PowerShell 7) and `tools/bootstrap/golib.sh` (POSIX sh) are twins: same commands, options, output format and exit codes.

Rules for both:

- Change both in the same commit, and test both.
- Check output: one fact per line, prefixed `[ok]`, `[info]`, `[warn]` or `[fail]`, then a summary line such as `doctor: 0 failed, 1 warning(s)`. Plain ASCII, no colors: the reader is often an agent parsing logs.
- Exit codes: `0` success, `1` failure, `2` usage error. Usage errors go to stderr.
- Never read or write anything outside the project folder, except downloading from pinned URLs. Every `go` command runs with the project environment described below.
- `setup` must be idempotent: running it twice is safe and fast.
- `doctor` only reads: it inspects files and never starts `go`.

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
| `GOFLAGS` | `-tags=raylib_no_embed` | Stop raylib-go from extracting its library into the user's cache folder when a game starts |
| `APPDATA` (Windows), `XDG_CONFIG_HOME` (Linux), `HOME` (macOS) | `.tools/config` (`.tools/home` on macOS) | Go writes telemetry counters to the user's config folder; this keeps them in the project. `golib run` restores the real value before starting the game. |

The variables exist only while golib runs; nothing is saved. Running `.tools/go/bin/go` or `gofmt` directly skips them, and Go then writes caches and telemetry into your user folders. Use `golib go <args>` instead.

## raylib library

raylib-go loads the raylib shared library when a program starts. Its module ships prebuilt libraries for Windows, Linux and macOS, on amd64 and arm64, in its `libs/` folder. golib extracts the one for this machine into `.tools/raylib/<raylib-go version>/` during `setup`, `build` and `test`, and makes sure programs find it:

| Platform | Library | `build` and `run` | `test` |
| --- | --- | --- | --- |
| Windows | `raylib.dll` | Copied next to the `.exe` | Added to `PATH` |
| Linux | `libraylib.so.6.0.0` | Copied next to the executable; `run` sets `LD_LIBRARY_PATH` | `LD_LIBRARY_PATH` |
| macOS | `libraylib.6.0.0.dylib` | Copied next to the executable; `run` sets `DYLD_LIBRARY_PATH` | `DYLD_LIBRARY_PATH` |

A program that can't find the library stops as soon as it starts, with `cannot load library ...`. That includes test binaries started with `golib go test`: use `golib test`.

The prebuilt Linux library links against `libX11.so.6` and loads `libGL.so.1` when a window opens. Desktop Linux systems have both; `golib doctor` checks for them and prints the install command when they are missing.

To update raylib-go, run `golib go -C framework get github.com/gen2brain/raylib-go/raylib@<version>`, then `golib go -C <module folder> mod tidy` for the framework and every game, then `golib setup`.

## PowerShell argument splitting

Windows PowerShell 5.1 splits arguments that start with `-` and contain a dot before they reach a native program. `.\golib go -C games/hello mod edit -replace=golib=../../framework` arrives with `-replace=golib=` and `../../framework` as separate arguments. Quote such arguments: `'-replace=golib=../../framework'`. cmd and Git Bash pass them unchanged.

## Folders owned by the tooling

| Folder | Contents |
| --- | --- |
| `.tools/go/` | The Go toolchain |
| `.tools/gopath/` | Downloaded Go modules. Go makes them read-only; `clean --all` handles that. |
| `.tools/gocache/` | Go's build cache |
| `.tools/raylib/<version>/` | The raylib library, extracted from raylib-go |
| `.tools/config/`, `.tools/home/` | Go's telemetry counters (Windows and Linux; macOS) |
| `.tools/gotools/` | Tools the VS Code Go extension installs, such as gopls |
| `build/<game>/` | A game's executable, next to its copy of the raylib library |

`.tools/` and `build/` are git-ignored. `golib setup` creates `.tools/` and `golib clean --all` removes it; `golib build` and `run` create `build/` and `golib clean` removes it. `.tools/downloads/` only exists while setup is downloading.

`.tools/` stays visible in the VS Code Explorer on purpose: nothing is hidden from the user. It is only excluded from search and file watching, for speed.

## Adding a command

1. Add `cmd_<name>` to `golib.sh` and `Invoke-<Name>` to `golib.ps1`.
2. Register it in both dispatchers and both help texts.
3. If people run it often, add a task to `.vscode/tasks.json`.
4. Document it in the Commands tables of `AGENTS.md` and `README.md`.
5. Test on Windows (PowerShell, cmd and Git Bash) and on Linux or macOS.

`.claude/settings.json` already allows every `golib` subcommand, so no permission change is needed.

## Line endings and file modes

- `.gitattributes` stores every text file with LF, except `*.cmd` and `*.bat`, which use CRLF because cmd.exe misparses LF batch files.
- `golib` must be committed as executable. From Windows, set the bit once with `git update-index --chmod=+x golib`.
- If the execute bit gets lost, for example after extracting a ZIP download, `sh golib <command>` works too.

## VS Code integration

| File | Provides |
| --- | --- |
| `.vscode/tasks.json` | "GoLib: ..." tasks that call the CLI (Terminal > Run Task...). "GoLib: run" is the default build task (Ctrl+Shift+B) and "GoLib: test" the default test task. Windows tasks run `cmd.exe /d /c .\golib.cmd`, so they work whatever the default terminal shell is. |
| `.vscode/settings.json` | Line endings and formatting defaults; `.tools/` and `build/` excluded from search and file watching. The Go extension uses `.tools/go` (`go.goroot`) with the CLI's environment (`go.toolsEnvVars`) and installs its tools into `.tools/gotools/`. Not yet checked in the editor; on macOS, tools started by the extension still write Go telemetry to the user folder. |
| `.vscode/extensions.json` | Recommended extensions: Go, EditorConfig, Claude Code. |

## Planned architecture

The shell scripts are meant to shrink to bootstrappers: provision the pinned Go toolchain into `.tools/`, then hand over to a Go program that implements every command once, for every platform. This hasn't started; until then, logic lives in the twin scripts.
