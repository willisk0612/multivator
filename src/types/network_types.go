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
	Bid | HallOrder | HallArrival
}

type HallOrder struct {
	Order ButtonEvent
}

type HallArrival struct {
	Order ButtonEvent
}

type PeerUpdate struct {
	Peers []string
	New   string
	Lost  []string
}

type Bid struct {
	NodeID int
	Order ButtonEvent
	Cost  []time.Duration
}
