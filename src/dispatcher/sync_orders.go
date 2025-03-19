package dispatcher

import (
	"log/slog"

	"multivator/src/config"
	"multivator/src/types"
)

// transmitOrderSync sends a synchronization message to all nodes
func transmitOrderSync(elevator types.ElevState, txBuf chan Msg[Sync], restoreCabOrders bool) {
	slog.Debug("Entered transmitOrderSync")
	txBuf <- Msg[Sync]{
		Type:      SyncMsg,
		LoopCount: 0,
		Content:   Sync{Orders: elevator.Orders, RestoreCabOrders: restoreCabOrders},
		SenderID:  config.NodeID,
	}
}

func syncOrders(elevator types.ElevState,
	syncRx Msg[Sync],
) types.ElevState {
	for node := range syncRx.Content.Orders {
		for floor := range syncRx.Content.Orders[node] {
			for btn := range syncRx.Content.Orders[node][floor] {
				switch types.ButtonType(btn) {
				case types.BT_Cab:
					if syncRx.Content.RestoreCabOrders && node == config.NodeID {
						elevator.Orders[node][floor][btn] = syncRx.Content.Orders[node][floor][btn]
						continue
					}
					// Cab orders are merged
					if node != config.NodeID {
						elevator.Orders[node][floor][btn] = elevator.Orders[node][floor][btn] || syncRx.Content.Orders[node][floor][btn]
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
