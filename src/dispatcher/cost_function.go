package dispatcher

import (
	"time"

	"multivator/src/config"
	"multivator/src/executor"
	"multivator/src/types"
)

func timeToServeOrder(elevator types.ElevState, btnEvent types.HallOrder) time.Duration {
	elevator.Orders[config.NodeID][btnEvent.Floor][btnEvent.Button] = true
	var duration time.Duration
	if elevator.Obstructed {
		duration = 100 * time.Second
		return duration
	}

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
