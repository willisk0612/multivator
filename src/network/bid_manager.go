// Bid manager stores the bids using the elevator state manager for synchronization.
package network

import (
	"context"
	"log/slog"
	"time"

	"multivator/src/config"
	"multivator/src/elev"
	"multivator/src/types"
)

type eventKey struct {
	floor  int
	button types.ButtonType
}

// Add at package level
var bidTimeoutCancels = make(map[eventKey]chan struct{})

// broadcastBid sends a bid message to the network for a hall call
func (ew *ElevStateMgrWrapper) broadcastBid(inMsg types.Message, netOutMsgCh chan types.Message) {
	slog.Info("Broadcasting bid for hall order", "floor", inMsg.Event.Floor, "button", inMsg.Event.Button, "type", inMsg.Type)

	// Register event if needed
	registerEventIfNeeded(ew, inMsg.Event)

	// Calculate our bid and add to our state
	bid := ew.TimeToServedOrder(inMsg.Event)
	updateOwnBid(ew, inMsg.Event, bid)
	slog.Info("Calculated bid for hall order", "floor", inMsg.Event.Floor, "button", inMsg.Event.Button, "bid", bid)

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

// registerEventIfNeeded registers an event if not already registered
func registerEventIfNeeded(ew *ElevStateMgrWrapper, event types.ButtonEvent) {
	if !ew.eventAlreadyRegistered(event) {
		slog.Debug("Registering new event for bidding", "event", event)
		ew.appendEvent(event)
	}
}

// updateOwnBid adds our own bid to the event bids
func updateOwnBid(ew *ElevStateMgrWrapper, event types.ButtonEvent, bid time.Duration) {
	elevator := ew.GetState()

	for i, pair := range elevator.EventBids {
		if pair.Event.Floor == event.Floor && pair.Event.Button == event.Button {
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
					slog.Debug("Adding our own bid directly", "event", event, "nodeID", elevator.NodeID, "bid", bid)
					(*bids)[i].Bids = append((*bids)[i].Bids, types.BidEntry{
						NodeID: elevator.NodeID,
						Cost:   bid,
					})
				}
			})
			break
		}
	}
}

// processBid handles incoming bid messages
func (ew *ElevStateMgrWrapper) processBid(msg types.Message, elevInMsgCh chan types.Message, lmChans *LightManagerChannels) {
	// Handle single elevator mode
	if handleSingleElevatorMode(ew, msg, elevInMsgCh, lmChans) {
		return
	}

	// Ensure event is registered and set up timeout if needed
	pairIndex := registerEventAndSetupTimeout(ew, msg, elevInMsgCh, lmChans)
	if pairIndex == -1 {
		slog.Error("Failed to find event index", "event", msg.Event)
		return
	}

	// Process the new bid
	processBidForEvent(ew, msg, pairIndex, elevInMsgCh, lmChans)
}

// handleSingleElevatorMode checks if we're in single elevator mode and handles orders immediately if so
func handleSingleElevatorMode(ew *ElevStateMgrWrapper, msg types.Message, elevInMsgCh chan types.Message, lmChans *LightManagerChannels) bool {
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
		return true
	}
	return false
}

// registerEventAndSetupTimeout registers event if needed and sets up timeout
func registerEventAndSetupTimeout(ew *ElevStateMgrWrapper, msg types.Message, elevInMsgCh chan types.Message, lmChans *LightManagerChannels) int {
	// Check if we have event registered
	elevator := ew.GetState()
	pairIndex := findEventIndex(elevator, msg.Event)

	// Register new event if needed
	if pairIndex == -1 {
		slog.Debug("No event registered for this bid - registering",
			"floor", msg.Event.Floor, "button", msg.Event.Button)

		// Register the event
		ew.appendEvent(msg.Event)

		// Setup timeout context
		setupBidTimeout(ew, msg.Event, elevInMsgCh, lmChans)

		// Find the index of the new event
		elevator = ew.GetState() // Update with new state
		pairIndex = findEventIndex(elevator, msg.Event)
	}

	return pairIndex
}

