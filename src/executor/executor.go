package executor

import (
	"fmt"
	"time"

	"multivator/lib/driver/elevio"
	"multivator/src/config"
	"multivator/src/types"
	"multivator/src/utils"
)

type DoorTimer struct {
	timer     *time.Timer
	timeoutCh chan bool
}

func Run(elevUpdateCh chan<- types.ElevState,
	orderUpdateCh <-chan types.Orders,
	hallOrderCh chan<- types.HallOrder,
	sendSyncCh chan<- bool,
	openDoorCh <-chan bool,
) {
	drvButtonsCh := make(chan types.ButtonEvent)
	drvFloorsCh := make(chan int)
	drvObstrCh := make(chan bool)
	var doorTimer DoorTimer
	doorTimer.timeoutCh = make(chan bool)

	var lastBtnPressTime time.Time

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
			syncLights(elevator, receivedOrders)
			elevator.Orders = receivedOrders
			chooseAction(elevator, doorTimer)
			elevUpdateCh <- *elevator

		case btn := <-drvButtonsCh:
			if btn.Button == types.BT_Cab {
				// If we are on the same floor, only open the door
				if elevator.Floor == btn.Floor {
					openDoor(elevator, doorTimer)
					continue
				}

				elevator.Orders[config.NodeID][btn.Floor][btn.Button] = true
				elevio.SetButtonLamp(types.BT_Cab, btn.Floor, true)
				chooseAction(elevator, doorTimer)
				elevUpdateCh <- *elevator
				sendSyncCh <- true
			} else {
				// In case of hall order, set a minimum interval between button presses
				// This is to ensure updated orders when calculating cost in dispatcher
				elapsedTime := time.Since(lastBtnPressTime)
				lastBtnPressTime = time.Now()
				if elapsedTime < config.BtnPressInterval {
					go func(order types.ButtonEvent) {
						time.Sleep(config.BtnPressInterval-elapsedTime)
						drvButtonsCh <- order
					}(btn)
					continue
				}
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
				openDoor(elevator, doorTimer)
				elevUpdateCh <- *elevator
				sendSyncCh <- true
			}

		case obstruction := <-drvObstrCh:
			elevator.Obstructed = obstruction
			openDoor(elevator, doorTimer)
		case <-doorTimer.timeoutCh:
			if elevator.Obstructed {
				openDoor(elevator, doorTimer)
				continue
			}
			elevio.SetDoorOpenLamp(false)
			elevator.Behaviour = types.Idle
			chooseAction(elevator, doorTimer)
			elevUpdateCh <- *elevator
			sendSyncCh <- true

		case <-openDoorCh:
			openDoor(elevator, doorTimer)
		}
	}
}

// initElevPos is called on startup.
//   - If between floors, moves elevator down
//   - If on floor, sets floor indicator
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
	doorTimer DoorTimer,
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
		openDoor(elevator, doorTimer)
	default:
		elevio.SetMotorDirection(types.MD_Stop)
	}
}

// syncLights is called on order updates from dispatcher.
//   - Syncs lights based on received orders and button type
func syncLights(elevator *types.ElevState, receivedOrders types.Orders) {
	utils.ForEachOrder(elevator.Orders, func(node, floor, btn int) {
		receivedOrder := receivedOrders[node][floor][btn]
		if elevator.Orders[node][floor][btn] != receivedOrder {
			// Sync hall lights
			if btn != int(types.BT_Cab) {
				elevio.SetButtonLamp(types.ButtonType(btn), floor, receivedOrders[node][floor][btn])
			}
			// Sync cab lights
			if btn == int(types.BT_Cab) && node == config.NodeID {
				elevio.SetButtonLamp(types.BT_Cab, floor, receivedOrders[node][floor][btn])
			}
		}
	})
}

// openDoor modifies elevator state, sets door lamp and starts the door timer
func openDoor(elevator *types.ElevState, doorTimer DoorTimer) {
	elevator.Behaviour = types.DoorOpen
	elevio.SetDoorOpenLamp(true)
	if doorTimer.timer != nil {
		(doorTimer.timer).Reset(config.DoorOpenDuration)
	} else {
		doorTimer.timer = time.AfterFunc(config.DoorOpenDuration, func() {
			doorTimer.timeoutCh <- true
		})
	}
}
