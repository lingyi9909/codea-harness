@echo off
setlocal
if "%~1"=="" (
  echo Usage: "%~nx0" "D:\teamai"
  exit /b 2
)
set "EMBED_WORKSPACE=%~f1"
pushd "%~dp0"
if not exist "apps\web\dist\index.html" (
  echo Build the page first: npm run web:build
  popd
  exit /b 2
)
node --experimental-strip-types apps\server\src\start.ts --embedded-only --workspace "%EMBED_WORKSPACE%"
set "EMBED_EXIT=%errorlevel%"
popd
exit /b %EMBED_EXIT%
