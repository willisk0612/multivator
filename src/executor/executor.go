package executor

import (
	"fmt"
	"log/slog"
	"time"

	"multivator/lib/driver/elevio"
	"multivator/src/config"
	"multivator/src/types"
)

func Run(elevUpdateCh chan<- types.ElevState,
	orderUpdateCh <-chan types.Orders,
	hallOrderCh chan<- types.HallOrder,
	sendSyncCh chan<- bool,
) {
	drvButtonsCh := make(chan types.ButtonEvent)
	drvFloorsCh := make(chan int)
	drvObstrCh := make(chan bool)
	doorTimeoutCh := make(chan bool)
	var doorTimer *time.Timer
	port := 15657 + config.NodeID
	elevio.Init(fmt.Sprintf("localhost:%d", port), config.NumFloors)

	elevator := types.ElevState{
		Dir:       types.MD_Stop,
		Orders:    types.Orders{},
		Behaviour: types.Idle,
	}
	slog.Debug("Initializing position")
	elevator = initElevPos(elevator)

	go elevio.PollButtons(drvButtonsCh)
	go elevio.PollFloorSensor(drvFloorsCh)
	go elevio.PollObstructionSwitch(drvObstrCh)

	slog.Debug("Sending initial elevator state")
	elevUpdateCh <- elevator

	for {
		select {

		case orderUpdate := <-orderUpdateCh:
			syncHallLights(elevator.Orders, orderUpdate)
			syncCabLights(elevator.Orders, orderUpdate)
			elevator.Orders = orderUpdate
			elevator = chooseAction(elevator, doorTimer, elevUpdateCh)

		case btn := <-drvButtonsCh:
			slog.Debug("Button press received", "button", btn.Button, "floor", btn.Floor)
			if btn.Button == types.BT_Cab {
				elevator.Orders[config.NodeID][btn.Floor][btn.Button] = true
				elevio.SetButtonLamp(types.BT_Cab, btn.Floor, true)
				elevator = chooseAction(elevator, doorTimer, elevUpdateCh)
				elevUpdateCh <- elevator
				sendSyncCh <- true
			} else {
				elevUpdateCh <- elevator
				hallOrderCh <- types.HallOrder{Floor: btn.Floor, Button: types.HallType(btn.Button)}
			}

		case floor := <-drvFloorsCh:
			elevator.Floor = floor
			elevio.SetFloorIndicator(floor)

			if ShouldStopHere(elevator) {
				elevio.SetMotorDirection(types.MD_Stop)
				elevator = clearAtCurrentFloor(elevator)
				elevator.Behaviour = types.DoorOpen
				elevio.SetDoorOpenLamp(true)
				doorTimer = time.AfterFunc(config.DoorOpenDuration, func() {
					doorTimeoutCh <- true
				})
				elevUpdateCh <- elevator
				sendSyncCh <- true
			}

		case obstruction := <-drvObstrCh:
			elevator.Obstructed = obstruction
			if doorTimer != nil {
				doorTimer.Reset(config.DoorOpenDuration)
			}

		case <-doorTimeoutCh:
			if elevator.Obstructed {
				if doorTimer != nil {
					doorTimer.Reset(config.DoorOpenDuration)
				}
				continue
			}
			elevio.SetDoorOpenLamp(false)
			elevator.Behaviour = types.Idle
			elevator = chooseAction(elevator, doorTimer, elevUpdateCh)
			elevUpdateCh <- elevator
			sendSyncCh <- true
		}
	}
}

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

// chooseAction is called on order updates from dispatcher, on cab calls and on door timeouts.
//   - Moves elevator if we have orders in different floors
//   - Opens door if we have orders here
func chooseAction(elevator types.ElevState,
	doorTimer *time.Timer,
	elevUpdateCh chan<- types.ElevState,
) types.ElevState {
	if elevator.Behaviour != types.Idle {
		return elevator
	}
	pair := ChooseDirection(elevator)
	elevator.Behaviour = pair.Behaviour
	elevator.Dir = pair.Dir

	switch pair.Behaviour {
	case types.Moving:
		slog.Debug("Moving elevator", "direction", pair.Dir)
		elevio.SetMotorDirection(elevator.Dir)
	case types.DoorOpen:
		elevio.SetDoorOpenLamp(true)
		elevator = clearAtCurrentFloor(elevator)
		if doorTimer != nil {
			doorTimer.Reset(config.DoorOpenDuration)
		}
	default:
		elevio.SetMotorDirection(types.MD_Stop)
	}
	elevUpdateCh <- elevator
	return elevator
}
