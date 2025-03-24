package executor

import (
	"fmt"
	"time"

	"multivator/lib/driver/elevio"
	"multivator/src/config"
	"multivator/src/types"
	"multivator/src/utils"
)

func Run(elevUpdateCh chan<- types.ElevState,
	orderUpdateCh <-chan types.Orders,
	hallOrderCh chan<- types.HallOrder,
	sendSyncCh chan<- bool,
	startDoorTimerCh <-chan bool,
) {
	drvButtonsCh := make(chan types.ButtonEvent)
	drvFloorsCh := make(chan int)
	drvObstrCh := make(chan bool)
	doorTimeoutCh := make(chan bool)
	var doorTimer *time.Timer
	port := config.BcastPort + config.NodeID
	elevio.Init(fmt.Sprintf("localhost:%d", port), config.NumFloors)

	elevator := types.ElevState{
		Dir:       types.MD_Stop,
		Orders:    types.Orders{},
		Behaviour: types.Idle,
	}
	elevator = initElevPos(elevator)

	go elevio.PollButtons(drvButtonsCh)
	go elevio.PollFloorSensor(drvFloorsCh)
	go elevio.PollObstructionSwitch(drvObstrCh)

	elevUpdateCh <- elevator

	for {
		select {

		case receivedOrders := <-orderUpdateCh:
			utils.ForEachOrder(elevator.Orders, func(node, floor, btn int) {
				// Sync hall lights
				if btn != int(types.BT_Cab) &&
					elevator.Orders[node][floor][btn] != receivedOrders[node][floor][btn] {

					elevio.SetButtonLamp(types.ButtonType(btn), floor, receivedOrders[node][floor][btn])
				}

				// Sync cab lights
				if config.NodeID == node &&
					btn == int(types.BT_Cab) &&
					elevator.Orders[node][floor][btn] != receivedOrders[node][floor][btn] {

					elevio.SetButtonLamp(types.BT_Cab, floor, receivedOrders[node][floor][btn])
				}
			})

			elevator.Orders = receivedOrders
			elevator = chooseAction(elevator, doorTimer, doorTimeoutCh)
			elevUpdateCh <- elevator

		case btn := <-drvButtonsCh:
			if btn.Button == types.BT_Cab {
				// If we are on the same floor, only open the door
				if elevator.Floor == btn.Floor {
					startDoorTimer(&doorTimer, doorTimeoutCh)
					continue
				}

				elevator.Orders[config.NodeID][btn.Floor][btn.Button] = true
				elevio.SetButtonLamp(types.BT_Cab, btn.Floor, true)
				elevator = chooseAction(elevator, doorTimer, doorTimeoutCh)
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
				startDoorTimer(&doorTimer, doorTimeoutCh)
				elevUpdateCh <- elevator
				sendSyncCh <- true
			}

		case obstruction := <-drvObstrCh:
			elevator.Obstructed = obstruction
			startDoorTimer(&doorTimer, doorTimeoutCh)
		case <-doorTimeoutCh:
			if elevator.Obstructed {
				startDoorTimer(&doorTimer, doorTimeoutCh)
				continue
			}
			elevio.SetDoorOpenLamp(false)
			elevator.Behaviour = types.Idle
			elevator = chooseAction(elevator, doorTimer, doorTimeoutCh)
			elevUpdateCh <- elevator
			sendSyncCh <- true

		case <-startDoorTimerCh:
			elevator.Behaviour = types.DoorOpen
			startDoorTimer(&doorTimer, doorTimeoutCh)
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
	doorTimeoutCh chan bool,
) types.ElevState {
	if elevator.Behaviour != types.Idle {
		return elevator
	}
	pair := ChooseDirection(elevator)
	elevator.Behaviour = pair.Behaviour
	elevator.Dir = pair.Dir

	switch pair.Behaviour {
	case types.Moving:
		elevio.SetMotorDirection(elevator.Dir)
	case types.DoorOpen:
		elevator = clearAtCurrentFloor(elevator)
		startDoorTimer(&doorTimer, doorTimeoutCh)
	default:
		elevio.SetMotorDirection(types.MD_Stop)
	}
	return elevator
}

// startDoorTimer starts/restarts the door timer and sets the door open lamp.
func startDoorTimer(doorTimer **time.Timer, doorTimeoutCh chan bool) {
	elevio.SetDoorOpenLamp(true)
	if *doorTimer != nil {
		(*doorTimer).Reset(config.DoorOpenDuration)
	} else {
		*doorTimer = time.AfterFunc(config.DoorOpenDuration, func() {
			doorTimeoutCh <- true
		})
	}
}
