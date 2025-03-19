package types

import "multivator/src/config"

type ElevState struct {
	Floor      int
	Orders     Orders
	Dir        MotorDirection
	Behaviour  ElevBehaviour
	Obstructed bool
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
	BT_HallUp ButtonType = iota
	BT_HallDown
	BT_Cab
)

type ButtonEvent struct {
	Floor  int
	Button ButtonType
}

type HallType int

const (
	HallUp HallType = iota
	HallDown
)

type HallOrder struct {
	Floor  int
	Button HallType
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
