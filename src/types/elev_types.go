package types

import "main/src/config"

type Elevator struct {
	NodeID     int
	Floor      int
	Dir        MotorDirection
	Orders     [config.NumFloors][config.NumButtons]bool
	Behaviour  ElevatorBehaviour
	Obstructed bool
}

type MotorDirection int

const (
	MD_UP   MotorDirection = 1
	MD_Down MotorDirection = -1
	MD_Stop MotorDirection = 0
)

type ButtonType int

const (
	BT_HallUp   ButtonType = 0
	BT_HallDown ButtonType = 1
	BT_Cab      ButtonType = 2
)

type ButtonEvent struct {
	Floor  int
	Button ButtonType
}

type ElevatorBehaviour int

const (
	Idle ElevatorBehaviour = iota
	Moving
	DoorOpen
)

type DirnBehaviourPair struct {
	Dir       MotorDirection
	Behaviour ElevatorBehaviour
}
