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
	hallOrderCh <-chan types.HallOrder,
	sendSyncCh <-chan bool,
) {
	bidTxCh := make(chan Msg[Bid])
	bidTxBufCh := make(chan Msg[Bid])
	bidRxCh := make(chan Msg[Bid])
	syncTxCh := make(chan Msg[Sync])
	syncTxBufCh := make(chan Msg[Sync])
	syncRxCh := make(chan Msg[Sync])
	peerUpdateCh := make(chan peers.PeerUpdate)

	var peerList peers.PeerUpdate
	hallOrders := make(BidMap)

	go bcast.Transmitter(config.BcastPort, bidTxCh, syncTxCh)
	go bcast.Receiver(config.BcastPort, bidRxCh, syncRxCh)
	go peers.Transmitter(config.PeersPort, fmt.Sprintf("node-%d", config.NodeID), make(chan bool))
	go peers.Receiver(config.PeersPort, peerUpdateCh)
	go msgBufferTx(bidTxBufCh, bidTxCh)
	go msgBufferTx(syncTxBufCh, syncTxCh)

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
				cost := timeToServeOrder(elevator, hallOrder)
				slog.Debug("Sending initial bid")
				bidEntry := Msg[Bid]{
					SenderID:  config.NodeID,
					Type:      BidInitial,
					Content:   Bid{Order: hallOrder, Cost: cost},
				}
				storeBid(bidEntry, hallOrders)
				bidTxBufCh <- bidEntry
			}
		case bidRx := <-bidRxCh:
			if bidRx.SenderID == config.NodeID {
				continue
			}

			switch bidRx.Type {
			case BidInitial:
				slog.Debug("Received initial bid. Sending reply bid")
				storeBid(bidRx, hallOrders)
				cost := timeToServeOrder(elevator, bidRx.Content.Order)
				bidEntry := Msg[Bid]{
					SenderID:  config.NodeID,
					Type:      BidReply,
					Content:   Bid{Order: bidRx.Content.Order, Cost: cost},
				}
				storeBid(bidEntry, hallOrders)
				bidTxBufCh <- bidEntry
			case BidReply:
				slog.Debug("Received reply bid")
				storeBid(bidRx, hallOrders)
			}

			numBids := len(hallOrders[bidRx.Content.Order])
			numPeers := len(peerList.Peers)
			slog.Debug("Checking if all bids are received", "bids:", numBids, "peers:", numPeers)
			if numBids == numPeers {
				lowestCost := 100 * time.Second
				var assignee int
				for nodeID, bid := range hallOrders[bidRx.Content.Order] {
					if bid < lowestCost || (bid == lowestCost && nodeID < assignee) {
						lowestCost = bid
						assignee = nodeID
					}
				}

				if assignee == config.NodeID {
					elevator.Orders[config.NodeID][bidRx.Content.Order.Floor][bidRx.Content.Order.Button] = true
					orderUpdateCh <- elevator.Orders
				} else if hallOrders[bidRx.Content.Order][assignee] != 0 {
					// Cost is 0 means we dont need to set the lamp
					elevator.Orders[assignee][bidRx.Content.Order.Floor][bidRx.Content.Order.Button] = true
					orderUpdateCh <- elevator.Orders
				}
			}
			delete(hallOrders, bidRx.Content.Order)

		case syncRx := <-syncRxCh:
			if syncRx.SenderID == config.NodeID {
				continue
			}
			elevator = syncOrders(elevator, syncRx)
			orderUpdateCh <- elevator.Orders
		case <-sendSyncCh:
			transmitOrderSync(elevator, syncTxBufCh, false)
		case update := <-peerUpdateCh:
			peerList.Peers = update.Peers
			// If a node different from our own connects, sync state with restoring cab orders.
			if update.New != fmt.Sprintf("node-%d", config.NodeID) && update.New != "" {
				transmitOrderSync(elevator, syncTxBufCh, true)
			}
		}
	}
}

// msgBufferTx listens for messages, and sends a burst of messages at a fixed interval
func msgBufferTx[T MsgContent](msgBufCh chan Msg[T], msgTxCh chan Msg[T]) {
	for msg := range msgBufCh {
		for range config.MsgRepetitions {
			msgTxCh <- msg
			time.Sleep(config.MsgInterval)
		}
	}
}

func storeBid(msg Msg[Bid], hallOrders BidMap) {
	slog.Debug("Trying to store bid", "order", msg.Content.Order, "cost", msg.Content.Cost)
	if hallOrders[msg.Content.Order] == nil {
		hallOrders[msg.Content.Order] = make(map[int]time.Duration)
	}

	hallOrders[msg.Content.Order][msg.SenderID] = msg.Content.Cost
}
