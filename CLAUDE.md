# CLAUDE.md

@AGENTS.md

## Claude Code specifics

- `AGENTS.md`, imported above, is the single source of truth for every agent. Shared instructions go there; this file only holds what is specific to Claude Code.
- Project skills live in `.claude/skills/`:
  - `/make-game <description>` builds or iterates on a game by following `docs/ai/making-a-game.md`.
- `.claude/settings.json` pre-approves the `golib` CLI in Bash and PowerShell, so the build, run and verify loop doesn't stall on permission prompts. Only add project-scoped, non-destructive commands to it.
- On Windows you have both a Bash tool (Git Bash) and a PowerShell tool: use `./golib` in Bash and `.\golib` in PowerShell. Both run the same Windows implementation.
- Personal or machine-specific settings go in `.claude/settings.local.json`, which is git-ignored. Never put them in the shared files.
