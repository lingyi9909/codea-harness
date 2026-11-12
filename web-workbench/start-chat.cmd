@echo off
setlocal DisableDelayedExpansion
if not "%~2"=="" (
  echo Usage: start-chat.cmd "[project directory]"
  exit /b 2
)
set "projectDir="
if not "%~1"=="" set "projectDir=%~f1"
pushd "%~dp0" || exit /b 1
if defined projectDir (
  node --experimental-strip-types apps/server/src/start.ts --local-only --workspace "%projectDir%\."
) else (
  node --experimental-strip-types apps/server/src/start.ts --local-only
)
set "workbenchStatus=%errorlevel%"
popd
if not "%workbenchStatus%"=="0" pause
exit /b %workbenchStatus%
