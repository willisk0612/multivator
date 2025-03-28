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
	openDoorCh <-chan bool,
) {
	drvButtonsCh := make(chan types.ButtonEvent)
	drvFloorsCh := make(chan int)
	drvObstrCh := make(chan bool)
	var doorTimer *time.Timer
	doorTimeoutCh := make(chan bool)
	var stuckTimer *time.Timer
	stuckTimeoutCh := make(chan bool)

	port := config.PeersPort + config.NodeID
	elevio.Init(fmt.Sprintf("localhost:%d", port), config.NumFloors)

	elevator := new(types.ElevState)
	initElevPos(elevator, &stuckTimer, stuckTimeoutCh)

	go elevio.PollButtons(drvButtonsCh)
	go elevio.PollFloorSensor(drvFloorsCh)
	go elevio.PollObstructionSwitch(drvObstrCh)

	elevUpdateCh <- *elevator

	for {
		select {

		case receivedOrders := <-orderUpdateCh:
			syncLights(elevator, receivedOrders)
			elevator.Orders = receivedOrders
			chooseAction(elevator,
				doorTimer,
				doorTimeoutCh,
				&stuckTimer,
				stuckTimeoutCh,
			)
			elevUpdateCh <- *elevator

		case btn := <-drvButtonsCh:
			switch types.ButtonType(btn.Button) {
			case types.BT_Cab:
				// If we are on the same floor in the correct motor direction, only open the door
				if elevator.Floor == btn.Floor && !elevator.BetweenFloors {
					openDoor(
						elevator,
						&doorTimer,
						doorTimeoutCh,
					)
					continue
				}

				elevator.Orders[config.NodeID][btn.Floor][btn.Button] = true
				elevio.SetButtonLamp(types.BT_Cab, btn.Floor, true)
				chooseAction(elevator,
					doorTimer,
					doorTimeoutCh,
					&stuckTimer,
					stuckTimeoutCh,
				)
				elevUpdateCh <- *elevator
				sendSyncCh <- true
			default:
				hallOrderCh <- types.HallOrder{
					Floor:  btn.Floor,
					Button: types.HallType(btn.Button),
				}
			}

		case floor := <-drvFloorsCh:
			elevator.Floor = floor
			elevator.IsStuck = false
			if stuckTimer != nil {
				stuckTimer.Stop()
			}
			elevio.SetFloorIndicator(floor)

			if ShouldStopHere(elevator) {
				elevio.SetMotorDirection(types.MD_Stop)
				elevator.BetweenFloors = false
				clearAtCurrentFloor(elevator)
				openDoor(elevator, &doorTimer, doorTimeoutCh)
				elevUpdateCh <- *elevator
				sendSyncCh <- true
			}

		case isObstructed := <-drvObstrCh:
			elevator.Obstructed = isObstructed
			if elevator.Behaviour == types.DoorOpen || elevator.IsStuck {
				openDoor(elevator, &doorTimer, doorTimeoutCh)
				if elevator.Obstructed {
					giveHallOrders(elevator, hallOrderCh, elevUpdateCh)
				}
			}
			elevUpdateCh <- *elevator
		case <-doorTimeoutCh:
			if elevator.Obstructed {
				openDoor(elevator, &doorTimer, doorTimeoutCh)
				continue
			}
			elevio.SetDoorOpenLamp(false)
			elevator.Behaviour = types.Idle
			chooseAction(elevator,
				doorTimer,
				doorTimeoutCh,
				&stuckTimer,
				stuckTimeoutCh,
			)
			elevUpdateCh <- *elevator
			sendSyncCh <- true

		case <-stuckTimeoutCh:
			elevator.IsStuck = true
			elevator.Behaviour = types.Idle
			if doorTimer != nil {
				stuckTimer.Stop()
			}
			giveHallOrders(elevator, hallOrderCh, elevUpdateCh)

		case <-openDoorCh:
			openDoor(elevator, &doorTimer, doorTimeoutCh)
			elevUpdateCh <- *elevator
		}
	}
}

// initElevPos is called on startup.
//   - If between floors, moves elevator down
//   - If on floor, sets floor indicator
func initElevPos(elevator *types.ElevState, stuckTimer **time.Timer, stuckTimeoutCh chan<- bool) {
	floor := elevio.GetFloor()
	if floor == -1 {
		elevator.BetweenFloors = true
		resetTimer(stuckTimer, stuckTimeoutCh, config.StuckTimeout)
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
	doorTimeoutCh chan<- bool,
	stuckTimer **time.Timer,
	stuckTimeoutCh chan<- bool,
) {
	if elevator.Behaviour != types.Idle {
		return // chooseAction will be called again when the elevator becomes idle
	}
	pair := ChooseDirection(elevator)
	elevator.Behaviour = pair.Behaviour
	elevator.Dir = pair.Dir

	switch pair.Behaviour {
	case types.Moving:
		elevator.BetweenFloors = true
		elevio.SetMotorDirection(elevator.Dir)
		resetTimer(stuckTimer, stuckTimeoutCh, config.StuckTimeout)

	case types.DoorOpen:
		clearAtCurrentFloor(elevator)
		openDoor(elevator, &doorTimer, doorTimeoutCh)
	default:
		elevio.SetMotorDirection(types.MD_Stop)
	}
}

// giveHallOrders is called on obstruction and stuck timeout.
//   - Sends active hall orders to dispatcher and removes them from this elevator
func giveHallOrders(elevator *types.ElevState, hallOrderCh chan<- types.HallOrder, elevUpdateCh chan<- types.ElevState) {
	utils.ForEachOrder(elevator.Orders, func(node, floor, btn int) {
		if node == config.NodeID &&
			types.ButtonType(btn) != types.BT_Cab &&
			elevator.Orders[node][floor][btn] {
			elevator.Orders[node][floor][btn] = false
			elevUpdateCh <- *elevator
			hallOrderCh <- types.HallOrder{
				Floor:  floor,
				Button: types.HallType(btn),
			}
		}
	})
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
			// Sync cab lights for own orders
			if btn == int(types.BT_Cab) && node == config.NodeID {
				elevio.SetButtonLamp(types.BT_Cab, floor, receivedOrders[node][floor][btn])
			}
		}
	})
}

// openDoor modifies elevator state, sets door lamp and starts the door timer
//   - Uses a hardware check to avoid opening door between floors
func openDoor(
	elevator *types.ElevState,
	doorTimer **time.Timer,
	doorTimeoutCh chan<- bool,
) {
	if elevio.GetFloor() == -1 {
		return
	}

	elevator.Behaviour = types.DoorOpen
	elevio.SetDoorOpenLamp(true)
	if !elevator.Obstructed {
		resetTimer(doorTimer, doorTimeoutCh, config.DoorOpenDuration)
	}
}

// resetTimer resets the timer if it is not nil, otherwise creates a new timer
func resetTimer(timer **time.Timer, timeoutCh chan<- bool, duration time.Duration) {
	if *timer != nil {
		(*timer).Reset(duration)
	} else {
		*timer = time.AfterFunc(duration, func() {
			timeoutCh <- true
		})
	}
}
