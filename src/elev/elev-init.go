package elev

import (
	"main/lib/driver-go/elevio"
	"main/src/types"
	"main/src/config"
)

// Initializes elevator with default values. Moves elevator down if between floors.
func InitElevator() *types.Elevator {
	elevator := types.Elevator{
		Dir:       elevio.MD_Stop,
		Orders:    [config.N_FLOORS][config.N_BUTTONS]int{},
		Behaviour: elevio.Idle,
	}
	// Move down to find a floor if starting between floors
	if elevio.GetFloor() == -1 {
		elevator.Dir = elevio.MD_Down
		elevator.Behaviour = elevio.Moving
		elevio.SetMotorDirection(elevator.Dir)
	}
	return &elevator
}
