package elev

import (
	"log/slog"

	"multivator/lib/driver-go/elevio"
	"multivator/src/config"
	"multivator/src/timer"
	"multivator/src/types"
)

func InitDriver() (chan types.ButtonEvent, chan int, chan bool) {
	drv_buttons := make(chan types.ButtonEvent, 10)
	drv_floors := make(chan int)
	drv_obstr := make(chan bool)

	go elevio.PollButtons(drv_buttons)
	go elevio.PollFloorSensor(drv_floors)
	go elevio.PollObstructionSwitch(drv_obstr)

	return drv_buttons, drv_floors, drv_obstr
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
		elevator.Behaviour = types.Moving
		elevator.Dir = types.MD_Down
	} else {
		elevator.Floor = floor
		elevio.SetFloorIndicator(floor)
	}
}

// Move elevator to floor, set order and lamp
//  - Opens door if elevator already is at the floor
//  - Else, it sets order and lamp
//  - If door is closed, set the motor direction
func MoveElevator(elevator *types.ElevState, btn types.ButtonEvent, timerAction chan timer.TimerAction) {
	slog.Debug("Moving elevator", "from", elevator.Floor, " to ", btn.Floor)
	if elevator.Floor == btn.Floor && elevator.Behaviour != types.Moving {
		OpenDoor(elevator, timerAction)
	} else {
		elevator.Orders[elevator.NodeID][btn.Floor][btn.Button] = true
		elevio.SetButtonLamp(btn.Button, btn.Floor, true)
		if elevator.Behaviour != types.DoorOpen {
			elevator.Dir = chooseDirection(elevator).Dir
			elevio.SetMotorDirection(elevator.Dir)
			elevator.Behaviour = types.Moving
		}
	}
}
