package types

import "time"

type MsgType int

const (
	BidMsg MsgType = iota
	HallOrderMsg
	HallArrivalMsg
	SyncOrdersMsg
)

type Message[Content MsgContent] struct {
	Type      MsgType
	LoopCount int
	Content   Content
	SenderID  int
}

type MsgContent interface {
	Bid |  SyncOrders
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

type SyncOrders struct {
	Orders [][][]bool
}
