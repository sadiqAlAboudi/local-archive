@echo off
title Local Archive - Remove Startup
echo Removing Local Archive from Windows startup...
"%~dp0archive.exe" -uninstall
if %ERRORLEVEL% EQU 0 (
    echo.
    echo ==========================================================
    echo SUCCESS: Automatic startup has been removed.
    echo You can still start Local Archive manually using start.bat
    echo ==========================================================
) else (
    echo.
    echo Removal finished.
)
echo.
pause
