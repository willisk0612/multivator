package dispatcher

import (
	"log/slog"

	"multivator/src/config"
	"multivator/src/types"
)

// transmitOrderSync sends a synchronization message to all nodes
func transmitOrderSync(elevator types.ElevState, txBuf chan Message[Sync], restoreCabOrders bool) {
	slog.Debug("Entered transmitOrderSync")
	txBuf <- Message[Sync]{
		Type:      SyncMsg,
		LoopCount: 0,
		Content:   Sync{Orders: elevator.Orders, RestoreCabOrders: restoreCabOrders},
		SenderID:  config.NodeID,
	}
}

// syncOrders returns new orders from network
//   - restores cab orders if sync.Content.RestoreCabOrders is true
//   - stores cab orders for other nodes if they differ from current orders
//   - updates hall orders if they differ from current orders
func syncOrders(elevator types.ElevState,
	receivedMsg Message[Sync]) types.ElevState {
	for node := range receivedMsg.Content.Orders {
		for floor := range receivedMsg.Content.Orders[node] {
			for btn := range receivedMsg.Content.Orders[node][floor] {
				switch types.ButtonType(btn) {
				case types.BT_Cab:
					if receivedMsg.Content.RestoreCabOrders {
						// Only restore cab orders for this specific node.
						if node == config.NodeID {
							elevator.Orders[node][floor][btn] = receivedMsg.Content.Orders[node][floor][btn]
						}
					} else {
						// Store cab orders for other nodes without affecting own cab orders.
						if node != config.NodeID && receivedMsg.Content.Orders[node][floor][btn] != elevator.Orders[node][floor][btn] {
							elevator.Orders[node][floor][btn] = receivedMsg.Content.Orders[node][floor][btn]
						}
					}
				default: // Hall orders
					if receivedMsg.Content.Orders[node][floor][btn] != elevator.Orders[node][floor][btn] && node != config.NodeID {
						elevator.Orders[node][floor][btn] = receivedMsg.Content.Orders[node][floor][btn]
					}
				}
			}
		}
	}
	return elevator
}
