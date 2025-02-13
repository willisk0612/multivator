package elev

import (
	"fmt"
	"log/slog"
	"main/lib/driver-go/elevio"
	"main/src/config"
	"main/src/timer"
	"main/src/types"
)

// Processes cab and hall orders
func ProcessOrder(elevator *types.Elevator, floor int, btn types.ButtonType, timerAction chan timer.TimerAction) error {
	if elevator.Floor == floor {
		return openDoor(elevator, timerAction)
	}
	elevator.Orders[floor][btn] = true
	elevator.Dir = chooseDirInit(elevator).Dir
	return moveElev(elevator)
}

func HandleButtonPress(elevator *types.Elevator, btn types.ButtonEvent, timerAction chan timer.TimerAction, btnEventCh chan<- types.ButtonEvent, outMsgCh chan<- types.Message, assignmentCh chan<- types.OrderAssignment) {
	if btn.Button == types.BT_Cab {
		elevator.Orders[btn.Floor][btn.Button] = true
		elevio.SetButtonLamp(btn.Button, btn.Floor, true)
		if elevator.Behaviour == types.DoorOpen && elevator.Floor == btn.Floor {
			timerAction <- timer.Start
			clearFloor(elevator)
		} else if elevator.Behaviour == types.Idle {
			if err := ProcessOrder(elevator, btn.Floor, btn.Button, timerAction); err != nil {
				slog.Error("Cab call: failed to process order", "error", err, "target_floor", btn.Floor)
			} else if elevator.Floor == btn.Floor {
				clearFloor(elevator)
			}
		}
	} else {
		if btnEventCh != nil {
			btnEventCh <- btn
		}
	}
}

// Move elevator, update state. Includes safety check to avoid moving while door is open.
func moveElev(elevator *types.Elevator) error {
	if elevator.Behaviour == types.DoorOpen {
		return fmt.Errorf("cannot move while door is open")
	}
	elevator.Behaviour = types.Moving
	elevio.SetMotorDirection(elevator.Dir)
	return nil
}

