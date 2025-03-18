package executor

import (
	"multivator/lib/driver/elevio"
	"multivator/src/config"
	"multivator/src/types"
)

// Clears order and turns off lamp for the current floor and direction
func clearAtCurrentFloor(elevator types.ElevState) types.ElevState {
	elevator.Orders[config.NodeID][elevator.Floor][types.BT_Cab] = false
	elevio.SetButtonLamp(types.BT_Cab, elevator.Floor, false)
	shouldClear := OrdersToClearHere(elevator)
	for btn := range config.NumButtons {
		if shouldClear[btn] {
			elevator.Orders[config.NodeID][elevator.Floor][btn] = false
			elevio.SetButtonLamp(types.ButtonType(btn), elevator.Floor, false)
		}
	}
	return elevator
}

func OrdersToClearHere(elevator types.ElevState) [config.NumButtons]bool {
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
		if !ordersAbove(elevator) {
			shouldClear[types.BT_HallDown] = true
		}
	case types.MD_Down:
		shouldClear[types.BT_HallDown] = true
		if !ordersBelow(elevator) {
			shouldClear[types.BT_HallUp] = true
		}
	case types.MD_Stop:
		shouldClear[types.BT_HallUp] = true
		shouldClear[types.BT_HallDown] = true
	}

	return shouldClear
}

// Checks if elevator should stop at current floor.
func ShouldStopHere(elevator types.ElevState) bool {
	switch elevator.Dir {
	case types.MD_Up:
		return elevator.Orders[config.NodeID][elevator.Floor][types.BT_HallUp] ||
			elevator.Orders[config.NodeID][elevator.Floor][types.BT_Cab] ||
			!ordersAbove(elevator)
	case types.MD_Down:
		return elevator.Orders[config.NodeID][elevator.Floor][types.BT_HallDown] ||
			elevator.Orders[config.NodeID][elevator.Floor][types.BT_Cab] ||
			!ordersBelow(elevator)
	default:
		return true
	}
}

// Algorithm for choosing direction of elevator.
//  1. If elevator is stopped, choose direction in which there are orders.
//  2. If elevator is moving, continue in the same direction until there are no more orders in that direction.
func ChooseDirection(elevator types.ElevState) types.DirnBehaviourPair {
	switch elevator.Dir {
	case types.MD_Up:
		switch {
		case ordersAbove(elevator):
			return types.DirnBehaviourPair{Dir: types.MD_Up, Behaviour: types.Moving}
		case ordersHere(elevator):
			return types.DirnBehaviourPair{Dir: types.MD_Down, Behaviour: types.DoorOpen}
		case ordersBelow(elevator):
			return types.DirnBehaviourPair{Dir: types.MD_Down, Behaviour: types.Moving}
		default:
			return types.DirnBehaviourPair{Dir: types.MD_Stop, Behaviour: types.Idle}
		}
	case types.MD_Down:
		switch {
		case ordersBelow(elevator):
			return types.DirnBehaviourPair{Dir: types.MD_Down, Behaviour: types.Moving}
		case ordersHere(elevator):
			return types.DirnBehaviourPair{Dir: types.MD_Up, Behaviour: types.DoorOpen}
		case ordersAbove(elevator):
			return types.DirnBehaviourPair{Dir: types.MD_Up, Behaviour: types.Moving}
		default:
			return types.DirnBehaviourPair{Dir: types.MD_Stop, Behaviour: types.Idle}
		}
	case types.MD_Stop:
		switch {
		case ordersHere(elevator):
			return types.DirnBehaviourPair{Dir: types.MD_Stop, Behaviour: types.DoorOpen}
		case ordersAbove(elevator):
			return types.DirnBehaviourPair{Dir: types.MD_Up, Behaviour: types.Moving}
		case ordersBelow(elevator):
			return types.DirnBehaviourPair{Dir: types.MD_Down, Behaviour: types.Moving}
		default:
			return types.DirnBehaviourPair{Dir: types.MD_Stop, Behaviour: types.Idle}
		}
	}
	return types.DirnBehaviourPair{Dir: types.MD_Stop, Behaviour: types.Idle}
}

func countOrders(elevator types.ElevState, startFloor int, endFloor int) (result int) {
	for floor := startFloor; floor < endFloor; floor++ {
		for btn := range config.NumButtons {
			if elevator.Orders[config.NodeID][floor][btn] {
				result++
			}
		}
	}
	return result
}

func ordersAbove(elevator types.ElevState) bool {
	return countOrders(elevator, elevator.Floor+1, config.NumFloors) > 0
}

func ordersBelow(elevator types.ElevState) bool {
	return countOrders(elevator, 0, elevator.Floor) > 0
}

func ordersHere(elevator types.ElevState) bool {
	return countOrders(elevator, elevator.Floor, elevator.Floor+1) > 0
}
