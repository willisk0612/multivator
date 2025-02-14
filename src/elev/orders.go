package elev

import (
	"main/src/types"
	"main/src/config"
	"main/lib/driver-go/elevio"
)

// Clears cab order and current direction hall order and lamp.
func clearFloor(elevator *types.Elevator) {
	// Always clear cab order if passing by
	clearOrderAndLamp(elevator, types.BT_Cab)
	shouldClear := clearOrdersAtFloor(elevator)

	// Loop through each button type and clear if indicated.
	for btn := 0; btn < config.N_BUTTONS; btn++ {
		if shouldClear[btn] {
			clearOrderAndLamp(elevator, types.ButtonType(btn))
		}
	}
}

func clearOrdersAtFloor(elevator *types.Elevator) [config.N_BUTTONS]bool {
	shouldClear := [config.N_BUTTONS]bool{}

	// At edge floors, clear all orders
	if elevator.Floor == 0 || elevator.Floor == config.N_FLOORS-1 {
		shouldClear[types.BT_HallUp] = true
		shouldClear[types.BT_HallDown] = true
		return shouldClear
	}

	// Clear hall orders in the same direction
	switch elevator.Dir {
	case types.MD_UP:
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


// Clears order and lamp for button press.
func clearOrderAndLamp(elevator *types.Elevator, btn types.ButtonType) {
	elevator.Orders[elevator.Floor][btn] = false
	elevio.SetButtonLamp(btn, elevator.Floor, false)
}


func countOrders(elevator *types.Elevator, startFloor int, endFloor int) (result int) {
	for floor := startFloor; floor < endFloor; floor++ {
		for btn := 0; btn < config.N_BUTTONS; btn++ {
			if elevator.Orders[floor][btn] {
				result++
			}
		}
	}
	return result
}

func ordersAbove(elevator *types.Elevator) int {
	return countOrders(elevator, elevator.Floor+1, config.N_FLOORS)
}

func ordersBelow(elevator *types.Elevator) int {
	return countOrders(elevator, 0, elevator.Floor)
}

func ordersHere(elevator *types.Elevator) int {
	return countOrders(elevator, elevator.Floor, elevator.Floor+1)
}
