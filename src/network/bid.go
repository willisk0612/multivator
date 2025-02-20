package network

import (
	"log/slog"
	"main/src/elev"
	"main/src/types"
)

func createBidMsg(elevMgr *types.ElevatorManager, inMsg types.Message, outMsgCh chan types.Message) {
	appendEvent(elevMgr, inMsg.Event)
	elevator := elev.GetElevState(elevMgr)
	bid := elev.TimeToServedOrder(elevMgr, inMsg.Event)
	msg := types.Message{
		Type:     types.Bid,
		Event:    inMsg.Event,
		Cost:     bid,
		SenderID: elevator.NodeID,
	}
	slog.Debug("Created bid", "event", inMsg.Event, "cost", bid)
	sendMultipleMessages(msg, outMsgCh)
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
func appendEvent(elevMgr *types.ElevatorManager, event types.ButtonEvent) {
	if !eventAlreadyRegistered(elevMgr, event) {
		elev.UpdateEventBids(elevMgr, func(bids *[]types.EventBidsPair) {
			*bids = append(*bids, types.EventBidsPair{
				Event: event,
				Bids:  []types.BidEntry{},
			})
		})
	}
}

func eventAlreadyRegistered(elevMgr *types.ElevatorManager, event types.ButtonEvent) bool {
	elevator := elev.GetElevState(elevMgr)
	for _, ebp := range elevator.EventBids {
		if ebp.Event == event {
			return true
		}
	}
	return false
}
