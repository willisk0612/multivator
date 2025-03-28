package executor

import (
	"multivator/lib/driver/elevio"
	"multivator/src/config"
	"multivator/src/types"
)

// ChooseDirection is called in chooseAction and in cost function.
//   - The algorithm prioritizes hall orders in the same direction as the elevator.
//   - If the elevator is between floors with ordersHere, it will choose the opposite direction.
func ChooseDirection(elevator *types.ElevState) types.DirnBehaviourPair {
	switch elevator.Dir {
	case types.MD_Up:
		switch {
		case ordersAbove(elevator):
			return types.DirnBehaviourPair{Dir: types.MD_Up, Behaviour: types.Moving}
		case ordersHere(elevator):
			if elevator.BetweenFloors {
				return types.DirnBehaviourPair{Dir: types.MD_Down, Behaviour: types.Moving}
			}
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
			if elevator.BetweenFloors {
				return types.DirnBehaviourPair{Dir: types.MD_Up, Behaviour: types.Moving}
			}
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
	default:
		return types.DirnBehaviourPair{Dir: types.MD_Stop, Behaviour: types.Idle}
	}
}

// OrdersToClearHere is called in clearAtCurrentFloor and in cost function.
//   - Returns a list of orders to clear at the current floor.
func OrdersToClearHere(elevator *types.ElevState) [config.NumButtons]bool {
	var shouldClear [config.NumButtons]bool
	shouldClear[types.BT_Cab] = true

	switch elevator.Dir {
	case types.MD_Up:
		if !ordersAbove(elevator) &&
			!elevator.Orders[config.NodeID][elevator.Floor][types.BT_HallUp] {
			shouldClear[types.BT_HallDown] = true
		}
		shouldClear[types.BT_HallUp] = true

	case types.MD_Down:
		if !ordersBelow(elevator) &&
			!elevator.Orders[config.NodeID][elevator.Floor][types.BT_HallDown] {
			shouldClear[types.BT_HallUp] = true
		}
		shouldClear[types.BT_HallDown] = true

	default:
		shouldClear[types.BT_HallUp] = true
		shouldClear[types.BT_HallDown] = true
	}

	return shouldClear
}

// ShouldStopHere is called on floor sensor updates, and in cost function.
//   - Returns true if there are hall orders in the same direction or cab orders at the current floor.
func ShouldStopHere(elevator *types.ElevState) bool {
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

// clearAtCurrentFloor is called in chooseAction and at floor arrival.
//   - Clears orders and lights in the same direction as the elevator.
func clearAtCurrentFloor(elevator *types.ElevState) {
	elevator.Orders[config.NodeID][elevator.Floor][types.BT_Cab] = false
	elevio.SetButtonLamp(types.BT_Cab, elevator.Floor, false)
	shouldClear := OrdersToClearHere(elevator)
	for btn := range config.NumButtons {
		if shouldClear[btn] {
			elevator.Orders[config.NodeID][elevator.Floor][btn] = false
			elevio.SetButtonLamp(types.ButtonType(btn), elevator.Floor, false)
		}
	}
}

// Helper functions for orders.go

func hasOrders(elevator *types.ElevState, startFloor int, endFloor int) bool {
	for floor := startFloor; floor < endFloor; floor++ {
		for btn := range config.NumButtons {
			if elevator.Orders[config.NodeID][floor][btn] {
				return true
			}
		}
	}
	return false
}

func ordersAbove(elevator *types.ElevState) bool {
	return hasOrders(elevator, elevator.Floor+1, config.NumFloors)
}

func ordersBelow(elevator *types.ElevState) bool {
	return hasOrders(elevator, 0, elevator.Floor)
}

func ordersHere(elevator *types.ElevState) bool {
	return hasOrders(elevator, elevator.Floor, elevator.Floor+1)
}
