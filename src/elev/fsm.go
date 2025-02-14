package elev

import (
	"log/slog"
	"main/lib/driver-go/elevio"
	"main/src/config"
	"main/src/timer"
	"main/src/types"
)

// Handles button presses. In case of cab button, move elevator to floor and open door. In case of hall button, send hall call to network module.
func HandleButtonPress(elevator *types.Elevator, btn types.ButtonEvent, timerAction chan timer.TimerAction, hallEventCh chan<- types.ButtonEvent, outMsgCh chan<- types.Message) {
	if btn.Button == types.BT_Cab || elevio.GetFloor() == -1 {
		MoveElevator(elevator, btn, timerAction)
	} else {
		if hallEventCh != nil {
			hallEventCh <- btn
		}
	}
}

// Move elevator to floor, set order and lamp
func MoveElevator(elevator *types.Elevator, btn types.ButtonEvent, timerAction chan timer.TimerAction) {
	elevator.Orders[btn.Floor][btn.Button] = true
	elevio.SetButtonLamp(btn.Button, btn.Floor, true)

	if elevator.Floor == btn.Floor {
		OpenDoor(elevator, timerAction)
		clearFloor(elevator)
		return
	}

	if elevator.Behaviour == types.Idle || elevio.GetFloor() == -1 {
		if elevator.Floor == btn.Floor {
			OpenDoor(elevator, timerAction)
		}
		elevator.Orders[btn.Floor][btn.Button] = true
		elevator.Dir = chooseDirInit(elevator).Dir
		moveMotor(elevator)
	}
}

// Checks if elevator should stop at floor and opens door if so.
func HandleFloorArrival(elevator *types.Elevator, floor int, timerAction chan timer.TimerAction) {
	if floor == -1 {
		slog.Error("Floor arrival with undefined floor")
		return
	}
	elevator.Floor = floor
	elevio.SetFloorIndicator(floor)

	if shouldStop(elevator) {
		elevio.SetMotorDirection(types.MD_Stop)
		elevator.Behaviour = types.Idle
		clearFloor(elevator)
		OpenDoor(elevator, timerAction)
	} else {
		slog.Debug("Continuing past floor",
			"floor", floor,
			"direction", elevator.Dir)
	}
}

// Monitors obstruction state and stops elevator and door from closing if obstruction is detected.
func HandleObstruction(elevator *types.Elevator, obstruction bool, timerAction chan timer.TimerAction) {
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
			pair := chooseDirInit(elevator)
			elevator.Dir = pair.Dir

			if pair.Behaviour == types.Moving {
				moveMotor(elevator)
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
		moveMotor(elevator)
	}
}

// Open door, update state. Includes safety check to avoid opening door while moving.
func OpenDoor(elevator *types.Elevator, timerAction chan timer.TimerAction) {
	if elevator.Behaviour == types.Moving || elevio.GetFloor() == -1 {
		slog.Warn("Cannot open door while moving or between floors")
		return
	}
	elevator.Behaviour = types.DoorOpen
	elevio.SetDoorOpenLamp(true)
	timerAction <- timer.Start
}

// Move motor with safety check to avoid moving while door is open.
func moveMotor(elevator *types.Elevator) {
	if elevator.Behaviour == types.DoorOpen {
		slog.Debug("Cannot move while door is open")
		return
	}
	elevator.Behaviour = types.Moving
	elevio.SetMotorDirection(elevator.Dir)
}

// Algorithm that only goes as far as the final order in that direction, then reverses.
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

	if pair.Behaviour == types.Moving {
		if elevator.Behaviour == types.DoorOpen {
			pair.Behaviour = types.Idle
			pair.Dir = types.MD_Stop
		}
	}
	return pair
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

func shouldStop(elevator *types.Elevator) bool {
	currentorders := elevator.Orders[elevator.Floor]

	if currentorders[types.BT_Cab] ||
		currentorders[types.BT_HallUp] ||
		currentorders[types.BT_HallDown] {
		return true
	}

	// Always stop at edge floors
	if elevator.Floor == 0 || elevator.Floor == config.N_FLOORS-1 {
		return true
	}

	return false
}
