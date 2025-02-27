// Contains finite state machine helper functions for single elevator control.
package elev

import (
	"log/slog"

	"multivator/lib/driver-go/elevio"
	"multivator/src/config"
	"multivator/src/timer"
	"multivator/src/types"
)

// Checks if elevator should stop at floor and opens door if so.
func (elevMgr *ElevStateMgr) HandleFloorArrival(floor int, timerAction chan timer.TimerAction) {
	if elevio.GetFloor() == -1 {
		slog.Error("Floor arrival with undefined floor")
		return
	}
	elevMgr.UpdateState(ElevFloorField, floor)
	elevMgr.UpdateState(ElevBetweenFloorsField, false)
	elevator := elevMgr.GetState()
	elevio.SetFloorIndicator(floor)
	if elevator.shouldStop() {
		slog.Debug("Stopping at floor", "floor", floor)
		elevio.SetMotorDirection(types.MD_Stop)
		elevMgr.clearFloor()
		elevMgr.OpenDoor(timerAction)
	}
}

// Monitors obstruction state and stops elevator and door from closing if obstruction is detected.
func (elevMgr *ElevStateMgr) HandleObstruction(obstruction bool, timerAction chan timer.TimerAction) {
	elevator := elevMgr.GetState()
	elevMgr.UpdateState(ElevObstructedField, obstruction)

	if obstruction {
		elevio.SetMotorDirection(types.MD_Stop)
		if elevio.GetFloor() != -1 {
			elevMgr.OpenDoor(timerAction)
		} else {
			elevMgr.UpdateState(ElevBehaviourField, types.Idle)
			slog.Debug("Stopped between floors due to obstruction")
		}
	} else {
		if elevator.Behaviour == types.DoorOpen {
			timerAction <- timer.Start
			slog.Debug("Obstruction cleared, restarting door timer")
		} else {
			pair := elevMgr.chooseDirInit()
			elevMgr.UpdateState(ElevDirField, pair.Dir)

			if pair.Behaviour == types.Moving {
				elevMgr.moveMotor()
			}
		}
	}
}

// Stops elevator and clears all orders and button lamps.
func (elevMgr *ElevStateMgr) HandleStop() {
	elevio.SetMotorDirection(types.MD_Stop)
	elevio.SetDoorOpenLamp(false)

	elevator := elevMgr.GetState()

	// Reset elevator state
	elevMgr.UpdateState(ElevDirField, types.MD_Stop)
	elevMgr.UpdateState(ElevBehaviourField, types.Idle)
	if elevio.GetFloor() == -1 {
		elevMgr.UpdateState(ElevBetweenFloorsField, true)
	}

	// Clear all orders and lamps
	for f := range config.NumFloors {
		for b := types.ButtonType(0); b < config.NumButtons; b++ {
			elevMgr.UpdateOrders(func(orders *[config.NumFloors][config.NumButtons]bool) {
				orders[f][b] = false
			})

			// Handle lights based on button type
			if b == types.BT_Cab {
				// Cab lights are handled locally
				elevio.SetButtonLamp(b, f, false)
			} else {
				// Hall lights are handled by the light manager
				elevMgr.lightMsgCh <- types.Message{
					Type: types.LocalLightOff,
					Event: types.ButtonEvent{
						Floor:  f,
						Button: b,
					},
					SenderID: elevator.NodeID,
				}
				slog.Debug("Turning off hall light in HandleStop", "floor", f, "button", b)
			}
		}
	}
}

// Handles door timeout with obstruction check.
func (elevMgr *ElevStateMgr) HandleDoorTimeout(timerAction chan<- timer.TimerAction) {
	elevator := elevMgr.GetState()
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
	elevMgr.UpdateState(ElevBehaviourField, types.Idle)
	elevMgr.clearFloor()

	pair := elevMgr.chooseDirInit()
	elevMgr.UpdateState(ElevDirField, pair.Dir)

	if pair.Behaviour == types.Moving {
		elevMgr.moveMotor()
	}
}

