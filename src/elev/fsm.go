// Contains finite state machine helper functions for single elevator control.
package elev

import (
	"log/slog"

	"multivator/lib/driver-go/elevio"
	"multivator/src/config"
	"multivator/src/timer"
	"multivator/src/types"
)

// Handles button presses. In case of cab button, move elevator to floor and open door. In case of hall button, send hall call to network module.
func HandleButtonPress(elevator *types.ElevState, btn types.ButtonEvent, timerAction chan timer.TimerAction, hallOrderCh chan<- types.ButtonEvent, outMsgCh chan<- types.Message[types.Bid]) {
	if btn.Button == types.BT_Cab || elevio.GetFloor() == -1 {
		MoveElevator(elevator, btn, timerAction)
	} else if hallOrderCh != nil {
		hallOrderCh <- btn
	}
}

// Checks if elevator should stop at floor and opens door if so.
func HandleFloorArrival(elevator *types.ElevState, floor int, timerAction chan timer.TimerAction) {
	if floor == -1 {
		slog.Error("Floor arrival with undefined floor")
		return
	}
	elevator.Floor = floor
	elevio.SetFloorIndicator(floor)

	if shouldStop(elevator) {
		slog.Debug("Stopping at floor", "floor", floor)
		elevio.SetMotorDirection(types.MD_Stop)
		clearFloor(elevator)
		OpenDoor(elevator, timerAction)
	} else {
		slog.Debug("Continuing past floor",
			"floor", floor,
			"direction", elevator.Dir)
	}
}

// Monitors obstruction state and stops elevator and door from closing if obstruction is detected.
func HandleObstruction(elevator *types.ElevState, obstruction bool, timerAction chan timer.TimerAction) {
	elevator.Obstructed = obstruction
	slog.Debug("Obstruction state changed",
		"obstructed", obstruction,
		"floor", elevator.Floor,
		"behaviour", elevator.Behaviour)

	if obstruction {
		elevio.SetMotorDirection(types.MD_Stop)
		if elevio.GetFloor() != -1 {
			OpenDoor(elevator, timerAction)
		} else {
			elevator.Behaviour = types.Idle
			slog.Debug("Stopped between floors due to obstruction")
		}
	} else {
		if elevator.Behaviour == types.DoorOpen {
			timerAction <- timer.Start
			slog.Debug("Obstruction cleared, restarting door timer")
		} else {
			pair := chooseDirIdle(elevator)
			elevator.Dir = pair.Dir

			if pair.Behaviour == types.Moving {
				moveMotor(elevator)
			}
		}
	}
}

// Stops elevator and clears all orders and button lamps.
func HandleStop(elevator *types.ElevState) {
	elevio.SetMotorDirection(types.MD_Stop)
	elevio.SetDoorOpenLamp(false)
	for f := range config.NumFloors {
		for b := types.ButtonType(0); b < config.NumButtons; b++ {
			elevator.Orders[elevator.NodeID][f][b] = false
			elevio.SetButtonLamp(b, f, false)
		}
	}
}

// Handles door timeout with obstruction check.
func HandleDoorTimeout(elevator *types.ElevState, timerAction chan<- timer.TimerAction) {
	if elevator.Behaviour != types.DoorOpen {
		slog.Debug("Door timeout ignored - door not open",
			"behaviour", elevator.Behaviour)
		return
	}
	slog.Debug("Door timer expired",
		"floor", elevator.Floor,
		"obstructed", elevator.Obstructed)

	if elevator.Obstructed {
		slog.Debug("Door obstructed, keeping open and restarting timer")
		timerAction <- timer.Start
		return
	}

	slog.Debug("Closing door and changing state",
		"floor", elevator.Floor)
	elevio.SetDoorOpenLamp(false)
	elevator.Behaviour = types.Idle
	clearFloor(elevator)

	pair := chooseDirIdle(elevator)
	elevator.Dir = pair.Dir

	if pair.Behaviour == types.Moving {
		moveMotor(elevator)
	}
}

// Move elevator to floor, set order and lamp
func MoveElevator(elevator *types.ElevState, btn types.ButtonEvent, timerAction chan timer.TimerAction) {
	slog.Debug("Moving elevator to floor", "floor", btn.Floor)
	if elevator.Floor == btn.Floor {
		slog.Debug("Elevator already at floor")
		OpenDoor(elevator, timerAction)
	} else {
		slog.Debug("Setting order and moving elevator")
		elevator.Orders[elevator.NodeID][btn.Floor][btn.Button] = true
		elevio.SetButtonLamp(btn.Button, btn.Floor, true)
		elevator.Dir = chooseDirIdle(elevator).Dir
		moveMotor(elevator)
	}
}

// Open door, update state. Includes safety check to avoid opening door while moving.
func OpenDoor(elevator *types.ElevState, timerAction chan<- timer.TimerAction) {
	if elevio.GetFloor() == -1 {
		slog.Warn("Cannot open door while between floors")
		return
	}
	elevator.Behaviour = types.DoorOpen
	elevio.SetDoorOpenLamp(true)
	slog.Debug("Starting door timer")
	timerAction <- timer.Start
}

// Move motor with safety check to avoid moving while door is open.
func moveMotor(elevator *types.ElevState) {
	if elevator.Behaviour == types.DoorOpen {
		slog.Debug("Cannot move while door is open")
		return
	}
	elevator.Behaviour = types.Moving
	elevio.SetMotorDirection(elevator.Dir)
}

// Algorithm that only goes as far as the final order in that direction, then reverses.
func chooseDirIdle(elevator *types.ElevState) types.DirnBehaviourPair {
	var pair types.DirnBehaviourPair

	if elevator.Dir == types.MD_Stop {
		switch {
		case ordersAbove(elevator) > 0:
			pair = types.DirnBehaviourPair{Dir: types.MD_Up, Behaviour: types.Moving}
		case ordersBelow(elevator) > 0:
			pair = types.DirnBehaviourPair{Dir: types.MD_Down, Behaviour: types.Moving}
		default:
			pair = types.DirnBehaviourPair{Dir: types.MD_Stop, Behaviour: types.Idle}
		}
	} else {
		pair = chooseDirMoving(elevator, elevator.Dir)
	}

	if pair.Behaviour == types.Moving {
		if elevator.Behaviour == types.DoorOpen {
			pair.Behaviour = types.Idle
			pair.Dir = types.MD_Stop
		}
	}
	return pair
}

func chooseDirMoving(elevator *types.ElevState, dir types.MotorDirection) types.DirnBehaviourPair {
	switch dir {
	case types.MD_Up:
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
	if dir == types.MD_Up && ordersBelow(elevator) > 0 {
		return types.DirnBehaviourPair{Dir: types.MD_Down, Behaviour: types.Moving}
	} else if dir == types.MD_Down && ordersAbove(elevator) > 0 {
		return types.DirnBehaviourPair{Dir: types.MD_Up, Behaviour: types.Moving}
	}

	return types.DirnBehaviourPair{Dir: types.MD_Stop, Behaviour: types.Idle}
}
