@echo off
setlocal DisableDelayedExpansion
call "%~dp0start-chat.cmd" %*
exit /b %errorlevel%
