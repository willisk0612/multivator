package network

import (
	"log/slog"

	"multivator/lib/driver-go/elevio"
	"multivator/src/config"
	"multivator/src/types"

	"github.com/tiendc/go-deepcopy"
)

func HandleSync(elevator *types.ElevState, sync types.Message[types.Sync]) {
	slog.Debug("Entered HandleSync")
	if sync.SenderID == elevator.NodeID {
		return // Ignore own messages
	}
	syncOrders(elevator, sync)
	if sync.Content.RestoreCabOrders {
		syncCabLights(elevator)
	}
	syncHallLights(elevator.Orders)
}

// syncOrders handles order synchronization between nodes
func syncOrders(elevator *types.ElevState, sync types.Message[types.Sync]) {
	for node := range sync.Content.Orders {
		for floor := range sync.Content.Orders[node] {
			for btn := range sync.Content.Orders[node][floor] {
				switch types.ButtonType(btn) {
				case types.BT_Cab:
					if sync.Content.RestoreCabOrders {
						// Only restore cab orders for this specific node.
						if node == elevator.NodeID {
							elevator.Orders[node][floor][btn] = sync.Content.Orders[node][floor][btn]
						}
					} else {
						// Store cab orders for other nodes without affecting own cab orders.
						if node != elevator.NodeID {
							elevator.Orders[node][floor][btn] = sync.Content.Orders[node][floor][btn]
						}
					}
				default:
					elevator.Orders[node][floor][btn] = sync.Content.Orders[node][floor][btn]
				}
			}
		}
	}
}

// syncCabLights updates cab lights based on current orders
func syncCabLights(elevator *types.ElevState) {
	for floor := range elevator.Orders[elevator.NodeID] {
		elevio.SetButtonLamp(types.BT_Cab, floor, elevator.Orders[elevator.NodeID][floor][types.BT_Cab])
	}
}

func syncHallLights(orders [][][]bool) {
	syncedOrders := make([][]bool, config.NumFloors)
	hallUpAndDown := 2

	for floor := range syncedOrders {
		syncedOrders[floor] = make([]bool, hallUpAndDown)
	}

	for node := range orders {
		for floor := range orders[node] {
			for btn := range orders[node][floor] {
				if btn < hallUpAndDown {
					syncedOrders[floor][btn] = syncedOrders[floor][btn] || orders[node][floor][btn]
				}
			}
		}
	}

	for floor := range syncedOrders {
		for btn := 0; btn < hallUpAndDown; btn++ {
			elevio.SetButtonLamp(types.ButtonType(btn), floor, syncedOrders[floor][btn])
		}
	}
}

// TransmitOrderSync sends a synchronization message to all nodes
// - performs a deep copy of the orders to avoid data races
func TransmitOrderSync(elevator *types.ElevState, txBuf chan types.Message[types.Sync], restoreCabOrders bool) {
	var ordersCopy [][][]bool
	err := deepcopy.Copy(&ordersCopy, elevator.Orders)
	if err != nil {
		slog.Error("Orders deepcopy failed, could not transmit sync.", "error", err)
		return
	}
	slog.Debug("Entered TransmitOrderSync")
	txBuf <- types.Message[types.Sync]{
		Type:      types.SyncMsg,
		LoopCount: 0,
		Content:   types.Sync{Orders: ordersCopy, RestoreCabOrders: restoreCabOrders},
		SenderID:  elevator.NodeID,
	}
}
