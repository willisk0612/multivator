package dispatcher

import (
	"time"

	"multivator/src/types"
)

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

type Bid struct {
	BtnEvent types.ButtonEvent
	Cost     time.Duration
}

type Sync struct {
	Orders           types.Orders
	RestoreCabOrders bool
}

type hallOrders map[types.ButtonEvent]map[int]Bid