// Stops elevator at floor and opens door.
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

	if ordersHere(elevator) > 0 {
		elevio.SetMotorDirection(types.MD_Stop)
		elevator.Behaviour = types.Idle
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

// Stops elevator and opens door.
func HandleObstruction(elevator *types.Elevator, obstruction bool, timerAction chan timer.TimerAction) {
	elevator.Obstructed = obstruction
	slog.Debug("Obstruction state changed",
		"obstructed", obstruction,
		"floor", elevator.Floor,
		"behaviour", elevator.Behaviour)

	if obstruction {
		elevio.SetMotorDirection(types.MD_Stop)
		if elevio.GetFloor() != -1 {
			if err := openDoor(elevator, timerAction); err == nil {
				slog.Debug("Door opened due to obstruction", "floor", elevator.Floor)
			} else {
				slog.Error("Failed to open door on obstruction", "error", err, "floor", elevator.Floor)
			}
		} else {
			elevator.Behaviour = types.Idle
			slog.Debug("Stopped between floors due to obstruction")
		}
	} else {
		if elevator.Behaviour == types.DoorOpen {
			timerAction <- timer.Start
			slog.Debug("Obstruction cleared, restarting door timer")
		} else {
			pair := chooseDirInit(elevator)
			elevator.Dir = pair.Dir

			if pair.Behaviour == types.Moving {
				if err := moveElev(elevator); err == nil {
					slog.Debug("Obstruction cleared, resuming movement", "direction", elevator.Dir)
				} else {
					slog.Error("Failed to resume after obstruction", "error", err, "floor", elevator.Floor)
				}
			}
		}
	}
}

// Stops elevator and clears all orders and button lamps.
func HandleStop(elevator *types.Elevator) {
	elevio.SetMotorDirection(types.MD_Stop)
	elevio.SetDoorOpenLamp(false)
	for f := 0; f < config.N_FLOORS; f++ {
		for b := types.ButtonType(0); b < config.N_BUTTONS; b++ {
			elevator.Orders[f][b] = false
			elevio.SetButtonLamp(b, f, false)
		}
	}
}

// Handles door timeout with obstruction check.
func HandleDoorTimeout(elevator *types.Elevator, timerAction chan timer.TimerAction) {
	if elevator.Behaviour != types.DoorOpen {
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
	elevator.Behaviour = types.Idle

	pair := chooseDirInit(elevator)
	elevator.Dir = pair.Dir

	if pair.Behaviour == types.Moving {
		if err := moveElev(elevator); err == nil {
			slog.Debug("Starting movement", "direction", elevator.Dir, "floor", elevator.Floor)
		} else {
			slog.Error("Failed to start movement", "error", err, "floor", elevator.Floor, "current_behaviour", elevator.Behaviour)
		}
	}
}

// Open door, update state. Includes safety check to avoid opening door while moving.
func openDoor(elevator *types.Elevator, timerAction chan timer.TimerAction) error {
	if elevator.Behaviour == types.Moving {
		return fmt.Errorf("cannot open door while moving")
	}
	elevator.Behaviour = types.DoorOpen
	elevio.SetDoorOpenLamp(true)
	timerAction <- timer.Start
	return nil
}

func chooseDirWhileMoving(elevator *types.Elevator, dir types.MotorDirection) types.DirnBehaviourPair {
	switch dir {
	case types.MD_UP:
		if ordersAbove(elevator) > 0 {
			return types.DirnBehaviourPair{Dir: dir, Behaviour: types.Moving}
		}
	case types.MD_Down:
		if ordersBelow(elevator) > 0 {
			return types.DirnBehaviourPair{Dir: dir, Behaviour: types.Moving}
		}
	}

	if ordersHere(elevator) > 0 {
		return types.DirnBehaviourPair{Dir: types.MD_Stop, Behaviour: types.DoorOpen}
	}

	// Check opposite direction if no orders in current direction.
	if dir == types.MD_UP && ordersBelow(elevator) > 0 {
		return types.DirnBehaviourPair{Dir: types.MD_Down, Behaviour: types.Moving}
	} else if dir == types.MD_Down && ordersAbove(elevator) > 0 {
		return types.DirnBehaviourPair{Dir: types.MD_UP, Behaviour: types.Moving}
	}

	return types.DirnBehaviourPair{Dir: types.MD_Stop, Behaviour: types.Idle}
}

// Uses the elevator algorithm (SCAN) to choose direction.
func chooseDirInit(elevator *types.Elevator) types.DirnBehaviourPair {
	var pair types.DirnBehaviourPair

	if elevator.Dir == types.MD_Stop {
		if ordersAbove(elevator) > 0 {
			pair = types.DirnBehaviourPair{Dir: types.MD_UP, Behaviour: types.Moving}
		} else if ordersBelow(elevator) > 0 {
			pair = types.DirnBehaviourPair{Dir: types.MD_Down, Behaviour: types.Moving}
		} else {
			pair = types.DirnBehaviourPair{Dir: types.MD_Stop, Behaviour: types.Idle}
		}
	} else {
		pair = chooseDirWhileMoving(elevator, elevator.Dir)
	}

	// Validate state transition if moving.
	if pair.Behaviour == types.Moving {
		if elevator.Behaviour == types.DoorOpen {
			pair.Behaviour = types.Idle
			pair.Dir = types.MD_Stop
		}
	}
	return pair
}

func shouldStop(elevator *types.Elevator, btn types.ButtonType) bool {
	currentorders := elevator.Orders[elevator.Floor]
	// At edge floors, stop if any order is set.
	if elevator.Floor == 0 || elevator.Floor == config.N_FLOORS-1 {
		if currentorders[types.BT_HallUp] || currentorders[types.BT_HallDown] || currentorders[types.BT_Cab] {
			return true
		}
		return false
	}
	switch elevator.Dir {
	case types.MD_Down:
		// Check if the button from the event is set
		return currentorders[btn] || currentorders[types.BT_Cab]
	case types.MD_UP:
		return currentorders[btn] || currentorders[types.BT_Cab]
	case types.MD_Stop:
		return true
	default:
		return false
	}
}

// Clears cab order and current direction hall order and lamp.
func clearFloor(elevator *types.Elevator) {
	clearOrderAndLamp(elevator, types.BT_Cab)

	// At edge floors, clear all orders.
	if elevator.Floor == 0 || elevator.Floor == config.N_FLOORS-1 {
		clearOrderAndLamp(elevator, types.BT_HallUp)
		clearOrderAndLamp(elevator, types.BT_HallDown)
		return
	}

	switch elevator.Dir {
	case types.MD_UP:
		clearOrderAndLamp(elevator, types.BT_HallUp)
		if ordersAbove(elevator) == 0 {
			clearOrderAndLamp(elevator, types.BT_HallDown)
		}
	case types.MD_Down:
		clearOrderAndLamp(elevator, types.BT_HallDown)
		if ordersBelow(elevator) == 0 {
			clearOrderAndLamp(elevator, types.BT_HallUp)
		}
	case types.MD_Stop:
		clearOrderAndLamp(elevator, types.BT_HallUp)
		clearOrderAndLamp(elevator, types.BT_HallDown)
	}
}

// Clears order and lamp for button press.
func clearOrderAndLamp(elevator *types.Elevator, btn types.ButtonType) {
	elevator.Orders[elevator.Floor][btn] = false
	elevio.SetButtonLamp(btn, elevator.Floor, false)
}

func countOrders(elevator *types.Elevator, startFloor int, endFloor int) (result int) {
	for floor := startFloor; floor < endFloor; floor++ {
		for btn := 0; btn < config.N_BUTTONS; btn++ {
			if elevator.Orders[floor][btn] {
				result++
			}
		}
	}
	return result
}

func ordersAbove(elevator *types.Elevator) int {
	return countOrders(elevator, elevator.Floor+1, config.N_FLOORS)
}

func ordersBelow(elevator *types.Elevator) int {
	return countOrders(elevator, 0, elevator.Floor)
}

func ordersHere(elevator *types.Elevator) int {
	return countOrders(elevator, elevator.Floor, elevator.Floor+1)
}
