# Contributing to GoLib

Rules for changing GoLib itself: the CLI, the framework, the template and its docs. If you are making a game with GoLib, read [ai/making-a-game.md](ai/making-a-game.md) instead.

## Definition of done

A change is done when all of these hold:

1. It works on Windows, Linux and macOS, or the gap is written down in [roadmap.md](roadmap.md).
2. On a clean checkout, `golib setup`, `golib doctor` and `golib test` succeed, and every game in `games/` builds with `golib build <game>`. Go code is formatted: `golib go -C <module folder> fmt ./...` lists no files.
3. Every doc that describes the changed behavior is updated in the same change: the status table, commands and layout in `AGENTS.md`, `README.md`, and the relevant files in `docs/`.
4. A fresh agent session that only reads `AGENTS.md` and follows its links would use the change correctly.

## Designing the API for agents

The framework's first reader is an AI agent writing a game from a short description. Design for that reader:

- **Predictable names.** Use the vocabulary game developers and raylib already use: `Update`, `Draw`, `Texture`, `Sound`, `Vector2`. No clever abbreviations.
- **One obvious way.** Avoid overlapping helpers that do almost the same thing.
- **Small surface.** A few well-documented functions beat many.
- **Working defaults.** Zero configuration should produce something that runs and looks decent.
- **Actionable errors.** Say what failed, why, and what to do next. For example: `assets/player.png not found: asset paths are relative to the game's assets folder`.
- **No hidden rules.** No initialization order or package-level mutable state that callers must know about.
- **Documented exports.** Every exported identifier has a doc comment, with a short example when usage isn't obvious. Examples must compile.
- **Generic, not game-specific.** A feature belongs in the framework only if games other than the one that asked for it would use it. The framework never imports or refers to a game. See [architecture.md](architecture.md).
- **Content comes from files.** Maps, sprites and models are loaded from the files Tiled, Aseprite and Blender save. Don't add editors or content GUIs.

## Writing docs for agents

- Describe the code as it is. Mark anything unimplemented as planned.
- Prefer tables, short rules and complete runnable examples over long prose.
- Keep `AGENTS.md` compact: agents load it in every session. Put depth in `docs/` and link to it.
- Give the reason behind non-obvious rules; agents apply rules better when they know why.
- In `AGENTS.md`, never write `@` directly followed by a path: `CLAUDE.md` imports `AGENTS.md`, and Claude Code treats that pattern as another file import.

## Commits

- English, imperative mood, short subject line: `Add OneDrive check to golib doctor`.
- One logical change per commit.
- Framework changes and game changes go in separate commits, even when a game prompted the framework change.
- Nothing sensitive: no secrets, personal data or absolute paths from your machine.
