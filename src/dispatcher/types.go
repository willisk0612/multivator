package dispatcher

import (
	"time"

	"multivator/src/types"
)

type MsgType int

const (
	BidInitial MsgType = iota
	BidReply
	SyncMsg
)

type Msg[Content MsgContent] struct {
	SenderID  int
	Type      MsgType
	Content   Content
}

type MsgContent interface {
	Bid | Sync
}

type Bid struct {
	Order types.HallOrder
	Cost  time.Duration
}

type Sync struct {
	Orders           types.Orders
	RestoreCabOrders bool
}

type BidMap map[types.HallOrder]map[int]time.Duration

// BidBook is a map of all bids received from peers
type BidBook map[int]int // [id][assignee]