// Move elevator to floor, set order and lamp
func (elevMgr *ElevStateMgr) MoveElevator(btn types.ButtonEvent, timerAction chan timer.TimerAction) {
	elevator := elevMgr.GetState()
	slog.Debug("Moving elevator to floor", "floor", btn.Floor, "button", btn.Button)

	// Update the current button event
	elevMgr.UpdateState(ElevCurrentBtnEventField, btn)

	if elevator.Floor == btn.Floor {
		slog.Debug("Elevator already at floor, opening door and clearing order",
			"floor", btn.Floor, "button", btn.Button)

		// Clear the order immediately since we're already here
		elevMgr.clearFloor()

		// Open the door
		elevMgr.OpenDoor(timerAction)
	} else {
		slog.Debug("Setting order and moving elevator",
			"from", elevator.Floor, "to", btn.Floor, "button", btn.Button)

		elevMgr.UpdateOrders(func(orders *[config.NumFloors][config.NumButtons]bool) {
			orders[btn.Floor][btn.Button] = true
		})

		if btn.Button == types.BT_Cab {
			elevio.SetButtonLamp(btn.Button, btn.Floor, true)
			slog.Debug("Turned on cab button lamp", "floor", btn.Floor)
		} else {
			// Send hall order light on message to network via lightMsgCh
			slog.Debug("Sending hall light ON message", "floor", btn.Floor, "button", btn.Button)
			elevMgr.lightMsgCh <- types.Message{
				Type:     types.LocalLightOn,
				Event:    btn,
				SenderID: elevator.NodeID,
			}
		}

		// Update direction and start moving
		dirnBehaviourPair := elevMgr.chooseDirInit()
		elevMgr.UpdateState(ElevDirField, dirnBehaviourPair.Dir)
		slog.Debug("Setting direction", "dir", dirnBehaviourPair.Dir, "behaviour", dirnBehaviourPair.Behaviour)
		elevMgr.moveMotor()
	}
}

// Open door, update state. Includes safety check to avoid opening door while moving.
func (elevMgr *ElevStateMgr) OpenDoor(timerAction chan<- timer.TimerAction) {
	if elevio.GetFloor() == -1 {
		slog.Warn("Cannot open door while between floors")
		return
	}
	elevMgr.UpdateState(ElevBehaviourField, types.DoorOpen)
	elevio.SetDoorOpenLamp(true)
	slog.Debug("Starting door timer")
	timerAction <- timer.Start
}

// Move motor with safety check to avoid moving while door is open.
func (elevMgr *ElevStateMgr) moveMotor() {
	elevator := elevMgr.GetState()
	if elevator.Behaviour == types.DoorOpen {
		slog.Debug("Cannot move while door is open")
		return
	}
	elevMgr.UpdateState(ElevBehaviourField, types.Moving)
	elevMgr.UpdateState(ElevBetweenFloorsField, true)
	elevio.SetMotorDirection(elevator.Dir)
}

// Algorithm that only goes as far as the final order in that direction, then reverses.
func (elevMgr *ElevStateMgr) chooseDirInit() types.DirnBehaviourPair {
	elevator := elevMgr.GetState()
	var pair types.DirnBehaviourPair

	if elevator.Dir == types.MD_Stop {
		switch {
		case elevator.ordersAbove() > 0:
			pair = types.DirnBehaviourPair{Dir: types.MD_Up, Behaviour: types.Moving}
		case elevator.ordersBelow() > 0:
			pair = types.DirnBehaviourPair{Dir: types.MD_Down, Behaviour: types.Moving}
		default:
			pair = types.DirnBehaviourPair{Dir: types.MD_Stop, Behaviour: types.Idle}
		}
	} else {
		pair = elevator.chooseDirWhileMoving(elevator.Dir)
	}

	if pair.Behaviour == types.Moving {
		if elevator.Behaviour == types.DoorOpen {
			pair.Behaviour = types.Idle
			pair.Dir = types.MD_Stop
		}
	}
	return pair
}

func (elevator *ElevState) chooseDirWhileMoving(dir types.MotorDirection) types.DirnBehaviourPair {
	switch dir {
	case types.MD_Up:
		if elevator.ordersAbove() > 0 {
			return types.DirnBehaviourPair{Dir: dir, Behaviour: types.Moving}
		}
	case types.MD_Down:
		if elevator.ordersBelow() > 0 {
			return types.DirnBehaviourPair{Dir: dir, Behaviour: types.Moving}
		}
	}

	if elevator.ordersHere() > 0 {
		return types.DirnBehaviourPair{Dir: types.MD_Stop, Behaviour: types.DoorOpen}
	}

	// Check opposite direction if no orders in current direction.
	if dir == types.MD_Up && elevator.ordersBelow() > 0 {
		return types.DirnBehaviourPair{Dir: types.MD_Down, Behaviour: types.Moving}
	} else if dir == types.MD_Down && elevator.ordersAbove() > 0 {
		return types.DirnBehaviourPair{Dir: types.MD_Up, Behaviour: types.Moving}
	}

	return types.DirnBehaviourPair{Dir: types.MD_Stop, Behaviour: types.Idle}
}
