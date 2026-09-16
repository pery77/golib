# GoLib command-line tool: Windows implementation.
# Runs on Windows PowerShell 5.1 (built into Windows 10 and 11) and PowerShell 7+.
#
# Do not run this file directly. Entry points:
#   golib.cmd   from cmd and PowerShell (.\golib <command>)
#   golib       from Git Bash or MSYS2, which delegates here
#
# tools/bootstrap/golib.sh is the Linux/macOS twin of this file. Both must expose the same
# commands, options, output format and exit codes: change one, change the other.
# Commands are moving into tools/cli, a Go program that both scripts build and start
# (Invoke-Cli here); so far it runs dist.
# Keep this file ASCII-only: Windows PowerShell 5.1 reads files without a BOM as ANSI.
#
# Exit codes: 0 success, 1 failure, 2 usage error.

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version 3.0

# Pinned Go toolchain. golib.sh pins the same version for Linux and macOS: change both together.
# Checksums come from https://go.dev/dl/?mode=json
$GoVersion = '1.27.1'
$GoSha256 = @{
    'amd64' = 'a3911b5e0e1b1053f25ed0675f4c1c6aad1e2bfcf253df2b9be4caabd2edd95d'
    'arm64' = '13b69b87bb0e83f96bc68560a8cace7f0343b1e03469f1110ea18d17e3234069'
}

# The raylib binding. Its version is pinned in framework/go.mod; the prebuilt raylib library
# for each platform ships inside the module, in its libs/ folder.
$RaylibModule = 'github.com/gen2brain/raylib-go/raylib'
# raylib-go calls raylib through libffi. The ffi module ships libffi for Windows amd64 in its
# assets/libffi/ folder.
$FfiModule = 'github.com/jupiterrider/ffi'

# golib shot: the frame captured when none is given (one second of game time), and how many
# seconds a game may run before it is stopped.
$ShotDefaultFrame = 60
$ShotTimeoutSeconds = 120

$Root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$ToolsDir = Join-Path $Root '.tools'
$BuildDir = Join-Path $Root 'build'
$GoRoot = Join-Path $ToolsDir 'go'
$GoExe = Join-Path $GoRoot 'bin\go.exe'
$RaylibDir = Join-Path $ToolsDir 'raylib'
$FrameworkDir = Join-Path $Root 'framework'
$GamesDir = Join-Path $Root 'games'
$TemplateDir = Join-Path $Root 'tools\template\game'
# The Go side of golib, and where Invoke-Cli builds it. build/golib/ can't clash with a game's
# build folder: new refuses golib as a game name.
$CliDir = Join-Path $Root 'tools\cli'
$CliExe = Join-Path $BuildDir 'golib\golib.exe'
# GoLib's own Go programs. test checks them too; they don't use raylib.
$ToolModules = @('tools/cli')

$script:Failures = 0
$script:Warnings = 0
$script:UserAppData = $env:APPDATA
# The user's values of the variables Set-GoEnvironment changes, for Restore-UserEnvironment.
$script:UserEnvironment = @{}
foreach ($name in @('GOROOT', 'GOPATH', 'GOMODCACHE', 'GOCACHE', 'GOENV', 'GOTOOLCHAIN', 'CGO_ENABLED', 'GOFLAGS', 'PATH', 'APPDATA')) {
    $script:UserEnvironment[$name] = [Environment]::GetEnvironmentVariable($name)
}

function Show-Help {
    Write-Host @'
GoLib - make games in Go, powered by raylib.

Usage: golib <command> [options]

Commands:
  setup                   Check the environment, then install Go, Go modules and raylib into .tools/
  doctor                  Diagnose the environment without changing anything
  new <name>              Create games/<name>/, a small game ready to run, from tools/template/game/
  build [game]            Debug build of games/<game> into build/<game>/
  dist [game]             Build games/<game> to share: a folder and its zip in build/<game>/dist/
  run [game]              Build games/<game>, then run it from its folder
  shot [game] [frame...]  Run games/<game> in a hidden window and save screenshots of the given
                          frames into build/<game>/shots/ (default: frame 60, one second in).
                          --input "Enter@1 Right@30-90 Mouse@100:640,360 MouseLeft@101" plays keyboard,
                          mouse and gamepad input in those updates (see docs/tooling.md)
  test                    Vet and test the framework, every game and GoLib's Go tools
  go <args>               Run the project's Go toolchain, with GoLib's settings
  clean                   Delete build outputs (build/)
  clean --all             Also delete downloaded tools (.tools/); run setup again afterwards
  help                    Show this help

[game] is a folder name in games/. Leave it out when there is only one game.

Invoke from the project root:
  Windows (PowerShell, cmd)    .\golib <command>
  Linux, macOS, Git Bash       ./golib <command>
'@
}