// findEventIndex finds the index of an event in the event bids list
func findEventIndex(elevator *elev.ElevState, event types.ButtonEvent) int {
	for i, pair := range elevator.EventBids {
		if pair.Event.Floor == event.Floor && pair.Event.Button == event.Button {
			return i
		}
	}
	return -1
}

// setupBidTimeout sets up a timeout for bid collection
func setupBidTimeout(ew *ElevStateMgrWrapper, event types.ButtonEvent, elevInMsgCh chan types.Message, lmChans *LightManagerChannels) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), config.BidProcessingTimeout)

	// Create done channel for this event
	key := eventKey{floor: event.Floor, button: event.Button}
	doneCh := make(chan struct{})
	bidTimeoutCancels[key] = doneCh

	go func(event types.ButtonEvent, ctx context.Context, cancel context.CancelFunc, done <-chan struct{}) {
		defer cancel() // Always clean up the context

		// Wait for either context timeout or manual cancellation
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				slog.Info("Bid collection timed out - making decision with available bids",
					"event", event)
				handleTimeoutDecision(ew, event, elevInMsgCh, lmChans)
			}
		case <-done:
			slog.Debug("Bid timeout cancelled - decision already made", "event", event)
		}

		// Clean up
		delete(bidTimeoutCancels, key)
	}(event, ctx, cancel, doneCh)
}

// processBidForEvent adds or updates a bid and checks if we have all bids
func processBidForEvent(ew *ElevStateMgrWrapper, msg types.Message, pairIndex int, elevInMsgCh chan types.Message, lmChans *LightManagerChannels) {
	// Update or add the bid
	updateBidFromSender(ew, msg, pairIndex)

	// Check if we have all bids and process if needed
	checkAndProcessWinner(ew, msg.Event, elevInMsgCh, lmChans)
}

// updateBidFromSender adds or updates a bid from a sender
func updateBidFromSender(ew *ElevStateMgrWrapper, msg types.Message, pairIndex int) {
	elevator := ew.GetState()

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
}

// checkAndProcessWinner checks if we have all bids and processes the winner if needed
func checkAndProcessWinner(ew *ElevStateMgrWrapper, event types.ButtonEvent, elevInMsgCh chan types.Message, lmChans *LightManagerChannels) {
	elevator := ew.GetState()

	// Find event in bids
	var eventPair types.EventBidsPair
	pairIndex := -1

	for i, pair := range elevator.EventBids {
		if pair.Event.Floor == event.Floor && pair.Event.Button == event.Button {
			eventPair = pair
			pairIndex = i
			break
		}
	}

	if pairIndex == -1 {
		slog.Error("Event not found when checking winner", "event", event)
		return
	}

	// Check if we have all expected bids
	bidLength := len(eventPair.Bids)
	numPeers := len(getCurrentPeers())

	slog.Debug("Checking bid counts", "current", bidLength, "expected", numPeers, "peers", numPeers)

	// If we have just one elevator (ourselves) or all expected bids, process the winner and cancel timeout
	if numPeers == 0 || bidLength >= numPeers {
		// Process winner and handle order assignment
		processWinningBid(ew, event, elevator.NodeID, elevInMsgCh, lmChans)

		// FIXED: Add debug log to confirm timeout cancellation
		slog.Debug("Processing winning bid and cancelling timeout",
			"event", event, "bidCount", bidLength, "peers", numPeers)

		// After processing winner, ensure we remove event - problem may be here
		ew.clearBidsForEvent(event)

		// Clear event from bidTimeoutCancels map explicitly
		key := eventKey{floor: event.Floor, button: event.Button}
		delete(bidTimeoutCancels, key)
	} else {
		slog.Debug("Waiting for more bids",
			"current", bidLength, "expected", numPeers, "event", event)
	}
}

