package dispatcher

import (
	"log/slog"
	"time"

	"multivator/lib/network/peers"

	"multivator/src/config"
	"multivator/src/types"
)

// handleHallOrder is called:
//   - When a hall order is received
//   - When a node overtakes a disconnected node
func handleHallOrder(
	elevator types.ElevState,
	hallOrder types.HallOrder,
	bidMap BidMap,
	peerList peers.PeerUpdate,
	orderUpdateCh chan<- types.Orders,
	bidTxBufCh chan<- Msg[Bid],
	bidTimeoutCh chan<- types.HallOrder,
) types.ElevState {
	slog.Debug("Received hall order. Checking if we are alone", "peers", peerList.Peers)
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

func syncOrders(elevator types.ElevState,
	syncRx Msg[Sync],
) types.ElevState {
	for node := range syncRx.Content.Orders {
		for floor := range syncRx.Content.Orders[node] {
			for btn := range syncRx.Content.Orders[node][floor] {
				switch types.ButtonType(btn) {
				case types.BT_Cab:
					// On RestoreCabOrders, merge local cab orders with received cab orders
					if syncRx.Content.RestoreCabOrders && node == config.NodeID {
						elevator.Orders[node][floor][btn] = elevator.Orders[node][floor][btn] ||
							syncRx.Content.Orders[node][floor][btn]
						continue
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
			}
		}
	}
	return elevator
}
