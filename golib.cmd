@echo off
rem GoLib command-line entry point for Windows (cmd and PowerShell).
rem Usage: .\golib <command> [options]     Run ".\golib help" to list commands.
rem
rem This file only starts tools\bootstrap\golib.ps1. "-ExecutionPolicy Bypass" applies to this
rem single run; no system setting is changed.
setlocal
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0tools\bootstrap\golib.ps1" %*
exit /b %ERRORLEVEL%