# Prints one check result. Level is ok, info, warn or fail.
function Write-Check([string]$Level, [string]$Message) {
    if ($Level -eq 'warn') { $script:Warnings++ }
    if ($Level -eq 'fail') { $script:Failures++ }
    Write-Host ('{0,-6} {1}' -f "[$Level]", $Message)
}

function Write-Summary([string]$Command) {
    Write-Host ''
    Write-Host ('{0}: {1} failed, {2} warning(s)' -f $Command, $script:Failures, $script:Warnings)
}

function Stop-WithUsageError([string]$Message) {
    [Console]::Error.WriteLine("golib: $Message")
    [Console]::Error.WriteLine('Run "golib help" for usage.')
    exit 2
}

# Returns the Go name of this machine's architecture (amd64, arm64), or $null if unsupported.
function Get-GoArch {
    # A 32-bit PowerShell on 64-bit Windows reports the real architecture in PROCESSOR_ARCHITEW6432.
    $arch = $env:PROCESSOR_ARCHITEW6432
    if (-not $arch) { $arch = $env:PROCESSOR_ARCHITECTURE }
    return @{ 'AMD64' = 'amd64'; 'ARM64' = 'arm64' }[$arch]
}

function Remove-Tree([string]$Path) {
    if (Test-Path -LiteralPath $Path) { Remove-Item -LiteralPath $Path -Recurse -Force }
}

# Checks shared by setup and doctor. Only reports; never changes anything.
function Test-Environment {
    $goArch = Get-GoArch
    $os = [Environment]::OSVersion.Version
    if ($os.Major -lt 10) {
        Write-Check fail "Windows $($os.Major).$($os.Minor) is not supported (Windows 10 or later required)"
    } elseif (-not $goArch) {
        Write-Check fail "$env:PROCESSOR_ARCHITECTURE processors are not supported (64-bit x86 or ARM required)"
    } else {
        Write-Check ok "platform windows/$goArch (Windows build $($os.Build))"
    }

    $ps = $PSVersionTable.PSVersion
    if ($ps.Major -gt 5 -or ($ps.Major -eq 5 -and $ps.Minor -ge 1)) {
        Write-Check ok "PowerShell $ps"
    } else {
        Write-Check fail "PowerShell $ps is too old (5.1 or later required)"
    }

    Write-Check info "project root $Root"

    foreach ($syncRoot in @($env:OneDrive, $env:OneDriveCommercial, $env:OneDriveConsumer)) {
        if ($syncRoot -and $Root.StartsWith($syncRoot, [StringComparison]::OrdinalIgnoreCase)) {
            Write-Check warn "project is inside OneDrive ($syncRoot): syncing downloaded tools is slow and can lock files. Prefer a local folder such as C:\dev"
            break
        }
    }

    $probe = Join-Path $Root ('.golib-write-test-' + [Guid]::NewGuid().ToString('N'))
    try {
        [System.IO.File]::WriteAllText($probe, '')
        [System.IO.File]::Delete($probe)
        Write-Check ok 'project folder is writable'
    } catch {
        Write-Check fail "cannot write to the project folder: $($_.Exception.Message)"
    }

    if (Get-Command git -ErrorAction SilentlyContinue) {
        $gitVersion = (git --version) -replace '^git version\s*', ''
        Write-Check ok "git $gitVersion"
    } else {
        Write-Check info 'git not found (optional: only needed to clone or update the repository)'
    }
}

# --- Go toolchain -------------------------------------------------------------------------

# Returns the version of the Go toolchain in .tools/go/ (such as 1.27.1), or $null.
function Get-InstalledGoVersion {
    $versionFile = Join-Path $GoRoot 'VERSION'
    if (-not (Test-Path -LiteralPath $versionFile -PathType Leaf)) { return $null }
    $first = @(Get-Content -LiteralPath $versionFile -TotalCount 1)
    if ($first.Count -eq 0) { return $null }
    return ([string]$first[0]) -replace '^go', ''
}

