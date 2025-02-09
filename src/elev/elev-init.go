package elev

import (
	"log/slog"
	"main/lib/driver-go/elevio"
	"main/src/config"
	"main/src/types"
	"os"
	"time"
)

// InitLogger sets up global logging configuration with compact time format
func InitLogger() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.Format("15:04:05"))
				}
			}
			return a
		},
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func InitElevator(nodeID int) *types.Elevator {
	elevator := &types.Elevator{
		NodeID:    nodeID,
		Dir:       types.MD_Stop,
		Orders:    [config.N_FLOORS][config.N_BUTTONS]int{},
		Behaviour: types.Idle,
	}
	slog.Debug("Elevator initialized", "nodeID", nodeID)
	return elevator
}

func InitSystem(nodeID int) *types.Elevator {
	elevator := InitElevator(nodeID)
	if elevio.GetFloor() == -1 {
		elevator.Dir = types.MD_Down
		if err := moveElev(elevator); err == nil {
			slog.Debug("Moving to known floor", "direction", "down")
		} else {
			slog.Error("Failed to start initial movement", "error", err)
		}
	}
	return elevator
}
