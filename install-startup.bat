@echo off
title Local Archive - Install Startup
echo Setting up Local Archive to start automatically on Windows boot...
"%~dp0archive.exe" -install
if %ERRORLEVEL% EQU 0 (
    echo.
    echo ==========================================================
    echo SUCCESS: Local Archive will now start automatically on boot!
    echo It runs quietly in the background without any console window.
    echo You can open your archive anytime at http://localhost:8080
    echo ==========================================================
) else (
    echo.
    echo Installation failed with error code %ERRORLEVEL%.
)
echo.
pause
