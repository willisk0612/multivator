package types

import (
	"main/lib/driver-go/elevio"
	"main/src/config"
	"time"
)

type ButtonEvent struct {
	Floor  int
	Button elevio.ButtonType
}

type MessageType int

const (
	MsgButtonEvent MessageType = iota
	MsgAcknowledge
)

type Message struct {
	BufferID  int64
	Type      MessageType
	SenderID  int
	Event     ButtonEvent
	AckID     int64
	Timestamp time.Time
}

type BufferEntry struct {
	Msg        Message
	SendTime   time.Time
	RetryCount int
	Done       chan struct{}
}

type Elevator struct {
	NodeID     int
	Floor      int
	Dir        elevio.MotorDirection
	Orders     [config.N_FLOORS][config.N_BUTTONS]int
	Behaviour  elevio.ElevatorBehaviour
	Config     elevio.ElevatorConfig
	Obstructed bool
}

// Stores direction and behaviour to keep track of direction even when elevator is idle
type DirnBehaviourPair struct {
	Dir       elevio.MotorDirection
	Behaviour elevio.ElevatorBehaviour
}
