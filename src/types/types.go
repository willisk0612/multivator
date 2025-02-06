package types

import (
	"main/lib/driver-go/elevio"
	"main/src/config"
)

type ButtonEvent struct {
	Floor  int
	Button elevio.ButtonType
}

type Message struct {
	SenderNodeID int
	Event        ButtonEvent
}

// Elevator type with methods for handling fsm events
type Elevator struct {
	NodeID     int
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
