package main

import (
	"fmt"
	"main/lib/driver-go/elevio"
	"main/src/timer"
	"main/src/elev"
	"main/src/config"
	"time"
)

func main() {
	elevio.Init("localhost:15657", config.N_FLOORS)
	elevator := elev.InitElevator()

	drv_buttons := make(chan elevio.ButtonEvent)
	drv_floors := make(chan int)
	drv_obstr := make(chan bool)
	drv_stop := make(chan bool)

	doorTimerDuration := time.NewTimer(timer.DOOR_OPEN_DURATION)
	doorTimerTimeout := make(chan bool)
	doorTimerAction := make(chan timer.TimerAction)

	go timer.Timer(doorTimerDuration, doorTimerTimeout, doorTimerAction)
	go elevio.PollButtons(drv_buttons)
	go elevio.PollFloorSensor(drv_floors)
	go elevio.PollObstructionSwitch(drv_obstr)
	go elevio.PollStopButton(drv_stop)

	fmt.Println("Driver started")
	for {
		select {
		case btn := <-drv_buttons:
			elev.HandleButtonPress(elevator, btn, doorTimerAction)

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
