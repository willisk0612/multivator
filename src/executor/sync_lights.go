package executor

import (
	"multivator/lib/driver/elevio"
	"multivator/src/config"
	"multivator/src/types"
	"multivator/src/utils"
)

func syncCabLights(localOrders types.Orders, receivedOrders types.Orders) {
	utils.ForEachOrder(localOrders, func(node, floor, btn int) {
		if node == config.NodeID &&
			btn == int(types.BT_Cab) &&
			localOrders[node][floor][btn] != receivedOrders[node][floor][btn] {

			elevio.SetButtonLamp(types.BT_Cab, floor, receivedOrders[node][floor][btn])
		}
	})
}

func syncHallLights(localOrders types.Orders, receivedOrders types.Orders) {
	utils.ForEachOrder(localOrders, func(node, floor, btn int) {
		if btn != int(types.BT_Cab) &&
			localOrders[node][floor][btn] != receivedOrders[node][floor][btn] {

			elevio.SetButtonLamp(types.ButtonType(btn), floor, receivedOrders[node][floor][btn])
		}
	})
}
