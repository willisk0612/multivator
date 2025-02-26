package network

import (
	"log/slog"
	"multivator/src/types"
)

// Store variable to hold the light message channel
// var networkLightMsgCh chan types.Message

// SetNetworkLightChannel sets the light message channel for the network package
// func SetNetworkLightChannel(ch chan types.Message) {
// 	networkLightMsgCh = ch
// }

func (ew *ElevStateMgrWrapper) broadcastBid(inMsg types.Message, netOutMsgCh chan types.Message) {
	slog.Info("Broadcasting bid for hall order", "floor", inMsg.Event.Floor, "button", inMsg.Event.Button, "type", inMsg.Type)

	// Register the event first if not already registered
	if !ew.eventAlreadyRegistered(inMsg.Event) {
		slog.Debug("Registering new event for bidding", "event", inMsg.Event)
		ew.appendEvent(inMsg.Event)
	}

	// Calculate our bid for this order
	bid := ew.TimeToServedOrder(inMsg.Event)
	slog.Info("Calculated bid for hall order", "floor", inMsg.Event.Floor, "button", inMsg.Event.Button, "bid", bid)

	// Add our own bid to the event bids directly
	elevator := ew.GetState()
	for i, pair := range elevator.EventBids {
		if pair.Event.Floor == inMsg.Event.Floor && pair.Event.Button == inMsg.Event.Button {
			// Add our bid to this event
			ew.UpdateEventBids(func(bids *[]types.EventBidsPair) {
				// Check if we already have a bid from us
				hasSelfBid := false
				for _, bid := range (*bids)[i].Bids {
					if bid.NodeID == elevator.NodeID {
						hasSelfBid = true
						break
					}
				}

				// Add our bid if not already present
				if !hasSelfBid {
					slog.Debug("Adding our own bid directly", "event", inMsg.Event, "nodeID", elevator.NodeID, "bid", bid)
					(*bids)[i].Bids = append((*bids)[i].Bids, types.BidEntry{
						NodeID: elevator.NodeID,
						Cost:   bid,
					})
				}
			})
			break
		}
	}

	// Create and broadcast the bid
	bidMsg := types.Message{
		Type:     types.Bid,
		Cost:     bid,
		Event:    inMsg.Event,
		SenderID: ew.GetState().NodeID,
	}

	// Broadcast to network
	slog.Debug("Broadcasting bid to network", "event", inMsg.Event, "bid", bid, "nodeID", ew.GetState().NodeID)
	sendMultipleMessages(bidMsg, netOutMsgCh)
}

