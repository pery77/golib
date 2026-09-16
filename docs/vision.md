# Vision

## The promise

Download the template, run a couple of commands, describe a game to an AI agent, and play it minutes later. Then keep improving it in plain language until it is the game you wanted.

## Who it is for

- **People with a game idea and an AI agent.** They may not know Go, raylib or game programming, and they should never have to fight a toolchain.
- **AI coding agents.** They do most of the typing. Docs, CLI output, API names and error messages are written to be read by them.
- **Go developers** who want a small, readable framework on top of raylib, without ceremony.

## Principles

1. **Zero install.** A fresh machine with nothing but its operating system works. `golib setup` downloads toolchains into the project's `.tools/` folder, at pinned versions with verified checksums. No global installs, PATH edits, environment variables or admin rights. Where an operating system genuinely requires a system package, `golib doctor` detects it and prints the exact command that fixes it.
2. **Transparent.** Every tool is small, readable code: short scripts to get started, plain Go for the rest. Downloads go to a visible folder, commands say what they do, and deleting `.tools/` resets everything.
3. **AI-first documentation.** Docs ship with the template and are part of the product. They state what exists and what doesn't, include complete examples, and change in the same commit as the code they describe.
4. **One obvious way.** A small API with predictable names beats a flexible one. An agent should be able to guess the right call, and be right.
5. **Fast feedback, for humans and agents.** Edit, run, see, in seconds. Agents must be able to check their own work as well: build output, tests, and screenshots taken without a human at the keyboard.
6. **Small dependency surface.** The Go standard library plus raylib. Anything else has to earn its place.
7. **Cross-platform by default.** Windows, Linux and macOS are first-class, on x86-64 and ARM64.
8. **Games, not engines.** GoLib covers what nearly every game needs, then gets out of the way. raylib stays reachable for everything else.
9. **Framework and games apart.** The framework is a library; each game is an ordinary Go program in its own folder that uses it. Anyone can make a game without touching the framework, and the framework never holds code for one particular game. See [architecture.md](architecture.md).
10. **Established tools, no editors of our own.** Content is made in mature tools people already know: Tiled for 2D maps, Aseprite for sprites and animations, Blender for 3D models. GoLib loads the files they save. The tools are only needed to edit content: a game builds and runs without them.

## Non-goals

- A visual editor, scene designer, level editor or asset GUI of any kind. Tiled, Aseprite and Blender already do that job well.
- A general-purpose engine competing with Godot or Unity on features.
- Hiding Go. A GoLib game is an ordinary Go program.

## Language

Everything in the repository is written in English: code, comments, docs, CLI output and commit messages. People can talk to their agent in any language; the files stay in English so every agent and every contributor can read them.
