@echo off
REM Build PM Jira for Windows. Requires Go 1.26.5 (https://go.dev/dl/).
REM Produces pmjira.exe (console) and pmjira-gui.exe (no console window).
setlocal
cd /d "%~dp0"

echo Building pmjira.exe (console) ...
go build -trimpath -ldflags="-s -w" -o pmjira.exe .
if errorlevel 1 ( echo Build failed. & exit /b 1 )

echo Building pmjira-gui.exe (no console window) ...
go build -trimpath -ldflags="-s -w -H windowsgui -X main.guiBuild=true" -o pmjira-gui.exe .
if errorlevel 1 ( echo Build failed. & exit /b 1 )

echo.
echo Done.
echo   pmjira.exe     - double-click opens a console window showing logs; Ctrl+C to stop.
echo   pmjira-gui.exe - runs with no window; stop via 设置 -^> 停止服务 (logs in pmjira.log).
endlocal