function Save-Url([string]$Url, [string]$Path) {
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    # The progress bar slows Invoke-WebRequest down by an order of magnitude on PowerShell 5.1.
    $ProgressPreference = 'SilentlyContinue'
    $partial = "$Path.partial"
    try {
        Invoke-WebRequest -UseBasicParsing -Uri $Url -OutFile $partial
    } catch {
        Remove-Tree $partial
        throw "download failed: $Url ($($_.Exception.Message))"
    }
    Move-Item -LiteralPath $partial -Destination $Path -Force
}

function Install-Go {
    $installed = Get-InstalledGoVersion
    if ($installed -eq $GoVersion) {
        Write-Check ok "Go $GoVersion already installed in .tools/go/"
        return
    }
    $goArch = Get-GoArch
    $file = "go$GoVersion.windows-$goArch.zip"
    $url = "https://go.dev/dl/$file"
    $downloads = Join-Path $ToolsDir 'downloads'
    New-Item -ItemType Directory -Force -Path $downloads | Out-Null
    $archive = Join-Path $downloads $file

    Write-Check info "downloading $url"
    try {
        Save-Url $url $archive
    } catch {
        Remove-Tree $downloads
        throw
    }
    $hash = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($hash -ne $GoSha256[$goArch]) {
        Remove-Tree $downloads
        throw "checksum mismatch for $file (expected $($GoSha256[$goArch]), got $hash). The download was deleted; run setup again"
    }

    # Extract next to the final location, then swap, so an interrupted setup never leaves a broken .tools/go/.
    $staging = Join-Path $ToolsDir 'go.partial'
    Remove-Tree $staging
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::ExtractToDirectory($archive, $staging)
    Remove-Tree $GoRoot
    Move-Item -LiteralPath (Join-Path $staging 'go') -Destination $GoRoot
    Remove-Tree $staging
    Remove-Tree $downloads
    if ($installed) {
        Write-Check ok "Go $GoVersion installed in .tools/go/ (replaced $installed)"
    } else {
        Write-Check ok "Go $GoVersion installed in .tools/go/"
    }
}

# Points Go at the toolchain and caches in .tools/, for this process and the programs it starts.
# Nothing outside the project changes, and nothing persists after golib exits.
function Set-GoEnvironment {
    $env:GOROOT = $GoRoot
    $env:GOPATH = Join-Path $ToolsDir 'gopath'
    $env:GOMODCACHE = Join-Path $ToolsDir 'gopath\pkg\mod'
    $env:GOCACHE = Join-Path $ToolsDir 'gocache'
    $env:GOENV = 'off'                        # ignore any global "go env -w" settings
    $env:GOTOOLCHAIN = 'local'                # never download a different Go version
    $env:CGO_ENABLED = '0'                    # raylib-go without a C compiler
    # Debug builds load raylib and libffi from build/ or .tools/ instead of extracting them into
    # a user folder. golib dist replaces these tags, so dist builds embed both.
    $env:GOFLAGS = '-tags=raylib_no_embed,ffi_no_embed'
    $env:PATH = (Join-Path $GoRoot 'bin') + ';' + $env:PATH
    # Go writes telemetry counters to the user's config folder (%APPDATA%\go\telemetry).
    # Redirect that folder into .tools/; Invoke-Run restores it before starting a game.
    $env:APPDATA = Join-Path $ToolsDir 'config'
}

# Undoes Set-GoEnvironment: puts back the user's values, and removes the variables the user didn't have.
function Restore-UserEnvironment {
    foreach ($name in $script:UserEnvironment.Keys) {
        [Environment]::SetEnvironmentVariable($name, $script:UserEnvironment[$name])
    }
}

# Prints a failure and exits unless the pinned Go toolchain is installed; then sets up its environment.
function Assert-Toolchain([string]$Command) {
    $installed = Get-InstalledGoVersion
    if ($installed -ne $GoVersion) {
        if ($installed) {
            Write-Check fail "Go $installed is in .tools/go/, but GoLib needs $GoVersion (run: golib setup)"
        } else {
            Write-Check fail 'the Go toolchain is not installed (run: golib setup)'
        }
        Write-Summary $Command
        exit 1
    }
    Set-GoEnvironment
}

# --- Modules, games and raylib --------------------------------------------------------------

