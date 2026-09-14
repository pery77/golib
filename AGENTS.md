# AGENTS.md

Instructions for AI coding agents working in this repository (Claude Code, OpenAI Codex, GitHub Copilot, Cursor and others). Humans should start with [README.md](README.md).

## What GoLib is

GoLib is a small game framework for **Go**, built on **raylib** (windowing, input, graphics, audio), distributed as a ready-to-use **VS Code template**.

The promise: someone downloads the template, runs a couple of commands, and builds a complete game with an AI agent from one prompt and a few iterations. That makes you a primary user of this project. Docs, CLI output and (soon) the API are written so you can work without guessing, and you are expected to keep them that way.

## Project status

Keep this section true: update it in the same change that lands or removes a feature. Never describe planned work as if it existed.

Last updated: 2026-09-14 (milestone M1, Toolchain and first window, in progress).

| Area | State |
| --- | --- |
| `golib` CLI: `setup`, `doctor`, `clean`, `help` | Done |
| VS Code workspace: extensions, settings, tasks | Done |
| AI instructions: this file, `CLAUDE.md`, Claude Code settings, `/make-game` skill | Done |
| Go 1.27.1 downloaded into `.tools/` by `golib setup` | Done |
| raylib 6.0 through raylib-go, with no C compiler | Done |
| `framework/` (package `golib`) and a first game, `games/hello`, that opens a window | Done |
| `golib run`, `golib build`, `golib test`, `golib go` | Done |
| VS Code: Go extension on the local toolchain | Done, not yet checked in the editor |
| VS Code debug configuration | Planned (M1) |
| Linux and macOS (`golib.sh`) | Written; not yet run on a real Linux or macOS machine |
| Framework basics: game loop, scenes, input, drawing, text, audio, assets, Tiled maps, Aseprite sprites, `golib new` | Planned (M2) |
| Screenshots captured without a human, for agent verification | Planned (M3) |

**The framework is only a seed.** It opens a window, runs the game loop, clears the screen and draws text: `golib.Run`, `Game`, `Config`, `Screen` and colors, documented in the doc comments in `framework/`. There is no input, sprites, audio or scenes yet (M2). If someone asks for a game that needs them, say what is missing and point to [docs/roadmap.md](docs/roadmap.md). Do not improvise a stand-alone engine to fill the gap.

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
10. **No editors of our own.** Content comes from established tools: Tiled for 2D maps, Aseprite for sprites, Blender for 3D models. Never build a level editor, sprite editor or asset GUI, not even inside a game; load those tools' files instead.

## Commands

Run from the project root. The command name is the same everywhere; only the prefix changes.

| Shell | Invocation |
| --- | --- |
| PowerShell or cmd (Windows) | `.\golib <command>` |
| bash, zsh, sh (Linux, macOS, Git Bash on Windows) | `./golib <command>` |

| Command | Effect |
| --- | --- |
| `setup` | Checks the environment, then installs Go, the Go modules and the raylib library into `.tools/`. Safe to run repeatedly. |
| `doctor` | Read-only diagnosis of the environment and the project. |
| `build [game]` | Builds `games/<game>` into `build/<game>/`, next to a copy of the raylib library. |
| `run [game]` | Builds the game, then runs it with `games/<game>/` as the working directory. |
| `test` | Runs `go vet` and `go test` for the framework and every game. |
| `go <args>` | Runs the project's Go toolchain with GoLib's environment, for example `go -C games/hello mod tidy`. |
| `clean` | Deletes `build/`. |
| `clean --all` | Also deletes `.tools/`. Run `setup` again afterwards. |
| `help` | Lists commands. |

`[game]` is a folder name in `games/`; leave it out when there is only one game. Check lines start with `[ok]`, `[info]`, `[warn]` or `[fail]`, followed by a summary line. Exit codes: `0` success, `1` failure, `2` usage error. When anything behaves unexpectedly, run `doctor` and read its output before trying fixes.

In PowerShell, quote arguments that start with `-` and contain a dot, such as `'-replace=golib=../../framework'`: Windows PowerShell 5.1 splits them before `golib` receives them.

## Repository layout

```text
AGENTS.md            Canonical instructions for AI agents (this file)
CLAUDE.md            Claude Code entry point; imports this file
README.md            Human quick start
golib, golib.cmd     CLI entry points for POSIX shells and Windows; thin shims, no logic
tools/bootstrap/     CLI implementations: golib.sh (Linux, macOS), golib.ps1 (Windows)
framework/           The framework: Go module and package "golib"
games/               One folder per game, each its own Go module
  hello/             The first game; tests the framework as features land
docs/                Vision, roadmap, architecture, tooling, contributing, AI playbooks
.claude/             Claude Code project settings and skills
.vscode/             Recommended extensions, editor settings, tasks
.tools/              Git-ignored. Go, Go modules and the raylib library, created by setup
build/               Git-ignored. Build outputs, created by build and run
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
| [docs/architecture.md](docs/architecture.md) | Deciding whether code belongs in the framework or in a game, or how maps, sprites and models get into a game |
| [docs/tooling.md](docs/tooling.md) | Changing the CLI, the VS Code config, line endings or `.tools/` |
| [docs/contributing.md](docs/contributing.md) | Changing GoLib itself: definition of done, API design for agents, commits |
| [docs/ai/making-a-game.md](docs/ai/making-a-game.md) | Building or iterating on a game |

## Platform notes

- Windows baseline: Windows 10 or later with the built-in Windows PowerShell 5.1. PowerShell 7 is not required.
- Always type the `.\` or `./` prefix. Some environments, including agent sandboxes, stop Windows from running programs from the current folder by bare name; an explicit relative path always works.
- On Windows, `./golib` from Git Bash and `.\golib` from PowerShell run the same implementation (`golib.ps1`), so their results match.
- Every program that uses raylib loads the raylib library when it starts, and stops with `cannot load library ...` if it can't find it. `golib build`, `run` and `test` take care of that; `golib go test` and running an executable from outside `build/<game>/` don't.
- `.gitattributes` enforces line endings: LF everywhere, CRLF only for `*.cmd` and `*.bat`. Don't change files to work around it.
