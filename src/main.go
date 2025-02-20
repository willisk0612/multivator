package main

import (
	"flag"
	"fmt"
	"log/slog"
	"main/lib/driver-go/elevio"
	"main/src/config"
	"main/src/elev"
	"main/src/network"
	"main/src/timer"
	"main/src/types"
	"time"
)

func main() {
	nodeID := flag.Int("id", 0, "Node ID of the elevator")
	flag.Parse()
	port := 15657 + *nodeID

	elev.InitLogger()
	elevio.Init(fmt.Sprintf("localhost:%d", port), config.NumFloors)
	elevator := elev.InitSystem(*nodeID)

	drv_buttons := make(chan types.ButtonEvent)
	drv_floors := make(chan int)
	drv_obstr := make(chan bool)
	drv_stop := make(chan bool)

	doorTimerDuration := time.NewTimer(config.DoorOpenDuration)
	doorTimerTimeout := make(chan bool, 1)
	doorTimerAction := make(chan timer.TimerAction, 1)

	outMsgCh := make(chan types.Message)
	elevMgr := elev.StartElevatorManager(elevator)

	go elevio.PollButtons(drv_buttons)
	go elevio.PollFloorSensor(drv_floors)
	go elevio.PollObstructionSwitch(drv_obstr)
	go elevio.PollStopButton(drv_stop)
	go timer.Timer(doorTimerDuration, doorTimerTimeout, doorTimerAction)
	go network.RunNetworkManager(elevMgr, outMsgCh, doorTimerAction)

	slog.Info("Driver initialized", "port", port, "nodeID", nodeID)

	for {
		select {
		case btn := <-drv_buttons:
			if btn.Button == types.BT_Cab || elevio.GetFloor() == -1 {
				elev.MoveElevator(elevMgr, btn, doorTimerAction)
			} else {
				slog.Debug("Hall button press discovered in main")
				network.HandleHallOrder(elevMgr, btn, outMsgCh)
			}
		case floor := <-drv_floors:
			elev.HandleFloorArrival(elevMgr, floor, doorTimerAction)
		case obstruction := <-drv_obstr:
			elev.HandleObstruction(elevMgr, obstruction, doorTimerAction)
		case <-drv_stop:
			elev.HandleStop(elevMgr)
		case <-doorTimerTimeout:
			elev.HandleDoorTimeout(elevMgr, doorTimerAction)
		}
	}
}