# Game names: folders in games/ that contain a go.mod.
function Get-Games {
    if (-not (Test-Path -LiteralPath $GamesDir -PathType Container)) { return @() }
    return @(Get-ChildItem -LiteralPath $GamesDir -Directory |
        Where-Object { Test-Path -LiteralPath (Join-Path $_.FullName 'go.mod') -PathType Leaf } |
        ForEach-Object { $_.Name } | Sort-Object)
}

# Go modules in the project, as paths relative to the root: framework first, then each game.
function Get-Modules {
    $modules = @()
    if (Test-Path -LiteralPath (Join-Path $FrameworkDir 'go.mod') -PathType Leaf) { $modules += 'framework' }
    foreach ($game in Get-Games) { $modules += "games/$game" }
    return $modules
}

function Resolve-Game([string]$Command, [string[]]$Options) {
    if ($Options.Count -gt 1) { Stop-WithUsageError "$Command takes at most one game name (got: $($Options -join ' '))" }
    $games = @(Get-Games)
    if ($Options.Count -eq 1) {
        $match = @($games | Where-Object { $_ -eq $Options[0] })
        if ($match.Count -eq 1) { return $match[0] }
        if ($games.Count -eq 0) { Stop-WithUsageError "no game named `"$($Options[0])`": games/ has no games yet (create one: golib new <name>)" }
        Stop-WithUsageError "no game named `"$($Options[0])`" in games/ (available: $($games -join ', '))"
    }
    if ($games.Count -eq 1) { return $games[0] }
    if ($games.Count -eq 0) { Stop-WithUsageError 'there are no games in games/ yet (create one: golib new <name>)' }
    Stop-WithUsageError "$Command needs a game name (available: $($games -join ', '))"
}

# Returns the raylib-go version whose library is in .tools/raylib/, or $null. It is the first line
# of .tools/raylib/VERSION.
function Get-RaylibVersion {
    $versionFile = Join-Path $RaylibDir 'VERSION'
    if (-not (Test-Path -LiteralPath $versionFile -PathType Leaf)) { return $null }
    $first = @(Get-Content -LiteralPath $versionFile -TotalCount 1)
    if ($first.Count -eq 0) { return $null }
    return ([string]$first[0]).Trim()
}

# Makes sure .tools/raylib/ holds the libraries that a module's debug builds load: raylib, from the
# module's raylib-go version, and libffi, from its ffi version (ffi ships it for amd64 only).
# VERSION records both versions. Returns the libraries' full paths. The folder has no version in
# its name, so editor settings can point at it. Requires Set-GoEnvironment.
function Sync-Raylib([string]$ModuleDir) {
    $modules = @{}
    foreach ($line in @(& $GoExe -C $ModuleDir list -m -f '{{.Path}}|{{.Version}}|{{.Dir}}' $RaylibModule $FfiModule)) {
        $path, $version, $dir = ([string]$line) -split '\|', 3
        $modules[$path] = @{ Version = $version; Dir = $dir }
    }
    if ($LASTEXITCODE -ne 0 -or -not $modules.ContainsKey($RaylibModule) -or -not $modules.ContainsKey($FfiModule)) {
        throw "cannot find $RaylibModule and $FfiModule for $ModuleDir (run: golib setup)"
    }
    $raylib = $modules[$RaylibModule]
    $ffi = $modules[$FfiModule]
    $lib = Join-Path $RaylibDir 'raylib.dll'
    $libs = @($lib)
    $ffiSource = $null
    if ((Get-GoArch) -eq 'amd64') {
        $ffiSource = 'assets\libffi\windows_amd64\libffi-8.dll'
        $libs += Join-Path $RaylibDir 'libffi-8.dll'
    }
    $versionFile = Join-Path $RaylibDir 'VERSION'
    $versionText = "$($raylib.Version)`nffi $($ffi.Version)`n"
    $ready = (Test-Path -LiteralPath $versionFile -PathType Leaf) -and ([System.IO.File]::ReadAllText($versionFile) -eq $versionText)
    foreach ($path in $libs) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { $ready = $false }
    }
    if ($ready) { return $libs }

    if (-not $raylib.Dir) { throw "$RaylibModule $($raylib.Version) is not downloaded yet (run: golib setup)" }
    if ($ffiSource -and -not $ffi.Dir) { throw "$FfiModule $($ffi.Version) is not downloaded yet (run: golib setup)" }
    $pattern = @{ 'amd64' = 'raylib-*_win64_msvc16.tar.gz'; 'arm64' = 'raylib-*_winarm64_msvc16.tar.gz' }[(Get-GoArch)]
    $archive = @(Get-ChildItem -LiteralPath (Join-Path $raylib.Dir 'libs') -Filter $pattern)
    if ($archive.Count -eq 0) { throw "$RaylibModule $($raylib.Version) has no prebuilt library matching $pattern" }

    Remove-Tree $RaylibDir
    New-Item -ItemType Directory -Force -Path $RaylibDir | Out-Null
    # tar.exe ships with Windows 10 (1803) and later.
    & (Join-Path $env:SystemRoot 'System32\tar.exe') -xzf $archive[0].FullName -C $RaylibDir | Out-Host
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $lib -PathType Leaf)) {
        throw "could not extract raylib.dll from $($archive[0].FullName)"
    }
    if ($ffiSource) {
        $ffiLib = Join-Path $ffi.Dir $ffiSource
        if (-not (Test-Path -LiteralPath $ffiLib -PathType Leaf)) { throw "$FfiModule $($ffi.Version) has no $ffiSource" }
        $copy = Copy-Item -LiteralPath $ffiLib -Destination $RaylibDir -PassThru
        $copy.IsReadOnly = $false    # Go's module cache is read-only; the copy doesn't need to be
    }
    [System.IO.File]::WriteAllText($versionFile, $versionText)
    return $libs
}

