package executor

import (
	"fmt"
	"log/slog"
	"time"

	"multivator/lib/driver/elevio"
	"multivator/src/config"
	"multivator/src/timer"
	"multivator/src/types"
	"multivator/src/utils"
)

func Run(elevUpdateCh chan types.ElevState,
	orderUpdateCh <-chan types.Orders,
	hallOrderCh chan<- types.ButtonEvent,
	sendSyncCh chan<- bool) {

	drvButtons := make(chan types.ButtonEvent)
	drvFloors := make(chan int)
	drvObstr := make(chan bool)
	doorTimerTimeoutCh := make(chan bool)
	doorTimerActionCh := make(chan timer.TimerAction)
	doorTimerDuration := time.NewTimer(config.DoorOpenDuration)
	port := 15657 + config.NodeID
	elevio.Init(fmt.Sprintf("localhost:%d", port), config.NumFloors)

	elevator := types.ElevState{
		Dir:       types.MD_Stop,
		Orders:    types.Orders{},
		Behaviour: types.Idle,
	}
	slog.Debug("Initializing position")
	elevator = initElevPos(elevator)

	go timer.Run(doorTimerDuration, doorTimerTimeoutCh, doorTimerActionCh)
	go elevio.PollButtons(drvButtons)
	go elevio.PollFloorSensor(drvFloors)
	go elevio.PollObstructionSwitch(drvObstr)

	slog.Debug("Sending initial elevator state")
	elevUpdateCh <- elevator

	for {
		select {

		case elevUpdate := <-elevUpdateCh:
			elevator = elevUpdate
		case orderUpdate := <-orderUpdateCh:
			syncHallLights(elevator.Orders, orderUpdate)
			syncCabLights(elevator.Orders, orderUpdate)
			elevator.Orders = orderUpdate
			elevator = chooseAction(elevator, doorTimerActionCh, elevUpdateCh)
		case btn := <-drvButtons:
			slog.Debug("Button press received", "button", utils.FormatBtnEvent(btn))
			if btn.Button == types.BT_Cab {
				elevator.Orders[config.NodeID][btn.Floor][btn.Button] = true
				elevio.SetButtonLamp(types.BT_Cab, btn.Floor, true)
				elevator = chooseAction(elevator, doorTimerActionCh, elevUpdateCh)
				elevUpdateCh <- elevator
				sendSyncCh <- true
			} else {
				elevUpdateCh <- elevator
				hallOrderCh <- btn
			}
		case floor := <-drvFloors:
			slog.Debug("Updating floor", "floor", floor)
			elevator.Floor = floor
			slog.Debug("Elevator state after floor update", "elevator", elevator)
			elevio.SetFloorIndicator(floor)

			if ShouldStopHere(elevator) {
				elevio.SetMotorDirection(types.MD_Stop)
				elevator = clearAtCurrentFloor(elevator)
				elevator.Dir = types.MD_Stop
				elevator.Behaviour = types.DoorOpen
				elevio.SetDoorOpenLamp(true)
				doorTimerActionCh <- timer.Start
			}
			elevUpdateCh <- elevator
			sendSyncCh <- true
		case obstruction := <-drvObstr:
			elevator.Obstructed = obstruction
			doorTimerActionCh <- timer.Start
		case <-doorTimerTimeoutCh:
			if elevator.Obstructed {
				doorTimerActionCh <- timer.Start
				continue
			}
			elevio.SetDoorOpenLamp(false)
			elevator.Behaviour = types.Idle
			elevator = chooseAction(elevator, doorTimerActionCh, elevUpdateCh)
			elevUpdateCh <- elevator
			sendSyncCh <- true
		}
	}
}

// InitElevPos moves down until the first floor is detected, then stops
func initElevPos(elevator types.ElevState) types.ElevState {
	floor := elevio.GetFloor()
	if floor == -1 {
		elevio.SetMotorDirection(types.MD_Down)
		elevator.Behaviour = types.Moving
		elevator.Dir = types.MD_Down
	} else {
		elevator.Floor = floor
		elevio.SetFloorIndicator(floor)
	}
	return elevator
}

// chooseAction decides what the elevator should do next
//  - Chooses direction if we have orders in different floors
//  - Opens door if we have orders here
func chooseAction(elevator types.ElevState,
	doorTimerActionCh chan timer.TimerAction,
	elevUpdateCh chan types.ElevState,
) types.ElevState {
	if elevator.Behaviour != types.Idle {
		// chooseAction will be called again when the elevator is idle
		return elevator
	}
	pair := ChooseDirection(elevator)
	switch pair.Behaviour {
	case types.Moving:
		elevator.Dir = pair.Dir
		elevator.Behaviour = pair.Behaviour
		elevio.SetMotorDirection(elevator.Dir)
		elevUpdateCh <- elevator
	case types.DoorOpen:
		elevator.Behaviour = types.DoorOpen
		elevio.SetDoorOpenLamp(true)
		elevator = clearAtCurrentFloor(elevator)
		doorTimerActionCh <- timer.Start
		elevUpdateCh <- elevator
	default:
		elevator.Behaviour = types.Idle
		elevator.Dir = types.MD_Stop
		elevio.SetMotorDirection(types.MD_Stop)
	}
	return elevator
}
