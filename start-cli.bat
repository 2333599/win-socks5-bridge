@echo off
setlocal
rem Replace this example with your upstream SOCKS5 server.
set "UPSTREAM=socks5://proxy.example.com:1080"
win-socks5-bridge-cli.exe -listen 127.0.0.1:1080 -upstream "%UPSTREAM%"
pause
