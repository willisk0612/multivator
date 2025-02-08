// Contains logic for single elevator
package elev

import (
	"fmt"
	"log/slog"
	"main/lib/driver-go/elevio"
	"main/src/config"
	"main/src/timer"
	"main/src/types"
)

// Updates elevator state and motor direction based on button press
func HandleButtonPress(elevator *types.Elevator, btn elevio.ButtonEvent, timerAction chan timer.TimerAction, eventCh chan<- types.ButtonEvent) {
	elevator.Orders[btn.Floor][btn.Button] = 1
	elevio.SetButtonLamp(btn.Button, btn.Floor, true)

	slog.Debug("Button pressed",
		"floor", btn.Floor,
		"button", btn.Button,
		"current_floor", elevator.Floor,
		"behaviour", elevator.Behaviour)

	if eventCh != nil {
		eventCh <- types.ButtonEvent{
			Floor:  btn.Floor,
			Button: btn.Button,
		}
	}

	switch elevator.Behaviour {
	case elevio.DoorOpen:
		if elevator.Floor == btn.Floor {
			timerAction <- timer.Start
			slog.Debug("Door timer reset due to button press at current floor")
			clearFloor(elevator)
		}
	case elevio.Moving:
		return
	case elevio.Idle:
		if elevator.Floor == btn.Floor {
			if err := openDoor(elevator, timerAction); err == nil {
				slog.Debug("Door opened for button press at current floor")
				clearFloor(elevator)
			} else {
				slog.Error("Failed to open door for button press", "error", err, "floor", btn.Floor)
			}
		} else {
			pair := chooseDirInit(elevator)
			elevator.Dir = pair.Dir
			if err := moveElev(elevator); err == nil {
				slog.Debug("Starting movement for button press", "target_floor", btn.Floor, "direction", elevator.Dir)
			} else {
				slog.Error("Failed to start movement for button press", "error", err, "target_floor", btn.Floor)
			}
		}
	}
}

// Stops elevator at floor and opens door
func HandleFloorArrival(elevator *types.Elevator, floor int, timerAction chan timer.TimerAction) {
	if floor == -1 {
		slog.Error("Floor arrival with undefined floor")
		return
	}
	elevator.Floor = floor
	elevio.SetFloorIndicator(floor)

	slog.Debug("Floor arrival",
		"floor", floor,
		"direction", elevator.Dir,
		"behaviour", elevator.Behaviour)

	if shouldStop(elevator) {
		elevio.SetMotorDirection(elevio.MD_Stop)
		elevator.Behaviour = elevio.Idle
		if err := openDoor(elevator, timerAction); err == nil {
			slog.Debug("Door opened", "floor", floor, "behaviour", elevator.Behaviour)
			clearFloor(elevator)
		} else {
			slog.Error("Failed to open door", "error", err, "floor", floor, "current_behaviour", elevator.Behaviour)
		}
	} else {
		slog.Debug("Continuing past floor",
			"floor", floor,
			"direction", elevator.Dir)
	}
}

// Stops elevator and opens door
func HandleObstruction(elevator *types.Elevator, obstruction bool, timerAction chan timer.TimerAction) {
	elevator.Obstructed = obstruction
	slog.Debug("Obstruction state changed",
		"obstructed", obstruction,
		"floor", elevator.Floor,
		"behaviour", elevator.Behaviour)

	if obstruction {
		elevio.SetMotorDirection(elevio.MD_Stop)
		if elevio.GetFloor() != -1 {
			if err := openDoor(elevator, timerAction); err == nil {
				slog.Debug("Door opened due to obstruction", "floor", elevator.Floor)
			} else {
				slog.Error("Failed to open door on obstruction", "error", err, "floor", elevator.Floor)
			}
		} else {
			elevator.Behaviour = elevio.Idle
			slog.Debug("Stopped between floors due to obstruction")
		}
	} else {
		if elevator.Behaviour == elevio.DoorOpen {
			timerAction <- timer.Start
			slog.Debug("Obstruction cleared, restarting door timer")
		} else {
			pair := chooseDirInit(elevator)
			elevator.Dir = pair.Dir

			if pair.Behaviour == elevio.Moving {
				if err := moveElev(elevator); err == nil {
					slog.Debug("Obstruction cleared, resuming movement", "direction", elevator.Dir)
				} else {
					slog.Error("Failed to resume after obstruction", "error", err, "floor", elevator.Floor)
				}
			}
		}
	}
}

