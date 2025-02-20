package elev

import (
	"main/lib/driver-go/elevio"
	"main/src/config"
	"main/src/types"
)

// Clears cab order and current direction hall order and lamp.
func clearFloor(elevator *types.Elevator) {
	clearOrderAndLamp(elevator, types.BT_Cab)
	shouldClear := clearOrdersAtFloor(elevator)
	for btn := range config.NumButtons {
		if shouldClear[btn] {
			clearOrderAndLamp(elevator, types.ButtonType(btn))
		}
	}
}

func clearOrdersAtFloor(elevator *types.Elevator) [config.NumButtons]bool {
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

func clearOrderAndLamp(elevator *types.Elevator, btn types.ButtonType) {
	elevator.Orders[elevator.Floor][btn] = false
	elevio.SetButtonLamp(btn, elevator.Floor, false)
}

func countOrders(elevator *types.Elevator, startFloor int, endFloor int) (result int) {
	for floor := startFloor; floor < endFloor; floor++ {
		for btn := range config.NumButtons {
			if elevator.Orders[floor][btn] {
				result++
			}
		}
	}
	return result
}

func ordersAbove(elevator *types.Elevator) int {
	return countOrders(elevator, elevator.Floor+1, config.NumFloors)
}

func ordersBelow(elevator *types.Elevator) int {
	return countOrders(elevator, 0, elevator.Floor)
}

func ordersHere(elevator *types.Elevator) int {
	return countOrders(elevator, elevator.Floor, elevator.Floor+1)
}
