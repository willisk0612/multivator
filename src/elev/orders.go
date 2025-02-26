// Contains order related helper functions for the elevator module.
package elev

import (
	"log/slog"
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

// Clears cab order and current direction hall order and lamp. It modifies the actual elevator state via the manager.
func (elevMgr *ElevStateMgr) clearFloor() {
	elevator := elevMgr.GetState()

	// If we have a valid current button event and its floor matches the current floor,
	// clear that specific button first
	if elevator.CurrentBtnEvent.Floor == elevator.Floor && elevator.CurrentBtnEvent.Floor >= 0 {
		slog.Debug("Clearing specific button event",
			"floor", elevator.Floor,
			"button", elevator.CurrentBtnEvent.Button)

		// Clear the specific order for the current button event
		elevMgr.UpdateOrders(func(orders *[config.NumFloors][config.NumButtons]bool) {
			orders[elevator.Floor][elevator.CurrentBtnEvent.Button] = false
		})

		// Handle light based on button type
		if elevator.CurrentBtnEvent.Button == types.BT_Cab {
			// Cab lights are handled locally
			elevio.SetButtonLamp(elevator.CurrentBtnEvent.Button, elevator.Floor, false)
		} else {
			// Hall lights are handled by the light manager - broadcast the light off message
			elevMgr.lightMsgCh <- types.Message{
				Type:     types.LocalLightOff,
				Event:    elevator.CurrentBtnEvent,
				SenderID: elevator.NodeID,
			}
			slog.Debug("Sent light off message for specific hall button",
				"floor", elevator.Floor,
				"button", elevator.CurrentBtnEvent.Button)
		}
	}

	// Default behavior for clearing multiple orders at current floor
	shouldClear := elevator.ordersToClear()

	elevMgr.UpdateOrders(func(orders *[config.NumFloors][config.NumButtons]bool) {
		// Clear cab order
		orders[elevator.Floor][types.BT_Cab] = false
		elevio.SetButtonLamp(types.BT_Cab, elevator.Floor, false)

		// Clear hall orders based on shouldClear
		for b := types.ButtonType(0); b < config.NumButtons; b++ {
			if shouldClear[b] && b != types.BT_Cab {
				// Clear the order
				orders[elevator.Floor][b] = false

				// Only send network messages for hall button types
				if b == types.BT_HallUp || b == types.BT_HallDown {
					// Send light off message through the light manager
					elevMgr.lightMsgCh <- types.Message{
						Type: types.LocalLightOff,
						Event: types.ButtonEvent{
							Floor:  elevator.Floor,
							Button: b,
						},
						SenderID: elevator.NodeID,
					}
					slog.Debug("Clearing hall order, sending light off broadcast",
						"floor", elevator.Floor,
						"button", b)
				}
			}
		}
	})
}

// Helper function for clearFloor and TimeToServedOrder. It determines which orders to clear at the current floor.
func (elevator *ElevState) ordersToClear() [config.NumButtons]bool {
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

func (elevator *ElevState) countOrders(startFloor int, endFloor int) (result int) {
	// Ensure floor range is valid
	if startFloor < 0 {
		startFloor = 0
	}
	if endFloor > config.NumFloors {
		endFloor = config.NumFloors
	}

	// Count orders for each floor and button type
	for floor := startFloor; floor < endFloor; floor++ {
		for btn := types.ButtonType(0); btn < config.NumButtons; btn++ {
			if elevator.Orders[floor][btn] {
				result++
				slog.Debug("Found order in count", "floor", floor, "button", btn)
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
