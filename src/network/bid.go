package network

import (
	"log/slog"
	"multivator/src/types"
)

func (elevMgr *ElevStateMgrWrapper) createBidMsg(inMsg types.Message, netOutMsgCh chan types.Message) {
	elevMgr.appendEvent(inMsg.Event)
	elevator := elevMgr.GetState()
	bid := elevMgr.TimeToServedOrder(inMsg.Event)
	msg := types.Message{
		Type:     types.Bid,
		Event:    inMsg.Event,
		Cost:     bid,
		SenderID: elevator.NodeID,
	}
	slog.Debug("Created bid", "event", inMsg.Event, "cost", bid)
	sendMultipleMessages(msg, netOutMsgCh)
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
func (elevMgr *ElevStateMgrWrapper) appendEvent(event types.ButtonEvent) {
	if !elevMgr.eventAlreadyRegistered(event) {
		elevMgr.UpdateEventBids(func(bids *[]types.EventBidsPair) {
			*bids = append(*bids, types.EventBidsPair{
				Event: event,
				Bids:  []types.BidEntry{},
			})
		})
	}
}

func (elevMgr *ElevStateMgrWrapper) eventAlreadyRegistered(event types.ButtonEvent) bool {
	elevator := elevMgr.GetState()
	for _, ebp := range elevator.EventBids {
		if ebp.Event == event {
			return true
		}
	}
	return false
}
