package elev

import (
	"multivator/lib/driver-go/elevio"
	"multivator/src/config"
	"multivator/src/types"
)

// Clears cab order and current direction hall order and lamp.
func clearFloor(elevator *types.ElevState) {
	clearOrderAndLamp(elevator, types.BT_Cab)
	shouldClear := ordersToClear(elevator)
	for btn := range config.NumButtons {
		if shouldClear[btn] {
			clearOrderAndLamp(elevator, types.ButtonType(btn))
		}
	}
}

func ordersToClear(elevator *types.ElevState) [config.NumButtons]bool {
	shouldClear := [config.NumButtons]bool{}

	// At edge floors, clear all orders
	if elevator.Floor == 0 || elevator.Floor == config.NumFloors-1 {
		shouldClear[types.BT_HallUp] = true
		shouldClear[types.BT_HallDown] = true
		return shouldClear
	}

	// Clear hall orders in the same direction
	switch elevator.Dir {
	case types.MD_Up:
		shouldClear[types.BT_HallUp] = true
		if ordersAbove(elevator) == 0 {
			shouldClear[types.BT_HallDown] = true
		}
	case types.MD_Down:
		shouldClear[types.BT_HallDown] = true
		if ordersBelow(elevator) == 0 {
			shouldClear[types.BT_HallUp] = true
		}
	case types.MD_Stop:
		shouldClear[types.BT_HallUp] = true
		shouldClear[types.BT_HallDown] = true
	}

	return shouldClear
}

func clearOrderAndLamp(elevator *types.ElevState, btn types.ButtonType) {
	elevator.Orders[elevator.NodeID][elevator.Floor][btn] = false
	elevio.SetButtonLamp(btn, elevator.Floor, false)
}

func shouldStop(elevator *types.ElevState) bool {
	currentorders := elevator.Orders[elevator.NodeID][elevator.Floor]

	if currentorders[types.BT_Cab] ||
		currentorders[types.BT_HallUp] ||
		currentorders[types.BT_HallDown] {
		return true
	}

	// Always stop at edge floors
	if elevator.Floor == 0 || elevator.Floor == config.NumFloors-1 {
		return true
	}

	return false
}

func countOrders(elevator *types.ElevState, startFloor int, endFloor int) (result int) {
	for floor := startFloor; floor < endFloor; floor++ {
		for btn := range config.NumButtons {
			if elevator.Orders[elevator.NodeID][floor][btn] {
				result++
			}
		}
	}
	return result
}

func ordersAbove(elevator *types.ElevState) int {
	return countOrders(elevator, elevator.Floor+1, config.NumFloors)
}

func ordersBelow(elevator *types.ElevState) int {
	return countOrders(elevator, 0, elevator.Floor)
}

func ordersHere(elevator *types.ElevState) int {
	return countOrders(elevator, elevator.Floor, elevator.Floor+1)
}
