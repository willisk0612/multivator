package network

import (
	"log/slog"
	"time"

	"multivator/lib/driver-go/elevio"
	"multivator/src/elev"
	"multivator/src/timer"
	"multivator/src/types"
)

var HallOrders map[types.ButtonEvent]map[int]types.Bid

// HandleBid processes incoming bids from other elevators. There are two cases:
//  1. If the bid is initial, store it and respond with our own bid if the bid is not our own.
//  2. If the bid is secondary, store it and check if we have received all bids. If so, assign the order to the elevator with the lowest bid.
func HandleBid(elevator *types.ElevState, msg types.Message[types.Bid], bidTxBuf chan types.Message[types.Bid], hallTxBuf chan types.Message[types.HallArrival], doorTimerAction chan timer.TimerAction) {
	slog.Debug("Entered HandleBid")
	isOwnBid := msg.SenderID == elevator.NodeID
	if msg.LoopCount == 0 && !isOwnBid { // Initial bid
		slog.Debug("Received initial bid")

		// Ignore own initial bid
		if msg.SenderID == elevator.NodeID {
			slog.Debug("Ignoring own bid")
			return
		}

		// Store own bid
		cost := elev.Cost(elevator, msg.Content.BtnEvent)
		if HallOrders[msg.Content.BtnEvent] == nil {
			HallOrders[msg.Content.BtnEvent] = make(map[int]types.Bid)
		}
		HallOrders[msg.Content.BtnEvent][elevator.NodeID] = types.Bid{
			BtnEvent: msg.Content.BtnEvent,
			Cost:     cost,
		}
		slog.Debug("Stored own bid", "HallOrders:", HallOrders[msg.Content.BtnEvent])

		// Store the received bid
		if HallOrders[msg.Content.BtnEvent] == nil {
			HallOrders[msg.Content.BtnEvent] = make(map[int]types.Bid)
		}
		HallOrders[msg.Content.BtnEvent][msg.SenderID] = types.Bid{
			Cost:     msg.Content.Cost,
			BtnEvent: msg.Content.BtnEvent,
		}
		slog.Debug("Stored received bid", "HallOrders:", HallOrders[msg.Content.BtnEvent])

		// Transmit own bid
		slog.Debug("Trying to append response bid cost")
		response := types.Message[types.Bid]{
			Type: types.BidMsg,
			Content: types.Bid{
				BtnEvent: msg.Content.BtnEvent,
				Cost:     cost,
			},
			SenderID:  elevator.NodeID,
			LoopCount: 1,
		}
		bidTxBuf <- response
	} else if msg.LoopCount == 1 { // Secondary bid
		// Store secondary bid
		slog.Debug("Received secondary bid")
		if HallOrders[msg.Content.BtnEvent] == nil {
			HallOrders[msg.Content.BtnEvent] = make(map[int]types.Bid)
		}
		HallOrders[msg.Content.BtnEvent][msg.SenderID] = types.Bid{
			Cost:     msg.Content.Cost,
			BtnEvent: msg.Content.BtnEvent,
		}
		slog.Debug("Stored secondary bid", "HallOrders:", HallOrders[msg.Content.BtnEvent])

		// Check if all bids are in
		numBids := len(HallOrders[msg.Content.BtnEvent])

		numPeers := len(getPeers())
		slog.Debug("Checking if all bids are received", "bids:", numBids, "peers:", numPeers)
		if numBids == numPeers {
			// Determine assignee: take order if local, otherwise set button lamp
			assignee := findAssignee(msg.Content.BtnEvent)
			if assignee == elevator.NodeID {
				elev.MoveElevator(elevator, msg.Content.BtnEvent, doorTimerAction)
				// If the elevator is at the same floor as the order, TransmitHallArrival
				if elevator.Floor == msg.Content.BtnEvent.Floor {
					TransmitHallArrival(elevator, msg.Content.BtnEvent, hallTxBuf)
				}
			} else {
				elevator.Orders[assignee][msg.Content.BtnEvent.Floor][msg.Content.BtnEvent.Button] = true
				elevio.SetButtonLamp(msg.Content.BtnEvent.Button, msg.Content.BtnEvent.Floor, true)

			}
		}
	}
}

func findAssignee(event types.ButtonEvent) int {
	slog.Debug("Entered findAssignee")
	var lowestCost time.Duration = time.Hour * 24
	var assignee int = -1

	// Iterate through the map instead of the array
	for nodeID, bid := range HallOrders[event] {
		if bid.Cost < lowestCost || (bid.Cost == lowestCost && nodeID < assignee) {
			lowestCost = bid.Cost
			assignee = nodeID
		}
	}

	if assignee == -1 {
		slog.Error("COULD NOT FIND ASSIGNEE")
	}
	slog.Debug("Assigning order to", "nodeID", assignee, "cost", lowestCost, "All bids", HallOrders[event])
	delete(HallOrders, event)
	return assignee
}
