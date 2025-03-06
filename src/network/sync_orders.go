package network

import (
	"log/slog"

	"multivator/lib/driver-go/elevio"
	"multivator/src/types"
)

// Synchronizes all orders between nodes.
//  - Cab orders are synced in case of network disconnection. Sent at each cab order.
//  - Hall orders are synced in case of network disconnection and light sync. Sent at each hall order arrival.
func HandleSyncOrders(elevator *types.ElevState, syncOrders types.Message[types.SyncOrders]) {
	if syncOrders.SenderID == elevator.NodeID {
		return // Ignore own messages
	}

	// Sync cab orders from other nodes, keep our own cab orders.

	for node := range syncOrders.Content.Orders {
		if node == elevator.NodeID {
			continue
		}
		for floor := range syncOrders.Content.Orders[node] {
			if syncOrders.Content.Orders[node][floor][int(types.BT_Cab)] {
				elevator.Orders[node][floor][int(types.BT_Cab)] = true
			}
		}
	}

	// Sync hall orders from other nodes, overwrite our own hall orders.

	for node := range syncOrders.Content.Orders {
		for floor := range syncOrders.Content.Orders[node] {
			for btn := range syncOrders.Content.Orders[node][floor] {
				if btn != int(types.BT_Cab) {
					elevator.Orders[node][floor][btn] = syncOrders.Content.Orders[node][floor][btn]
					elevio.SetButtonLamp(types.ButtonType(btn), floor, syncOrders.Content.Orders[node][floor][btn])
				}
			}
		}
	}
}


func TransmitOrderSync(elevator *types.ElevState, txBuffer chan types.Message[types.SyncOrders]) {
	slog.Debug("Entered TransmitHallArrival")
	txBuffer <- types.Message[types.SyncOrders]{
		Type:      types.SyncOrdersMsg,
		LoopCount: 0,
		Content:   types.SyncOrders{Orders: elevator.Orders},
		SenderID:  elevator.NodeID,
	}
}

