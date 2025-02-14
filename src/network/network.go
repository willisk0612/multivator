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
	broadcastPort = 15647
	peersPort     = 15648
	ackTimeout    = 500 * time.Millisecond
)
func PollMessages(elevator *types.Elevator, mgr *elev.ElevatorManager, hallEventCh <-chan types.ButtonEvent, outgoingMsg chan types.Message, timerAction chan timer.TimerAction) {
	nodeIDStr := fmt.Sprintf("node-%d", elevator.NodeID)
	incomingMsg := make(chan types.Message)
	peerUpdate := make(chan types.PeerUpdate)
	go bcast.Receiver(broadcastPort, incomingMsg)
	go bcast.Transmitter(broadcastPort, outgoingMsg)
	go peers.Transmitter(peersPort, nodeIDStr, make(chan bool))
	go peers.Receiver(peersPort, peerUpdate)
	go handlePeerUpdates(peerUpdate)
	go createBidMsg(elevator, hallEventCh, outgoingMsg)
	go peerManager()

	// Incomming messages are coming
	for msg := range incomingMsg {
		mgr.Execute(elev.ElevatorCmd{
			Exec: func(elevator *types.Elevator) {
				handleMessageEvent(elevator, mgr, msg, outgoingMsg, timerAction)
			},
		})
	}
	select {}
}

// Handles messages by listening
func handleMessageEvent(elevator *types.Elevator, mgr *elev.ElevatorManager, inMsg types.Message, outMsgCh chan<- types.Message, timerAction chan timer.TimerAction) {
	switch inMsg.Type {
	case types.HallOrder:
		numPeers := len(getCurrentPeers())
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

		msg := types.Message{
			Type:     types.Bid,
			Event:    inMsg.Event,
			Cost:     bid,
			SenderID: elevCopy.NodeID,
		}
		outMsgCh <- msg // Send bid to other nodes

	case types.Bid:
		if inMsg.SenderID == elevator.NodeID {
			return // Ignore own bid messages
		}
		for i := range eventBids {
			if eventBids[i].Event == inMsg.Event {
				eventBids[i].Bids = append(eventBids[i].Bids, types.BidEntry{
					NodeID: inMsg.SenderID,
					Cost:   inMsg.Cost,
				})
				slog.Debug("Received bid", "nodeID", inMsg.SenderID, "event", inMsg.Event, "cost", inMsg.Cost)
				slog.Debug("Event bids state", "event", inMsg.Event, "bids", eventBids[i].Bids)
				bidArrLenght := len(eventBids[i].Bids)
				numPeers := len(getCurrentPeers())
				slog.Debug("Event bids length", "bidLength", bidArrLenght, "numPeers", numPeers)
				if bidArrLenght == numPeers {
					assignment := findBestBid(eventBids[i], elevator.NodeID)
					if assignment.IsLocal {
						slog.Info("This node won the bid", "event", assignment.Event, "cost", assignment.Cost)
						elev.MoveElevator(elevator, assignment.Event, timerAction)
					}
					eventBids = append(eventBids[:i], eventBids[i+1:]...)
				}
				break
			}
		}
	}
}
