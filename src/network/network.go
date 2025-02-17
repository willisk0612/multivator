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
	broadcastPort      = 15657
	PeersPort          = 15658
)

func RunNetworkManager(elevator *types.Elevator, mgr *elev.ElevatorManager, hallEventCh chan types.ButtonEvent, outgoingMsg chan types.Message, timerAction chan timer.TimerAction) {
	nodeIDStr := fmt.Sprintf("node-%d", elevator.NodeID)
	incomingMsg := make(chan types.Message)
	peerUpdateCh := make(chan types.PeerUpdate)
	doorTimeoutCh := make(chan bool)

	// Pass doorTimeoutCh to main.go
	go bcast.Receiver(broadcastPort, incomingMsg)
	go bcast.Transmitter(broadcastPort, outgoingMsg)
	go peers.Transmitter(PeersPort, nodeIDStr, make(chan bool))
	go peers.Receiver(PeersPort, peerUpdateCh)
	go createBidMsg(elevator, hallEventCh, outgoingMsg)
	go handlePeerUpdates(peerUpdateCh)
	go peerManager()

	// Listen for both door timeout and incoming messages
	for {
		select {
		case msg := <-incomingMsg:
			mgr.Execute(elev.ElevatorCmd{
				Exec: func(elevator *types.Elevator) {
					handleMessageEvent(elevator, msg, outgoingMsg, timerAction)
				},
			})
		case <-doorTimeoutCh:
			mgr.Execute(elev.ElevatorCmd{
				Exec: func(elevator *types.Elevator) {
					ProcessQueuedOrders(elevator, timerAction)
				},
			})
		}
	}
}

// ProcessQueuedOrders processes queued orders when the door closes
func ProcessQueuedOrders(elevator *types.Elevator, timerAction chan timer.TimerAction) {
	if len(elevator.EventBids) == 0 || elevator.Behaviour != types.Idle {
		return
	}

	if elevator.Behaviour != types.Idle {
		return
	}

	// Process the first queued order
	nextOrder := elevator.EventBids[0]
	slog.Debug("Processing queued order",
		"event", nextOrder.Event)

	elev.MoveElevator(elevator, nextOrder.Event, timerAction)
	elevator.EventBids = elevator.EventBids[1:]
}

// handleMessageEvent handles messages from bcast.Receiver
func handleMessageEvent(elevator *types.Elevator, inMsg types.Message, outMsgCh chan types.Message, timerAction chan timer.TimerAction) {
	if elevator.Behaviour == types.DoorOpen {
		if elevator.Floor == inMsg.Event.Floor {
			return
		}
		queueOrder(elevator, inMsg)
		return
	}

	numPeers := len(getCurrentPeers())
	// If numPeers < 2, transform into single elevator system
	if numPeers < 2 {
		elev.MoveElevator(elevator, inMsg.Event, timerAction)
		return
	}
	switch inMsg.Type {
	case types.HallOrder:
		handleHallOrder(elevator, inMsg, outMsgCh)
	case types.Bid:
		handleBidMessage(elevator, inMsg, timerAction)
	}
}

// handleHallOrder appends event, calculates bid and broadcasts it
func handleHallOrder(elevator *types.Elevator, inMsg types.Message, outMsgCh chan<- types.Message) {
	appendEvent(elevator, inMsg.Event)
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
		bidLenght := len(elevator.EventBids[i].Bids)
		slog.Debug("Received bid", "numPeers", numPeers, "bidLength", bidLenght)
		if bidLenght == numPeers {
			assignment := findBestBid(elevator.EventBids[i], elevator.NodeID)
			slog.Debug("Found best bid", "assignment", assignment)
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

// queueOrder queues an order in case the door is open
func queueOrder(elevator *types.Elevator, msg types.Message) {
	for _, order := range elevator.EventBids {
		if order.Event == msg.Event {
			return // Order already queued
		}
	}

	elevator.EventBids = append(elevator.EventBids, types.EventBidsPair{
		Event: msg.Event,
		Bids:  []types.BidEntry{},
	})
	slog.Debug("Order queued while door open",
		"event", msg.Event,
		"queueLength", len(elevator.EventBids))
}
