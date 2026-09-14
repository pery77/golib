#!/bin/sh
# GoLib command-line tool: Linux and macOS implementation (POSIX sh).
#
# Do not run this file directly. Entry point: ./golib from the project root.
#
# tools/bootstrap/golib.ps1 is the Windows twin of this file. Both must expose the same
# commands, options, output format and exit codes: change one, change the other.
# POSIX sh only, so it runs under dash: no bash arrays, no [[ ]], no "local". Functions
# share one variable namespace, so their variables carry a short prefix.
#
# Exit codes: 0 success, 1 failure, 2 usage error.
set -eu

# Pinned Go toolchain. golib.ps1 pins the same version for Windows: change both together.
# Checksums come from https://go.dev/dl/?mode=json
go_version=1.27.1
go_sha256_linux_amd64=63d339f0da5ab53635a56f2490a7984dfe12dfcff22ad749f63edaf590168445
go_sha256_linux_arm64=3450b45a3f9ee8568792736a5c5e70a1f2e9b36c35a8f74958c03e51d7d92bec
go_sha256_darwin_amd64=8f8f52c6649542cf027bbc9b9c68d1ec042f9f34808a40413f0b8b3f66f3caa4
go_sha256_darwin_arm64=ee215d57e0ec269c60cc9ceca68e6bda321ba9ee5afe24f4b0988703c2d87d12

# The raylib binding. Its version is pinned in framework/go.mod; the prebuilt raylib library
# for each platform ships inside the module, in its libs/ folder.
raylib_module=github.com/gen2brain/raylib-go/raylib

root=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
tools_dir="$root/.tools"
build_dir="$root/build"
go_root="$tools_dir/go"
go_exe="$go_root/bin/go"
raylib_dir="$tools_dir/raylib"
framework_dir="$root/framework"
games_dir="$root/games"

failures=0
warnings=0

show_help() {
  cat <<'EOF'
GoLib - make games in Go, powered by raylib.

Usage: golib <command> [options]

Commands:
  setup          Check the environment, then install Go, Go modules and raylib into .tools/
  doctor         Diagnose the environment without changing anything
  build [game]   Build games/<game> into build/<game>/
  run [game]     Build games/<game>, then run it from its folder
  test           Vet and test the framework and every game
  go <args>      Run the project's Go toolchain, with GoLib's settings
  clean          Delete build outputs (build/)
  clean --all    Also delete downloaded tools (.tools/); run setup again afterwards
  help           Show this help

[game] is a folder name in games/. Leave it out when there is only one game.

Invoke from the project root:
  Windows (PowerShell, cmd)    .\golib <command>
  Linux, macOS, Git Bash       ./golib <command>
EOF
}

# Prints one check result: check <ok|info|warn|fail> <message>
check() {
  case "$1" in
    warn) warnings=$((warnings + 1)) ;;
    fail) failures=$((failures + 1)) ;;
  esac
  printf '%-6s %s\n' "[$1]" "$2"
}

summary() {
  printf '\n%s: %d failed, %d warning(s)\n' "$1" "$failures" "$warnings"
}

usage_error() {
  printf 'golib: %s\nRun "golib help" for usage.\n' "$1" >&2
  exit 2
}

have() {
  command -v "$1" >/dev/null 2>&1
}

# Joins space-separated words with commas: "a b" becomes "a, b".
comma_list() {
  printf '%s' "$1" | sed 's/ /, /g'
}

# Sets goos and goarch to Go's names for this machine, or to '' when unsupported.
detect_platform() {
  case "$(uname -s)" in
    Linux) goos=linux ;;
    Darwin) goos=darwin ;;
    *) goos='' ;;
  esac
  case "$(uname -m)" in
    x86_64 | amd64) goarch=amd64 ;;
    aarch64 | arm64) goarch=arm64 ;;
    *) goarch='' ;;
  esac
}

