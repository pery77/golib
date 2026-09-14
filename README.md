# GoLib

Make games in **Go**, powered by **[raylib](https://www.raylib.com)**, designed so an AI agent can turn a single prompt into a playable game.

> **Status: early development.** GoLib installs its own Go toolchain and opens its first window. Input, sprites, sound and scenes come next: see the [roadmap](docs/roadmap.md).

## Quick start

1. Download or clone this repository and open the folder in VS Code. Accept the recommended extensions.
2. Open a terminal in the project folder and run setup. It downloads Go (about 75 MB) and the raylib binding into the project:

   | Windows (PowerShell or cmd) | Linux, macOS |
   | --- | --- |
   | `.\golib setup` | `./golib setup` |

3. Run the first game. A window says hello:

   | Windows (PowerShell or cmd) | Linux, macOS |
   | --- | --- |
   | `.\golib run` | `./golib run` |

That's all. There is nothing to install first:

- Everything GoLib downloads lives inside the project, in `.tools/`. No C compiler is needed.
- No global installs, no PATH changes, no environment variables, no admin rights.
- To reset, run `golib clean --all` and then `golib setup` again. To uninstall, delete the folder.

Prefer clicking? In VS Code use **Terminal > Run Task... > GoLib: setup**, then **GoLib: run** (also on Ctrl+Shift+B).

## Making a game with AI

Open the project with your AI coding agent. It picks up the project instructions automatically:

| Agent | Reads |
| --- | --- |
| Claude Code | `CLAUDE.md`, which imports `AGENTS.md` |
| OpenAI Codex, Cursor, GitHub Copilot and other agents that follow the AGENTS.md convention | `AGENTS.md` |

Then describe the game you want, for example:

> Make a small arena game: I'm a triangle ship that moves with WASD and aims with the mouse. Asteroids split in two when shot. Three lives, score in the corner, and it gets faster every 30 seconds.

In Claude Code, `/make-game <your description>` runs the full game-making playbook.

The framework is still small (a window and text), so for now the agent tells you what is missing instead of faking it.

## How games are made

- **Your game and the framework are separate.** The framework lives in `framework/` and each game in its own folder under `games/`. You make a game without touching the framework.
- **No built-in editors.** Maps are made in [Tiled](https://www.mapeditor.org), sprites in [Aseprite](https://www.aseprite.org) and 3D models in [Blender](https://www.blender.org); GoLib loads their files. You only need these tools to edit content: games build and run without them.

Loading files from these tools is planned for M2: see the [architecture](docs/architecture.md).

## Commands

| Command | What it does |
| --- | --- |
| `golib setup` | Checks your system and prepares the local tools. Safe to run again. |
| `golib doctor` | Diagnoses problems without changing anything. |
| `golib run [game]` | Builds a game and runs it. |
| `golib build [game]` | Builds a game into `build/`. |
| `golib test` | Checks and tests the framework and every game. |
| `golib go <args>` | Runs the project's own Go, for example `golib go version`. |
| `golib clean` | Deletes build outputs. Add `--all` to also delete the downloaded tools. |
| `golib help` | Lists all commands. |

Type `.\golib` on Windows (PowerShell or cmd) and `./golib` in bash or zsh. `[game]` is a folder name in `games/`; leave it out when there is only one game.

## What's inside

| Path | Purpose |
| --- | --- |
| `golib`, `golib.cmd` | Command-line entry points |
| `tools/bootstrap/` | The CLI itself: short, readable scripts |
| `framework/` | The GoLib framework (Go package `golib`) |
| `games/` | Games, one folder each; `hello` is the first |
| `AGENTS.md`, `CLAUDE.md` | Instructions for AI agents |
| `docs/` | Vision, roadmap, internals and AI playbooks |
| `.vscode/`, `.claude/` | Editor and Claude Code configuration |

## Learn more

- [Vision and principles](docs/vision.md)
- [Roadmap](docs/roadmap.md)
- [Architecture: framework, games and content](docs/architecture.md)
- [Tooling internals](docs/tooling.md)
- [Contributing to GoLib](docs/contributing.md)
- [How agents make games](docs/ai/making-a-game.md)
