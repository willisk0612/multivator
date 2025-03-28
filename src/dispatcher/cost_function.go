package dispatcher

import (
	"time"

	"multivator/src/config"
	"multivator/src/executor"
	"multivator/src/types"
	"multivator/src/utils"
)

// timeToserveOrder is called before a bid is stored in bidMap
//   - returns a high duration if the elevator is obstructed
//   - adjusts the duration based on the next elevator action
//   - adds time penalty for existing orders
//   - uses recursive calls, and accumulates the duration for each floor
func timeToServeOrder(elevator types.ElevState, btnEvent types.HallOrder) time.Duration {
	if elevator.Obstructed || elevator.IsStuck {
		return 100 * time.Second
	}

	var duration time.Duration
	elevator.Orders[config.NodeID][btnEvent.Floor][btnEvent.Button] = true

	// Adjust duration based on the next elevator action
	switch elevator.Behaviour {
	case types.Idle:
		elevator.Dir = executor.ChooseDirection(&elevator).Dir
		if elevator.Dir == types.MD_Stop {
			return duration
		}
	case types.Moving:
		duration += config.TravelDuration/2 + config.DoorOpenDuration
		elevator.Floor += int(elevator.Dir)
	case types.DoorOpen:
		duration += config.DoorOpenDuration / 2
	}

	// Recursively add travel time and door open time for each floor
	for {
		if executor.ShouldStopHere(&elevator) {
			shouldClear := executor.OrdersToClearHere(&elevator)

			if btnEvent.Floor == elevator.Floor && shouldClear[btnEvent.Button] {
				// Check if we still have active orders that are not between elevator and target floor
				utils.ForEachOrder(elevator.Orders, func(node, floor, btn int) {
					if node == config.NodeID && elevator.Orders[node][floor][btn] && elevator.Floor != floor {
						duration += config.DoorOpenDuration
						duration += time.Duration(elevator.Floor-floor).Abs() * config.TravelDuration
					}
				})
				return duration
			}

			// Determine if the elevator should clear the orders at the current floor
			for btn, clear := range shouldClear {
				if clear {
					elevator.Orders[config.NodeID][elevator.Floor][btn] = false
				}
			}
			duration += config.DoorOpenDuration
			elevator.Dir = executor.ChooseDirection(&elevator).Dir
		}

		elevator.Floor += int(elevator.Dir)
		duration += config.TravelDuration
	}
}
