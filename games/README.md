# games/

Your games live here, one folder each, and each folder is a Go module of its own. Create one with:

```text
golib new <name>
golib run <name>
```

Git ignores everything in this folder but this file, so nothing you make here ever enters the GoLib repository. To keep a game's history, give it a git repository of its own, inside its folder:

```text
cd games/<name>
git init
```

GoLib's own examples, such as `platformer`, are in [examples/](../examples). A game can't take one of their names. The rules for both folders are in [docs/architecture.md](../docs/architecture.md#your-games-and-the-examples).
