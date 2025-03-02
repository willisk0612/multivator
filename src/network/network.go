package network

import (
	"fmt"
	"multivator/lib/driver-go/elevio"
	"multivator/lib/network-go/network/bcast"
	"multivator/lib/network-go/network/peers"
	"multivator/src/config"
	"multivator/src/elev"
	"multivator/src/timer"
	"multivator/src/types"
	"time"
)

func Init(elevator *types.ElevState) (
	chan types.Message[types.Bid],
	chan types.Message[types.Bid],
	chan types.Message[types.HallArrival],
	chan types.Message[types.HallArrival],
	chan types.PeerUpdate,
) {
	bidTx := make(chan types.Message[types.Bid])
	bidRx := make(chan types.Message[types.Bid])
	hallArrivalTx := make(chan types.Message[types.HallArrival])
	hallArrivalRx := make(chan types.Message[types.HallArrival])
	peerUpdateCh := make(chan types.PeerUpdate)

	bidMsgBuf := make(chan types.Message[types.Bid])
	hallArrivalMsgBuf := make(chan types.Message[types.HallArrival])

	go bcast.Transmitter(config.BcastPort, bidTx, hallArrivalTx)
	go bcast.Receiver(config.BcastPort, bidRx, hallArrivalRx)
	go peers.Transmitter(config.PeersPort, fmt.Sprintf("node-%d", elevator.NodeID), make(chan bool))
	go peers.Receiver(config.PeersPort, peerUpdateCh)
	go msgBuffer(bidMsgBuf, hallArrivalMsgBuf, bidTx, hallArrivalTx)
	return bidTx, bidRx, hallArrivalTx, hallArrivalRx, peerUpdateCh
}

// HandleBid processes incoming bids from other elevators
func HandleBid(elevator *types.ElevState, msg types.Message[types.Bid]) {
	// Ignore own bids
	if msg.SenderID == elevator.NodeID {
		return
	}

	// Check if this bid is already stored
	for _, bid := range elevator.Bids {
		if bid.NodeID == msg.SenderID &&
			bid.Order.Floor == msg.Content.Order.Floor &&
			bid.Order.Button == msg.Content.Order.Button {
			return
		}
	}
	cost := elev.Cost(elevator, msg.Content.Order)

	newBid := types.Bid{
		NodeID: msg.SenderID,
		Order:  msg.Content.Order,
		// Append the cost to the bid
		Cost: []time.Duration{cost},
	}

	// Store the bid
	elevator.Bids = append(elevator.Bids, newBid)

	// Check if we have received bids from all peers

	// We need bids from all peers plus ourselves for this specific order
	bidCount := 0
	for _, bid := range elevator.Bids {
		if bid.Order.Floor == msg.Content.Order.Floor && bid.Order.Button == msg.Content.Order.Button {
			bidCount++
		}
	}
	peers := getPeers()

	// If we have all bids, determine the winner
	if bidCount == len(peers)+1 {
		assignee := findBestBid(elevator, msg.Content.Order)
		elevator.Orders[assignee][msg.Content.Order.Floor][msg.Content.Order.Button] = true
		elevio.SetButtonLamp(msg.Content.Order.Button, msg.Content.Order.Floor, true)
		clearBidsForOrder(elevator, msg.Content.Order)
	}
}

func HandleHallOrder(elevator *types.ElevState, event types.ButtonEvent, doorTimerAction chan timer.TimerAction, txBuffer chan types.Message[types.Bid]) {
	// If single elevator, move elevator and return
	if len(getPeers()) < 2 {
		elev.MoveElevator(elevator, event, doorTimerAction)
		return
	}
	msg := types.Message[types.Bid]{
		Type:      types.BidMsg,
		Content:   types.Bid{Order: event},
		SenderID:  elevator.NodeID,
		LoopCount: 0,
	}

	cost := elev.Cost(elevator, event)
	elevator.Bids = append(elevator.Bids,
		types.Bid{
			NodeID: elevator.NodeID,
			Order:  event,
			Cost:   []time.Duration{cost},
		},
	)
	txBuffer <- msg
}

// determineWinner returns the NodeID of the elevator with the lowest bid
func findBestBid(elevator *types.ElevState, event types.ButtonEvent) int {
	var lowestCost time.Duration = time.Hour * 24 // Start with a very high cost
	bestNode := -1

	for _, bid := range elevator.Bids {
		if bid.Order.Floor == event.Floor && bid.Order.Button == event.Button {
			if len(bid.Cost) > 0 && bid.Cost[0] < lowestCost {
				lowestCost = bid.Cost[0]
				bestNode = bid.NodeID
			}
		}
	}
	return bestNode
}
func clearBidsForOrder(elevator *types.ElevState, event types.ButtonEvent) {
	newBids := make([]types.Bid, 0)

	for _, bid := range elevator.Bids {
		if bid.Order.Floor != event.Floor || bid.Order.Button != event.Button {
			newBids = append(newBids, bid)
		}
	}

	elevator.Bids = newBids
}

// HandleHallArrival processes notifications that an elevator has arrived at a hall call
func HandleHallArrival(elevator *types.ElevState, msg types.Message[types.HallArrival], hallArrivalTx chan types.Message[types.HallArrival]) {
	// Ignore own hall arrivals
	if msg.SenderID == elevator.NodeID {
		return
	}

	// Update the order matrix to mark this order as completed
	if msg.SenderID < len(elevator.Orders) &&
		msg.Content.Order.Floor < len(elevator.Orders[msg.SenderID]) {
		elevator.Orders[msg.SenderID][msg.Content.Order.Floor][msg.Content.Order.Button] = false
	}

	// Turn off the light for this order
	elevio.SetButtonLamp(msg.Content.Order.Button, msg.Content.Order.Floor, false)
}

// msgBuffer listens for messages, and sends a burst of messages at a fixed interval
func msgBuffer(
	bidBuf chan types.Message[types.Bid],
	hallBuf chan types.Message[types.HallArrival],
	bidTx chan types.Message[types.Bid],
	hallArrivalTx chan types.Message[types.HallArrival],
) {
	for {
		select {
		case msg := <-bidBuf:
			burstTransmit(msg, bidTx)
		case msg := <-hallBuf:
			burstTransmit(msg, hallArrivalTx)
		}
	}
}

func burstTransmit[T types.MsgContent](msg types.Message[T], tx chan types.Message[T]) {
	for i := 0; i < config.MsgRepetitions; i++ {
		tx <- msg
		time.Sleep(config.MsgInterval)
	}
}
