// Contains helper functions for main.go
package elev

import (
	"io"
	"log"
	"main/lib/driver-go/elevio"
	"main/src/config"
	"main/src/timer"
	"main/src/types"
)

func init() {
	// Disable logging
	log.SetOutput(io.Discard)
}

// Moves elevator if order floor is different from current floor
func HandleButtonPress(elevator *types.Elevator, btn elevio.ButtonEvent, timerAction chan timer.TimerAction, eventCh chan<- types.ButtonEvent) {
	elevator.Orders[btn.Floor][btn.Button] = 1
	elevio.SetButtonLamp(btn.Button, btn.Floor, true)

	// Broadcast button press event
	eventCh <- types.ButtonEvent{
		Floor:  btn.Floor,
		Button: btn.Button,
	}

	switch elevator.Behaviour {
	case elevio.DoorOpen:
		if elevator.Floor == btn.Floor {
			timerAction <- timer.Start
		}
	case elevio.Moving:
		// Keep moving, orders handled at floor arrival
		return
	case elevio.Idle:
		if elevator.Floor == btn.Floor {
			elevator.Behaviour = elevio.DoorOpen
			elevio.SetDoorOpenLamp(true)
			timerAction <- timer.Start
		} else {
			pair := chooseDirection(elevator)
			elevator.Dir = pair.Dir
			elevator.Behaviour = pair.Behaviour
			elevio.SetMotorDirection(elevator.Dir)
		}
		//case elevio.Error:
	}
}

// Stops elevator at floor and opens door
func HandleFloorArrival(elevator *types.Elevator, floor int, timerAction chan timer.TimerAction) {
	elevator.Floor = floor
	elevio.SetFloorIndicator(floor)

	log.Printf("Arrived at floor %d\n", floor)

	if shouldStop(elevator) {
		elevio.SetMotorDirection(elevio.MD_Stop)
		elevio.SetDoorOpenLamp(true)
		timerAction <- timer.Start
		log.Println("Door opened and timer reset")
		clearOrdersAtFloor(elevator)
		elevator.Behaviour = elevio.DoorOpen
	} else {
		log.Println("Continuing to next floor")
	}
}

// Stops elevator and opens door
func HandleObstruction(elevator *types.Elevator, obstruction bool, timerAction chan timer.TimerAction) {
	elevator.Obstructed = obstruction

	if obstruction {
		elevio.SetMotorDirection(elevio.MD_Stop)
		if elevator.Behaviour == elevio.Moving {
			elevator.Behaviour = elevio.DoorOpen
		}
		elevio.SetDoorOpenLamp(true)
	} else {
		timerAction <- timer.Start
	}
}

// Stops elevator and clears all orders and button lamps
func HandleStop(elevator *types.Elevator) {
	elevio.SetMotorDirection(elevio.MD_Stop)
	elevio.SetDoorOpenLamp(false)
	for f := 0; f < config.N_FLOORS; f++ {
		for b := elevio.ButtonType(0); b < 3; b++ {
			elevator.Orders[f][b] = 0
			elevio.SetButtonLamp(b, f, false)
		}
	}
}

// Handles door timeout with obstruction check
func HandleDoorTimeout(elevator *types.Elevator, timerAction chan timer.TimerAction) {
	if elevator.Behaviour != elevio.DoorOpen {
		return
	}

	log.Println("HandleDoorTimeout: Timer expired")
	if elevator.Obstructed {
		log.Println("Door obstructed - keeping open")
		timerAction <- timer.Start
		return
	}

	log.Println("Closing door")
	elevio.SetDoorOpenLamp(false)
	clearOrdersAtFloor(elevator)
	pair := chooseDirection(elevator)
	elevator.Dir = pair.Dir
	elevator.Behaviour = pair.Behaviour

	if elevator.Behaviour == elevio.Moving {
		elevio.SetMotorDirection(elevator.Dir)
	}
}

// Helper function to count orders
func countOrders(elevator *types.Elevator, startFloor int, endFloor int) (result int) {
	for floor := startFloor; floor < endFloor; floor++ {
		for btn := 0; btn < config.N_BUTTONS; btn++ {
			if elevator.Orders[floor][btn] != 0 {
				result++
			}
		}
	}
	return result
}

// Counts button orders above
func ordersAbove(elevator *types.Elevator) (result int) {
	return countOrders(elevator, elevator.Floor+1, config.N_FLOORS)
}

