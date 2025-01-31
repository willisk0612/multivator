package types

import (
	"main/lib/driver-go/elevio"
	"main/src/config"
)

// Elevator type with methods for handling fsm events
type Elevator struct {
	Floor      int
	Dir        elevio.MotorDirection
	Orders     [config.N_FLOORS][config.N_BUTTONS]int
	Behaviour  elevio.ElevatorBehaviour
	Config     elevio.ElevatorConfig
	Obstructed bool
}

type DirnBehaviourPair struct {
	Dir       elevio.MotorDirection
	Behaviour elevio.ElevatorBehaviour
}
