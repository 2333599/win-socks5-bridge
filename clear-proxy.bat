@echo off
setlocal
win-socks5-bridge-cli.exe -clear-proxy
if errorlevel 1 (
  echo Failed to clear the Windows manual proxy settings.
) else (
  echo Windows manual proxy settings cleared.
)
pause
