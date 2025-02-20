package types

import "main/src/config"

type Elevator struct {
	NodeID        int
	Floor         int
	BetweenFloors bool
	Dir           MotorDirection
	Orders        [config.NumFloors][config.NumButtons]bool
	Behaviour     ElevatorBehaviour
	Obstructed    bool
	EventBids     []EventBidsPair
}

type ElevatorCmd struct {
	Exec func(elevator *Elevator)
}

// ElevatorManager owns the elevator and serializes its access.
type ElevatorManager struct {
	Cmds chan ElevatorCmd
}

type ElevMgrField string

const (
	ElevFloor         ElevMgrField = "Floor"
	ElevBetweenFloors ElevMgrField = "BetweenFloors"
	ElevDir           ElevMgrField = "Dir"
	ElevOrders        ElevMgrField = "Orders"
	ElevBehaviour     ElevMgrField = "Behaviour"
	ElevObstructed    ElevMgrField = "Obstructed"
	ElevEventBids     ElevMgrField = "EventBids"
)

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
