# GoLib command-line tool: Windows implementation.
# Runs on Windows PowerShell 5.1 (built into Windows 10 and 11) and PowerShell 7+.
#
# Do not run this file directly. Entry points:
#   golib.cmd   from cmd and PowerShell (.\golib <command>)
#   golib       from Git Bash or MSYS2, which delegates here
#
# tools/bootstrap/golib.sh is the Linux/macOS twin of this file. Both must expose the same
# commands, options, output format and exit codes: change one, change the other.
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

$script:Failures = 0
$script:Warnings = 0
$script:UserAppData = $env:APPDATA

function Show-Help {
    Write-Host @'
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
    $env:GOFLAGS = '-tags=raylib_no_embed'    # load raylib from build/ or .tools/, never extract it to a user folder
    $env:PATH = (Join-Path $GoRoot 'bin') + ';' + $env:PATH
    # Go writes telemetry counters to the user's config folder (%APPDATA%\go\telemetry).
    # Redirect that folder into .tools/; Invoke-Run restores it before starting a game.
    $env:APPDATA = Join-Path $ToolsDir 'config'
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
        if ($games.Count -eq 0) { Stop-WithUsageError "no game named `"$($Options[0])`": games/ has no games yet" }
        Stop-WithUsageError "no game named `"$($Options[0])`" in games/ (available: $($games -join ', '))"
    }
    if ($games.Count -eq 1) { return $games[0] }
    if ($games.Count -eq 0) { Stop-WithUsageError 'there are no games in games/ yet' }
    Stop-WithUsageError "$Command needs a game name (available: $($games -join ', '))"
}

# Makes sure the raylib library matching a module's raylib-go version is extracted into
# .tools/raylib/<version>/, and returns the library's full path. Requires Set-GoEnvironment.
function Sync-Raylib([string]$ModuleDir) {
    $info = @(& $GoExe -C $ModuleDir list -m -f '{{.Version}}|{{.Dir}}' $RaylibModule)
    if ($LASTEXITCODE -ne 0 -or $info.Count -eq 0) {
        throw "cannot find $RaylibModule for $ModuleDir (run: golib setup)"
    }
    $version, $moduleCacheDir = ([string]$info[0]) -split '\|', 2
    $lib = Join-Path (Join-Path $RaylibDir $version) 'raylib.dll'
    if (Test-Path -LiteralPath $lib -PathType Leaf) { return $lib }

    if (-not $moduleCacheDir) { throw "$RaylibModule $version is not downloaded yet (run: golib setup)" }
    $pattern = @{ 'amd64' = 'raylib-*_win64_msvc16.tar.gz'; 'arm64' = 'raylib-*_winarm64_msvc16.tar.gz' }[(Get-GoArch)]
    $archive = @(Get-ChildItem -LiteralPath (Join-Path $moduleCacheDir 'libs') -Filter $pattern)
    if ($archive.Count -eq 0) { throw "$RaylibModule $version has no prebuilt library matching $pattern" }

    $target = Split-Path -Parent $lib
    New-Item -ItemType Directory -Force -Path $target | Out-Null
    # tar.exe ships with Windows 10 (1803) and later.
    & (Join-Path $env:SystemRoot 'System32\tar.exe') -xzf $archive[0].FullName -C $target | Out-Host
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $lib -PathType Leaf)) {
        throw "could not extract raylib.dll from $($archive[0].FullName)"
    }
    return $lib
}

# Builds games/<Game> into build/<Game>/ next to a copy of raylib.dll. Returns the executable
# path, or $null after printing a failure.
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
        $lib = Sync-Raylib $gameDir
    } catch {
        Write-Check fail $_.Exception.Message
        return $null
    }
    Copy-Item -LiteralPath $lib -Destination $outDir -Force
    Write-Check ok "built games/$Game into build/$Game/$Game.exe"
    return $exe
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
            $lib = Sync-Raylib $dir
            Write-Check ok ("{0}: modules downloaded, raylib library in .tools/raylib/{1}/" -f $module, (Split-Path -Leaf (Split-Path -Parent $lib)))
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
    foreach ($module in Get-Modules) {
        $goMod = Get-Content -Raw -LiteralPath (Join-Path $Root "$module\go.mod")
        $match = [regex]::Match($goMod, '(?m)^\s*(?:require\s+)?github\.com/gen2brain/raylib-go/raylib\s+(v\S+)')
        if (-not $match.Success) {
            Write-Check warn "${module}: go.mod does not require $RaylibModule"
        } elseif (Test-Path -LiteralPath (Join-Path $RaylibDir "$($match.Groups[1].Value)\raylib.dll") -PathType Leaf) {
            Write-Check ok "${module}: raylib library for raylib-go $($match.Groups[1].Value) ready"
        } else {
            Write-Check warn "${module}: raylib library for raylib-go $($match.Groups[1].Value) not ready (run: golib setup)"
        }
    }

    Write-Summary 'doctor'
    if ($script:Failures -gt 0) { exit 1 }
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

function Invoke-Test([string[]]$Options) {
    if ($Options.Count -gt 0) { Stop-WithUsageError "test takes no options (got: $($Options -join ' '))" }
    Assert-Toolchain 'test'
    $modules = @(Get-Modules)
    if ($modules.Count -eq 0) { Write-Check warn 'no Go modules to test' }
    foreach ($module in $modules) {
        $dir = Join-Path $Root $module
        & $GoExe -C $dir vet ./... | Out-Host
        if ($LASTEXITCODE -ne 0) {
            Write-Check fail "${module}: go vet found problems (see above)"
            continue
        }
        try {
            $lib = Sync-Raylib $dir
        } catch {
            Write-Check fail "${module}: $($_.Exception.Message)"
            continue
        }
        # Test binaries load raylib.dll when they start, so it must be on the search path.
        $savedPath = $env:PATH
        $env:PATH = (Split-Path -Parent $lib) + ';' + $env:PATH
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
    'build'  { Invoke-Build $options }
    'run'    { Invoke-Run $options }
    'test'   { Invoke-Test $options }
    'go'     { Invoke-GoCommand $options }
    'clean'  { Invoke-Clean $options }
    'help'   { Show-Help; exit 0 }
    '-h'     { Show-Help; exit 0 }
    '--help' { Show-Help; exit 0 }
    default  { Stop-WithUsageError "unknown command `"$command`"" }
}
