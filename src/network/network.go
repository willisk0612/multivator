package network

import (
	"errors"
	"fmt"
	"log/slog"
	"multivator/lib/network-go/network/bcast"
	"multivator/lib/network-go/network/peers"
	"multivator/src/elev"
	"multivator/src/types"
	"time"
)

const (
	peerUpdateTimeout  = 100 * time.Millisecond
	messageRepetitions = 5
	messageInterval    = 10 * time.Millisecond
	broadcastPort      = 15657
	PeersPort          = 15658
)

type ElevStateMgrWrapper struct {
	*elev.ElevStateMgr
}

// Run starts the network subsystem and sends messages to the elevator subsystem in case of hall assignments.
func Run(elevMgr *elev.ElevStateMgr, elevInMsgCh chan types.Message, elevOutMshCh chan types.Message) {
	elevator := elevMgr.GetState()
	nodeIDStr := fmt.Sprintf("node-%d", elevator.NodeID)
	netInMsgCh := make(chan types.Message)
	netOutMsgCh := make(chan types.Message)
	peerUpdateCh := make(chan types.PeerUpdate)

	go bcast.Receiver(broadcastPort, netInMsgCh)
	go bcast.Transmitter(broadcastPort, netOutMsgCh)
	go peers.Transmitter(PeersPort, nodeIDStr, make(chan bool))
	go peers.Receiver(PeersPort, peerUpdateCh)
	go handlePeerUpdates(peerUpdateCh)
	go peerManager()

	// Use wrapper to access methods
	elevMgrWrapper := ElevStateMgrWrapper{elevMgr}

	for {
		select {
			case elevInMsg := <- elevOutMshCh: // We received a hall order from the elevator subsystem. If there is one peer, send it back, else send to network subsystem.
				elevMgrWrapper.handleMessageEvent(elevInMsg, elevInMsgCh, netOutMsgCh)
			case netInMsg := <-netInMsgCh: // We received a message from the network subsystem. If its a hall assignment, send it to the elevator subsystem. If its a hall order, send it to the network subsystem.
				elevMgrWrapper.handleMessageEvent(netInMsg, elevInMsgCh, netOutMsgCh)
		}
	}
}

// handleHallOrder appends event, calculates bid and broadcasts it
func (elevMgr *ElevStateMgrWrapper) handleHallOrder(hallEvent types.ButtonEvent, netOutMsgCh chan types.Message) {
	slog.Debug("Received hall order in HandleHallOrder")
	elevMgr.clearExistingCostCalculations(hallEvent)
	costCh := make(chan time.Duration, 1)
	errCh := make(chan error, 1)

	go func() {
		bid := elevMgr.TimeToServedOrder(hallEvent)
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
			Type:     types.NetHallOrder,
			Cost:     bid,
			Event:    hallEvent,
			SenderID: elevMgr.GetState().NodeID,
		}
		numPeers := len(getCurrentPeers())
		slog.Info("TimeToservedOrder", "bid", bid, "numPeers", numPeers)
		elevMgr.appendEvent(hallEvent)
		sendMultipleMessages(msg, netOutMsgCh)
	case err := <-errCh:
		slog.Error("Cost calculation failed", "error", err)
	}
}

// handleMessageEvent handles messages from bcast.Receiver
func (elevMgr *ElevStateMgrWrapper) handleMessageEvent(inMsg types.Message, elevInMsgCh chan types.Message, netOutMsgCh chan types.Message) {
	numPeers := len(getCurrentPeers())
	// If numPeers < 2, transform into single elevator system
	if numPeers < 2 {
		slog.Info("Single elevator system", "numPeers", numPeers)
		msg := types.Message{
			Type:     types.LocalHallAssignment,
			Event:    inMsg.Event,
		}
		elevInMsgCh <- msg // Send hall assignment to elevator subsystem
		return
	}
	switch inMsg.Type {
	case types.LocalHallOrder:
		elevMgr.handleHallOrder(inMsg.Event, netOutMsgCh)
	case types.NetHallOrder:
		elevMgr.createBidMsg(inMsg, netOutMsgCh)
	case types.Bid:
		elevMgr.handleBidMessage(inMsg, elevInMsgCh)
	}
}

func (elevMgr *ElevStateMgrWrapper) clearExistingCostCalculations(hallEvent types.ButtonEvent) {
	for _, ebp := range elevMgr.GetState().EventBids {
		if ebp.Event.Floor == hallEvent.Floor && ebp.Event.Button == hallEvent.Button {
			elevMgr.UpdateEventBids(func(bids *[]types.EventBidsPair) {
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

func sendMultipleMessages(msg types.Message, netOutMsgCh chan<- types.Message) {
	for i := 0; i < messageRepetitions; i++ {
		netOutMsgCh <- msg
		time.Sleep(messageInterval)
	}
}

func (elevMgr *ElevStateMgrWrapper) handleBidMessage(inMsg types.Message, elevInMsgCh chan<- types.Message) {
	elevator := elevMgr.GetState()

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
	elevMgr.UpdateEventBids(func(bids *[]types.EventBidsPair) {
		(*bids)[pairIndex].Bids = append((*bids)[pairIndex].Bids, types.BidEntry{
			NodeID: inMsg.SenderID,
			Cost:   inMsg.Cost,
		})
	})

	// Get state again to check if all bids are in
	elevator = elevMgr.GetState()
	numPeers := len(getCurrentPeers())
	bidLength := len(elevator.EventBids[pairIndex].Bids)

	slog.Debug("Received bid", "numPeers", numPeers, "bidLength", bidLength)

	if bidLength == numPeers {
		assignment := findBestBid(elevator.EventBids[pairIndex], elevator.NodeID)

		if assignment.IsLocal {
			slog.Info("This node won the bid", "event", assignment.Event, "cost", assignment.Cost)
			msg := types.Message{
				Type:  types.LocalHallAssignment,
				Event: assignment.Event,
			}
			elevInMsgCh <- msg // Send hall assignment to elevator subsystem
			slog.Debug("Sent hall assignment to elevator subsystem")
		}

		// Clean up by removing the completed event
		elevMgr.UpdateEventBids(func(bids *[]types.EventBidsPair) {
			*bids = append((*bids)[:pairIndex], (*bids)[pairIndex+1:]...)
		})
	}
}
