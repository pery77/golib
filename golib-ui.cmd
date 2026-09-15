@echo off
rem Opens the GoLib window: a button for each golib command. Double-click this file in Explorer.
rem
rem This file only starts tools\ui\golib-ui.ps1 in a hidden PowerShell window and returns at once.
rem "-ExecutionPolicy Bypass" applies to that single run; no system setting is changed.
start "" powershell.exe -NoProfile -Sta -ExecutionPolicy Bypass -WindowStyle Hidden -File "%~dp0tools\ui\golib-ui.ps1"
