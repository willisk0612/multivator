package dispatcher

import (
	"time"

	"multivator/src/types"
)

type MsgType int

const (
	BidInitial MsgType = iota
	BidReply
	SyncOrders
	RestoreCabOrders
)

type Msg[Content MsgContent] struct {
	SenderID int
	Type     MsgType
	Content  Content
	Counter  uint64
}

type MsgContent interface {
	Bid | Sync
}

type Bid struct {
	Order types.HallOrder
	Cost  time.Duration
}

type BidMapValues struct {
	Costs map[int]time.Duration
	Timer *time.Timer
}

type BidMap map[types.HallOrder]BidMapValues

type Sync struct {
	Orders types.Orders
}
