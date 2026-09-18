# GoLib command-line tool: Windows implementation.
# Runs on Windows PowerShell 5.1 (built into Windows 10 and 11) and PowerShell 7+.
#
# Do not run this file directly. Entry points:
#   golib.cmd   from cmd and PowerShell (.\golib <command>)
#   golib       from Git Bash or MSYS2, which delegates here
#
# tools/bootstrap/golib.sh is the Linux/macOS twin of this file. Both must expose the same
# commands, options, output format and exit codes: change one, change the other.
# Most commands live in tools/cli, a Go program that both scripts build and start (Invoke-Cli
# here): new, build, run, shot, test, dist and the end of setup. This file keeps what has to
# work before that program can be built: help, setup up to installing Go, doctor, go and clean.
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

$Root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$ToolsDir = Join-Path $Root '.tools'
$BuildDir = Join-Path $Root 'build'
$GoRoot = Join-Path $ToolsDir 'go'
$GoExe = Join-Path $GoRoot 'bin\go.exe'
$RaylibDir = Join-Path $ToolsDir 'raylib'
$FrameworkDir = Join-Path $Root 'framework'
$GamesDir = Join-Path $Root 'games'
# The Go side of golib, and where Invoke-Cli builds it. build/golib/ can't clash with a game's
# build folder: new refuses golib as a game name.
$CliDir = Join-Path $Root 'tools\cli'
$CliBuildDir = Join-Path $BuildDir 'golib'

$script:Failures = 0
$script:Warnings = 0
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
  dist [game] [--web]     Build games/<game> to share: a folder and its zip in build/<game>/dist/.
                          --web builds it for the browser instead, zipped for itch.io
  run [game] [--dist]     Build games/<game>, then run it from its folder. --dist builds and
                          runs the build players get, from build/<game>/dist/<game>/
  shot [game] [frame...]  Run games/<game> in a hidden window and save screenshots of the given
                          frames into build/<game>/shots/ (default: frame 60, one second in).
                          --input "Enter@1 Right@30-90 Mouse@100:640,360 MouseLeft@101" plays keyboard,
                          mouse and gamepad input in those updates.
                          --save <file.json> starts the game with that data saved, such as a
                          finished level; --scale <1-8> enlarges the pictures (see docs/tooling.md)
  test [game]             Vet and test the framework, every game and GoLib's Go tools,
                          or only games/<game>
  web [game] [--port n]   Build games/<game> for the browser into build/<game>/web/ and serve it
                          on this machine. --no-open keeps the browser closed. No sound or
                          post-processing shaders yet (see docs/roadmap.md)
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
    # Redirect that folder into .tools/; Restore-UserEnvironment puts it back.
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

# Returns the raylib-go version whose library is in .tools/raylib/, or $null. It is the first line
# of .tools/raylib/VERSION.
function Get-RaylibVersion {
    $versionFile = Join-Path $RaylibDir 'VERSION'
    if (-not (Test-Path -LiteralPath $versionFile -PathType Leaf)) { return $null }
    $first = @(Get-Content -LiteralPath $versionFile -TotalCount 1)
    if ($first.Count -eq 0) { return $null }
    return ([string]$first[0]).Trim()
}

# Returns where Invoke-Cli builds and starts tools/cli: build/golib/golib.exe, unless that program
# is running, as it is while a game started by run or shot is open. Windows can't replace a
# running program, and go build moves only one copy out of its way, so the next name that isn't
# in use is taken instead: golib-2.exe, golib-3.exe and so on. go build keeps each up to date.
function Get-CliExe {
    for ($i = 1; ; $i++) {
        $name = if ($i -eq 1) { 'golib.exe' } else { "golib-$i.exe" }
        $path = Join-Path $CliBuildDir $name
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { return $path }
        try {
            # A running program's file can't be opened for writing.
            [System.IO.File]::Open($path, 'Open', 'ReadWrite', 'None').Dispose()
            return $path
        } catch {
            continue
        }
    }
}

# Builds tools/cli into build/golib/ when its code has changed, then runs it with the command and
# its options, and exits with its exit code. The CLI gives the go commands it runs GoLib's
# environment itself, so it starts with the user's.
function Invoke-Cli([string]$Command, [string[]]$Options) {
    Assert-Toolchain $Command
    $exe = Get-CliExe
    # go build leaves an up-to-date executable alone, so this costs about a tenth of a second.
    & $GoExe -C $CliDir build -o $exe . | Out-Host
    if ($LASTEXITCODE -ne 0) {
        Write-Check fail 'could not build tools/cli, the part of golib written in Go (see the Go errors above)'
        Write-Summary $Command
        exit 1
    }
    Restore-UserEnvironment
    & $exe $Command @Options
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
    # tools/cli downloads the Go modules, fills .tools/raylib/ and prints the summary, which
    # counts the warnings printed so far.
    Invoke-Cli 'setup' @("--warnings=$script:Warnings")
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

function Invoke-GoCommand([string[]]$Options) {
    if ($Options.Count -eq 0) { Stop-WithUsageError 'go needs arguments, for example: golib go version' }
    Assert-Toolchain 'go'
    # go run and go test start programs that load raylib when they do, as games do. build, run,
    # shot and test put the libraries next to the executable; here they come from .tools/raylib/,
    # so that "golib go test ./..." and generators written against raylib work.
    if (Test-Path -LiteralPath $RaylibDir) { $env:PATH = $RaylibDir + ';' + $env:PATH }
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
            try {
                Remove-Item -LiteralPath $target -Recurse -Force
            } catch {
                # Windows can't delete the file of a running program, such as a game golib started.
                [Console]::Error.WriteLine($_.Exception.Message)
                [Console]::Error.WriteLine(('golib: cannot delete {0}/ completely (see the error above)' -f (Split-Path -Leaf $target)))
                [Console]::Error.WriteLine('Close the games that golib started, and any program running from that folder, then run clean again.')
                exit 1
            }
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
    'new'    { Invoke-Cli 'new' $options }
    'build'  { Invoke-Cli 'build' $options }
    'dist'   { Invoke-Cli 'dist' $options }
    'run'    { Invoke-Cli 'run' $options }
    'shot'   { Invoke-Cli 'shot' $options }
    'test'   { Invoke-Cli 'test' $options }
    'web'    { Invoke-Cli 'web' $options }
    'go'     { Invoke-GoCommand $options }
    'clean'  { Invoke-Clean $options }
    'help'   { Show-Help; exit 0 }
    '-h'     { Show-Help; exit 0 }
    '--help' { Show-Help; exit 0 }
    default  { Stop-WithUsageError "unknown command `"$command`"" }
}
