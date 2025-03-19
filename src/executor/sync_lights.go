package executor

import (
	"multivator/lib/driver/elevio"
	"multivator/src/config"
	"multivator/src/types"
)

// syncCabLights updates cab lights based on current orders
func syncCabLights(localOrders types.Orders, receivedOrders types.Orders) {
	for floor := range localOrders[config.NodeID] {
		for btn := range localOrders[config.NodeID][floor] {
			if btn == int(types.BT_Cab) && localOrders[config.NodeID][floor][btn] != receivedOrders[config.NodeID][floor][btn] {
				elevio.SetButtonLamp(types.BT_Cab, floor, receivedOrders[config.NodeID][floor][btn])
			}
		}
	}
}

func syncHallLights(localOrders types.Orders, receivedOrders types.Orders) {
	for node := range localOrders {
		for floor := range localOrders[node] {
			for btn := range localOrders[node][floor] {
				if btn != int(types.BT_Cab) && localOrders[node][floor][btn] != receivedOrders[node][floor][btn] {
					elevio.SetButtonLamp(types.ButtonType(btn), floor, receivedOrders[node][floor][btn])
				}
			}
		}
	}
}
