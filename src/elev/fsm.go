package elev

import (
	"log/slog"
	"main/lib/driver-go/elevio"
	"main/src/config"
	"main/src/timer"
	"main/src/types"
)

// Checks if elevator should stop at floor and opens door if so.
func HandleFloorArrival(elevMgr *types.ElevatorManager, floor int, timerAction chan timer.TimerAction) {
	if elevio.GetFloor() == -1 {
		slog.Error("Floor arrival with undefined floor")
		return
	}
	// Need to use elevMgr.Execute to update the elevator state
	UpdateState(elevMgr, types.ElevFloor, floor)
	UpdateState(elevMgr, types.ElevBetweenFloors, false)
	elevator := GetElevState(elevMgr)
	elevio.SetFloorIndicator(floor)
	if shouldStop(elevator) {
		slog.Debug("Stopping at floor", "floor", floor)
		elevio.SetMotorDirection(types.MD_Stop)
		clearFloor(elevator)
		OpenDoor(elevMgr, timerAction)
	} else {
		slog.Debug("Continuing past floor",
			"floor", floor,
			"direction", elevator.Dir)
	}
}

// Monitors obstruction state and stops elevator and door from closing if obstruction is detected.
func HandleObstruction(elevMgr *types.ElevatorManager, obstruction bool, timerAction chan timer.TimerAction) {
	elevator := GetElevState(elevMgr)
	UpdateState(elevMgr, types.ElevObstructed, obstruction)

	if obstruction {
		elevio.SetMotorDirection(types.MD_Stop)
		if elevio.GetFloor() != -1 {
			OpenDoor(elevMgr, timerAction)
		} else {
			UpdateState(elevMgr, types.ElevBehaviour, types.Idle)
			slog.Debug("Stopped between floors due to obstruction")
		}
	} else {
		if elevator.Behaviour == types.DoorOpen {
			timerAction <- timer.Start
			slog.Debug("Obstruction cleared, restarting door timer")
		} else {
			pair := chooseDirInit(elevMgr)
			UpdateState(elevMgr, types.ElevDir, pair.Dir)

			if pair.Behaviour == types.Moving {
				moveMotor(elevMgr)
			}
		}
	}
}

// Stops elevator and clears all orders and button lamps.
func HandleStop(elevMgr *types.ElevatorManager) {
	elevio.SetMotorDirection(types.MD_Stop)
	elevio.SetDoorOpenLamp(false)

	// Reset elevator state
	UpdateState(elevMgr, types.ElevDir, types.MD_Stop)
	UpdateState(elevMgr, types.ElevBehaviour, types.Idle)
	if elevio.GetFloor() == -1 {
		UpdateState(elevMgr, types.ElevBetweenFloors, true)
	}

	// Clear all orders and lamps
	for f := range config.NumFloors {
		for b := types.ButtonType(0); b < config.NumButtons; b++ {
			UpdateOrders(elevMgr, func(orders *[config.NumFloors][config.NumButtons]bool) {
				orders[f][b] = false
			})
			elevio.SetButtonLamp(b, f, false)
		}
	}
}

// Handles door timeout with obstruction check.
func HandleDoorTimeout(elevMgr *types.ElevatorManager, timerAction chan<- timer.TimerAction) {
	elevator := GetElevState(elevMgr)
	slog.Debug("Entered HandleDoorTimeout")
	if elevator.Behaviour != types.DoorOpen {
		slog.Debug("Door timeout ignored - door not open",
			"behaviour", elevator.Behaviour)
		return
	}
	if elevator.Obstructed {
		slog.Debug("Door obstructed, keeping open and restarting timer")
		timerAction <- timer.Start
		return
	}

	slog.Debug("Closing door and changing state",
		"floor", elevator.Floor)
	elevio.SetDoorOpenLamp(false)
	UpdateState(elevMgr, types.ElevBehaviour, types.Idle)
	clearFloor(elevator)

	pair := chooseDirInit(elevMgr)
	UpdateState(elevMgr, types.ElevDir, pair.Dir)

	if pair.Behaviour == types.Moving {
		moveMotor(elevMgr)
	}
}

// Move elevator to floor, set order and lamp
func MoveElevator(elevMgr *types.ElevatorManager, btn types.ButtonEvent, timerAction chan timer.TimerAction) {
	elevator := GetElevState(elevMgr)
	slog.Debug("Moving elevator to floor", "floor", btn.Floor)
	if elevator.Floor == btn.Floor {
		slog.Debug("Elevator already at floor")
		OpenDoor(elevMgr, timerAction)
	} else {
		slog.Debug("Setting order and moving elevator")
		UpdateOrders(elevMgr, func(orders *[config.NumFloors][config.NumButtons]bool) {
			orders[btn.Floor][btn.Button] = true
		})
		elevio.SetButtonLamp(btn.Button, btn.Floor, true)
		UpdateState(elevMgr, types.ElevDir, chooseDirInit(elevMgr).Dir)
		moveMotor(elevMgr)
	}
}

// Open door, update state. Includes safety check to avoid opening door while moving.
func OpenDoor(elevMgr *types.ElevatorManager, timerAction chan<- timer.TimerAction) {
	if elevio.GetFloor() == -1 {
		slog.Warn("Cannot open door while between floors")
		return
	}
	UpdateState(elevMgr, types.ElevBehaviour, types.DoorOpen)
	elevio.SetDoorOpenLamp(true)
	slog.Debug("Starting door timer")
	timerAction <- timer.Start
}

// Move motor with safety check to avoid moving while door is open.
func moveMotor(elevMgr *types.ElevatorManager) {
	elevator := GetElevState(elevMgr)
	if elevator.Behaviour == types.DoorOpen {
		slog.Debug("Cannot move while door is open")
		return
	}
	UpdateState(elevMgr, types.ElevBehaviour, types.Moving)
	UpdateState(elevMgr, types.ElevBetweenFloors, true)
	elevio.SetMotorDirection(elevator.Dir)
}

// Algorithm that only goes as far as the final order in that direction, then reverses.
func chooseDirInit(elevMgr *types.ElevatorManager) types.DirnBehaviourPair {
	elevator := GetElevState(elevMgr)
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

func shouldStop(elevator *types.Elevator) bool {
	if elevator.Floor < 0 {
		return false
	}
	currentorders := elevator.Orders[elevator.Floor]

	if currentorders[types.BT_Cab] ||
		currentorders[types.BT_HallUp] ||
		currentorders[types.BT_HallDown] {
		return true
	}

	// Always stop at edge floors
	if elevator.Floor == 0 || elevator.Floor == config.NumFloors-1 {
		return true
	}

	return false
}