// processWinningBid determines the winner and sends appropriate messages
func processWinningBid(ew *ElevStateMgrWrapper, event types.ButtonEvent, localNodeID int, elevInMsgCh chan types.Message, lmChans *LightManagerChannels) {
	// Find the winning bid
	elevator := ew.GetState()
	var eventPair types.EventBidsPair

	for _, pair := range elevator.EventBids {
		if pair.Event.Floor == event.Floor && pair.Event.Button == event.Button {
			eventPair = pair
			break
		}
	}

	assignment := findBestBid(eventPair, localNodeID)

	slog.Debug("All bids received, determining winner",
		"event", event, "winner", assignment.Assignee,
		"localID", localNodeID, "winningCost", assignment.Cost)

	// Cancel timeout using channel
	cancelBidTimeout(event)

	// Handle the winning bid assignment
	handleOrderAssignment(ew, assignment, event, elevInMsgCh, lmChans)
}

// cancelBidTimeout cancels the bid timeout for an event
func cancelBidTimeout(event types.ButtonEvent) {
	key := eventKey{floor: event.Floor, button: event.Button}
	if doneCh, exists := bidTimeoutCancels[key]; exists {
		close(doneCh)
		// FIXED: Add extra safety - immediately delete from map
		delete(bidTimeoutCancels, key)
		slog.Debug("Cancelled bid timeout - received all bids", "event", event)
	} else {
		// FIXED: Add debug log when no timeout exists to cancel
		slog.Debug("No timeout to cancel for event", "event", event)
	}
}

// handleOrderAssignment assigns order to local or acknowledges another elevator's assignment
func handleOrderAssignment(ew *ElevStateMgrWrapper, assignment types.OrderAssignment, event types.ButtonEvent, elevInMsgCh chan types.Message, lmChans *LightManagerChannels) {
	if assignment.Assignee == ew.GetState().NodeID {
		slog.Info("WE WON THE BID - Assigning hall order locally",
			"event", event, "cost", assignment.Cost)

		// Send order to elevator
		elevMsg := types.Message{
			Type:     types.LocalHallAssignment,
			Event:    event,
			Cost:     assignment.Cost,
			SenderID: ew.GetState().NodeID,
		}
		elevInMsgCh <- elevMsg
		slog.Debug("Sent hall assignment to elevator subsystem", "event", event)

	} else {
		slog.Info("Another elevator won the bid",
			"event", event, "winner", assignment.Assignee)

		// Turn on light for hall order that was assigned to another elevator
		if lmChans != nil && event.Button != types.BT_Cab {
			lightMsg := types.Message{
				Event: event,
			}
			lmChans.lightOnChan <- lightMsg.Event
			slog.Debug("Turning on light for hall order assigned to another elevator",
				"event", event, "winner", assignment.Assignee)
		}
	}

	// Clear processed event from the list
	ew.clearBidsForEvent(event)
}

// findBestBid determines which elevator has the best (lowest) bid
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

// eventAlreadyRegistered checks if an event is already in the bids list
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

// handleTimeoutDecision handles the case when bid collection times out
func handleTimeoutDecision(ew *ElevStateMgrWrapper, event types.ButtonEvent,
	elevInMsgCh chan types.Message, lmChans *LightManagerChannels) {

	latestState := ew.GetState()
	for _, pair := range latestState.EventBids {
		if pair.Event.Floor == event.Floor && pair.Event.Button == event.Button {
			assignment := findBestBid(pair, latestState.NodeID)

			if assignment.Assignee == latestState.NodeID {
				slog.Info("WE WON THE BID (on timeout) - Assigning hall order locally",
					"event", event, "cost", assignment.Cost)

				elevInMsgCh <- types.Message{
					Type:     types.LocalHallAssignment,
					Event:    event,
					Cost:     assignment.Cost,
					SenderID: latestState.NodeID,
				}
			} else {
				slog.Info("Another elevator won the bid (on timeout)",
					"event", event, "winner", assignment.Assignee)

				// Turn on light for hall order that was assigned to another elevator
				if lmChans != nil && event.Button != types.BT_Cab {
					lmChans.lightOnChan <- event
					slog.Debug("Turning on light for hall order assigned to another elevator (timeout)",
						"event", event, "winner", assignment.Assignee)
				}
			}

			// Clear processed event from the list
			ew.clearBidsForEvent(event)
			break
		}
	}
}
