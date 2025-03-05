package elev

import (
	"log/slog"
	"time"

	"multivator/src/config"
	"multivator/src/types"

	"github.com/tiendc/go-deepcopy"
)

func TimeToServeOrder(elevator *types.ElevState, btnEvent types.ButtonEvent) time.Duration {
	simElev := new(types.ElevState)
	err := deepcopy.Copy(simElev, elevator)
	if err != nil {
		slog.Error("Failed to copy elevator state", "Error", err)
	}
	simElev.Orders[simElev.NodeID][btnEvent.Floor][btnEvent.Button] = true
	// Log for debugging purposes
	for nodeID := range simElev.Orders {
		for floor := range simElev.Orders[nodeID] {
			for btn, isActive := range simElev.Orders[nodeID][floor] {
				if isActive {
					btnEvent := types.ButtonEvent{
						Floor:  floor,
						Button: types.ButtonType(btn),
					}
					slog.Debug("Active order", "nodeID", nodeID, "button", FormatBtnEvent(btnEvent))
				}
			}
		}
	}

	duration := time.Duration(0)

	switch simElev.Behaviour {
	case types.Idle:
		simElev.Dir = chooseDirection(simElev).Dir
		if simElev.Dir == types.MD_Stop {
			// Elevator is already at the floor
			slog.Debug("FINAL COST", "Duration", duration)
			return duration
		}
	case types.Moving:
		duration += config.TravelDuration / 2
		simElev.Floor += int(simElev.Dir)
	case types.DoorOpen:
		duration -= config.DoorOpenDuration / 2
	}

	for {
		if shouldStop(simElev) {
			shouldClear := ordersToClear(simElev)

			if btnEvent.Floor == simElev.Floor && shouldClear[btnEvent.Button] {
				slog.Info("FINAL COST", "Duration", duration)
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
