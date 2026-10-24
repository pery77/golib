#!/bin/sh
# Builds GoLib's native library for this machine and puts it in .tools/native/,
# where golib finds it, for a platform the release doesn't cover yet or to try
# a change to native/. It needs CMake 3.20 or later, git and a C++17 compiler;
# games and players never do. See native/README.md.
set -eu
native=$(cd "$(dirname "$0")" && pwd)
root=$(dirname "$native")
build="$root/build/native"
cmake -S "$native" -B "$build" -DCMAKE_BUILD_TYPE=Release
cmake --build "$build" --config Release --parallel
case "$(uname -s)" in
  Darwin) name=golibnative.dylib ;;
  *) name=golibnative.so ;;
esac
mkdir -p "$root/.tools/native"
cp "$build/$name" "$root/.tools/native/$name"
echo "Built $name into .tools/native/. Build your game again with golib to use it."
