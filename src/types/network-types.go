package types

import "time"

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

type PeerUpdate struct {
	Peers []string
	New   string
	Lost  []string
}
