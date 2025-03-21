@echo off

pushd %~dp0\..\..

set basePath=%CD%
set configFile=%basePath%\src\config\config.go
set newValue=3

set powershell="C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe"

%powershell% -Command ^
  "(Get-Content '%configFile%') -replace '(?m)(^\s*NumElevators\s*=\s*)\d+', '${1}%newValue%' | Set-Content '%configFile%'"

wt -M -p "Command Prompt" cmd /k "cd /d "%basePath%\lib\simulator" && .\SimElevatorServer.exe --port 15657" ^
    ; split-pane -V -p "Command Prompt" cmd /k "cd /d "%basePath%\lib\simulator" && .\SimElevatorServer.exe --port 15658" ^
    ; split-pane -H -p "Command Prompt" cmd /k "cd /d "%basePath%\lib\simulator" && .\SimElevatorServer.exe --port 15659" ^
    ; focus-pane -t 0 ; split-pane -H -p "Command Prompt" cmd /k "cd /d "%basePath%\src" && go run . --id 0" ^
    ; split-pane -H -p "Command Prompt" cmd /k "cd /d "%basePath%\src" && go run . --id 1" ^
    ; split-pane -H -p "Command Prompt" cmd /k "cd /d "%basePath%\src" && go run . --id 2"

popd