// Counts button orders below
func ordersBelow(elevator *types.Elevator) (result int) {
	return countOrders(elevator, 0, elevator.Floor)
}

// Counts button orders at current floor
func ordersHere(elevator *types.Elevator) (result int) {
	return countOrders(elevator, elevator.Floor, elevator.Floor+1)
}

// Chooses elevator direction based on current orders. Prio: Up > Down > Stop
func chooseDirection(elevator *types.Elevator) types.DirnBehaviourPair {
	switch elevator.Dir {
	case elevio.MD_Up:
		if ordersAbove(elevator) > 0 {
			return types.DirnBehaviourPair{
				Dir:       elevio.MD_Up,
				Behaviour: elevio.Moving,
			}
		} else if ordersHere(elevator) > 0 {
			return types.DirnBehaviourPair{
				Dir:       elevio.MD_Stop,
				Behaviour: elevio.DoorOpen,
			}
		} else if ordersBelow(elevator) > 0 {
			return types.DirnBehaviourPair{
				Dir:       elevio.MD_Down,
				Behaviour: elevio.Moving,
			}
		}
	case elevio.MD_Down:
		if ordersBelow(elevator) > 0 {
			return types.DirnBehaviourPair{
				Dir:       elevio.MD_Down,
				Behaviour: elevio.Moving,
			}
		} else if ordersHere(elevator) > 0 {
			return types.DirnBehaviourPair{
				Dir:       elevio.MD_Stop,
				Behaviour: elevio.DoorOpen,
			}
		} else if ordersAbove(elevator) > 0 {
			return types.DirnBehaviourPair{
				Dir:       elevio.MD_Up,
				Behaviour: elevio.Moving,
			}
		}
	case elevio.MD_Stop:
		if ordersAbove(elevator) > 0 {
			return types.DirnBehaviourPair{
				Dir:       elevio.MD_Up,
				Behaviour: elevio.Moving,
			}
		} else if ordersBelow(elevator) > 0 {
			return types.DirnBehaviourPair{
				Dir:       elevio.MD_Down,
				Behaviour: elevio.Moving,
			}
		} else if ordersHere(elevator) > 0 {
			return types.DirnBehaviourPair{
				Dir:       elevio.MD_Stop,
				Behaviour: elevio.DoorOpen,
			}
		}
	}
	return types.DirnBehaviourPair{
		Dir:       elevio.MD_Stop,
		Behaviour: elevio.Idle,
	}
}

// Checks if the elevator should stop at the current floor
func shouldStop(elevator *types.Elevator) bool {
	currentorders := elevator.Orders[elevator.Floor]
	// At edge floors, always stop
	if elevator.Floor == 0 || elevator.Floor == config.N_FLOORS-1 {
		return true
	}
	switch elevator.Dir {
	case elevio.MD_Down:
		return currentorders[elevio.BT_HallDown] != 0 ||
			currentorders[elevio.BT_Cab] != 0
	case elevio.MD_Up:
		return currentorders[elevio.BT_HallUp] != 0 ||
			currentorders[elevio.BT_Cab] != 0
	case elevio.MD_Stop:
		return true
	default:
		return false
	}
}

// Clears orders at current floor
func clearOrdersAtFloor(elevator *types.Elevator) {
	switch elevator.Config.ClearOrderVariant {
	case elevio.CV_All:
		// At edge floors, clear all orders
		if elevator.Floor == 0 || elevator.Floor == config.N_FLOORS-1 {
			for btn := 0; btn < config.N_BUTTONS; btn++ {
				elevator.Orders[elevator.Floor][btn] = 0
				elevio.SetButtonLamp(elevio.ButtonType(btn), elevator.Floor, false)
			}
			return
		}
		fallthrough
	case elevio.CV_InDirn:
		clearOrderAndLamp(elevator, elevio.BT_Cab)
		switch elevator.Dir {
		case elevio.MD_Up:
			clearOrderAndLamp(elevator, elevio.BT_HallUp)
		case elevio.MD_Down:
			clearOrderAndLamp(elevator, elevio.BT_HallDown)
		}
	}
}

// Helper function to clear orders and lamps
func clearOrderAndLamp(elevator *types.Elevator, btn elevio.ButtonType) {
	elevator.Orders[elevator.Floor][btn] = 0
	elevio.SetButtonLamp(btn, elevator.Floor, false)
}
