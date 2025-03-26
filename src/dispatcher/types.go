package dispatcher

import (
	"time"

	"multivator/src/types"
)

// Message types

type Msg[Content MsgContent] struct {
	SenderID int
	Content  Content
	Counter  uint64
}

type MsgContent interface {
	Bid | Sync
}

type (
	BidType  int
	SyncType int
)

const (
	BidInitial BidType = iota
	BidReply
)

const (
	SyncOrders SyncType = iota // Sync without restoring cab orders
	SyncCab                    // Sync with restoring cab orders
)

type Bid struct {
	Type  BidType
	Order types.HallOrder
	Cost  time.Duration
}

type Sync struct {
	Type   SyncType
	Orders types.Orders
}

// Local types

type BidMapValues struct {
	Costs map[int]time.Duration
	Timer *time.Timer
}

type BidMap map[types.HallOrder]BidMapValues
