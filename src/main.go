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
	// Initialize elevator and wrap it in the manager.
	elevator := elev.InitSystem(nodeID)
	mgr := elev.StartElevatorManager(elevator)

	drv_buttons := make(chan types.ButtonEvent)
	drv_floors := make(chan int)
	drv_obstr := make(chan bool)
	drv_stop := make(chan bool)

	doorTimerDuration := time.NewTimer(config.DOOR_OPEN_DURATION)
	doorTimerTimeout := make(chan bool)
	doorTimerAction := make(chan timer.TimerAction)

	outMsgCh := make(chan types.Message)
	btnEventCh := make(chan types.ButtonEvent)
	assignmentCh := make(chan types.OrderAssignment)

	go elevio.PollButtons(drv_buttons)
	go elevio.PollFloorSensor(drv_floors)
	go elevio.PollObstructionSwitch(drv_obstr)
	go elevio.PollStopButton(drv_stop)
	go timer.Timer(doorTimerDuration, doorTimerTimeout, doorTimerAction)
	// Pass mgr.Get as the getter function.
	go network.PollMessages(mgr.Get, btnEventCh, assignmentCh)

	slog.Info("Driver initialized", "port", port)

	for {
		select {
		case btn := <-drv_buttons:
			mgr.Execute(elev.ElevatorCmd{
				Exec: func(e *types.Elevator) {
					elev.HandleButtonPress(e, btn, doorTimerAction, btnEventCh, outMsgCh, assignmentCh)
				},
			})
		case floor := <-drv_floors:
			mgr.Execute(elev.ElevatorCmd{
				Exec: func(e *types.Elevator) {
					elev.HandleFloorArrival(e, floor, doorTimerAction)
				},
			})
		case obstruction := <-drv_obstr:
			mgr.Execute(elev.ElevatorCmd{
				Exec: func(e *types.Elevator) {
					elev.HandleObstruction(e, obstruction, doorTimerAction)
				},
			})
		case <-drv_stop:
			mgr.Execute(elev.ElevatorCmd{
				Exec: func(e *types.Elevator) {
					elev.HandleStop(e)
				},
			})
		case <-doorTimerTimeout:
			mgr.Execute(elev.ElevatorCmd{
				Exec: func(e *types.Elevator) {
					elev.HandleDoorTimeout(e, doorTimerAction)
				},
			})
		case assignment := <-assignmentCh:
			// Process order if locally assigned.
			if assignment.IsLocal {
				mgr.Execute(elev.ElevatorCmd{
					Exec: func(e *types.Elevator) {
						if err := elev.ProcessOrder(e, assignment.Event.Floor, assignment.Event.Button, doorTimerAction); err != nil {
							slog.Error("Failed to process order", "error", err, "event", assignment.Event)
						}
					},
				})
			}
		}
	}
}
