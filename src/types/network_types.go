package types

import "time"

type MessageType int

const (
	// LocalHallOrder is sent from elev.go via netInMsgCh to network.go when a hall button is pressed.
	LocalHallOrder MessageType = iota
	// LocalHallAssignment is sent from bid.go to fsm.go via elevInMsgCh when an elevator wins a bid.
	LocalHallAssignment
	// NetHallOrder is sent over the network by an elevator to broadcast a hall order which then triggers a bidding process.
	NetHallOrder
	// Bid is sent by an elevator as a cost proposal in response to a NetHallOrder.
	Bid
	// LocalLightOn is used locally to switch on a button light when a hall order is assigned.
	LocalLightOn
	// LocalLightOff is used locally to switch off a button light when a hall order is cleared.
	LocalLightOff
	// BcastLightOff is broadcast to all nodes to clear the button light once a hall order is served.
	BcastLightOff
)

// Global channel for light events
var LightEventCh = make(chan Message)

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
	Event    ButtonEvent
	Cost     time.Duration
	Assignee int
}
