# GoLib's native library

`golibnative` is the one piece of GoLib written in C++: [Jolt Physics](https://github.com/jrouwe/JoltPhysics) v5.6.0, the engine behind rigid bodies (`golib.PhysicsWorld.AddRigidBody`), wrapped in a small C interface that package `golib` calls through purego, with no C compiler on the player's or the game maker's machine.

Nobody making or playing a game builds it. `golib setup` downloads it into `.tools/native/` from a GitHub release, checking it against the checksum in `tools/cli/native.go`. Each build of a game that uses rigid bodies then carries it: `build`, `run` and `shot` copy it next to the executable, `dist` copies it next to the executable or, on macOS, into the app's `Contents/Frameworks/`, signed. A game without rigid bodies never loads it, and its builds leave it out: golib looks for `PhysicsMarker` (in `framework/internal/device/physics.go`) in the executable to tell.

In a browser, rigid bodies run on [JoltPhysics.js](https://github.com/jrouwe/JoltPhysics.js) 1.1.0, the same engine built for WebAssembly by Jolt's author. `golib setup` downloads its `jolt-physics.wasm.js` and `jolt-physics.wasm.wasm` from the npm package into `.tools/jolt/`, checked the same way, and web builds of a game with rigid bodies carry them next to the game's `.wasm`. `web.js` loads them the first time the game makes a world of rigid bodies.

Both are MIT-licensed, and their notices go into the game's `THIRD-PARTY-LICENSES.txt` (from `tools/cli/notices/jolt-*.txt`).

## Files

| File | What it is |
| --- | --- |
| `CMakeLists.txt` | Fetches Jolt at a pinned commit and builds `golibnative` as a shared library: SSE4.2 at most on x86, so it runs on older processors; the C++ runtime inside it on Windows and Linux |
| `golib_physics.cpp` | The C interface: `golib_world_new`, `golib_add_mesh`, `golib_add_body`, `golib_step`, `golib_read`, `golib_set` and the rest. Every float goes through a pointer, because purego's `SyscallN` passes integers only |
| `build.sh` | Builds it on this machine, Linux or macOS, into `.tools/native/` |
| `../.github/workflows/native.yml` | Builds it for each platform and publishes a release when a `native-<n>` tag is pushed |

The interface's version is `golib_version()`. `framework/internal/device/raylib_physics.go` checks it against `nativeVersion`, so a game never calls a library whose functions it doesn't know. Change both when the interface changes, and make a new release.

## Building it by hand

For a platform without a release yet, or to try a change to `golib_physics.cpp`, build it with CMake 3.20 or later, git and a C++17 compiler (Xcode's command line tools on macOS, GCC or Clang on Linux):

```bash
./native/build.sh
```

It builds into `build/native/` and copies the library into `.tools/native/`, where `golib setup` leaves a library built by hand alone on a platform the release doesn't cover. On Windows, from a Visual Studio developer prompt:

```bash
cmake -S native -B build/native
cmake --build build/native --config Release
```

then copy `build/native/Release/golibnative.dll` into `.tools/native/`.

## Making a release

1. Change `nativeRelease` in `tools/cli/native.go` to the next `native-<n>`.
2. Push a tag of that name to the repository named by `nativeRepository`: `git tag native-2 && git push <remote> native-2`. The workflow builds Windows (x86-64), Linux (x86-64 and ARM64) and macOS (Intel and Apple silicon) and publishes a release with the five files and `SHA256SUMS`.
3. Copy the checksums from `SHA256SUMS` into `nativeChecksums` in `tools/cli/native.go`, one per platform, such as `"windows/amd64": "…"`, and commit that.
4. Run `golib setup` on each platform you can, then `golib test`: the framework's rigid body tests skip without the library and run with it.

## golibllm, for language models

`native/llm/` builds `golibllm`, the second piece of GoLib in C++: [llama.cpp](https://github.com/ggml-org/llama.cpp) b11972 (MIT), behind a small C interface, `golib_llm.cpp`, for `golib.LanguageModel`. It is a library apart from `golibnative`, so games with rigid bodies don't carry it.

| File | What it is |
| --- | --- |
| `llm/CMakeLists.txt` | Fetches llama.cpp at a pinned commit and builds `golibllm`. With `-DGOLIB_LLM_BACKENDS_APART=ON`, as Windows and Linux are built, llama.cpp and its backends are libraries beside it: a CPU backend for each x86 processor generation, which llama.cpp picks at run time, and with `-DGOLIB_LLM_VULKAN=ON` the Vulkan one, skipped on computers without Vulkan. Without it, as on macOS, everything is inside `golibllm`: Metal on Apple silicon, AVX2 on Intel |
| `llm/golib_llm.cpp` | `golib_llm_load`, `golib_llm_template`, `golib_llm_start`, `golib_llm_next` and the rest, pointers and integers only. A model keeps one conversation's context and reads only what is new in each prompt |
| `llm/build.sh` | Builds it on this machine into `.tools/llm/` |
| `../.github/workflows/llm.yml` | Builds a zip per platform and publishes a release when an `llm-<n>` tag is pushed |

Its version is `golib_llm_version()`, checked against `llmVersion` in `framework/internal/device/raylib_llm.go`. To make a release: change `llmRelease` in `tools/cli/llm.go`, push a tag of that name to `nativeRepository`, copy the checksums from the release's `SHA256SUMS` into `llmChecksums`, and run `golib setup`. To try a model with the library: `GOLIB_TEST_MODEL=/path/model.gguf golib test --run TestLanguageModel` (the framework's test, which runs only with the variable).
