package elev

import (
	"log/slog"

	"multivator/lib/driver-go/elevio"
	"multivator/src/config"
	"multivator/src/timer"
	"multivator/src/types"
)

func InitDriver() (chan types.ButtonEvent, chan int, chan bool, chan bool) {
	drv_buttons := make(chan types.ButtonEvent, 10)
	drv_floors := make(chan int)
	drv_obstr := make(chan bool)
	drv_stop := make(chan bool)

	go elevio.PollButtons(drv_buttons)
	go elevio.PollFloorSensor(drv_floors)
	go elevio.PollObstructionSwitch(drv_obstr)
	go elevio.PollStopButton(drv_stop)

	return drv_buttons, drv_floors, drv_obstr, drv_stop
}

func InitElevState(nodeID int) *types.ElevState {
	elevator := &types.ElevState{
		NodeID:    nodeID,
		Dir:       types.MD_Stop,
		Orders:    make([][][]bool, config.NumPeers),
		Behaviour: types.Idle,
	}

	for nodeIndex := range elevator.Orders {
		elevator.Orders[nodeIndex] = make([][]bool, config.NumFloors)
		for floorNum := range elevator.Orders[nodeIndex] {
			elevator.Orders[nodeIndex][floorNum] = make([]bool, config.NumButtons)
		}
	}

	return elevator
}

// InitElevPos moves down until the first floor is detected, then stops
func InitElevPos(elevator *types.ElevState) {
	floor := elevio.GetFloor()
	if floor == -1 {
		elevio.SetMotorDirection(types.MD_Down)
		for {
			floor = elevio.GetFloor()
			if floor != -1 {
				elevio.SetMotorDirection(types.MD_Stop)
				elevator.Floor = floor
				break
			}
		}
	}
}

// Move elevator to floor, set order and lamp
func MoveElevator(elevator *types.ElevState, btn types.ButtonEvent, timerAction chan timer.TimerAction) {
	slog.Debug("Moving elevator", "from", elevator.Floor, " to ", btn.Floor)
	if elevator.Floor == btn.Floor && elevator.Behaviour != types.Moving {
		slog.Debug("Elevator already at floor")
		OpenDoor(elevator, timerAction)
	} else {
		elevator.Orders[elevator.NodeID][btn.Floor][btn.Button] = true
		elevio.SetButtonLamp(btn.Button, btn.Floor, true)
		elevator.Dir = chooseDirection(elevator).Dir
		elevator.CurrentBtnEvent = btn
		moveMotor(elevator)
	}
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
