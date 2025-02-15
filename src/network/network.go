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
	broadcastPort     = 15647
	peersPort         = 15648
	ackTimeout        = 500 * time.Millisecond
	peerUpdateTimeout = 100 * time.Millisecond
)

func RunNetworkManager(elevator *types.Elevator, mgr *elev.ElevatorManager, hallEventCh <-chan types.ButtonEvent, outgoingMsg chan types.Message, timerAction chan timer.TimerAction) {
	nodeIDStr := fmt.Sprintf("node-%d", elevator.NodeID)
	incomingMsg := make(chan types.Message)
	peerUpdateCh := make(chan types.PeerUpdate)
	go bcast.Receiver(broadcastPort, incomingMsg)
	go bcast.Transmitter(broadcastPort, outgoingMsg)
	go peers.Transmitter(peersPort, nodeIDStr, make(chan bool))
	go peers.Receiver(peersPort, peerUpdateCh)
	go handlePeerUpdates(peerUpdateCh)
	go createBidMsg(elevator, hallEventCh, outgoingMsg)
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
func handleMessageEvent(elevator *types.Elevator, inMsg types.Message, outMsgCh chan<- types.Message, timerAction chan timer.TimerAction) {
	switch inMsg.Type {
	case types.HallOrder:
		handleHallOrder(elevator, inMsg, outMsgCh, timerAction)
	case types.Bid:
		handleBidMessage(elevator, inMsg, timerAction)
	}
}

// handleHallOrder adds the hall order to the eventBids array and broadcasts the bid.
func handleHallOrder(elevator *types.Elevator, inMsg types.Message, outMsgCh chan<- types.Message, timerAction chan timer.TimerAction) {
	numPeers := len(getCurrentPeers())
	// Transform into single elevator system if only one peer
	if numPeers < 2 {
		elev.MoveElevator(elevator, inMsg.Event, timerAction)
		return
	}

	registerHallOrder(inMsg.Event)
	elevCopy := *elevator
	bid := elev.TimeToServedOrder(inMsg.Event, elevCopy)

	// Add our own bid to the eventBids array
	for i := range eventBids {
		if eventBids[i].Event == inMsg.Event {
			eventBids[i].Bids = append(eventBids[i].Bids, types.BidEntry{
				NodeID: elevator.NodeID,
				Cost:   bid,
			})
			break
		}
	}

	// Broadcast bid to other nodes
	outMsgCh <- types.Message{
		Type:     types.Bid,
		Event:    inMsg.Event,
		Cost:     bid,
		SenderID: elevCopy.NodeID,
	}
}

// handleBidMessage appends the bid to the eventBids array and checks if all bids are received. If so, find the best bid and move the elevator.
func handleBidMessage(elevator *types.Elevator, inMsg types.Message, timerAction chan timer.TimerAction) {
	if inMsg.SenderID == elevator.NodeID {
		return // Ignore own bid messages
	}

	for i := range eventBids {
		if eventBids[i].Event != inMsg.Event {
			continue
		}

		// Add received bid to the event
		eventBids[i].Bids = append(eventBids[i].Bids, types.BidEntry{
			NodeID: inMsg.SenderID,
			Cost:   inMsg.Cost,
		})

		// Check if we have received all bids
		if len(eventBids[i].Bids) == len(getCurrentPeers()) {
			assignment := findBestBid(eventBids[i], elevator.NodeID)
			if assignment.IsLocal {
				slog.Info("This node won the bid", "event", assignment.Event, "cost", eventBids[i].Bids)
				elev.MoveElevator(elevator, assignment.Event, timerAction)
			}
			eventBids = append(eventBids[:i], eventBids[i+1:]...) // Remove event from eventBids
		}
		break
	}
}
