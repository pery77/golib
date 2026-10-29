#!/bin/sh
# Builds GoLib's library for language models, golibllm, for this machine and
# puts it in .tools/llm/, where golib finds it, for a platform the release
# doesn't cover yet or to try a change to native/llm/. It needs CMake 3.20 or
# later, git and a C++17 compiler; games and players never do. See
# native/README.md.
set -eu
llm=$(cd "$(dirname "$0")" && pwd)
root=$(dirname "$(dirname "$llm")")
build="$root/build/llm"
cmake -S "$llm" -B "$build" -DCMAKE_BUILD_TYPE=Release
cmake --build "$build" --config Release --parallel
case "$(uname -s)" in
  Darwin) name=golibllm.dylib ;;
  *) name=golibllm.so ;;
esac
mkdir -p "$root/.tools/llm"
cp "$build/$name" "$root/.tools/llm/$name"
echo "Built $name into .tools/llm/. Build your game again with golib to use it."
