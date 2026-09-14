@echo off
setlocal EnableExtensions

set "PACKAGE_ROOT=%~dp0"
set "PROJECT_ROOT=%~1"

if not defined PROJECT_ROOT (
  echo Codea Harness 1.6.4 - First Install
  echo.
  echo Enter the absolute path of the Java project to install into.
  echo Tip: you can also drag the project folder onto install.cmd next time.
  echo.
  set /p "PROJECT_ROOT=Project root: "
)

if not defined PROJECT_ROOT (
  echo.
  echo ERROR: project root is required.
  pause
  exit /b 2
)

if not exist "%PROJECT_ROOT%\." (
  echo.
  echo ERROR: project root does not exist: "%PROJECT_ROOT%"
  pause
  exit /b 3
)

echo.
echo Installing Codea Harness into:
echo   %PROJECT_ROOT%
echo.

powershell.exe -NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "%PACKAGE_ROOT%install.ps1" -ProjectRoot "%PROJECT_ROOT%"
set "EXIT_CODE=%ERRORLEVEL%"

if not "%EXIT_CODE%"=="0" (
  echo.
  echo Installation failed. Exit code: %EXIT_CODE%
  echo No destructive overwrite should be performed by the installer on a failed preflight.
  pause
  exit /b %EXIT_CODE%
)

echo.
echo Codea Harness 1.6.4 installation completed.
echo.
echo Next step in OpenCode:
echo   Read .code-harness/bootstrap.md and execute harness init
echo.
pause
exit /b 0
