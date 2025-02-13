package types

import "time"

type MessageType int

const (
    HallOrder MessageType = iota
    Bid
    Ack
    Assignment
)

type Message struct {
    Type     MessageType
    Event    ButtonEvent
    Cost     time.Duration
    SenderID int
}

type PeerUpdate struct {
    Peers []string
    New   string
    Lost  []string
}

type BidEntry struct {
    NodeID int
    Cost   time.Duration
}

type EventBidsPair struct {
    Event ButtonEvent
    Bids  []BidEntry
}

type OrderAssignment struct {
    Event      ButtonEvent
    AssignedTo int
    Cost       time.Duration
    IsLocal    bool
}
