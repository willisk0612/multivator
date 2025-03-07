package types

import "time"

type MsgType int

const (
	BidMsg MsgType = iota
	HallOrderMsg
	HallArrivalMsg
	SyncMsg
)

type Message[Content MsgContent] struct {
	Type      MsgType
	LoopCount int
	Content   Content
	SenderID  int
}

type MsgContent interface {
	Bid | Sync
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

type Sync struct {
	Orders           [][][]bool
	RestoreCabOrders bool
}
