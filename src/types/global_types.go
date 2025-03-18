package types

import "multivator/src/config"

type ElevState struct {
	Floor           int
	Orders          Orders
	Dir             MotorDirection
	Behaviour       ElevBehaviour
	Obstructed      bool
}

type Orders [config.NumElevators][config.NumFloors][config.NumButtons]bool

type MotorDirection int

const (
	MD_Up   MotorDirection = 1
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

type ElevBehaviour int

const (
	Idle ElevBehaviour = iota
	Moving
	DoorOpen
)

type DirnBehaviourPair struct {
	Dir       MotorDirection
	Behaviour ElevBehaviour
}
