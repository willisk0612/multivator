package elev

import (
	"main/lib/driver-go/elevio"
	"main/src/config"
	"main/src/types"
)

func InitElevator(nodeID int) *types.Elevator {
	elevator := &types.Elevator{
		NodeID:    nodeID,
		Dir:       elevio.MD_Stop,
		Orders:    [config.N_FLOORS][config.N_BUTTONS]int{},
		Behaviour: elevio.Idle,
	}
	return elevator
}

func InitSystem(nodeID int) *types.Elevator {
	elevator := InitElevator(nodeID)

	if elevio.GetFloor() == -1 {
		elevator.Dir = elevio.MD_Down
		elevator.Behaviour = elevio.Moving
		elevio.SetMotorDirection(elevator.Dir)
	}

	return elevator
}
