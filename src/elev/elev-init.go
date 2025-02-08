package elev

import (
	"log/slog"
	"main/lib/driver-go/elevio"
	"main/src/config"
	"main/src/types"
	"os"
)

// InitLogger sets up global logging configuration
func InitLogger() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func InitElevator(nodeID int) *types.Elevator {
	elevator := &types.Elevator{
		NodeID:    nodeID,
		Dir:       elevio.MD_Stop,
		Orders:    [config.N_FLOORS][config.N_BUTTONS]int{},
		Behaviour: elevio.Idle,
	}
	slog.Debug("Elevator initialized", "nodeID", nodeID)
	return elevator
}

func InitSystem(nodeID int) *types.Elevator {
	elevator := InitElevator(nodeID)
	if elevio.GetFloor() == -1 {
		elevator.Dir = elevio.MD_Down
		if err := moveElev(elevator); err == nil {
			slog.Debug("Moving to known floor", "direction", "down")
		} else {
			slog.Error("Failed to start initial movement", "error", err)
		}
	}
	return elevator
}
