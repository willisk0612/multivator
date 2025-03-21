package dispatcher

import (
	"log/slog"

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
) types.ElevState {
	// If we are alone, send order back
	slog.Debug("Received hall order. Checking if we are alone", "peers", peerList.Peers)
	if len(peerList.Peers) < 2 {
		elevator.Orders[config.NodeID][hallOrder.Floor][hallOrder.Button] = true
		orderUpdateCh <- elevator.Orders
	} else {
		// Start timer
		cost := timeToServeOrder(elevator, hallOrder)
		slog.Debug("Sending initial bid")
		bidEntry := Msg[Bid]{
			SenderID: config.NodeID,
			Type:     BidInitial,
			Content:  Bid{Order: hallOrder, Cost: cost},
		}
		storeBid(bidEntry, bidMap)
		bidTxBufCh <- bidEntry
	}
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
