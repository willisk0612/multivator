package elev

import (
	"main/src/config"
	"main/src/types"
	"time"
)

// TimeToServedOrder calculates the time it takes for the elevator to serve an order, given the previous orders and the current elevator state.
func TimeToServedOrder(btnEvent types.ButtonEvent, elevCopy types.Elevator) time.Duration {
	// Add the new order to a copy of the current orders
	var orders [config.NumFloors][config.NumButtons]bool
	for i := range orders {
		copy(orders[i][:], elevCopy.Orders[i][:])
	}
	elevCopy.Orders = orders
	elevCopy.Orders[btnEvent.Floor][btnEvent.Button] = true

	// If previous no orders, use distance to calculate time
	if elevCopy.Behaviour == types.Idle {
		distance := abs(elevCopy.Floor - btnEvent.Floor)
		return time.Duration(distance) * config.TravelDuration
	}

	// If the elevator had previous orders, calculate time to serve all orders
	duration := time.Duration(0)
	if elevCopy.Behaviour == types.DoorOpen {
		duration += config.DoorOpenDuration
	}
	for {
		if shouldStop(&elevCopy) {
			shouldClear := clearOrdersAtFloor(&elevCopy)
			if elevCopy.Floor == btnEvent.Floor && shouldClear[btnEvent.Button] {
				return duration
			}
			// Clear served orders and update direction
			for b := range config.NumButtons {
				if shouldClear[b] {
					elevCopy.Orders[elevCopy.Floor][b] = false
				}
			}
			duration += config.DoorOpenDuration
			elevCopy.Dir = chooseDirInit(&elevCopy).Dir
		}

		elevCopy.Floor += int(elevCopy.Dir)
		duration += config.TravelDuration
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
