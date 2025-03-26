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
	port := config.PeersPort + config.NodeID
	elevio.Init(fmt.Sprintf("localhost:%d", port), config.NumFloors)

	elevator := new(types.ElevState)
	initElevPos(elevator)

	go elevio.PollButtons(drvButtonsCh)
	go elevio.PollFloorSensor(drvFloorsCh)
	go elevio.PollObstructionSwitch(drvObstrCh)

	elevUpdateCh <- *elevator

	for {
		select {

		case receivedOrders := <-orderUpdateCh:
			utils.ForEachOrder(elevator.Orders, func(node, floor, btn int) {
				receivedOrder := receivedOrders[node][floor][btn]
				if elevator.Orders[node][floor][btn] != receivedOrder {
					// Sync hall lights
					if btn != int(types.BT_Cab) {
						elevio.SetButtonLamp(types.ButtonType(btn), floor, receivedOrder)
					}
					// Sync cab lights, orders are only present on network init
					if config.NodeID == node &&
						btn == int(types.BT_Cab) {
						elevio.SetButtonLamp(types.BT_Cab, floor, receivedOrder)
					}
				}
			})

			elevator.Orders = receivedOrders
			chooseAction(elevator, doorTimer, doorTimeoutCh)
			elevUpdateCh <- *elevator

		case btn := <-drvButtonsCh:
			if btn.Button == types.BT_Cab {
				// If we are on the same floor, only open the door
				if elevator.Floor == btn.Floor {
					elevator.Behaviour = types.DoorOpen
					elevio.SetDoorOpenLamp(true)
					startDoorTimer(&doorTimer, doorTimeoutCh)
					continue
				}

				elevator.Orders[config.NodeID][btn.Floor][btn.Button] = true
				elevio.SetButtonLamp(types.BT_Cab, btn.Floor, true)
				chooseAction(elevator, doorTimer, doorTimeoutCh)
				elevUpdateCh <- *elevator
				sendSyncCh <- true
			} else {
				elevUpdateCh <- *elevator
				hallOrderCh <- types.HallOrder{
					Floor:  btn.Floor,
					Button: types.HallType(btn.Button),
				}
			}

		case floor := <-drvFloorsCh:
			elevator.Floor = floor
			elevio.SetFloorIndicator(floor)

			if ShouldStopHere(elevator) {
				elevio.SetMotorDirection(types.MD_Stop)
				clearAtCurrentFloor(elevator)
				elevator.Behaviour = types.DoorOpen
				elevio.SetDoorOpenLamp(true)
				startDoorTimer(&doorTimer, doorTimeoutCh)
				elevUpdateCh <- *elevator
				sendSyncCh <- true
			}

		case obstruction := <-drvObstrCh:
			elevator.Obstructed = obstruction
			elevator.Behaviour = types.DoorOpen
			elevio.SetDoorOpenLamp(true)
			startDoorTimer(&doorTimer, doorTimeoutCh)
		case <-doorTimeoutCh:
			if elevator.Obstructed {
				elevator.Behaviour = types.DoorOpen
				elevio.SetDoorOpenLamp(true)
				startDoorTimer(&doorTimer, doorTimeoutCh)
				continue
			}
			elevio.SetDoorOpenLamp(false)
			elevator.Behaviour = types.Idle
			chooseAction(elevator, doorTimer, doorTimeoutCh)
			elevUpdateCh <- *elevator
			sendSyncCh <- true

		case <-startDoorTimerCh:
			elevator.Behaviour = types.DoorOpen
			elevio.SetDoorOpenLamp(true)
			startDoorTimer(&doorTimer, doorTimeoutCh)
		}
	}
}

func initElevPos(elevator *types.ElevState) {
	floor := elevio.GetFloor()
	if floor == -1 {
		elevio.SetMotorDirection(types.MD_Down)
		elevator.Behaviour = types.Moving
		elevator.Dir = types.MD_Down
	} else {
		elevator.Floor = floor
		elevio.SetFloorIndicator(floor)
	}
}

// chooseAction is called on order updates from dispatcher, on cab calls and on door timeouts.
//   - Moves elevator if we have orders in different floors
//   - Opens door if we have orders here
func chooseAction(elevator *types.ElevState,
	doorTimer *time.Timer,
	doorTimeoutCh chan bool,
) {
	if elevator.Behaviour != types.Idle {
		return
	}
	pair := ChooseDirection(elevator)
	elevator.Behaviour = pair.Behaviour
	elevator.Dir = pair.Dir

	switch pair.Behaviour {
	case types.Moving:
		elevio.SetMotorDirection(elevator.Dir)
	case types.DoorOpen:
		clearAtCurrentFloor(elevator)
		elevio.SetDoorOpenLamp(true)
		startDoorTimer(&doorTimer, doorTimeoutCh)
	default:
		elevio.SetMotorDirection(types.MD_Stop)
	}
}

// startDoorTimer starts/restarts the door timer and sets the door open lamp.
func startDoorTimer(doorTimer **time.Timer, doorTimeoutCh chan bool) {
	if *doorTimer != nil {
		(*doorTimer).Reset(config.DoorOpenDuration)
	} else {
		*doorTimer = time.AfterFunc(config.DoorOpenDuration, func() {
			doorTimeoutCh <- true
		})
	}
}
