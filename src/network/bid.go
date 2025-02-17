package network

import (
	"log/slog"
	"main/src/types"
)

func createBidMsg(elevator *types.Elevator, hallEventCh <-chan types.ButtonEvent, outgoingMsgCh chan<- types.Message) {
	for event := range hallEventCh {
		appendEvent(elevator, event)

		numPeers := len(getCurrentPeers())
		slog.Info("Received hall call", "event", event, "connectedPeers", numPeers)
		msg := types.Message{
			Type:     types.HallOrder,
			Event:    event,
			SenderID: elevator.NodeID,
		}
		outgoingMsgCh <- msg // Send bid to bcast.Transmitter
	}
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

// appendEvent creates/modifies eventBids on a hall order
func appendEvent(elevator *types.Elevator, event types.ButtonEvent) {
	if !eventAlreadyRegistered(elevator, event) {
		elevator.EventBids = append(elevator.EventBids, types.EventBidsPair{
			Event: event,
			Bids:  []types.BidEntry{},
		})
	}
}

func eventAlreadyRegistered(elevator *types.Elevator, event types.ButtonEvent) bool {
	for _, ebp := range elevator.EventBids {
		if ebp.Event == event {
			return true
		}
	}
	return false
}