# Deletes a folder. Go toolchains and Go's module cache contain read-only files and folders.
remove_tree() {
  if [ -e "$1" ]; then
    chmod -R u+w "$1" 2>/dev/null || true
    rm -rf "$1"
  fi
}

# On macOS, /usr/bin/git is a stub that opens an installer dialog when the
# Command Line Tools are missing. Treat that case as "git not installed".
git_installed() {
  have git || return 1
  if [ "$goos" = darwin ] && [ "$(command -v git)" = /usr/bin/git ]; then
    xcode-select -p >/dev/null 2>&1 || return 1
  fi
  return 0
}

# Sets library_state to found, missing or unknown for a Linux shared library such as libX11.so.6.
find_linux_library() {
  library_state=unknown
  for fl_ldconfig in ldconfig /sbin/ldconfig /usr/sbin/ldconfig; do
    if have "$fl_ldconfig"; then
      if "$fl_ldconfig" -p 2>/dev/null | grep -q "$1"; then library_state=found; else library_state=missing; fi
      return 0
    fi
  done
}

# Checks shared by setup and doctor. Only reports; never changes anything.
check_environment() {
  if [ -z "$goos" ]; then
    check fail "$(uname -s) is not supported (Linux or macOS required; on Windows use golib.cmd)"
  elif [ -z "$goarch" ]; then
    check fail "$(uname -m) processors are not supported (64-bit x86 or ARM required)"
  elif [ "$goos" = darwin ]; then
    check ok "platform $goos/$goarch (macOS $(sw_vers -productVersion 2>/dev/null || uname -r))"
  else
    check ok "platform $goos/$goarch (Linux $(uname -r))"
  fi

  check info "project root $root"

  probe="$root/.golib-write-test-$$"
  if (: >"$probe") 2>/dev/null; then
    rm -f "$probe"
    check ok "project folder is writable"
  else
    check fail "cannot write to the project folder"
  fi

  if git_installed; then
    check ok "git $(git --version | sed 's/^git version *//')"
  else
    check info "git not found (optional: only needed to clone or update the repository)"
  fi

  if have curl; then
    check ok "curl found (used to download Go)"
  elif have wget; then
    check ok "wget found (used to download Go)"
  else
    check warn "neither curl nor wget found: setup can't download Go. Install curl (Debian/Ubuntu: sudo apt install curl; Fedora: sudo dnf install curl)"
  fi

  # raylib's prebuilt Linux library links against X11, and loads OpenGL when a window opens.
  if [ "$goos" = linux ]; then
    for ce_library in libX11.so.6 libGL.so.1; do
      find_linux_library "$ce_library"
      case "$library_state" in
        found) check ok "system library $ce_library found" ;;
        missing) check warn "system library $ce_library not found: games can't open a window. Install it (Debian/Ubuntu: sudo apt install libx11-6 libgl1; Fedora: sudo dnf install libX11 mesa-libGL)" ;;
        *) check info "could not check for $ce_library (ldconfig not found)" ;;
      esac
    done
  fi
}

# --- Go toolchain -------------------------------------------------------------------------

# Prints the version of the Go toolchain in .tools/go/ (such as 1.27.1), or nothing.
installed_go_version() {
  if [ -f "$go_root/VERSION" ]; then
    sed -n '1s/^go//p' "$go_root/VERSION"
  fi
}

download() {
  if have curl; then
    curl -fsSL --retry 2 -o "$2" "$1"
  elif have wget; then
    wget -q -O "$2" "$1"
  else
    return 1
  fi
}

sha256_of() {
  if have sha256sum; then
    sha256sum "$1" | cut -d ' ' -f 1
  elif have shasum; then
    shasum -a 256 "$1" | cut -d ' ' -f 1
  else
    openssl dgst -sha256 "$1" | sed 's/^.*= *//'
  fi
}