# Builds games/<Game> as a debug build into build/<Game>/, next to copies of the libraries it
# loads. Returns the executable path, or $null after printing a failure.
function New-GameBuild([string]$Game) {
    $gameDir = Join-Path $GamesDir $Game
    $outDir = Join-Path $BuildDir $Game
    $exe = Join-Path $outDir "$Game.exe"
    New-Item -ItemType Directory -Force -Path $outDir | Out-Null
    & $GoExe -C $gameDir build -o $exe . | Out-Host
    if ($LASTEXITCODE -ne 0) {
        Write-Check fail "build failed for games/$Game (see the Go errors above)"
        return $null
    }
    try {
        $libs = @(Sync-Raylib $gameDir)
    } catch {
        Write-Check fail $_.Exception.Message
        return $null
    }
    foreach ($lib in $libs) { Copy-Item -LiteralPath $lib -Destination $outDir -Force }
    Write-Check ok "built games/$Game into build/$Game/$Game.exe"
    return $exe
}

# Builds tools/cli into build/golib/ when its code has changed, then runs it with the command and
# its options, and exits with its exit code. The CLI gives the go commands it runs GoLib's
# environment itself, so it starts with the user's.
function Invoke-Cli([string]$Command, [string[]]$Options) {
    Assert-Toolchain $Command
    # go build leaves an up-to-date executable alone, so this costs about a tenth of a second.
    & $GoExe -C $CliDir build -o $CliExe . | Out-Host
    if ($LASTEXITCODE -ne 0) {
        Write-Check fail 'could not build tools/cli, the part of golib written in Go (see the Go errors above)'
        Write-Summary $Command
        exit 1
    }
    Restore-UserEnvironment
    & $CliExe $Command @Options
    exit $LASTEXITCODE
}

# --- Commands -------------------------------------------------------------------------------

function Invoke-Setup([string[]]$Options) {
    if ($Options.Count -gt 0) { Stop-WithUsageError "setup takes no options (got: $($Options -join ' '))" }
    Test-Environment
    if ($script:Failures -gt 0) {
        Write-Summary 'setup'
        Write-Host 'Setup stopped. Fix the [fail] items above, then run setup again.'
        exit 1
    }
    if (-not (Test-Path -LiteralPath $ToolsDir -PathType Container)) {
        New-Item -ItemType Directory -Path $ToolsDir | Out-Null
    }
    Write-Check ok 'local tools folder ready: .tools/'

    try {
        Install-Go
    } catch {
        Write-Check fail $_.Exception.Message
        Write-Summary 'setup'
        exit 1
    }
    Set-GoEnvironment

    $modules = @(Get-Modules)
    if ($modules.Count -eq 0) {
        Write-Check info 'no Go modules yet: nothing more to download'
    }
    foreach ($module in $modules) {
        $dir = Join-Path $Root $module
        & $GoExe -C $dir mod download | Out-Host
        if ($LASTEXITCODE -ne 0) {
            Write-Check fail "could not download the Go modules for $module (see the errors above)"
            continue
        }
        try {
            $null = Sync-Raylib $dir
            Write-Check ok "${module}: modules downloaded, raylib library for raylib-go $(Get-RaylibVersion) in .tools/raylib/"
        } catch {
            Write-Check fail "${module}: $($_.Exception.Message)"
        }
    }

    Write-Summary 'setup'
    if ($script:Failures -gt 0) {
        Write-Host 'Setup incomplete. Fix the [fail] items above, then run setup again.'
        exit 1
    }
    Write-Host 'Setup complete.'
    exit 0
}

