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

	elevio.Init(fmt.Sprintf("localhost:%d", port), config.N_FLOORS)
	nodeID := network.AssignNodeID()
	elevator := elev.InitSystem(nodeID)

	drv_buttons := make(chan types.ButtonEvent)
	drv_floors := make(chan int)
	drv_obstr := make(chan bool)
	drv_stop := make(chan bool)

	doorTimerDuration := time.NewTimer(config.DOOR_OPEN_DURATION)
	doorTimerTimeout := make(chan bool)
	doorTimerAction := make(chan timer.TimerAction)

	eventCh := make(chan types.ButtonEvent)
	networkEventCh := make(chan types.Message)

	go elevio.PollButtons(drv_buttons)
	go elevio.PollFloorSensor(drv_floors)
	go elevio.PollObstructionSwitch(drv_obstr)
	go elevio.PollStopButton(drv_stop)
	go timer.Timer(doorTimerDuration, doorTimerTimeout, doorTimerAction)
	go network.PollMessages(elevator, eventCh, networkEventCh)

	slog.Info("Driver initialized", "port", port)

	for {
		select {
		case btn := <-drv_buttons:
			elev.HandleButtonPress(elevator, btn, doorTimerAction, eventCh)

		case floor := <-drv_floors:
			elev.HandleFloorArrival(elevator, floor, doorTimerAction)

		case obstruction := <-drv_obstr:
			elev.HandleObstruction(elevator, obstruction, doorTimerAction)

		case <-drv_stop:
			elev.HandleStop(elevator)

		case <-doorTimerTimeout:
			elev.HandleDoorTimeout(elevator, doorTimerAction)

		case netEvent := <-networkEventCh:
			network.HandleMessageEvent(netEvent)
		}
	}
}
