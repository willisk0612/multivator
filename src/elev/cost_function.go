package elev

import (
	"log/slog"
	"time"

	"multivator/src/config"
	"multivator/src/types"

	"github.com/tiendc/go-deepcopy"
)

// Creates a simulation to calculate the time based on:
// - State: penalize for moving, reward for door open
// - Accumulate time for each floor passed
func TimeToServeOrder(elevator *types.ElevState, btnEvent types.ButtonEvent) time.Duration {
	simElev := new(types.ElevState)
	err := deepcopy.Copy(simElev, elevator)
	if err != nil {
		slog.Error("Failed to copy elevator state", "Error", err)
		return time.Duration(100) // Return a high value
	}
	simElev.Orders[simElev.NodeID][btnEvent.Floor][btnEvent.Button] = true
	duration := time.Duration(0)

	switch simElev.Behaviour {
	case types.Idle:
		simElev.Dir = chooseDirection(simElev).Dir
		if simElev.Dir == types.MD_Stop {
			return duration
		}
	case types.Moving:
		duration += config.TravelDuration / 2
		simElev.Floor += int(simElev.Dir)
	case types.DoorOpen:
		duration -= config.DoorOpenDuration / 2
	}

	for {
		if shouldStopHere(simElev) {
			shouldClear := ordersToClearHere(simElev)

			if btnEvent.Floor == simElev.Floor && shouldClear[btnEvent.Button] {
				if duration < 0 {
					duration = 0
				}
				return duration
			}

			for btn, clear := range shouldClear {
				if clear {
					simElev.Orders[simElev.NodeID][simElev.Floor][btn] = false
				}
			}
			duration += config.DoorOpenDuration
			simElev.Dir = chooseDirection(simElev).Dir
		}

		simElev.Floor += int(simElev.Dir)
		duration += config.TravelDuration
	}
}