func (ew *ElevStateMgrWrapper) processBid(msg types.Message, elevInMsgCh chan types.Message, lmChans *LightManagerChannels) {
	// Single elevator mode or zero peers - immediately assign the order locally
	peers := getCurrentPeers()
	if len(peers) == 0 {
		slog.Info("Single elevator mode detected (no peers) - Assigning hall order locally",
			"floor", msg.Event.Floor, "button", msg.Event.Button)

		// Send order directly to elevator
		elevMsg := types.Message{
			Type:     types.LocalHallAssignment,
			Event:    msg.Event,
			Cost:     msg.Cost,
			SenderID: ew.GetState().NodeID,
		}
		elevInMsgCh <- elevMsg

		// Send light on message
		if lmChans != nil && msg.Event.Button != types.BT_Cab {
			lightMsg := types.Message{
				Event: msg.Event,
			}
			lmChans.lightOnChan <- lightMsg.Event
		}
		return
	}

	// Check if we have event registered
	elevator := ew.GetState()
	pairIndex := -1
	for i, pair := range elevator.EventBids {
		if pair.Event.Floor == msg.Event.Floor && pair.Event.Button == msg.Event.Button {
			pairIndex = i
			break
		}
	}

	// Register new event if needed
	if pairIndex == -1 {
		slog.Debug("No event registered for this bid - registering",
			"floor", msg.Event.Floor, "button", msg.Event.Button)

		// Register the event first
		ew.appendEvent(msg.Event)

		// Find the index of the new event
		elevator = ew.GetState() // Update with new state
		for i, pair := range elevator.EventBids {
			if pair.Event.Floor == msg.Event.Floor && pair.Event.Button == msg.Event.Button {
				pairIndex = i
				break
			}
		}

		if pairIndex == -1 {
			slog.Error("Failed to find newly added event", "event", msg.Event)
			return
		}
	}

	// Check if we already have a bid from this sender
	bidExists := false
	for i, existingBid := range elevator.EventBids[pairIndex].Bids {
		if existingBid.NodeID == msg.SenderID {
			slog.Debug("Updating existing bid",
				"nodeID", msg.SenderID, "floor", msg.Event.Floor, "button", msg.Event.Button)

			// Update the bid
			ew.UpdateEventBids(func(bids *[]types.EventBidsPair) {
				(*bids)[pairIndex].Bids[i].Cost = msg.Cost
			})

			bidExists = true
			break
		}
	}

	// Append bid if it's new
	if !bidExists {
		slog.Debug("Adding new bid",
			"nodeID", msg.SenderID, "floor", msg.Event.Floor, "button", msg.Event.Button)

		ew.UpdateEventBids(func(bids *[]types.EventBidsPair) {
			(*bids)[pairIndex].Bids = append(
				(*bids)[pairIndex].Bids,
				types.BidEntry{NodeID: msg.SenderID, Cost: msg.Cost},
			)
		})
	}

	// Process winning bid if we have all bids
	elevator = ew.GetState() // Get updated state after appending bid
	bidLength := len(elevator.EventBids[pairIndex].Bids)
	numPeers := len(peers)

	slog.Debug("Checking bid counts", "current", bidLength, "expected", numPeers+1, "peers", peers)

	// If we have just one elevator (ourselves) or all expected bids, process the winner
	if numPeers == 0 || bidLength >= numPeers {
		// Find the winning bid
		assignment := findBestBid(elevator.EventBids[pairIndex], elevator.NodeID)

		slog.Debug("All bids received, determining winner",
			"event", msg.Event, "winner", assignment.Assignee,
			"localID", elevator.NodeID, "winningCost", assignment.Cost)

		// Handle the winning bid
		if assignment.Assignee == elevator.NodeID {
			slog.Info("WE WON THE BID - Assigning hall order locally",
				"event", msg.Event, "cost", assignment.Cost)

			// Send order to elevator
			elevMsg := types.Message{
				Type:     types.LocalHallAssignment,
				Event:    msg.Event,
				Cost:     assignment.Cost,
				SenderID: elevator.NodeID,
			}
			elevInMsgCh <- elevMsg
			slog.Debug("Sent hall assignment to elevator subsystem", "event", msg.Event)

		} else {
			slog.Info("Another elevator won the bid",
				"event", msg.Event, "winner", assignment.Assignee)

			// Turn on light for hall order that was assigned to another elevator
			if lmChans != nil && msg.Event.Button != types.BT_Cab {
				lightMsg := types.Message{
					Event: msg.Event,
				}
				lmChans.lightOnChan <- lightMsg.Event
				slog.Debug("Turning on light for hall order assigned to another elevator",
					"event", msg.Event, "winner", assignment.Assignee)
			}
		}

		// Clear processed event from the list
		ew.clearBidsForEvent(msg.Event)
	} else {
		slog.Debug("Waiting for more bids",
			"current", bidLength, "expected", numPeers, "event", msg.Event)
	}
}

func findBestBid(ebp types.EventBidsPair, localNodeID int) types.OrderAssignment {
	if len(ebp.Bids) == 0 {
		slog.Info("No bids received, assigning to local elevator", "event", ebp.Event)
		return types.OrderAssignment{
			Event:    ebp.Event,
			Cost:     0,
			Assignee: localNodeID, // Assign to local elevator if no bids
		}
	}

	// Find the best bid
	bestBid := ebp.Bids[0]
	for _, bid := range ebp.Bids {
		if bid.Cost < bestBid.Cost || (bid.Cost == bestBid.Cost && bid.NodeID < bestBid.NodeID) {
			bestBid = bid
		}
	}

	slog.Debug("Found best bid", "event", ebp.Event, "assignee", bestBid.NodeID, "cost", bestBid.Cost)
	return types.OrderAssignment{
		Event:    ebp.Event,
		Cost:     bestBid.Cost,
		Assignee: bestBid.NodeID,
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

// clearBidsForEvent removes a processed event from the event bids list
func (elevMgr *ElevStateMgrWrapper) clearBidsForEvent(event types.ButtonEvent) {
	slog.Debug("Clearing bids for processed event", "event", event)

	elevMgr.UpdateEventBids(func(bids *[]types.EventBidsPair) {
		for i, pair := range *bids {
			if pair.Event.Floor == event.Floor && pair.Event.Button == event.Button {
				// Remove this event by replacing it with the last one and shrinking the slice
				(*bids)[i] = (*bids)[len(*bids)-1]
				*bids = (*bids)[:len(*bids)-1]
				slog.Debug("Event bids cleared", "event", event)
				break
			}
		}
	})
}
