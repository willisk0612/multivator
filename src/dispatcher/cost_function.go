package dispatcher

import (
	"time"

	"multivator/src/config"
	"multivator/src/executor"
	"multivator/src/types"
)

// Creates a simulation to calculate the time based on:
// - State: penalize for moving, reward for door open
// - Accumulate time for each floor passed
func timeToServeOrder(elevator types.ElevState, btnEvent types.ButtonEvent) time.Duration {
	elevator.Orders[config.NodeID][btnEvent.Floor][btnEvent.Button] = true
	duration := time.Duration(0)

	switch elevator.Behaviour {
	case types.Idle:
		elevator.Dir = executor.ChooseDirection(elevator).Dir
		if elevator.Dir == types.MD_Stop {
			return duration
		}
	case types.Moving:
		duration += config.TravelDuration / 2
		elevator.Floor += int(elevator.Dir)
	case types.DoorOpen:
		duration -= config.DoorOpenDuration / 2
	}

	for {
		if executor.ShouldStopHere(elevator) {
			shouldClear := executor.OrdersToClearHere(elevator)

			if btnEvent.Floor == elevator.Floor && shouldClear[btnEvent.Button] {
				if duration < 0 {
					duration = 0
				}
				return duration
			}

			for btn, clear := range shouldClear {
				if clear {
					elevator.Orders[config.NodeID][elevator.Floor][btn] = false
				}
			}
			duration += config.DoorOpenDuration
			elevator.Dir = executor.ChooseDirection(elevator).Dir
		}

		elevator.Floor += int(elevator.Dir)
		duration += config.TravelDuration
	}
}
