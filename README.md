# GoLib

Make games in **Go**, powered by **[raylib](https://www.raylib.com)**, designed so an AI agent can turn a single prompt into a playable game.

> **Status: early development.** GoLib installs its own Go toolchain, runs games with keyboard, mouse and gamepad input, shapes, text, random numbers, scenes, fullscreen, post-processing shaders, sound effects made in code and music, and builds them into a zip to share, and `golib new` starts a new game; sprites and maps come later: see the [roadmap](docs/roadmap.md).

## Quick start

1. Download or clone this repository and open the folder in VS Code. Accept the recommended extensions.
2. Open a terminal in the project folder and run setup. It downloads Go (about 75 MB) and the raylib binding into the project:

   | Windows (PowerShell or cmd) | Linux, macOS |
   | --- | --- |
   | `.\golib setup` | `./golib setup` |

3. Run the example game, a small platformer:

   | Windows (PowerShell or cmd) | Linux, macOS |
   | --- | --- |
   | `.\golib run` | `./golib run` |

That's all. There is nothing to install first:

- Everything GoLib downloads lives inside the project, in `.tools/`. No C compiler is needed.
- No global installs, no PATH changes, no environment variables, no admin rights.
- To reset, run `golib clean --all` and then `golib setup` again. To uninstall, delete the folder.

Prefer clicking? On Windows, double-click `golib-ui.cmd` in the project folder. It opens a window with a button for every command (Setup, Run, Screenshots, Dist build and more) and shows what they print. In VS Code you can also use **Terminal > Run Task... > GoLib: setup**, then **GoLib: run** (also on Ctrl+Shift+B). Press F5 to debug a game.

If you opened a Go file before setup finished, run **Developer: Reload Window** (Ctrl+Shift+P) once setup is done. The Go extension looks for Go only when it starts, so until you reload it can't find the one setup installed.

## Making a game with AI

Open the project with your AI coding agent. It picks up the project instructions automatically:

| Agent | Reads |
| --- | --- |
| Claude Code | `CLAUDE.md`, which imports `AGENTS.md` |
| OpenAI Codex, Cursor, GitHub Copilot and other agents that follow the AGENTS.md convention | `AGENTS.md` |

Then describe the game you want, for example:

> Make a small arena game: I'm a triangle ship that moves with WASD and aims with the mouse. Asteroids split in two when shot. Three lives, score in the corner, and it gets faster every 30 seconds.

In Claude Code, `/make-game <your description>` runs the full game-making playbook.

The framework is still small (shapes, text, keyboard, mouse, gamepad, scenes, screen effects, sound effects made in code and music), so for now the agent tells you what is missing instead of faking it.

## How games are made

- **Your game and the framework are separate.** The framework lives in `framework/` and each game in its own folder under `games/`. You make a game without touching the framework, and a game moves to a newer GoLib by copying its folder into the newer template (see [Distribution](docs/architecture.md#distribution)).
- **No built-in editors.** Maps are made in [Tiled](https://www.mapeditor.org), sprites in [Aseprite](https://www.aseprite.org) and 3D models in [Blender](https://www.blender.org); GoLib loads their files. You only need these tools to edit content: games build and run without them.

Loading Tiled maps and Aseprite sprites is planned for M4, which is postponed for now, and Blender models come with 3D in M6, after it: see the [architecture](docs/architecture.md).

## Commands

| Command | What it does |
| --- | --- |
| `golib setup` | Checks your system and prepares the local tools. Safe to run again. |
| `golib doctor` | Diagnoses problems without changing anything. |
| `golib new <name>` | Creates a new game in `games/<name>/`, ready to run. |
| `golib run [game]` | Builds a game and runs it. |
| `golib build [game]` | Builds a game into `build/`, for development. |
| `golib dist [game]` | Builds a game to share: a folder with everything it needs, zipped, with the licenses that go with it and, on Windows, its icon and version details. |
| `golib shot [game] [frame...]` | Saves screenshots of a game at the given frames, without opening a visible window. |
| `golib test` | Checks and tests the framework, every game and GoLib's own Go program. |
| `golib go <args>` | Runs the project's own Go, for example `golib go version`. |
| `golib clean` | Deletes build outputs. Add `--all` to also delete the downloaded tools. |
| `golib help` | Lists all commands. |

Type `.\golib` on Windows (PowerShell or cmd) and `./golib` in bash or zsh. `[game]` is a folder name in `games/`; leave it out when there is only one game.

## What's inside

| Path | Purpose |
| --- | --- |
| `golib`, `golib.cmd` | Command-line entry points |
| `golib-ui.cmd`, `tools/ui/` | The GoLib window: a button for each command (Windows) |
| `tools/bootstrap/`, `tools/cli/` | The CLI itself: short scripts that install Go and start a Go program, which has most of the commands |
| `framework/` | The GoLib framework (Go package `golib`), with its API guide, `README.md` |
| `games/` | Games, one folder each; `platformer` is the example to learn from |
| `AGENTS.md`, `CLAUDE.md` | Instructions for AI agents |
| `docs/` | Vision, roadmap, internals and AI playbooks |
| `.vscode/`, `.claude/` | Editor and Claude Code configuration |
| `LICENSE` | GoLib's license: zlib |

## Learn more

- [Vision and principles](docs/vision.md)
- [Roadmap](docs/roadmap.md)
- [Architecture: framework, games and content](docs/architecture.md)
- [Tooling internals](docs/tooling.md)
- [Contributing to GoLib](docs/contributing.md)
- [How agents make games](docs/ai/making-a-game.md)
- [The framework's API, by task](framework/README.md)

## License

GoLib is released under the [zlib license](LICENSE), the same as raylib: use it for anything, commercial games included, with no need to credit GoLib inside your game. It covers the framework, the tools, the docs and the example games, except files with terms of their own, such as the music in `games/asteroids/assets/` (see its `ATTRIBUTION.md`).

The games you make in `games/` are yours: license them as you like.

A dist build also contains Go, purego, raylib-go, ffi, raylib and libffi. The licenses of Go, purego, ffi and libffi ask for their notices to go with the game, so `golib dist` puts their licenses in `THIRD-PARTY-LICENSES.txt`, next to the game, with a copy of the game's `assets/ATTRIBUTION.md`: keep that file with the game. It also has the notices of the libraries inside raylib that ask for one, such as cgltf and QOI. It is GoLib's best effort, not legal advice: check what you ship when you publish a game, especially files and modules you add yourself.
