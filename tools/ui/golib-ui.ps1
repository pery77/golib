# GoLib window: buttons for the golib commands, for people who would rather click than type.
# Windows only: it uses Windows PowerShell 5.1 and WPF, both built into Windows 10 and 11.
#
# Start it by double-clicking golib-ui.cmd in the project root.
#
# The window holds no build logic. Every command button runs tools/bootstrap/golib.ps1, exactly
# like typing ".\golib <command>", and shows its output. To add a button, add an entry to $Actions.
# Keep this file ASCII-only: Windows PowerShell 5.1 reads files without a BOM as ANSI.

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version 3.0

Add-Type -AssemblyName PresentationFramework, PresentationCore, WindowsBase

$Root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$Cli = Join-Path $Root 'tools\bootstrap\golib.ps1'
$GamesDir = Join-Path $Root 'games'
$PowerShellExe = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'

# The buttons, one row per group, in this order. Command is golib's arguments: {game} becomes the
# game picked in the list, {frames} the frame numbers typed in the box, {input} the input
# typed in the other box, as --input, and {name} the answer to Prompt, asked in a small dialog.
# Folder opens a folder in Explorer instead. Confirm asks before running.
$Actions = @(
    @{ Group = 'Play'; Label = 'Run'; Command = 'run {game}'; Tip = 'Build the game and play it' }
    @{ Group = 'Play'; Label = 'Screenshots'; Command = 'shot {game} {frames} {input}'; Tip = 'Run the game in a hidden window, playing the input in the box, and save screenshots of the frames in the box (60 frames are one second)' }
    @{ Group = 'Build'; Label = 'Debug build'; Command = 'build {game}'; Tip = 'Build into build\<game>\, next to the raylib libraries. Started from Explorer, it reads games\<game>\assets\, and shows errors in a message box because its console window closes when the game ends' }
    @{ Group = 'Build'; Label = 'Dist build'; Command = 'dist {game}'; Tip = 'Build the game to share into build\<game>\dist\: a folder with the executable, the libraries it loads and THIRD-PARTY-LICENSES.txt, and a zip of it. It takes its icon from icon.png and its title, version and author from game.json, in the game''s folder' }
    @{ Group = 'Check'; Label = 'Test'; Command = 'test'; Tip = 'Vet and test the framework, every game and GoLib''s own Go program' }
    @{ Group = 'Check'; Label = 'Doctor'; Command = 'doctor'; Tip = 'Diagnose the environment without changing anything' }
    @{ Group = 'Open'; Label = 'Game folder'; Folder = 'games\{game}'; Tip = 'Open the game''s folder: its code, DESIGN.md and assets' }
    @{ Group = 'Open'; Label = 'Screenshots folder'; Folder = 'build\{game}\shots'; Tip = 'Open the screenshots from the latest Screenshots' }
    @{ Group = 'Open'; Label = 'Dist folder'; Folder = 'build\{game}\dist'; Tip = 'Open the zip to share, and its folder, from the latest Dist build' }
    @{ Group = 'Tools'; Label = 'New game'; Command = 'new {name}'; Prompt = 'Name of the new game: lowercase letters, digits, - and _, such as asteroids'; Tip = 'Create games\<name>\, a small game ready to run' }
    @{ Group = 'Tools'; Label = 'Setup'; Command = 'setup'; Tip = 'Install Go, the Go modules and raylib into .tools\ (safe to run again)' }
    @{ Group = 'Tools'; Label = 'Clean'; Command = 'clean'; Tip = 'Delete the build outputs in build\' }
    @{ Group = 'Tools'; Label = 'Clean all'; Command = 'clean --all'; Tip = 'Also delete the downloaded tools in .tools\; run Setup again afterwards'; Confirm = 'Delete build\ and the downloaded tools in .tools\? You will need to press Setup again.' }
)

[xml]$Xaml = @'
<Window xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation"
        xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml"
        Title="GoLib" Width="960" Height="680" MinWidth="640" MinHeight="420"
        WindowStartupLocation="CenterScreen" FontSize="13">
  <DockPanel Margin="14">
    <WrapPanel DockPanel.Dock="Top" Margin="0,0,0,12">
      <TextBlock Text="Game" Width="60" VerticalAlignment="Center"/>
      <ComboBox x:Name="GameList" MinWidth="200" VerticalContentAlignment="Center"/>
      <Button x:Name="RefreshButton" Content="Refresh" Margin="6,0,0,0" Padding="12,4"
              ToolTip="Look for games in games\ again"/>
      <TextBlock Text="Screenshot frames" Margin="24,0,8,0" VerticalAlignment="Center"/>
      <TextBox x:Name="FramesBox" Text="60" Width="110" VerticalContentAlignment="Center"
               ToolTip="Frame numbers for Screenshots, separated by spaces, such as: 1 60 300"/>
      <TextBlock Text="input" Margin="12,0,8,0" VerticalAlignment="Center"/>
      <TextBox x:Name="InputBox" Width="260" VerticalContentAlignment="Center"
               ToolTip="Input to play in Screenshots, such as: Enter@1 Right@30-90 Mouse@100:640,360 MouseLeft@101 GamepadA@120 (Name@update holds a key or button for one update, Name@first-last for a range, Mouse@update:x,y moves the pointer; see docs/tooling.md for the wheel and sticks)"/>
    </WrapPanel>
    <StackPanel x:Name="ActionRows" DockPanel.Dock="Top"/>
    <DockPanel DockPanel.Dock="Top" Margin="0,6,0,8">
      <Button x:Name="StopButton" DockPanel.Dock="Right" Content="Stop" Padding="16,4" IsEnabled="False"
              ToolTip="Stop the running command, and the game if it is running"/>
      <TextBlock x:Name="StatusText" VerticalAlignment="Center" FontWeight="SemiBold" TextWrapping="Wrap"/>
    </DockPanel>
    <TextBox x:Name="OutputBox" IsReadOnly="True" FontFamily="Consolas" FontSize="12"
             TextWrapping="NoWrap" VerticalScrollBarVisibility="Auto" HorizontalScrollBarVisibility="Auto"/>
  </DockPanel>
</Window>
'@

$Window = [Windows.Markup.XamlReader]::Load((New-Object System.Xml.XmlNodeReader $Xaml))
$GameList = $Window.FindName('GameList')
$RefreshButton = $Window.FindName('RefreshButton')
$FramesBox = $Window.FindName('FramesBox')
$InputBox = $Window.FindName('InputBox')
$ActionRows = $Window.FindName('ActionRows')
$StopButton = $Window.FindName('StopButton')
$StatusText = $Window.FindName('StatusText')
$OutputBox = $Window.FindName('OutputBox')

$ActionButtons = New-Object System.Collections.Generic.List[System.Windows.Controls.Button]
$Timer = New-Object System.Windows.Threading.DispatcherTimer
$Timer.Interval = [TimeSpan]::FromMilliseconds(50)

# The running golib process, the readers for its output streams, and what it is running.
$script:Process = $null
$script:Streams = @()
$script:CommandName = ''
$script:Stopped = $false
$script:SelectAfter = $null    # a game to pick in the list once the command succeeds, such as a new one

# Runs a script block from an event handler. Errors show in a message box instead of closing the
# window.
function Invoke-Safely([scriptblock]$Script) {
    try {
        & $Script
    } catch {
        $null = [System.Windows.MessageBox]::Show("Something went wrong in the GoLib window:`n`n$_", 'GoLib', 'OK', 'Error')
    }
}

# Asks for one line of text in a small dialog over the window. Returns the text, trimmed, or $null
# when the dialog is cancelled.
function Read-Text([string]$Title, [string]$Prompt) {
    [xml]$dialogXaml = @'
<Window xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation"
        xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml"
        Width="440" SizeToContent="Height" ResizeMode="NoResize" ShowInTaskbar="False"
        WindowStartupLocation="CenterOwner" FontSize="13">
  <StackPanel Margin="14">
    <TextBlock x:Name="PromptText" TextWrapping="Wrap" Margin="0,0,0,8"/>
    <TextBox x:Name="AnswerBox" Padding="2,3"/>
    <StackPanel Orientation="Horizontal" HorizontalAlignment="Right" Margin="0,12,0,0">
      <Button x:Name="OkButton" Content="OK" IsDefault="True" MinWidth="80" Padding="10,3" Margin="0,0,6,0"/>
      <Button Content="Cancel" IsCancel="True" MinWidth="80" Padding="10,3"/>
    </StackPanel>
  </StackPanel>
</Window>
'@
    $dialog = [Windows.Markup.XamlReader]::Load((New-Object System.Xml.XmlNodeReader $dialogXaml))
    $dialog.Title = $Title
    $dialog.Owner = $Window
    $dialog.FindName('PromptText').Text = $Prompt
    # Enter presses OK (IsDefault) and Esc presses Cancel (IsCancel).
    $dialog.FindName('OkButton').Add_Click({ [System.Windows.Window]::GetWindow($this).DialogResult = $true })
    $dialog.Add_ContentRendered({ $null = $this.FindName('AnswerBox').Focus() })
    if ($dialog.ShowDialog()) { return $dialog.FindName('AnswerBox').Text.Trim() }
    return $null
}

# State is Idle, Running, Done or Failed.
function Set-Status([string]$Text, [string]$State) {
    $StatusText.Text = $Text
    $StatusText.Foreground = @{ Idle = 'Black'; Running = 'DarkGoldenrod'; Done = 'ForestGreen'; Failed = 'Firebrick' }[$State]
}

function Add-Output([string]$Line) {
    $OutputBox.AppendText($Line + "`r`n")
    $OutputBox.ScrollToEnd()
}

# Game names: folders in games\ that contain a go.mod, as golib looks for them.
function Update-GameList {
    $selected = $GameList.SelectedItem
    $GameList.Items.Clear()
    if (Test-Path -LiteralPath $GamesDir -PathType Container) {
        Get-ChildItem -LiteralPath $GamesDir -Directory |
            Where-Object { Test-Path -LiteralPath (Join-Path $_.FullName 'go.mod') -PathType Leaf } |
            Sort-Object Name |
            ForEach-Object { $null = $GameList.Items.Add($_.Name) }
    }
    if ($null -ne $selected -and $GameList.Items.Contains($selected)) {
        $GameList.SelectedItem = $selected
    } elseif ($GameList.Items.Count -gt 0) {
        $GameList.SelectedIndex = 0
    }
}

# While a command runs, only Stop and the folder buttons work, so two commands never overlap.
function Set-Busy([bool]$Busy) {
    foreach ($button in $ActionButtons) { $button.IsEnabled = -not $Busy }
    $GameList.IsEnabled = -not $Busy
    $RefreshButton.IsEnabled = -not $Busy
    $FramesBox.IsEnabled = -not $Busy
    $InputBox.IsEnabled = -not $Busy
    $StopButton.IsEnabled = $Busy
}

function Invoke-Action($Action) {
    $game = [string]$GameList.SelectedItem
    $template = if ($Action.ContainsKey('Folder')) { $Action.Folder } else { $Action.Command }
    if ($template -match '\{game\}' -and -not $game) {
        Set-Status 'There are no games in games\ yet.' 'Failed'
        return
    }

    if ($Action.ContainsKey('Folder')) {
        $folder = Join-Path $Root ($template -replace '\{game\}', $game)
        if (Test-Path -LiteralPath $folder -PathType Container) {
            Start-Process -FilePath 'explorer.exe' -ArgumentList "`"$folder`""
        } else {
            Set-Status "$folder doesn't exist yet." 'Failed'
        }
        return
    }

    if ($Action.ContainsKey('Confirm')) {
        $answer = [System.Windows.MessageBox]::Show($Window, $Action.Confirm, 'GoLib', 'YesNo', 'Warning')
        if ($answer -ne 'Yes') { return }
    }
    $script:SelectAfter = $null
    $reply = ''
    if ($Action.ContainsKey('Prompt')) {
        $reply = Read-Text $Action.Label $Action.Prompt
        if (-not $reply) { return }    # cancelled, or left empty
        $script:SelectAfter = $reply
    }
    $arguments = @()
    foreach ($token in ($template -split ' ')) {
        if ($token -eq '{name}') {
            $arguments += $reply
        } elseif ($token -eq '{game}') {
            $arguments += $game
        } elseif ($token -eq '{frames}') {
            $arguments += @($FramesBox.Text -split '[\s,]+' | Where-Object { $_ })
        } elseif ($token -eq '{input}') {
            if ($InputBox.Text.Trim()) { $arguments += @('--input', $InputBox.Text.Trim()) }
        } else {
            $arguments += $token
        }
    }
    Start-Command $arguments
}

# Starts golib.ps1 with Arguments, reading its output as it comes.
function Start-Command([string[]]$Arguments) {
    $quoted = @($Arguments | ForEach-Object { if ($_ -match '[\s"]') { '"' + ($_ -replace '"', '\"') + '"' } else { $_ } })
    $info = New-Object System.Diagnostics.ProcessStartInfo
    $info.FileName = $PowerShellExe
    $info.Arguments = "-NoProfile -ExecutionPolicy Bypass -File `"$Cli`" " + ($quoted -join ' ')
    $info.WorkingDirectory = $Root
    $info.UseShellExecute = $false
    $info.CreateNoWindow = $true
    $info.RedirectStandardOutput = $true
    $info.RedirectStandardError = $true
    $info.StandardOutputEncoding = [System.Text.Encoding]::UTF8
    $info.StandardErrorEncoding = [System.Text.Encoding]::UTF8

    $OutputBox.Clear()
    Add-Output ('> golib ' + ($Arguments -join ' '))
    Add-Output ''
    $script:Process = [System.Diagnostics.Process]::Start($info)
    # Read both streams with ReadLineAsync and collect the lines on a timer, on this thread:
    # PowerShell script blocks can't run as callbacks on other threads.
    $script:Streams = @(
        @{ Reader = $script:Process.StandardOutput; Line = $script:Process.StandardOutput.ReadLineAsync() }
        @{ Reader = $script:Process.StandardError; Line = $script:Process.StandardError.ReadLineAsync() }
    )
    $script:CommandName = $Arguments[0]
    $script:Stopped = $false
    Set-Busy $true
    Set-Status "$($script:CommandName): running..." 'Running'
    $Timer.Start()
}

# Timer tick: shows new output, and wraps up once the command has finished.
function Update-Command {
    $open = 0
    foreach ($stream in $script:Streams) {
        while ($null -ne $stream.Line -and $stream.Line.IsCompleted) {
            $text = $null
            if ($stream.Line.Status -eq 'RanToCompletion') { $text = $stream.Line.Result }
            if ($null -eq $text) {
                $stream.Line = $null    # end of the stream
            } else {
                Add-Output $text
                $stream.Line = $stream.Reader.ReadLineAsync()
            }
        }
        if ($null -ne $stream.Line) { $open++ }
    }
    if ($open -gt 0 -or -not $script:Process.HasExited) { return }

    $Timer.Stop()
    $code = $script:Process.ExitCode
    $script:Process.Dispose()
    $script:Process = $null
    if ($script:Stopped) {
        Set-Status "$($script:CommandName): stopped." 'Failed'
    } elseif ($code -eq 0) {
        Set-Status "$($script:CommandName): done." 'Done'
    } else {
        Set-Status "$($script:CommandName): failed (exit code $code). The output below says why." 'Failed'
    }
    Set-Busy $false
    # Commands can add or remove games, so look again.
    Update-GameList
    if ($code -eq 0 -and $script:SelectAfter -and $GameList.Items.Contains($script:SelectAfter)) {
        $GameList.SelectedItem = $script:SelectAfter
    }
}

function Stop-Command {
    if ($null -eq $script:Process -or $script:Process.HasExited) { return }
    $script:Stopped = $true
    # golib starts go and the game as child processes: stop the whole tree.
    & (Join-Path $env:SystemRoot 'System32\taskkill.exe') /PID $script:Process.Id /T /F | Out-Null
}

$groups = @($Actions | ForEach-Object { $_.Group } | Select-Object -Unique)
foreach ($group in $groups) {
    $row = New-Object System.Windows.Controls.WrapPanel
    $row.Margin = '0,0,0,6'
    $label = New-Object System.Windows.Controls.TextBlock
    $label.Text = $group
    $label.Width = 60
    $label.VerticalAlignment = 'Center'
    $null = $row.Children.Add($label)
    foreach ($action in @($Actions | Where-Object { $_.Group -eq $group })) {
        $button = New-Object System.Windows.Controls.Button
        $button.Content = $action.Label
        $button.ToolTip = $action.Tip
        $button.Tag = $action
        $button.MinWidth = 100
        $button.Margin = '0,0,6,0'
        $button.Padding = '12,4'
        $button.Add_Click({ $action = $this.Tag; Invoke-Safely { Invoke-Action $action } })
        $null = $row.Children.Add($button)
        if ($action.ContainsKey('Command')) { $ActionButtons.Add($button) }
    }
    $null = $ActionRows.Children.Add($row)
}

$RefreshButton.Add_Click({ Invoke-Safely { Update-GameList } })
$StopButton.Add_Click({ Invoke-Safely { Stop-Command } })
$Timer.Add_Tick({ Invoke-Safely { Update-Command } })
$Window.Add_Closing({ Invoke-Safely { Stop-Command } })

Update-GameList
if (Test-Path -LiteralPath (Join-Path $Root '.tools\go\bin\go.exe') -PathType Leaf) {
    Set-Status 'Pick a game and press a button. Hover over a button to see what it does.' 'Idle'
} else {
    Set-Status 'First time here? Press Setup: it installs Go and raylib into .tools\.' 'Idle'
}
$null = $Window.ShowDialog()
