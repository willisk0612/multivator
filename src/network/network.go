package network

import (
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
	messageInterval    = 50 * time.Millisecond
)

var (
	BroadcastPort = 15657
	PeersPort     = 15658
)

func RunNetworkManager(elevator *types.Elevator, mgr *elev.ElevatorManager, hallEventCh chan types.ButtonEvent, outgoingMsg chan types.Message, timerAction chan timer.TimerAction) {
	nodeIDStr := fmt.Sprintf("node-%d", elevator.NodeID)
	incomingMsg := make(chan types.Message)
	peerUpdateCh := make(chan types.PeerUpdate)
	go bcast.Receiver(BroadcastPort, incomingMsg)
	go bcast.Transmitter(BroadcastPort, outgoingMsg)
	go peers.Transmitter(PeersPort, nodeIDStr, make(chan bool))
	go peers.Receiver(PeersPort, peerUpdateCh)
	go createBidMsg(elevator, hallEventCh, outgoingMsg)
	go handlePeerUpdates(peerUpdateCh)
	go peerManager()

	for msg := range incomingMsg {
		mgr.Execute(elev.ElevatorCmd{
			Exec: func(elevator *types.Elevator) {
				handleMessageEvent(elevator, msg, outgoingMsg, timerAction)
			},
		})
	}
	select {}
}

// handleMessageEvent handles messages from bcast.Receiver
func handleMessageEvent(elevator *types.Elevator, inMsg types.Message, outMsgCh chan types.Message, timerAction chan timer.TimerAction) {
	switch inMsg.Type {
	case types.HallOrder:
		handleHallOrder(elevator, inMsg, outMsgCh, timerAction)
	case types.Bid:
		handleBidMessage(elevator, inMsg, timerAction)
	}
}

// Modified handleHallOrder to ensure we send our own bid
func handleHallOrder(elevator *types.Elevator, inMsg types.Message, outMsgCh chan<- types.Message, timerAction chan timer.TimerAction) {
	numPeers := len(getCurrentPeers())
	if numPeers < 2 {
		elev.MoveElevator(elevator, inMsg.Event, timerAction)
		return
	}

	registerHallOrder(elevator, inMsg.Event)
	elevCopy := *elevator
	bid := elev.TimeToServedOrder(inMsg.Event, elevCopy)

	// Add our own bid first
	for i := range elevator.EventBids {
		if elevator.EventBids[i].Event == inMsg.Event {
			elevator.EventBids[i].Bids = append(elevator.EventBids[i].Bids, types.BidEntry{
				NodeID: elevator.NodeID,
				Cost:   bid,
			})
			break
		}
	}

	// Send bid multiple times to ensure delivery
	go func() {
		for i := 0; i < messageRepetitions; i++ {
			outMsgCh <- types.Message{
				Type:     types.Bid,
				Event:    inMsg.Event,
				Cost:     bid,
				SenderID: elevator.NodeID,
			}
			time.Sleep(messageInterval)
		}
	}()
}

// Modified handleBidMessage with better bid processing
func handleBidMessage(elevator *types.Elevator, inMsg types.Message, timerAction chan timer.TimerAction) {
	if inMsg.SenderID == elevator.NodeID {
		return // Ignore own messages
	}

	for i := range elevator.EventBids {
		if elevator.EventBids[i].Event != inMsg.Event {
			continue
		}

		// Check if we already have a bid from this sender
		for _, existingBid := range elevator.EventBids[i].Bids {
			if existingBid.NodeID == inMsg.SenderID {
				return // Skip duplicate bid
			}
		}

		// Add new bid
		elevator.EventBids[i].Bids = append(elevator.EventBids[i].Bids, types.BidEntry{
			NodeID: inMsg.SenderID,
			Cost:   inMsg.Cost,
		})

		numPeers := len(getCurrentPeers())
		if len(elevator.EventBids[i].Bids) == numPeers {
			assignment := findBestBid(elevator.EventBids[i], elevator.NodeID)
			if assignment.IsLocal {
				slog.Info("This node won the bid",
					"event", assignment.Event,
					"cost", assignment.Cost,
					"totalBids", len(elevator.EventBids[i].Bids))
				elev.MoveElevator(elevator, assignment.Event, timerAction)
			} else {
				slog.Info("Another node won the bid",
					"event", assignment.Event,
					"cost", assignment.Cost)
			}
			// Remove event from list
			elevator.EventBids = append(elevator.EventBids[:i], elevator.EventBids[i+1:]...)
		}
	}
}
