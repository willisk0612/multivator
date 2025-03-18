package dispatcher

import (
	"fmt"
	"log/slog"
	"time"

	"multivator/lib/network/bcast"
	"multivator/lib/network/peers"
	"multivator/src/config"
	"multivator/src/types"
)

func Run(elevUpdateCh <-chan types.ElevState,
	orderUpdateCh chan<- types.Orders,
	hallOrderCh <-chan types.ButtonEvent,
	sendSyncCh <-chan bool) {

	bidTx := make(chan Message[Bid])
	bidTxBuf := make(chan Message[Bid])
	bidRx := make(chan Message[Bid])
	syncTx := make(chan Message[Sync])
	syncTxBuf := make(chan Message[Sync])
	syncRx := make(chan Message[Sync])
	peerUpdateCh := make(chan peers.PeerUpdate)
	var peerList peers.PeerUpdate
	hallOrders := make(hallOrders)

	go bcast.Transmitter(config.BcastPort, bidTx, syncTx)
	go bcast.Receiver(config.BcastPort, bidRx, syncRx)
	go peers.Transmitter(config.PeersPort, fmt.Sprintf("node-%d", config.NodeID), make(chan bool))
	go peers.Receiver(config.PeersPort, peerUpdateCh)
	go msgBufferTx(bidTxBuf, bidTx)
	go msgBufferTx(syncTxBuf, syncTx)

	elevator := <-elevUpdateCh

	for {
		select {
		case elevUpdate := <-elevUpdateCh:
			elevator = elevUpdate
		case hallOrder := <-hallOrderCh:
			// If we are alone, send order back
			if len(peerList.Peers) < 2 {
				elevator.Orders[config.NodeID][hallOrder.Floor][hallOrder.Button] = true
				orderUpdateCh <- elevator.Orders
			} else {
				// Store and transmit initial bid
				cost := timeToServeOrder(elevator, hallOrder)
				msg := Message[Bid]{
					Type:      BidMsg,
					Content:   Bid{BtnEvent: hallOrder, Cost: cost},
					SenderID:  config.NodeID,
					LoopCount: 0,
				}
				storeBid(msg, hallOrders)
				bidTxBuf <- msg

			}
		case receivedBid := <-bidRx:
			isOwnBid := receivedBid.SenderID == config.NodeID
			switch {
			case receivedBid.LoopCount == 0 && !isOwnBid: // Received initial bid
				// Store own bid
				cost := timeToServeOrder(elevator, receivedBid.Content.BtnEvent)
				storeBid(Message[Bid]{
					Type:     BidMsg,
					Content:  Bid{BtnEvent: receivedBid.Content.BtnEvent, Cost: cost},
					SenderID: config.NodeID,
				}, hallOrders)
				storeBid(receivedBid, hallOrders)

				// Transmit own bid
				bidTxBuf <- Message[Bid]{
					Type:      BidMsg,
					Content:   Bid{BtnEvent: receivedBid.Content.BtnEvent, Cost: cost},
					SenderID:  config.NodeID,
					LoopCount: 1,
				}
			case receivedBid.LoopCount == 1: // Received reply bid
				if !isOwnBid {
					storeBid(receivedBid, hallOrders)
				}
				// Check if all bids are in
				numBids := len(hallOrders[receivedBid.Content.BtnEvent])
				numPeers := len(peerList.Peers)
				slog.Debug("Checking if all bids are received", "bids:", numBids, "peers:", numPeers)
				if numBids == numPeers {
					// Determine assignee: take order if local, otherwise set button lamp
					assignee := findAssignee(receivedBid.Content.BtnEvent, hallOrders)
					if assignee == config.NodeID {
						elevator.Orders[config.NodeID][receivedBid.Content.BtnEvent.Floor][receivedBid.Content.BtnEvent.Button] = true
						orderUpdateCh <- elevator.Orders
					} else {
						// If assignee cost is 0, dont set button lamp
						if hallOrders[receivedBid.Content.BtnEvent][assignee].Cost != 0 {
							elevator.Orders[assignee][receivedBid.Content.BtnEvent.Floor][receivedBid.Content.BtnEvent.Button] = true
							orderUpdateCh <- elevator.Orders
						}
					}
					delete(hallOrders, receivedBid.Content.BtnEvent)
				}
			}
		case receivedSync := <-syncRx:
			if receivedSync.SenderID == config.NodeID {
				continue
			}
			slog.Debug("Received sync from node", "nodeID", receivedSync.SenderID)
			elevator = syncOrders(elevator, receivedSync)
			slog.Debug("New state", "elevator", elevator)
			orderUpdateCh <- elevator.Orders
			slog.Debug("lights synced")
		case <-sendSyncCh:
			transmitOrderSync(elevator, syncTxBuf, false)
		case update := <-peerUpdateCh:
			peerList.Peers = update.Peers
			slog.Info("Peer update", "peerUpdate", update)
			// If a node different from our own connects, sync state with restoring cab orders.
			if update.New != fmt.Sprintf("node-%d", config.NodeID) && update.New != "" {
				transmitOrderSync(elevator, syncTxBuf, true)
			}
		}
	}
}

// msgBufferTx listens for messages, and sends a burst of messages at a fixed interval
func msgBufferTx[T MsgContent](msgBufCh chan Message[T], msgTxCh chan Message[T]) {
	for msg := range msgBufCh {
		for range config.MsgRepetitions {
			msgTxCh <- msg
			time.Sleep(config.MsgInterval)
		}
	}
}

func findAssignee(event types.ButtonEvent, hallOrders hallOrders) int {
	slog.Debug("Entered findAssignee")
	lowestCost := 24 * time.Hour
	assignee := 0 // Default to node 0

	for nodeID, bid := range hallOrders[event] {
		if bid.Cost < lowestCost || (bid.Cost == lowestCost && nodeID < assignee) {
			lowestCost = bid.Cost
			assignee = nodeID
		}
	}

	slog.Debug("Assigning order to", "nodeID", assignee, "cost", lowestCost, "All bids", hallOrders[event])
	return assignee
}

func storeBid(msg Message[Bid], hallOrders hallOrders) {
	if hallOrders[msg.Content.BtnEvent] == nil {
		hallOrders[msg.Content.BtnEvent] = make(map[int]Bid)
	}
	hallOrders[msg.Content.BtnEvent][msg.SenderID] = Bid{
		Cost:     msg.Content.Cost,
		BtnEvent: msg.Content.BtnEvent,
	}
}