function Invoke-Doctor([string[]]$Options) {
    if ($Options.Count -gt 0) { Stop-WithUsageError "doctor takes no options (got: $($Options -join ' '))" }
    Test-Environment
    if (Test-Path -LiteralPath $ToolsDir -PathType Container) {
        Write-Check ok '.tools/ exists'
    } else {
        Write-Check info '.tools/ not created yet (run: golib setup)'
    }

    $installed = Get-InstalledGoVersion
    if ($installed -eq $GoVersion) {
        Write-Check ok "Go $GoVersion in .tools/go/"
    } elseif ($installed) {
        Write-Check warn "Go $installed is in .tools/go/, but GoLib needs $GoVersion (run: golib setup)"
    } else {
        Write-Check warn 'Go toolchain not installed (run: golib setup)'
    }

    $games = @(Get-Games)
    if (Test-Path -LiteralPath (Join-Path $FrameworkDir 'go.mod') -PathType Leaf) {
        Write-Check ok 'framework module in framework/'
    } else {
        Write-Check fail 'framework/go.mod is missing: the framework is not in this project'
    }
    if ($games.Count -gt 0) {
        Write-Check info "games in games/: $($games -join ', ')"
    } else {
        Write-Check info 'no games in games/ yet'
    }

    # Read go.mod as text instead of asking Go, so doctor starts nothing and changes nothing.
    $libNames = @('raylib.dll')
    if ((Get-GoArch) -eq 'amd64') { $libNames += 'libffi-8.dll' }
    $missingLibs = @($libNames | Where-Object { -not (Test-Path -LiteralPath (Join-Path $RaylibDir $_) -PathType Leaf) })
    foreach ($module in Get-Modules) {
        $goMod = Get-Content -Raw -LiteralPath (Join-Path $Root "$module\go.mod")
        $match = [regex]::Match($goMod, '(?m)^\s*(?:require\s+)?github\.com/gen2brain/raylib-go/raylib\s+(v\S+)')
        if (-not $match.Success) {
            Write-Check warn "${module}: go.mod does not require $RaylibModule"
        } elseif ((Get-RaylibVersion) -eq $match.Groups[1].Value -and $missingLibs.Count -eq 0) {
            Write-Check ok "${module}: raylib library for raylib-go $($match.Groups[1].Value) ready"
        } else {
            Write-Check warn "${module}: raylib library for raylib-go $($match.Groups[1].Value) not ready (run: golib setup)"
        }
    }

    Write-Summary 'doctor'
    if ($script:Failures -gt 0) { exit 1 }
    exit 0
}

