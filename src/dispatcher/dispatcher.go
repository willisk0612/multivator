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
	openDoorCh chan<- bool,
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
	var atomicCounter atomic.Uint64

	go bcast.Transmitter(config.BcastPort, bidTxCh, syncTxCh)
	go bcast.Receiver(config.BcastPort, bidRxCh, syncRxCh)
	go peers.Transmitter(config.PeersPort, fmt.Sprintf("node-%d", config.NodeID), make(chan bool))
	go peers.Receiver(config.PeersPort, peerUpdateCh)

	go msgBufferTx(bidTxBufCh, bidTxCh, &atomicCounter)
	go msgBufferTx(syncTxBufCh, syncTxCh, &atomicCounter)
	go msgBufferRx(bidRxBufCh, bidRxCh, &atomicCounter)
	go msgBufferRx(syncRxBufCh, syncRxCh, &atomicCounter)

	elevator := new(types.ElevState)
	*elevator = <-elevUpdateCh

	for {
		select {
		case elevUpdate := <-elevUpdateCh:
			*elevator = elevUpdate

		case hallOrder := <-hallOrderCh:
			createHallOrder(
				elevator,
				hallOrder,
				bidMap,
				peerList,
				orderUpdateCh,
				bidTxBufCh,
				bidTimeoutCh,
			)

		case bidRx := <-bidRxBufCh:
			switch bidRx.Content.Type {
			case BidInitial:
				storeBid(bidRx, bidMap)
				cost := timeToServeOrder(*elevator, bidRx.Content.Order)
				bidEntry := Msg[Bid]{
					SenderID: config.NodeID,
					Content:  Bid{Type: BidReply, Order: bidRx.Content.Order, Cost: cost},
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
					// If we are on the same floor, only open the door
					if elevator.Floor == bidRx.Content.Order.Floor {
						elevator.Behaviour = types.DoorOpen
						openDoorCh <- true
						continue
					} // Else store the order
					elevator.Orders[assignee][bidRx.Content.Order.Floor][bidRx.Content.Order.Button] = true
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
					// Clear same floor immediately
					if node == config.NodeID &&
						syncRx.Content.Type == SyncCab &&
						elevator.Floor != floor {
						elevator.Orders[node][floor][btn] = elevator.Orders[node][floor][btn] ||
							syncRx.Content.Orders[node][floor][btn]
					} else if node != config.NodeID { // Only sync cab orders from other nodes
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
				Content:  Sync{Type: SyncOrders, Orders: elevator.Orders},
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

		case peerUpdate := <-peerUpdateCh:
			ownID := fmt.Sprintf("node-%d", config.NodeID)
			// Print status on network init or network loss
			if peerUpdate.New == ownID ||
				(slices.Contains(peerList.Peers, ownID) && slices.Contains(peerUpdate.Lost, ownID)) {
				utils.PrintStatus(peerUpdate)
			}

			// If we detect change from prevLostPeers to update.New, RestoreCabOrders
			if slices.Contains(peerList.Lost, peerUpdate.New) {
				syncTxBufCh <- Msg[Sync]{
					Content:  Sync{Type: SyncCab, Orders: elevator.Orders},
					SenderID: config.NodeID,
				}
			}

			// If a node goes from PeerUpdate.Peers to PeerUpdate.Lost, overtake active hall orders
			for _, lostPeer := range peerUpdate.Lost {
				if !slices.Contains(peerList.Peers, lostPeer) {
					continue
				}

				lostPeerInt, _ := strconv.Atoi(lostPeer[5:])
				utils.ForEachOrder(elevator.Orders, func(node, floor, btn int) {
					if node == lostPeerInt &&
						btn != int(types.BT_Cab) &&
						elevator.Orders[lostPeerInt][floor][btn] {
						hallOrder := types.HallOrder{Floor: floor, Button: types.HallType(btn)}
						createHallOrder(
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
			peerList = peerUpdate
		}
	}
}

// createHallOrder is called on: hall orders, overtake lost peers hall orders
//   - If we are alone, take the order immediately.
//   - Else, start a bidding timeout, store own bid, and send the bid to the network.
func createHallOrder(
	elevator *types.ElevState,
	hallOrder types.HallOrder,
	bidMap BidMap,
	peerList peers.PeerUpdate,
	orderUpdateCh chan<- types.Orders,
	bidTxBufCh chan<- Msg[Bid],
	bidTimeoutCh chan<- types.HallOrder,
) {
	if len(peerList.Peers) < 2 {
		elevator.Orders[config.NodeID][hallOrder.Floor][hallOrder.Button] = true
		orderUpdateCh <- elevator.Orders
		return
	}

	// Start timeout timer for the bid
	timer := time.AfterFunc(config.BidTimeout, func() {
		bidTimeoutCh <- hallOrder
	})

	cost := timeToServeOrder(*elevator, hallOrder)
	bidEntry := Msg[Bid]{
		SenderID: config.NodeID,
		Content:  Bid{Type: BidInitial, Order: hallOrder, Cost: cost},
	}
	storeBid(bidEntry, bidMap)
	// Attach the timer and timeout channel to the bid entry
	// Maps are reference types, so we can update it here directly
	entry := bidMap[hallOrder]
	entry.Timer = timer
	bidMap[hallOrder] = entry

	bidTxBufCh <- bidEntry
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
