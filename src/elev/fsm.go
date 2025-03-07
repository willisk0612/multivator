// Contains finite state machine helper functions for single elevator control.
package elev

import (
	"log/slog"

	"multivator/lib/driver-go/elevio"
	"multivator/src/timer"
	"multivator/src/types"
)

// Checks if elevator should stop at floor and opens door if so.
func HandleFloorArrival(elevator *types.ElevState, floor int, timerAction chan timer.TimerAction) {
	slog.Debug("Entered HandleFloorArrival", "floor", floor)
	elevator.Floor = floor
	elevio.SetFloorIndicator(floor)

	if shouldStopHere(elevator) {
		elevio.SetMotorDirection(types.MD_Stop)
		clearAtCurrentFloor(elevator)
		OpenDoor(elevator, timerAction)
	}
}

// Monitors obstruction state and stops elevator and door from closing if obstruction is detected.
func HandleObstruction(elevator *types.ElevState, obstruction bool, timerAction chan timer.TimerAction) {
	elevator.Obstructed = obstruction

	if obstruction {
		elevio.SetMotorDirection(types.MD_Stop)
		if elevio.GetFloor() != -1 {
			OpenDoor(elevator, timerAction)
		} else {
			elevator.Behaviour = types.Idle
		}
	} else {
		if elevator.Behaviour == types.DoorOpen {
			timerAction <- timer.Start
		} else {
			pair := chooseDirection(elevator)
			elevator.Dir = pair.Dir

			if pair.Behaviour == types.Moving {
				elevator.Dir = chooseDirection(elevator).Dir
				elevio.SetMotorDirection(elevator.Dir)
				elevator.Behaviour = types.Moving
			}
		}
	}
}

// Handles door timeout with obstruction check.
func HandleDoorTimeout(elevator *types.ElevState, timerAction chan<- timer.TimerAction) {
	if elevator.Behaviour != types.DoorOpen {
		return
	}

	if elevator.Obstructed {
		timerAction <- timer.Start
		return
	}
	elevio.SetDoorOpenLamp(false)
	elevator.Behaviour = types.Idle
	clearAtCurrentFloor(elevator)

	pair := chooseDirection(elevator)
	elevator.Dir = pair.Dir

	if pair.Behaviour == types.Moving {
		elevator.Dir = chooseDirection(elevator).Dir
		elevio.SetMotorDirection(elevator.Dir)
		elevator.Behaviour = types.Moving
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
	timerAction <- timer.Start
}
