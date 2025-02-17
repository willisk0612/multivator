package main

import (
	"fmt"
	"log/slog"
	"main/lib/driver-go/elevio"
	"main/src/config"
	"main/src/elev"
	"main/src/network"
	"main/src/timer"
	"main/src/types"
	"os"
	"strconv"
	"time"
)

func main() {
	elev.InitLogger()

	port := 15657
	// Check if port number provided as argument
	if len(os.Args) > 1 {
		if p, err := strconv.Atoi(os.Args[1]); err == nil {
			port = p
		}
	}

	elevio.Init(fmt.Sprintf("localhost:%d", port), config.NumFloors)
	nodeID := network.AssignNodeID()

	elevator := elev.InitSystem(nodeID)
	mgr := elev.StartElevatorManager(elevator)

	drv_buttons := make(chan types.ButtonEvent)
	drv_floors := make(chan int)
	drv_obstr := make(chan bool)
	drv_stop := make(chan bool)

	doorTimerDuration := time.NewTimer(config.DoorOpenDuration)
	doorTimerTimeout := make(chan bool)
	doorTimerAction := make(chan timer.TimerAction)

	outMsgCh := make(chan types.Message)
	hallEventCh := make(chan types.ButtonEvent)

	go elevio.PollButtons(drv_buttons)
	go elevio.PollFloorSensor(drv_floors)
	go elevio.PollObstructionSwitch(drv_obstr)
	go elevio.PollStopButton(drv_stop)
	go timer.Timer(doorTimerDuration, doorTimerTimeout, doorTimerAction)
	go network.RunNetworkManager(elevator, mgr, hallEventCh, outMsgCh, doorTimerAction)

	slog.Info("Driver initialized", "port", port, "nodeID", nodeID)

	for {
		select {
		case btn := <-drv_buttons:
			mgr.Execute(elev.HandleButtonPress, btn, doorTimerAction, hallEventCh, outMsgCh)
		case floor := <-drv_floors:
			mgr.Execute(elev.HandleFloorArrival, floor, doorTimerAction)
		case obstruction := <-drv_obstr:
			mgr.Execute(elev.HandleObstruction, obstruction, doorTimerAction)
		case <-drv_stop:
			mgr.Execute(elev.HandleStop)
		case <-doorTimerTimeout:
			mgr.Execute(elev.HandleDoorTimeout, doorTimerAction)
		}
	}
}
