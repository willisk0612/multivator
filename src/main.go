// Single elvator main program
package main

import (
	"fmt"
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
	// Default port if none specified
	port := 15657

	// Check if port number provided as argument
	if len(os.Args) > 1 {
		if p, err := strconv.Atoi(os.Args[1]); err == nil {
			port = p
		}
	}

	elevio.Init(fmt.Sprintf("localhost:%d", port), config.N_FLOORS)
	nodeID := network.AssignNodeID()
	fmt.Println("Node ID assigned:", nodeID)

	elevator := elev.InitSystem(nodeID) // Changed from InitElevator to InitSystem

	drv_buttons := make(chan elevio.ButtonEvent)
	drv_floors := make(chan int)
	drv_obstr := make(chan bool)
	drv_stop := make(chan bool)

	doorTimerDuration := time.NewTimer(timer.DOOR_OPEN_DURATION)
	doorTimerTimeout := make(chan bool)
	doorTimerAction := make(chan timer.TimerAction)

	// Create event channel for button events only
	eventCh := make(chan types.ButtonEvent)

	// Start goroutines
	go elevio.PollButtons(drv_buttons)
	go elevio.PollFloorSensor(drv_floors)
	go elevio.PollObstructionSwitch(drv_obstr)
	go elevio.PollStopButton(drv_stop)
	go timer.Timer(doorTimerDuration, doorTimerTimeout, doorTimerAction)
	go network.PollMessages(elevator, eventCh)

	fmt.Println("Driver started")
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
		}
	}
}
