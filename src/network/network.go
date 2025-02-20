package network

import (
	"errors"
	"fmt"
	"log/slog"
	"main/lib/network-go/network/bcast"
	"main/lib/network-go/network/peers"
	"main/src/elev"
	"main/src/timer"
	"main/src/types"
	"time"
)

const (
	peerUpdateTimeout  = 100 * time.Millisecond
	messageRepetitions = 5
	messageInterval    = 10 * time.Millisecond
	broadcastPort      = 15657
	PeersPort          = 15658
)

func RunNetworkManager(elevMgr *types.ElevatorManager, outgoingMsg chan types.Message, timerAction chan timer.TimerAction) {
	elevator := elev.GetElevState(elevMgr)
	nodeIDStr := fmt.Sprintf("node-%d", elevator.NodeID)
	incomingMsg := make(chan types.Message)
	peerUpdateCh := make(chan types.PeerUpdate)

	go bcast.Receiver(broadcastPort, incomingMsg)
	go bcast.Transmitter(broadcastPort, outgoingMsg)
	go peers.Transmitter(PeersPort, nodeIDStr, make(chan bool))
	go peers.Receiver(PeersPort, peerUpdateCh)
	go handlePeerUpdates(peerUpdateCh)
	go peerManager()

	for msg := range incomingMsg {
		handleMessageEvent(elevMgr, msg, outgoingMsg, timerAction)
	}
	select {}
}

// handleHallOrder appends event, calculates bid and broadcasts it
func HandleHallOrder(elevMgr *types.ElevatorManager, hallEvent types.ButtonEvent, outMsgCh chan types.Message) {
	slog.Debug("Received hall order in HandleHallOrder")
	clearExistingCostCalculations(elevMgr, hallEvent)
	costCh := make(chan time.Duration, 1)
	errCh := make(chan error, 1)

	go func() {
		bid := elev.TimeToServedOrder(elevMgr, hallEvent)
		select {
		case costCh <- bid:
		default:
			errCh <- errors.New("cost calculation failed")
		}
	}()

	// Wait for calculation or timeout
	select {
	case bid := <-costCh:
		msg := types.Message{
			Type:     types.HallOrder,
			Cost:     bid,
			Event:    hallEvent,
			SenderID: elev.GetElevState(elevMgr).NodeID,
		}
		numPeers := len(getCurrentPeers())
		slog.Info("TimeToservedOrder", "bid", bid, "numPeers", numPeers)
		appendEvent(elevMgr, hallEvent)
		sendMultipleMessages(msg, outMsgCh)
	case err := <-errCh:
		slog.Error("Cost calculation failed", "error", err)
	}
}

// handleMessageEvent handles messages from bcast.Receiver
func handleMessageEvent(elevMgr *types.ElevatorManager, inMsg types.Message, outMsgCh chan types.Message, timerAction chan timer.TimerAction) {
	numPeers := len(getCurrentPeers())
	// If numPeers < 2, transform into single elevator system
	if numPeers < 2 {
		elev.MoveElevator(elevMgr, inMsg.Event, timerAction)
		return
	}
	switch inMsg.Type {
	case types.HallOrder:
		createBidMsg(elevMgr, inMsg, outMsgCh)
	case types.Bid:
		handleBidMessage(elevMgr, inMsg, timerAction)
	}
}

func clearExistingCostCalculations(elevMgr *types.ElevatorManager, hallEvent types.ButtonEvent) {
	for _, ebp := range elev.GetElevState(elevMgr).EventBids {
		if ebp.Event.Floor == hallEvent.Floor && ebp.Event.Button == hallEvent.Button {
			elev.UpdateEventBids(elevMgr, func(bids *[]types.EventBidsPair) {
				for i, b := range *bids {
					if b.Event == ebp.Event {
						*bids = append((*bids)[:i], (*bids)[i+1:]...)
						break
					}
				}
			})
			break
		}
	}
}

func sendMultipleMessages(msg types.Message, outMsgCh chan<- types.Message) {
	for i := 0; i < messageRepetitions; i++ {
		outMsgCh <- msg
		time.Sleep(messageInterval)
	}
}

func handleBidMessage(elevMgr *types.ElevatorManager, inMsg types.Message, timerAction chan timer.TimerAction) {
	elevator := elev.GetElevState(elevMgr)

	// Find matching event bid pair
	var matchingPair *types.EventBidsPair
	var pairIndex int
	for i := range elevator.EventBids {
		if elevator.EventBids[i].Event == inMsg.Event {
			matchingPair = &elevator.EventBids[i]
			pairIndex = i
			break
		}
	}

	// Return if no matching event found
	if matchingPair == nil {
		return
	}

	// Check if bid from this sender already exists
	for _, bid := range matchingPair.Bids {
		if bid.NodeID == inMsg.SenderID {
			return // Skip duplicate bid
		}
	}

	// Append new bid
	elev.UpdateEventBids(elevMgr, func(bids *[]types.EventBidsPair) {
		(*bids)[pairIndex].Bids = append((*bids)[pairIndex].Bids, types.BidEntry{
			NodeID: inMsg.SenderID,
			Cost:   inMsg.Cost,
		})
	})

	// Check if we have all bids
	updatedElev := elev.GetElevState(elevMgr)
	numPeers := len(getCurrentPeers())
	bidLength := len(updatedElev.EventBids[pairIndex].Bids)

	slog.Debug("Received bid", "numPeers", numPeers, "bidLength", bidLength)

	if bidLength == numPeers {
		// Find winner and handle result
		assignment := findBestBid(updatedElev.EventBids[pairIndex], updatedElev.NodeID)

		if assignment.IsLocal {
			slog.Info("This node won the bid", "event", assignment.Event, "cost", assignment.Cost)
			elev.MoveElevator(elevMgr, assignment.Event, timerAction)
		}

		// Clean up by removing the completed event
		elev.UpdateEventBids(elevMgr, func(bids *[]types.EventBidsPair) {
			*bids = append((*bids)[:pairIndex], (*bids)[pairIndex+1:]...)
		})
	}
}
