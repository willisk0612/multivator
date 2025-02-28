package types

type ElevState struct {
	NodeID          int
	Floor           int
	BetweenFloors   bool
	Dir             MotorDirection
	Orders          [][][]bool // Cab, HallUp, HallDown
	Behaviour       ElevBehaviour
	Obstructed      bool
	EventBids       []EventBidsPair
	CurrentBtnEvent ButtonEvent
}

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
