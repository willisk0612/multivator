package executor

import (
	"fmt"
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
	drvObstructionCh := make(chan bool)
	port := config.BcastPort + config.NodeID
	elevio.Init(fmt.Sprintf("localhost:%d", port), config.NumFloors)

	elevator := initializeElevator()
	var doorTimer = DoorTimer{nil, make(chan bool)}

	go elevio.PollButtons(drvButtonsCh)
	go elevio.PollFloorSensor(drvFloorsCh)
	go elevio.PollObstructionSwitch(drvObstructionCh)

	elevUpdateCh <- elevator

	for {
		select {

		case receivedOrders := <-orderUpdateCh:
			synchronizeLights(elevator, receivedOrders)
			elevator.Orders = receivedOrders
			elevator = chooseAction(elevator, doorTimer)
			elevUpdateCh <- elevator

		case btn := <-drvButtonsCh:
			if btn.Button == types.BT_Cab {
				// If we are on the same floor, only open the door
				if elevator.Floor == btn.Floor {
					doorTimer.Start()
					continue
				}

				elevator.Orders[config.NodeID][btn.Floor][btn.Button] = true
				elevio.SetButtonLamp(types.BT_Cab, btn.Floor, true)
				elevator = chooseAction(elevator, doorTimer)
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
				doorTimer.Start()
				elevUpdateCh <- elevator
				sendSyncCh <- true
			}

		case obstruction := <-drvObstructionCh:
			elevator.Obstructed = obstruction
			doorTimer.Start()
		case <-doorTimer.timeoutChannel:
			if elevator.Obstructed {
				doorTimer.Start()
				continue
			}
			elevio.SetDoorOpenLamp(false)
			elevator.Behaviour = types.Idle
			elevator = chooseAction(elevator, doorTimer)
			elevUpdateCh <- elevator
			sendSyncCh <- true

		case <-startDoorTimerCh:
			elevator.Behaviour = types.DoorOpen
			doorTimer.Start()
		}
	}
}

func initializeElevator() types.ElevState {
	elevator := types.ElevState{
		Dir:       types.MD_Stop,
		Orders:    types.Orders{},
		Behaviour: types.Idle,
	}
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

func synchronizeLights(elevator types.ElevState, receivedOrders types.Orders) {
	utils.ForEachOrder(elevator.Orders, func(node, floor, btn int) {
		// Sync hall lights
		if btn != int(types.BT_Cab) &&
			elevator.Orders[node][floor][btn] != receivedOrders[node][floor][btn] {
			elevio.SetButtonLamp(types.ButtonType(btn), floor, receivedOrders[node][floor][btn])
		}
		// Sync cab lights
		if config.NodeID == node && btn == int(types.BT_Cab) &&
			elevator.Orders[node][floor][btn] != receivedOrders[node][floor][btn] {
			elevio.SetButtonLamp(types.BT_Cab, floor, receivedOrders[node][floor][btn])
		}
	})
}

// chooseAction is called on order updates from dispatcher, on cab calls and on door timeouts.
//   - Moves elevator if we have orders in different floors
//   - Opens door if we have orders here
func chooseAction(elevator types.ElevState,
	doorTimer DoorTimer,
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
		doorTimer.Start()
	default:
		elevio.SetMotorDirection(types.MD_Stop)
	}
	return elevator
}
