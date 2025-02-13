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
		Level: slog.LevelDebug,
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
		Orders:    [config.N_FLOORS][config.N_BUTTONS]bool{},
		Behaviour: types.Idle,
	}
	slog.Debug("Elevator initialized", "nodeID", nodeID)
	return elevator
}

func InitSystem(nodeID int) *types.Elevator {
	elevator := InitElevator(nodeID)
	// If floor sensor returns -1, keep moving down until we reach the first detected floor.
	if elevio.GetFloor() == -1 {
		elevio.SetMotorDirection(types.MD_Down)
		slog.Debug("InitSystem: No floor detected, moving down to first floor sensor")
		for {
			floor := elevio.GetFloor()
			if floor != -1 {
				elevio.SetMotorDirection(types.MD_Stop)
				elevator.Floor = floor
				slog.Debug("InitSystem: Floor sensor triggered", "floor", floor)
				break
			}
		}
	}
	return elevator
}