// Stops elevator and clears all orders and button lamps
func HandleStop(elevator *types.Elevator) {
	elevio.SetMotorDirection(elevio.MD_Stop)
	elevio.SetDoorOpenLamp(false)
	for f := 0; f < config.N_FLOORS; f++ {
		for b := elevio.ButtonType(0); b < config.N_BUTTONS; b++ {
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
	slog.Debug("Door timer expired")

	if elevator.Obstructed {
		slog.Debug("Door obstructed, keeping open")
		timerAction <- timer.Start
		return
	}

	slog.Debug("Closing door", "floor", elevator.Floor)
	elevio.SetDoorOpenLamp(false)
	elevator.Behaviour = elevio.Idle

	pair := chooseDirInit(elevator)
	elevator.Dir = pair.Dir

	if pair.Behaviour == elevio.Moving {
		if err := moveElev(elevator); err == nil {
			slog.Debug("Starting movement", "direction", elevator.Dir, "floor", elevator.Floor)
		} else {
			slog.Error("Failed to start movement", "error", err, "floor", elevator.Floor, "current_behaviour", elevator.Behaviour)
		}
	}
}

// Move elevator, update state. Includes safety check to avoid moving while door is open
func moveElev(elevator *types.Elevator) error {
	if elevator.Behaviour == elevio.DoorOpen {
		return fmt.Errorf("cannot move while door is open")
	}
	elevator.Behaviour = elevio.Moving
	elevio.SetMotorDirection(elevator.Dir)
	return nil
}

// Open door, update state. Includes safety check to avoid opening door while moving
func openDoor(elevator *types.Elevator, timerAction chan timer.TimerAction) error {
	if elevator.Behaviour == elevio.Moving {
		return fmt.Errorf("cannot open door while moving")
	}
	elevator.Behaviour = elevio.DoorOpen
	elevio.SetDoorOpenLamp(true)
	timerAction <- timer.Start
	return nil
}

func chooseDirWhileMoving(elevator *types.Elevator, dir elevio.MotorDirection) types.DirnBehaviourPair {
	switch dir {
	case elevio.MD_Up:
		if ordersAbove(elevator) > 0 {
			return types.DirnBehaviourPair{Dir: dir, Behaviour: elevio.Moving}
		}
	case elevio.MD_Down:
		if ordersBelow(elevator) > 0 {
			return types.DirnBehaviourPair{Dir: dir, Behaviour: elevio.Moving}
		}
	}

	if ordersHere(elevator) > 0 {
		return types.DirnBehaviourPair{Dir: elevio.MD_Stop, Behaviour: elevio.DoorOpen}
	}

	// Check opposite direction if no orders in current direction
	if dir == elevio.MD_Up && ordersBelow(elevator) > 0 {
		return types.DirnBehaviourPair{Dir: elevio.MD_Down, Behaviour: elevio.Moving}
	} else if dir == elevio.MD_Down && ordersAbove(elevator) > 0 {
		return types.DirnBehaviourPair{Dir: elevio.MD_Up, Behaviour: elevio.Moving}
	}

	return types.DirnBehaviourPair{Dir: elevio.MD_Stop, Behaviour: elevio.Idle}
}

// Uses the elevator algorithm (SCAN) to choose direction
func chooseDirInit(elevator *types.Elevator) types.DirnBehaviourPair {
	var pair types.DirnBehaviourPair

	if elevator.Dir == elevio.MD_Stop {
		if ordersAbove(elevator) > 0 {
			pair = types.DirnBehaviourPair{Dir: elevio.MD_Up, Behaviour: elevio.Moving}
		} else if ordersBelow(elevator) > 0 {
			pair = types.DirnBehaviourPair{Dir: elevio.MD_Down, Behaviour: elevio.Moving}
		} else {
			pair = types.DirnBehaviourPair{Dir: elevio.MD_Stop, Behaviour: elevio.Idle}
		}
	} else {
		pair = chooseDirWhileMoving(elevator, elevator.Dir)
	}

	// Validate state transition if moving
	if pair.Behaviour == elevio.Moving {
		if elevator.Behaviour == elevio.DoorOpen {
			pair.Behaviour = elevio.Idle
			pair.Dir = elevio.MD_Stop
		}
	}
	return pair
}

// Checks if elevator should stop at current floor
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

// Clears cab order and current direction hall order and lamp
func clearFloor(elevator *types.Elevator) {
	clearOrderAndLamp(elevator, elevio.BT_Cab)

	// At edge floors, clear all orders
	if elevator.Floor == 0 || elevator.Floor == config.N_FLOORS-1 {
		clearOrderAndLamp(elevator, elevio.BT_HallUp)
		clearOrderAndLamp(elevator, elevio.BT_HallDown)
		return
	}

	switch elevator.Dir {
	case elevio.MD_Up:
		clearOrderAndLamp(elevator, elevio.BT_HallUp)
		if ordersAbove(elevator) == 0 {
			clearOrderAndLamp(elevator, elevio.BT_HallDown)
		}
	case elevio.MD_Down:
		clearOrderAndLamp(elevator, elevio.BT_HallDown)
		if ordersBelow(elevator) == 0 {
			clearOrderAndLamp(elevator, elevio.BT_HallUp)
		}
	case elevio.MD_Stop:
		clearOrderAndLamp(elevator, elevio.BT_HallUp)
		clearOrderAndLamp(elevator, elevio.BT_HallDown)
	}
}

func clearOrderAndLamp(elevator *types.Elevator, btn elevio.ButtonType) {
	elevator.Orders[elevator.Floor][btn] = 0
	elevio.SetButtonLamp(btn, elevator.Floor, false)
}

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

func ordersAbove(elevator *types.Elevator) (result int) {
	return countOrders(elevator, elevator.Floor+1, config.N_FLOORS)
}

func ordersBelow(elevator *types.Elevator) (result int) {
	return countOrders(elevator, 0, elevator.Floor)
}

func ordersHere(elevator *types.Elevator) (result int) {
	return countOrders(elevator, elevator.Floor, elevator.Floor+1)
}
