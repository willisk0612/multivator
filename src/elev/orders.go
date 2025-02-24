// Contains order related helper functions for the elevator module.
package elev

import (
	"multivator/lib/driver-go/elevio"
	"multivator/src/config"
	"multivator/src/types"
)

func (elevator *ElevState) shouldStop() bool {
	if elevator.Floor < 0 {
		return false
	}
	currentorders := elevator.Orders[elevator.Floor]

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

// Clears cab order and current direction hall order and lamp.
func (elevator *ElevState) clearFloor() {
	elevator.clearOrderAndLamp(types.BT_Cab)
	shouldClear := elevator.clearOrdersAtFloor()
	for btn := range config.NumButtons {
		if shouldClear[btn] {
			elevator.clearOrderAndLamp(types.ButtonType(btn))
		}
	}
}

func (elevator *ElevState) clearOrdersAtFloor() [config.NumButtons]bool {
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
		if elevator.ordersAbove() == 0 {
			shouldClear[types.BT_HallDown] = true
		}
	case types.MD_Down:
		shouldClear[types.BT_HallDown] = true
		if elevator.ordersBelow() == 0 {
			shouldClear[types.BT_HallUp] = true
		}
	case types.MD_Stop:
		shouldClear[types.BT_HallUp] = true
		shouldClear[types.BT_HallDown] = true
	}

	return shouldClear
}

func (elevator *ElevState) clearOrderAndLamp(btn types.ButtonType) {
	elevator.Orders[elevator.Floor][btn] = false
	elevio.SetButtonLamp(btn, elevator.Floor, false)
}

func (elevator *ElevState) countOrders(startFloor int, endFloor int) (result int) {
	for floor := startFloor; floor < endFloor; floor++ {
		for btn := range config.NumButtons {
			if elevator.Orders[floor][btn] {
				result++
			}
		}
	}
	return result
}

func (elevator *ElevState) ordersAbove() int {
	return elevator.countOrders(elevator.Floor+1, config.NumFloors)
}

func (elevator *ElevState) ordersBelow() int {
	return elevator.countOrders(0, elevator.Floor)
}

func (elevator *ElevState) ordersHere() int {
	return elevator.countOrders(elevator.Floor, elevator.Floor+1)
}
