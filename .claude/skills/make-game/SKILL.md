---
name: make-game
description: Build a playable game with GoLib from a description, or keep iterating on the existing game. Use when the user asks to create, prototype, change, balance or polish a game in this project.
argument-hint: "<describe the game>"
---

# Make a game

Request: $ARGUMENTS

1. Read `docs/ai/making-a-game.md` in full and follow it, and read `framework/README.md`, the framework's API guide, before writing code. The playbook is the source of truth; this skill only starts it.
2. Check the "Project status" table in `AGENTS.md`. If the framework can't support this game yet, tell the user exactly what is missing and which roadmap milestone adds it, then stop.
3. If the request is empty and a game already exists in `games/`, read its `DESIGN.md` and ask what to change next (if there are several games, ask which one). If there is no game either, ask the user to describe one in a sentence or two, and offer three short example ideas.
4. Talk to the user in their language. Everything written to files stays in English.