# new <name>: creates games/<name>/ from the templates in tools/template/game/.
function Invoke-New([string[]]$Options) {
    if ($Options.Count -ne 1) { Stop-WithUsageError 'new needs one game name, for example: golib new asteroids' }
    $name = $Options[0]
    if ($name -cnotmatch '^[a-z][a-z0-9_-]{0,31}$') {
        Stop-WithUsageError "invalid game name `"$name`": use 1 to 32 lowercase letters, digits, - and _, starting with a letter"
    }
    # golib would clash with the framework's import path; the others are device names on Windows.
    if ($name -match '^(golib|con|prn|aux|nul|com[1-9]|lpt[1-9])$') { Stop-WithUsageError "the game name `"$name`" is reserved: pick another one" }
    $gameDir = Join-Path $GamesDir $name
    if (Test-Path -LiteralPath $gameDir) { Stop-WithUsageError "games/$name already exists: pick another name, or delete that folder first" }
    Assert-Toolchain 'new'

    New-Item -ItemType Directory -Force -Path $gameDir | Out-Null
    $utf8 = New-Object System.Text.UTF8Encoding $false
    $today = Get-Date -Format 'yyyy-MM-dd'
    foreach ($template in @(Get-ChildItem -LiteralPath $TemplateDir -File -Filter '*.tmpl')) {
        $text = [System.IO.File]::ReadAllText($template.FullName).Replace('{{name}}', $name).Replace('{{go}}', $GoVersion).Replace('{{date}}', $today)
        [System.IO.File]::WriteAllText((Join-Path $gameDir $template.BaseName), $text, $utf8)
    }
    # The framework's checksums cover the modules a new game needs, so tidy doesn't have to look them up.
    Copy-Item -LiteralPath (Join-Path $FrameworkDir 'go.sum') -Destination (Join-Path $gameDir 'go.sum')
    & $GoExe -C $gameDir mod tidy | Out-Host
    if ($LASTEXITCODE -ne 0) {
        Remove-Tree $gameDir
        Write-Check fail "go mod tidy failed for games/$name (see the Go errors above); the folder was deleted, so new can run again"
        Write-Summary 'new'
        exit 1
    }
    Write-Check ok "created games/$name/ from tools/template/game/"
    Write-Check info "next: golib run $name, and describe the game in games/$name/DESIGN.md"
    Write-Summary 'new'
    exit 0
}

function Invoke-Build([string[]]$Options) {
    $game = Resolve-Game 'build' $Options
    Assert-Toolchain 'build'
    $exe = New-GameBuild $game
    Write-Summary 'build'
    if (-not $exe) { exit 1 }
    exit 0
}

function Invoke-Run([string[]]$Options) {
    $game = Resolve-Game 'run' $Options
    Assert-Toolchain 'run'
    $exe = New-GameBuild $game
    if (-not $exe) {
        Write-Summary 'run'
        exit 1
    }
    Write-Check info "running build/$game/$game.exe with games/$game/ as the working directory"
    $env:APPDATA = $script:UserAppData
    Push-Location -LiteralPath (Join-Path $GamesDir $game)
    try {
        & $exe
        $code = $LASTEXITCODE
    } finally {
        Pop-Location
    }
    Write-Host ''
    Write-Host "run: $game exited with code $code"
    if ($code -ne 0) { exit 1 }
    exit 0
}

# shot [game] [frame...] [--input <script>]: numbers are frames, anything else names the game.
function Invoke-Shot([string[]]$Options) {
    $names = @()
    $frames = @()
    $inputScript = ''    # not $input: PowerShell reserves that name
    for ($i = 0; $i -lt $Options.Count; $i++) {
        $option = $Options[$i]
        if ($option -eq '') {
            Stop-WithUsageError 'shot got an empty argument'
        } elseif ($option -eq '--input') {
            if ($i + 1 -ge $Options.Count) { Stop-WithUsageError 'shot --input needs input to play, for example: --input "Enter@1 Right@30-90"' }
            $i++
            $inputScript = $Options[$i]
        } elseif ($option.StartsWith('--input=')) {
            $inputScript = $option.Substring('--input='.Length)
        } elseif ($option.StartsWith('-')) {
            Stop-WithUsageError "unknown option for shot: $option"
        } elseif ($option -match '^[0-9]+$') {
            if ($option.Length -gt 6 -or [int]$option -lt 1) { Stop-WithUsageError "frame numbers go from 1 to 999999 (got: $option)" }
            $frames += [int]$option
        } else {
            $names += $option
        }
    }
    if ($frames.Count -eq 0) { $frames = @($ShotDefaultFrame) }
    $frames = @($frames | Sort-Object -Unique)
    $game = Resolve-Game 'shot' $names
    Assert-Toolchain 'shot'
    $exe = New-GameBuild $game
    if (-not $exe) {
        Write-Summary 'shot'
        exit 1
    }

    $shotsDir = Join-Path (Join-Path $BuildDir $game) 'shots'
    Remove-Tree $shotsDir
    New-Item -ItemType Directory -Force -Path $shotsDir | Out-Null
    $playing = if ($inputScript) { ", playing $inputScript" } else { '' }
    Write-Check info "running $game for $($frames[-1]) frame(s) in a hidden window$playing"
    $env:APPDATA = $script:UserAppData
    $env:GOLIB_SHOT_DIR = $shotsDir
    $env:GOLIB_SHOT_FRAMES = $frames -join ','
    $env:GOLIB_SHOT_INPUT = $inputScript    # an empty value removes the variable
    $process = Start-Process -FilePath $exe -WorkingDirectory (Join-Path $GamesDir $game) -NoNewWindow -PassThru
    $null = $process.Handle    # keeps the exit code readable after the process ends
    # Stop a game that never finishes, so whoever waits for shot isn't stuck.
    if (-not $process.WaitForExit($ShotTimeoutSeconds * 1000)) {
        $process.Kill()
        Write-Check fail "$game didn't finish within $ShotTimeoutSeconds seconds and was stopped (does Update or Draw loop forever?)"
    } elseif ($process.ExitCode -ne 0) {
        Write-Check fail "$game exited with code $($process.ExitCode) (see its output above)"
    }
    foreach ($frame in $frames) {
        $name = 'frame-{0:D6}.png' -f $frame
        if (Test-Path -LiteralPath (Join-Path $shotsDir $name) -PathType Leaf) {
            Write-Check ok "frame ${frame}: build/$game/shots/$name"
        } else {
            Write-Check fail "frame ${frame}: no screenshot was saved"
        }
    }
    Write-Summary 'shot'
    if ($script:Failures -gt 0) { exit 1 }
    exit 0
}

function Invoke-Test([string[]]$Options) {
    if ($Options.Count -gt 0) { Stop-WithUsageError "test takes no options (got: $($Options -join ' '))" }
    Assert-Toolchain 'test'
    $modules = @(Get-Modules) + @($ToolModules | Where-Object { Test-Path -LiteralPath (Join-Path $Root "$_\go.mod") -PathType Leaf })
    if ($modules.Count -eq 0) { Write-Check warn 'no Go modules to test' }
    foreach ($module in $modules) {
        $dir = Join-Path $Root $module
        & $GoExe -C $dir vet ./... | Out-Host
        if ($LASTEXITCODE -ne 0) {
            Write-Check fail "${module}: go vet found problems (see above)"
            continue
        }
        $usesRaylib = $ToolModules -notcontains $module
        if ($usesRaylib) {
            try {
                $null = Sync-Raylib $dir
            } catch {
                Write-Check fail "${module}: $($_.Exception.Message)"
                continue
            }
        }
        # Test binaries load raylib.dll and libffi-8.dll when they start, so .tools/raylib/ must be
        # on the search path.
        $savedPath = $env:PATH
        if ($usesRaylib) { $env:PATH = $RaylibDir + ';' + $env:PATH }
        try {
            & $GoExe -C $dir test ./... | Out-Host
            $code = $LASTEXITCODE
        } finally {
            $env:PATH = $savedPath
        }
        if ($code -ne 0) {
            Write-Check fail "${module}: tests failed (see above)"
        } else {
            Write-Check ok "${module}: vet and tests passed"
        }
    }
    Write-Summary 'test'
    if ($script:Failures -gt 0) { exit 1 }
    exit 0
}

function Invoke-GoCommand([string[]]$Options) {
    if ($Options.Count -eq 0) { Stop-WithUsageError 'go needs arguments, for example: golib go version' }
    Assert-Toolchain 'go'
    & $GoExe @Options
    exit $LASTEXITCODE
}

function Invoke-Clean([string[]]$Options) {
    $all = $false
    foreach ($option in $Options) {
        if ($option -eq '--all') { $all = $true } else { Stop-WithUsageError "unknown option for clean: $option" }
    }
    $targets = @($BuildDir)
    if ($all) { $targets += $ToolsDir }
    $removed = 0
    foreach ($target in $targets) {
        if (Test-Path -LiteralPath $target) {
            Remove-Item -LiteralPath $target -Recurse -Force
            Write-Host ('removed {0}/' -f (Split-Path -Leaf $target))
            $removed++
        }
    }
    if ($removed -eq 0) { Write-Host 'Nothing to clean.' }
    exit 0
}

$command = if ($args.Count -gt 0) { [string]$args[0] } else { 'help' }
$options = @(if ($args.Count -gt 1) { $args[1..($args.Count - 1)] })

switch -CaseSensitive ($command) {
    'setup'  { Invoke-Setup $options }
    'doctor' { Invoke-Doctor $options }
    'new'    { Invoke-New $options }
    'build'  { Invoke-Build $options }
    'dist'   { Invoke-Cli 'dist' $options }
    'run'    { Invoke-Run $options }
    'shot'   { Invoke-Shot $options }
    'test'   { Invoke-Test $options }
    'go'     { Invoke-GoCommand $options }
    'clean'  { Invoke-Clean $options }
    'help'   { Show-Help; exit 0 }
    '-h'     { Show-Help; exit 0 }
    '--help' { Show-Help; exit 0 }
    default  { Stop-WithUsageError "unknown command `"$command`"" }
}
