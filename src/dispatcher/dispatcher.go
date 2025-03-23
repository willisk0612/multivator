package dispatcher

import (
	"fmt"
	"slices"
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
	bidTimeoutCh := make(chan types.HallOrder)

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
			elevator = createHallOrder(
				elevator,
				hallOrder,
				bidMap,
				peerList,
				orderUpdateCh,
				bidTxBufCh,
				bidTimeoutCh,
			)

		case bidRx := <-bidRxBufCh:
			switch bidRx.Type {
			case BidInitial:
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
				storeBid(bidRx, bidMap)
			}

			numBids := len(bidMap[bidRx.Content.Order].Costs)
			numPeers := len(peerList.Peers)
			if numBids == numPeers {
				bidEntry := bidMap[bidRx.Content.Order]
				if bidEntry.Timer != nil {
					bidEntry.Timer.Stop()
				}

				lowestCost := 100 * time.Second
				var assignee int
				for nodeID, cost := range bidEntry.Costs {
					if cost < lowestCost || (cost == lowestCost && nodeID < assignee) {
						lowestCost = cost
						assignee = nodeID
					}
				}

				if assignee == config.NodeID {
					elevator.Orders[config.NodeID][bidRx.Content.Order.Floor][bidRx.Content.Order.Button] = true
					orderUpdateCh <- elevator.Orders
				} else if bidEntry.Costs[assignee] != 0 {
					elevator.Orders[assignee][bidRx.Content.Order.Floor][bidRx.Content.Order.Button] = true
					orderUpdateCh <- elevator.Orders
				}
				delete(bidMap, bidRx.Content.Order)
			}

		case syncRx := <-syncRxBufCh:
			// Sync received orders
			utils.ForEachOrder(syncRx.Content.Orders, func(node, floor, btn int) {
				switch types.ButtonType(btn) {
				case types.BT_Cab:
					// On RestoreCabOrders, merge local cab orders with received cab orders
					if syncRx.Content.RestoreCabOrders && node == config.NodeID {
						elevator.Orders[node][floor][btn] = elevator.Orders[node][floor][btn] ||
							syncRx.Content.Orders[node][floor][btn]
						return
					}
					// Only sync cab orders for different nodes, our own cab orders are handled in executor
					if node != config.NodeID {
						elevator.Orders[node][floor][btn] = syncRx.Content.Orders[node][floor][btn]
					}
				default: // Hall orders are overwritten
					if node != config.NodeID {
						elevator.Orders[node][floor][btn] = syncRx.Content.Orders[node][floor][btn]
					}
				}
			})
			orderUpdateCh <- elevator.Orders

		case <-sendSyncCh:
			syncTxBufCh <- Msg[Sync]{
				Type:     SyncMsg,
				Content:  Sync{Orders: elevator.Orders, RestoreCabOrders: false},
				SenderID: config.NodeID,
			}

		case order := <-bidTimeoutCh:
			if entry, exists := bidMap[order]; exists {
				elevator.Orders[config.NodeID][order.Floor][order.Button] = true
				orderUpdateCh <- elevator.Orders
				if entry.Timer != nil {
					entry.Timer.Stop()
				}
				delete(bidMap, order)
			}

		case update := <-peerUpdateCh:
			peerList.Peers = update.Peers
			// If we connect or disconnect, print status
			ownID := fmt.Sprintf("node-%d", config.NodeID)
			if update.New == ownID || (slices.Contains(prevPeerList.Peers, ownID) && slices.Contains(update.Lost, ownID)) {
				utils.PrintStatus(peerList)
			}

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
					if slices.Contains(prevPeerList.Peers, lostPeer) {
						peerInt, _ := strconv.Atoi(lostPeer[5:])
						nodeIDs := make([]int, 0, len(peerList.Peers))
						for _, node := range peerList.Peers {
							nodeID, _ := strconv.Atoi(node[5:])
							nodeIDs = append(nodeIDs, nodeID)
						}
						minID := slices.Min(nodeIDs)

						utils.ForEachOrder(elevator.Orders, func(_, floor, btn int) {
							if btn != int(types.BT_Cab) &&
								elevator.Orders[peerInt][floor][btn] &&
								config.NodeID == minID {
								hallOrder := types.HallOrder{Floor: floor, Button: types.HallType(btn)}
								elevator = createHallOrder(
									elevator,
									hallOrder,
									bidMap,
									peerList,
									orderUpdateCh,
									bidTxBufCh,
									bidTimeoutCh,
								)
							}
						})
					}
				}
			}
			prevPeerList = peerList
		}
	}
}

// createHallOrder is called when a hall order is received, or if we need to overtake lost hall orders.
//   - If we are alone, take the order immediately.
//   - Else, start a bidding timeout, store own bid, and send the bid to the network.
func createHallOrder(
	elevator types.ElevState,
	hallOrder types.HallOrder,
	bidMap BidMap,
	peerList peers.PeerUpdate,
	orderUpdateCh chan<- types.Orders,
	bidTxBufCh chan<- Msg[Bid],
	bidTimeoutCh chan<- types.HallOrder,
) types.ElevState {
	if len(peerList.Peers) < 2 {
		elevator.Orders[config.NodeID][hallOrder.Floor][hallOrder.Button] = true
		orderUpdateCh <- elevator.Orders
		return elevator
	}

	// Start timeout timer for the bid
	timer := time.AfterFunc(config.BidTimeout, func() {
		bidTimeoutCh <- hallOrder
	})

	cost := timeToServeOrder(elevator, hallOrder)
	bidEntry := Msg[Bid]{
		SenderID: config.NodeID,
		Type:     BidInitial,
		Content:  Bid{Order: hallOrder, Cost: cost},
	}
	storeBid(bidEntry, bidMap)
	// Attach the timer and timeout channel to the bid entry
	// Maps are reference types, so we can update it here directly
	entry := bidMap[hallOrder]
	entry.Timer = timer
	bidMap[hallOrder] = entry

	bidTxBufCh <- bidEntry

	return elevator
}

// storeBid is called on hall orders, initial bids, and reply bids.
//   - Creates or stores the bid in the bidMap.
func storeBid(msg Msg[Bid], bidMap BidMap) {
	order := msg.Content.Order
	entry, exists := bidMap[order]
	if !exists {
		entry = BidMapValues{
			Costs: make(map[int]time.Duration),
			Timer: nil,
		}
	}
	entry.Costs[msg.SenderID] = msg.Content.Cost
	bidMap[order] = entry
}
