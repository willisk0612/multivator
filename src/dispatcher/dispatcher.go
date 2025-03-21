package dispatcher

import (
	"fmt"
	"log/slog"
	"strconv"
	"sync/atomic"
	"time"

	"multivator/lib/network/bcast"
	"multivator/lib/network/peers"
	"multivator/src/config"
	"multivator/src/types"
	"multivator/src/utils"
)

func Run(elevUpdateCh <-chan types.ElevState,
	orderUpdateCh chan<- types.Orders,
	hallOrderCh <-chan types.HallOrder,
	sendSyncCh <-chan bool,
) {
	bidTxCh := make(chan Msg[Bid])
	bidTxBufCh := make(chan Msg[Bid])
	bidRxCh := make(chan Msg[Bid])
	bidRxBufCh := make(chan Msg[Bid])
	syncTxCh := make(chan Msg[Sync])
	syncTxBufCh := make(chan Msg[Sync])
	syncRxCh := make(chan Msg[Sync])
	syncRxBufCh := make(chan Msg[Sync])
	peerUpdateCh := make(chan peers.PeerUpdate)

	bidMap := make(BidMap)

	var peerList peers.PeerUpdate
	var prevPeerList peers.PeerUpdate
	var atomicCounter atomic.Uint64

	go bcast.Transmitter(config.BcastPort, bidTxCh, syncTxCh)
	go bcast.Receiver(config.BcastPort, bidRxCh, syncRxCh)
	go peers.Transmitter(config.PeersPort, fmt.Sprintf("node-%d", config.NodeID), make(chan bool))
	go peers.Receiver(config.PeersPort, peerUpdateCh)

	go msgBufferTx(bidTxBufCh, bidTxCh, &atomicCounter)
	go msgBufferTx(syncTxBufCh, syncTxCh, &atomicCounter)
	go msgBufferRx(bidRxBufCh, bidRxCh, &atomicCounter)
	go msgBufferRx(syncRxBufCh, syncRxCh, &atomicCounter)

	elevator := <-elevUpdateCh

	for {
		select {
		case elevUpdate := <-elevUpdateCh:
			elevator = elevUpdate

		case hallOrder := <-hallOrderCh:
			elevator = handleHallOrder(
				elevator,
				hallOrder,
				bidMap,
				peerList,
				orderUpdateCh,
				bidTxBufCh,
			)

		case bidRx := <-bidRxBufCh:
			switch bidRx.Type {
			case BidInitial:
				slog.Debug("Received initial bid. Sending reply bid")
				storeBid(bidRx, bidMap)
				cost := timeToServeOrder(elevator, bidRx.Content.Order)
				bidEntry := Msg[Bid]{
					SenderID: config.NodeID,
					Type:     BidReply,
					Content:  Bid{Order: bidRx.Content.Order, Cost: cost},
				}
				storeBid(bidEntry, bidMap)
				bidTxBufCh <- bidEntry
			case BidReply:
				slog.Debug("Received reply bid")
				storeBid(bidRx, bidMap)
			}

			numBids := len(bidMap[bidRx.Content.Order])
			numPeers := len(peerList.Peers)
			slog.Debug("Checking if all bids are received", "bids:", numBids, "peers:", numPeers)
			if numBids == numPeers {
				lowestCost := 100 * time.Second
				var assignee int
				for nodeID, bid := range bidMap[bidRx.Content.Order] {
					if bid < lowestCost || (bid == lowestCost && nodeID < assignee) {
						lowestCost = bid
						assignee = nodeID
					}
				}

				if assignee == config.NodeID {
					elevator.Orders[config.NodeID][bidRx.Content.Order.Floor][bidRx.Content.Order.Button] = true
					orderUpdateCh <- elevator.Orders
				} else if bidMap[bidRx.Content.Order][assignee] != 0 {
					elevator.Orders[assignee][bidRx.Content.Order.Floor][bidRx.Content.Order.Button] = true
					orderUpdateCh <- elevator.Orders
				}
				delete(bidMap, bidRx.Content.Order)
			}

		case syncRx := <-syncRxBufCh:
			elevator = syncOrders(elevator, syncRx)
			orderUpdateCh <- elevator.Orders

		case <-sendSyncCh:
			syncTxBufCh <- Msg[Sync]{
				Type:     SyncMsg,
				Content:  Sync{Orders: elevator.Orders, RestoreCabOrders: false},
				SenderID: config.NodeID,
			}

		case update := <-peerUpdateCh:
			slog.Debug("Peer update", "update", update)
			peerList.Peers = update.Peers
			// If a node different from our own connects, sync state with restoring cab orders.
			if update.New != fmt.Sprintf("node-%d", config.NodeID) && update.New != "" {
				syncTxBufCh <- Msg[Sync]{
					Type:     SyncMsg,
					Content:  Sync{Orders: elevator.Orders, RestoreCabOrders: true},
					SenderID: config.NodeID,
				}

				// If a node goes from PeerUpdate.Peers to PeerUpdate.Lost, we overtake active hall orders
				// Lowest node id initiates the bidding process
				for _, lostPeer := range update.Lost {
					for _, peer := range prevPeerList.Peers {
						slog.Debug("Checking lost peer", "peer", peer, "lostPeer", lostPeer)
						if peer == lostPeer {
							peerInt, _ := strconv.Atoi(peer[5:])
							minID := utils.FindLowestID(update.Peers)
							for floor := range elevator.Orders[peerInt] {
								for btn := range elevator.Orders[peerInt][floor] {
									if btn != int(types.BT_Cab) &&
										elevator.Orders[peerInt][floor][btn] &&
										config.NodeID == minID {
										slog.Debug("Starting bidding process for lost order", "floor", floor, "btn", btn)
										hallOrder := types.HallOrder{Floor: floor, Button: types.HallType(btn)}
										elevator = handleHallOrder(
											elevator,
											hallOrder,
											bidMap,
											peerList,
											orderUpdateCh,
											bidTxBufCh,
										)
									}
								}
							}
						}
					}
				}
				prevPeerList = peerList
			}
		}
	}
}

func storeBid(msg Msg[Bid], bidMap BidMap) {
	slog.Debug("Trying to store bid", "order", msg.Content.Order, "cost", msg.Content.Cost)
	if bidMap[msg.Content.Order] == nil {
		bidMap[msg.Content.Order] = make(map[int]time.Duration)
	}

	bidMap[msg.Content.Order][msg.SenderID] = msg.Content.Cost
}
