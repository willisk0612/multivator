@echo off
set basePath=C:\Users\willi\Desktop\NTNU-V25\Sanntid\multivator

wt -M -p "Command Prompt" cmd /k "cd /d "%basePath%\lib\simulator" && .\SimElevatorServer.exe --port 15657" ^
    ; split-pane -V -p "Command Prompt" cmd /k "cd /d "%basePath%\lib\simulator" && .\SimElevatorServer.exe --port 15658" ^
    ; split-pane -H -p "Command Prompt" cmd /k "cd /d "%basePath%\lib\simulator" && .\SimElevatorServer.exe --port 15659" ^
    ; focus-pane -t 0 ; split-pane -H -p "Command Prompt" cmd /k "cd /d "%basePath%\src" && go run -race . --id 0" ^
    ; split-pane -H -p "Command Prompt" cmd /k "cd /d "%basePath%\src" && go run -race . --id 1" ^
    ; split-pane -H -p "Command Prompt" cmd /k "cd /d "%basePath%\src" && go run -race . --id 2"
