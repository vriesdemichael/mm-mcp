@echo off
rem Spike harness (#37), Windows: starts the spike build of mm-mcp over stdio,
rem with the connection from .tmp\spike.env, which is never committed.
setlocal
set "ROOT=%~dp0..\.."
for /f "usebackq eol=# tokens=1,* delims==" %%a in ("%ROOT%\.tmp\spike.env") do set "%%a=%%b"
set MM_MCP_ALLOW_WRITES=false
if "%SPIKE_LOG%"=="" set "SPIKE_LOG=%ROOT%\.tmp\spike-server.log"
"%ROOT%\.tmp\mm-mcp-spike.exe" serve