# Installs the pinned Go toolchain into .tools/go/. Prints a failure and returns 1 if it can't.
install_go() {
  ig_installed=$(installed_go_version)
  if [ "$ig_installed" = "$go_version" ]; then
    check ok "Go $go_version already installed in .tools/go/"
    return 0
  fi
  ig_file="go$go_version.$goos-$goarch.tar.gz"
  ig_url="https://go.dev/dl/$ig_file"
  eval "ig_expected=\$go_sha256_${goos}_${goarch}"
  mkdir -p "$tools_dir/downloads"
  ig_archive="$tools_dir/downloads/$ig_file"

  check info "downloading $ig_url"
  if ! download "$ig_url" "$ig_archive.partial"; then
    remove_tree "$tools_dir/downloads"
    check fail "download failed: $ig_url"
    return 1
  fi
  mv -f "$ig_archive.partial" "$ig_archive"
  ig_hash=$(sha256_of "$ig_archive") || ig_hash=''
  if [ "$ig_hash" != "$ig_expected" ]; then
    remove_tree "$tools_dir/downloads"
    check fail "checksum mismatch for $ig_file (expected $ig_expected, got $ig_hash). The download was deleted; run setup again"
    return 1
  fi

  # Extract next to the final location, then swap, so an interrupted setup never leaves a broken .tools/go/.
  ig_staging="$tools_dir/go.partial"
  remove_tree "$ig_staging"
  mkdir -p "$ig_staging"
  if ! tar -xzf "$ig_archive" -C "$ig_staging"; then
    remove_tree "$ig_staging"
    check fail "could not extract $ig_file"
    return 1
  fi
  remove_tree "$go_root"
  mv "$ig_staging/go" "$go_root"
  remove_tree "$ig_staging"
  remove_tree "$tools_dir/downloads"
  if [ -n "$ig_installed" ]; then
    check ok "Go $go_version installed in .tools/go/ (replaced $ig_installed)"
  else
    check ok "Go $go_version installed in .tools/go/"
  fi
}

# Points Go at the toolchain and caches in .tools/, for this process and the programs it starts.
# Nothing outside the project changes, and nothing persists after golib exits.
set_go_environment() {
  GOROOT="$go_root"
  GOPATH="$tools_dir/gopath"
  GOMODCACHE="$tools_dir/gopath/pkg/mod"
  GOCACHE="$tools_dir/gocache"
  GOENV=off                        # ignore any global "go env -w" settings
  GOTOOLCHAIN=local                # never download a different Go version
  CGO_ENABLED=0                    # raylib-go without a C compiler
  GOFLAGS=-tags=raylib_no_embed    # load raylib from build/ or .tools/, never extract it to a user folder
  PATH="$go_root/bin:$PATH"
  export GOROOT GOPATH GOMODCACHE GOCACHE GOENV GOTOOLCHAIN CGO_ENABLED GOFLAGS PATH
  # Go writes telemetry counters to the user's config folder ($XDG_CONFIG_HOME or ~/.config on
  # Linux, ~/Library/Application Support on macOS). Redirect that folder into .tools/;
  # restore_user_environment undoes this before a game starts.
  if [ "$goos" = darwin ]; then
    mkdir -p "$tools_dir/home"
    HOME="$tools_dir/home"
    export HOME
  else
    XDG_CONFIG_HOME="$tools_dir/config"
    export XDG_CONFIG_HOME
  fi
}

restore_user_environment() {
  HOME=$user_home
  export HOME
  if [ "$user_xdg_set" = true ]; then
    XDG_CONFIG_HOME=$user_xdg_config_home
    export XDG_CONFIG_HOME
  else
    unset XDG_CONFIG_HOME
  fi
}

