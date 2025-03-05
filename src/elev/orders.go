package elev

import (
	"multivator/lib/driver-go/elevio"
	"multivator/src/config"
	"multivator/src/types"
)

// Clears order and turns off lamp for the current floor and direction
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

// Checks if elevator should stop at current floor.
func shouldStop(elevator *types.ElevState) bool {
	switch elevator.Dir {
	case types.MD_Up:
		return elevator.Orders[elevator.NodeID][elevator.Floor][types.BT_HallUp] || elevator.Orders[elevator.NodeID][elevator.Floor][types.BT_Cab] || ordersAbove(elevator) == 0
	case types.MD_Down:
		return elevator.Orders[elevator.NodeID][elevator.Floor][types.BT_HallDown] || elevator.Orders[elevator.NodeID][elevator.Floor][types.BT_Cab] || ordersBelow(elevator) == 0
	default:
		return true
	}
}

// Algorithm for choosing direction of elevator.
//  1. If elevator is stopped, choose direction in which there are orders.
//  2. If elevator is moving, continue in the same direction until there are no more orders in that direction.
func chooseDirection(elevator *types.ElevState) types.DirnBehaviourPair {
	switch elevator.Dir {
	case types.MD_Stop:
		switch {
		case ordersAbove(elevator) > 0:
			return types.DirnBehaviourPair{Dir: types.MD_Up, Behaviour: types.Moving}
		case ordersHere(elevator) > 0:
			return types.DirnBehaviourPair{Dir: types.MD_Stop, Behaviour: types.DoorOpen}
		case ordersBelow(elevator) > 0:
			return types.DirnBehaviourPair{Dir: types.MD_Down, Behaviour: types.Moving}
		default:
			return types.DirnBehaviourPair{Dir: types.MD_Stop, Behaviour: types.Idle}
		}
	case types.MD_Up:
		switch {
		case ordersAbove(elevator) > 0:
			return types.DirnBehaviourPair{Dir: types.MD_Up, Behaviour: types.Moving}
		case ordersBelow(elevator) > 0:
			return types.DirnBehaviourPair{Dir: types.MD_Down, Behaviour: types.Moving}
		case ordersHere(elevator) > 0:
			return types.DirnBehaviourPair{Dir: types.MD_Stop, Behaviour: types.DoorOpen}
		default:
			return types.DirnBehaviourPair{Dir: types.MD_Stop, Behaviour: types.Idle}
		}
	case types.MD_Down:
		switch {
		case ordersHere(elevator) > 0:
			return types.DirnBehaviourPair{Dir: types.MD_Stop, Behaviour: types.DoorOpen}
		case ordersAbove(elevator) > 0:
			return types.DirnBehaviourPair{Dir: types.MD_Up, Behaviour: types.Moving}
		case ordersBelow(elevator) > 0:
			return types.DirnBehaviourPair{Dir: types.MD_Down, Behaviour: types.Moving}
		default:
			return types.DirnBehaviourPair{Dir: types.MD_Stop, Behaviour: types.Idle}
		}
	}
	return types.DirnBehaviourPair{Dir: types.MD_Stop, Behaviour: types.Idle}
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
