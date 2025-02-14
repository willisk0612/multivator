package network

import (
	"log/slog"
	"main/src/types"
)

var eventBids []types.EventBidsPair

func createBidMsg(elevator *types.Elevator, hallEventCh <-chan types.ButtonEvent, outgoingMsgCh chan<- types.Message) {
	for event := range hallEventCh {
		registerHallOrder(event)

		numPeers := len(getCurrentPeers())
		slog.Info("Received hall call", "event", event, "connectedPeers", numPeers)
		msg := types.Message{
			Type:     types.HallOrder,
			Event:    event,
			SenderID: elevator.NodeID,
		}
		outgoingMsgCh <- msg
	}
}

func eventAlreadyRegistered(event types.ButtonEvent) bool {
	for _, ebp := range eventBids {
		if ebp.Event == event {
			return true
		}
	}
	return false
}

func findBestBid(ebp types.EventBidsPair, localNodeID int) types.OrderAssignment {
	if len(ebp.Bids) == 0 {
		return types.OrderAssignment{
			Event:   ebp.Event,
			Cost:    0,
			IsLocal: false,
		}
	}

	bestBid := ebp.Bids[0]
	for _, bid := range ebp.Bids {
		if bid.Cost < bestBid.Cost || (bid.Cost == bestBid.Cost && bid.NodeID < bestBid.NodeID) {
			bestBid = bid
		}
	}
	return types.OrderAssignment{
		Event:   ebp.Event,
		Cost:    bestBid.Cost,
		IsLocal: bestBid.NodeID == localNodeID,
	}
}

// registerHallOrder creates/modifies eventBids on a hall order
func registerHallOrder(event types.ButtonEvent) {
	if !eventAlreadyRegistered(event) {
		eventBids = append(eventBids, types.EventBidsPair{
			Event: event,
			Bids:  []types.BidEntry{},
		})
	}
}