# Prints a failure and exits unless the pinned Go toolchain is installed; then sets up its environment.
assert_toolchain() {
  at_installed=$(installed_go_version)
  if [ "$at_installed" != "$go_version" ]; then
    if [ -n "$at_installed" ]; then
      check fail "Go $at_installed is in .tools/go/, but GoLib needs $go_version (run: golib setup)"
    else
      check fail "the Go toolchain is not installed (run: golib setup)"
    fi
    summary "$1"
    exit 1
  fi
  set_go_environment
}

# --- Modules, games and raylib --------------------------------------------------------------

# Sets games to the space-separated game names: folders in games/ that contain a go.mod.
list_games() {
  games=''
  if [ -d "$games_dir" ]; then
    for lg_dir in "$games_dir"/*/; do
      if [ -f "${lg_dir}go.mod" ]; then
        games="${games:+$games }$(basename "$lg_dir")"
      fi
    done
  fi
}

# Sets modules to the project's Go modules, relative to the root: framework first, then each game.
list_modules() {
  modules=''
  if [ -f "$framework_dir/go.mod" ]; then modules=framework; fi
  list_games
  for lm_game in $games; do
    modules="${modules:+$modules }games/$lm_game"
  done
}

# resolve_game <command> [name]: sets game, or exits with a usage error.
resolve_game() {
  rg_command=$1
  shift
  if [ $# -gt 1 ]; then usage_error "$rg_command takes at most one game name (got: $*)"; fi
  list_games
  if [ $# -eq 1 ]; then
    for rg_game in $games; do
      if [ "$rg_game" = "$1" ]; then
        game=$1
        return 0
      fi
    done
    if [ -z "$games" ]; then usage_error "no game named \"$1\": games/ has no games yet"; fi
    usage_error "no game named \"$1\" in games/ (available: $(comma_list "$games"))"
  fi
  set -- $games
  if [ $# -eq 1 ]; then
    game=$1
    return 0
  fi
  if [ $# -eq 0 ]; then usage_error "there are no games in games/ yet"; fi
  usage_error "$rg_command needs a game name (available: $(comma_list "$games"))"
}

# Makes sure the raylib library matching a module's raylib-go version is extracted into
# .tools/raylib/<version>/. Sets raylib_lib to its path, or sets raylib_error and returns 1.
# Requires set_go_environment.
sync_raylib() {
  if ! sr_info=$("$go_exe" -C "$1" list -m -f '{{.Version}}|{{.Dir}}' "$raylib_module"); then
    raylib_error="cannot find $raylib_module for $1 (run: golib setup)"
    return 1
  fi
  sr_version=${sr_info%%|*}
  sr_cache=${sr_info#*|}
  if [ -z "$sr_cache" ]; then
    raylib_error="$raylib_module $sr_version is not downloaded yet (run: golib setup)"
    return 1
  fi
  case "$goos" in
    darwin) sr_pattern="raylib-*_macos.tar.gz" ;;
    *) sr_pattern="raylib-*_linux_$goarch.tar.gz" ;;
  esac
  sr_archive=''
  for sr_candidate in "$sr_cache"/libs/$sr_pattern; do
    if [ -f "$sr_candidate" ]; then
      sr_archive=$sr_candidate
      break
    fi
  done
  if [ -z "$sr_archive" ]; then
    raylib_error="$raylib_module $sr_version has no prebuilt library matching $sr_pattern"
    return 1
  fi
  # The archive holds a single file, such as libraylib.so.6.0.0.
  sr_name=$(tar -tzf "$sr_archive" | head -n 1)
  raylib_lib="$raylib_dir/$sr_version/$sr_name"
  if [ -f "$raylib_lib" ]; then return 0; fi
  mkdir -p "$raylib_dir/$sr_version"
  if ! tar -xzf "$sr_archive" -C "$raylib_dir/$sr_version" || [ ! -f "$raylib_lib" ]; then
    raylib_error="could not extract $sr_name from $sr_archive"
    return 1
  fi
}

# Adds a folder to the dynamic library search path. Call it in a subshell.
use_library_dir() {
  if [ "$goos" = darwin ]; then
    DYLD_LIBRARY_PATH="$1${DYLD_LIBRARY_PATH:+:$DYLD_LIBRARY_PATH}"
    export DYLD_LIBRARY_PATH
  else
    LD_LIBRARY_PATH="$1${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
    export LD_LIBRARY_PATH
  fi
}

# Builds games/<game> into build/<game>/ next to a copy of the raylib library. Sets built_exe;
# prints a failure and returns 1 if something goes wrong.
build_game() {
  bg_dir="$games_dir/$1"
  bg_out="$build_dir/$1"
  built_exe="$bg_out/$1"
  mkdir -p "$bg_out"
  if ! "$go_exe" -C "$bg_dir" build -o "$built_exe" .; then
    check fail "build failed for games/$1 (see the Go errors above)"
    return 1
  fi
  if ! sync_raylib "$bg_dir"; then
    check fail "$raylib_error"
    return 1
  fi
  cp -f "$raylib_lib" "$bg_out/"
  check ok "built games/$1 into build/$1/$1"
}

# --- Commands -------------------------------------------------------------------------------

cmd_setup() {
  if [ $# -gt 0 ]; then usage_error "setup takes no options (got: $*)"; fi
  check_environment
  if [ "$failures" -gt 0 ]; then
    summary setup
    echo 'Setup stopped. Fix the [fail] items above, then run setup again.'
    exit 1
  fi
  mkdir -p "$tools_dir"
  check ok "local tools folder ready: .tools/"

  if ! install_go; then
    summary setup
    exit 1
  fi
  set_go_environment

  list_modules
  if [ -z "$modules" ]; then check info "no Go modules yet: nothing more to download"; fi
  for cs_module in $modules; do
    if ! "$go_exe" -C "$root/$cs_module" mod download; then
      check fail "could not download the Go modules for $cs_module (see the errors above)"
      continue
    fi
    if sync_raylib "$root/$cs_module"; then
      check ok "$cs_module: modules downloaded, raylib library in .tools/raylib/$(basename "$(dirname "$raylib_lib")")/"
    else
      check fail "$cs_module: $raylib_error"
    fi
  done

  summary setup
  if [ "$failures" -gt 0 ]; then
    echo 'Setup incomplete. Fix the [fail] items above, then run setup again.'
    exit 1
  fi
  echo 'Setup complete.'
  exit 0
}

cmd_doctor() {
  if [ $# -gt 0 ]; then usage_error "doctor takes no options (got: $*)"; fi
  check_environment
  if [ -d "$tools_dir" ]; then
    check ok ".tools/ exists"
  else
    check info ".tools/ not created yet (run: golib setup)"
  fi

  cd_installed=$(installed_go_version)
  if [ "$cd_installed" = "$go_version" ]; then
    check ok "Go $go_version in .tools/go/"
  elif [ -n "$cd_installed" ]; then
    check warn "Go $cd_installed is in .tools/go/, but GoLib needs $go_version (run: golib setup)"
  else
    check warn "Go toolchain not installed (run: golib setup)"
  fi

  if [ -f "$framework_dir/go.mod" ]; then
    check ok "framework module in framework/"
  else
    check fail "framework/go.mod is missing: the framework is not in this project"
  fi
  list_modules
  if [ -n "$games" ]; then
    check info "games in games/: $(comma_list "$games")"
  else
    check info "no games in games/ yet"
  fi

  # Read go.mod as text instead of asking Go, so doctor starts nothing and changes nothing.
  for cd_module in $modules; do
    cd_version=$(sed -n 's#^[[:space:]]*\(require[[:space:]][[:space:]]*\)\{0,1\}github\.com/gen2brain/raylib-go/raylib[[:space:]][[:space:]]*\(v[^[:space:]]*\).*#\2#p' "$root/$cd_module/go.mod" | head -n 1)
    if [ -z "$cd_version" ]; then
      check warn "$cd_module: go.mod does not require $raylib_module"
    elif ls "$raylib_dir/$cd_version"/libraylib.* >/dev/null 2>&1; then
      check ok "$cd_module: raylib library for raylib-go $cd_version ready"
    else
      check warn "$cd_module: raylib library for raylib-go $cd_version not ready (run: golib setup)"
    fi
  done

  summary doctor
  if [ "$failures" -gt 0 ]; then exit 1; fi
  exit 0
}

cmd_build() {
  resolve_game build "$@"
  assert_toolchain build
  if build_game "$game"; then
    summary build
    exit 0
  fi
  summary build
  exit 1
}

cmd_run() {
  resolve_game run "$@"
  assert_toolchain run
  if ! build_game "$game"; then
    summary run
    exit 1
  fi
  check info "running build/$game/$game with games/$game/ as the working directory"
  cr_code=0
  (
    restore_user_environment
    use_library_dir "$build_dir/$game"
    cd "$games_dir/$game" || exit 1
    exec "$built_exe"
  ) || cr_code=$?
  printf '\nrun: %s exited with code %d\n' "$game" "$cr_code"
  if [ "$cr_code" -ne 0 ]; then exit 1; fi
  exit 0
}

cmd_test() {
  if [ $# -gt 0 ]; then usage_error "test takes no options (got: $*)"; fi
  assert_toolchain test
  list_modules
  if [ -z "$modules" ]; then check warn "no Go modules to test"; fi
  for ct_module in $modules; do
    ct_dir="$root/$ct_module"
    if ! "$go_exe" -C "$ct_dir" vet ./...; then
      check fail "$ct_module: go vet found problems (see above)"
      continue
    fi
    if ! sync_raylib "$ct_dir"; then
      check fail "$ct_module: $raylib_error"
      continue
    fi
    # Test binaries load the raylib library when they start, so it must be on the search path.
    if (use_library_dir "$(dirname "$raylib_lib")" && "$go_exe" -C "$ct_dir" test ./...); then
      check ok "$ct_module: vet and tests passed"
    else
      check fail "$ct_module: tests failed (see above)"
    fi
  done
  summary test
  if [ "$failures" -gt 0 ]; then exit 1; fi
  exit 0
}

cmd_go() {
  if [ $# -eq 0 ]; then usage_error "go needs arguments, for example: golib go version"; fi
  assert_toolchain go
  exec "$go_exe" "$@"
}

removed=0

remove_dir() {
  if [ -e "$1" ]; then
    remove_tree "$1"
    printf 'removed %s/\n' "$(basename "$1")"
    removed=$((removed + 1))
  fi
}

cmd_clean() {
  all=false
  for option in "$@"; do
    case "$option" in
      --all) all=true ;;
      *) usage_error "unknown option for clean: $option" ;;
    esac
  done
  remove_dir "$build_dir"
  if [ "$all" = true ]; then remove_dir "$tools_dir"; fi
  if [ "$removed" -eq 0 ]; then echo 'Nothing to clean.'; fi
  exit 0
}

detect_platform
# Saved before set_go_environment redirects them, so games run with the user's own values.
user_home=${HOME-}
if [ "${XDG_CONFIG_HOME+set}" = set ]; then
  user_xdg_set=true
  user_xdg_config_home=$XDG_CONFIG_HOME
else
  user_xdg_set=false
  user_xdg_config_home=''
fi

command=${1:-help}
if [ $# -gt 0 ]; then shift; fi

case "$command" in
  setup) cmd_setup "$@" ;;
  doctor) cmd_doctor "$@" ;;
  build) cmd_build "$@" ;;
  run) cmd_run "$@" ;;
  test) cmd_test "$@" ;;
  go) cmd_go "$@" ;;
  clean) cmd_clean "$@" ;;
  help | -h | --help) show_help ;;
  *) usage_error "unknown command \"$command\"" ;;
esac
