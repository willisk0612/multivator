@echo off

pushd %~dp0\..\..

set basePath=%CD%
set configFile=%basePath%\src\config\config.go
set newValue=2

set powershell="C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe"

%powershell% -Command ^
  "(Get-Content '%configFile%') -replace '(?m)(^\s*NumElevators\s*=\s*)\d+', '${1}%newValue%' | Set-Content '%configFile%'"

wt -M -p "Command Prompt" cmd /k "cd /d "%basePath%\lib\simulator" && .\SimElevatorServer.exe --port 17400" ^
    ; split-pane -H -p "Command Prompt" cmd /k "cd /d "%basePath%\lib\simulator" && .\SimElevatorServer.exe --port 17401" ^
    ; focus-pane -t 0 ; split-pane -V -p "Command Prompt" cmd /k "cd /d "%basePath%\src" && go run -race . --id 0" ^
    ; focus-pane -t 1 ; split-pane -V -p "Command Prompt" cmd /k "cd /d "%basePath%\src" && go run -race . --id 1"

popd
