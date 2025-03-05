package types

import "time"

type MsgType int

const (
	BidMsg MsgType = iota
	HallOrderMsg
	HallArrivalMsg
)

type Message[Content MsgContent] struct {
	Type      MsgType
	LoopCount int
	Content   Content
	SenderID  int
}

type MsgContent interface {
	Bid | HallArrival
}

type HallArrival struct {
	BtnEvent ButtonEvent
}

type BufferedEvent struct {
	EventType      string
	BidMsg         Message[Bid]
	HallArrivalMsg Message[HallArrival]
}

type PeerUpdate struct {
	Peers []string
	New   string
	Lost  []string
}

type Bid struct {
	BtnEvent ButtonEvent
	Cost     time.Duration
}
