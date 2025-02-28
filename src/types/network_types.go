package types

import "time"

type MsgType int

const (
	BidMsg MsgType = iota
	HallArrivalMsg
	SyncMsg
)

type Message struct {
	Type      MsgType
	LoopCount int
	Event     ButtonEvent
	SenderID  int
}

type PeerUpdate struct {
	Peers []string
	New   string
	Lost  []string
}

type Bid struct {
	NodeID int
	Cost   time.Duration
}

type EventBidsPair struct {
	Event ButtonEvent
	Bids  []Bid
}
