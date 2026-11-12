@echo off
setlocal
cd /d "%~dp0"
if not exist "chat-files\" mkdir "chat-files"
node --experimental-strip-types apps/server/src/start.ts --local-only
if errorlevel 1 pause
