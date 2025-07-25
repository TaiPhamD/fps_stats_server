@echo off
echo Building FPS Tray Application (Windows GUI)...
go build -ldflags="-H windowsgui" -o fps_tray.exe fps_tray.go
if %ERRORLEVEL% EQU 0 (
    echo Build successful! Run fps_tray.exe to start the application.
    echo The application will run in the system tray without showing a console window.
) else (
    echo Build failed!
)
pause 