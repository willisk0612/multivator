package network

import (
	"log/slog"

	"multivator/lib/driver-go/elevio"
	"multivator/src/config"
	"multivator/src/types"
)

// Synchronizes all orders between nodes.
//   - Cab orders are merged with existing orders if restoreCabOrders
//   - If !restoreCabOrders, cab orders are overwritten by syncOrders.
//   - Hall orders overwritten by syncOrders.
func HandleSyncOrders(elevator *types.ElevState, syncOrders types.Message[types.SyncOrders], restoreCabOrders bool) {
	if syncOrders.SenderID == elevator.NodeID {
		return // Ignore own messages
	}

	for node := range syncOrders.Content.Orders {
		for floor := range syncOrders.Content.Orders[node] {
			for btn := range syncOrders.Content.Orders[node][floor] {
				switch types.ButtonType(btn) {
				case types.BT_Cab:
					if restoreCabOrders {
						elevator.Orders[node][floor][btn] = syncOrders.Content.Orders[node][floor][btn]
					} else {
						elevator.Orders[node][floor][btn] = elevator.Orders[node][floor][btn] || syncOrders.Content.Orders[node][floor][btn]
					}
				default:
					elevator.Orders[node][floor][btn] = syncOrders.Content.Orders[node][floor][btn]
				}

			}
		}
	}
	syncCabLights(elevator.Orders[elevator.NodeID])
	syncHallLights(elevator.Orders)
}

// Synchronizes cab lights with orders.
func syncCabLights(orders [][]bool) {
	for floor := range orders {
		elevio.SetButtonLamp(types.BT_Cab, floor, orders[floor][types.BT_Cab])
	}
}

// Synchronizes hall lights with orders.
func syncHallLights(orders [][][]bool) {
	syncedOrders := make([][]bool, config.NumFloors)
	hallUpAndDown := 2

	for floor := range syncedOrders {
		syncedOrders[floor] = make([]bool, hallUpAndDown)
	}

	for node := range orders {
		for floor := range orders[node] {
			for btn := range orders[node][floor] {
				if btn < hallUpAndDown { // Only process hall buttons (0 and 1)
					syncedOrders[floor][btn] = syncedOrders[floor][btn] || orders[node][floor][btn]
				}
			}
		}
	}

	for floor := range syncedOrders {
		for btn := 0; btn < hallUpAndDown; btn++ { // Use regular for loop with index
			elevio.SetButtonLamp(types.ButtonType(btn), floor, syncedOrders[floor][btn])
		}
	}
}

func TransmitOrderSync(elevator *types.ElevState, txBuf chan types.Message[types.SyncOrders]) {
	slog.Debug("Entered TransmitOrderSync")
	txBuf <- types.Message[types.SyncOrders]{
		Type:      types.SyncOrdersMsg,
		LoopCount: 0,
		Content:   types.SyncOrders{Orders: elevator.Orders},
		SenderID:  elevator.NodeID,
	}
}
