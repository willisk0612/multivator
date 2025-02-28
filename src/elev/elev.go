package elev

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"multivator/lib/driver-go/elevio"
	"multivator/src/config"
	"multivator/src/timer"
	"multivator/src/types"
)

type ElevChannels struct {
	Drv_buttons      chan types.ButtonEvent
	Drv_floors       chan int
	Drv_obstr        chan bool
	Dv_stop          chan bool
	DoorTimerAction  chan timer.TimerAction
	DoorTimerTimeout chan bool
	HallOrderMsg        chan types.Message
}

func InitChannels() ElevChannels {
	return ElevChannels{
		Drv_buttons:      make(chan types.ButtonEvent),
		Drv_floors:       make(chan int),
		Drv_obstr:        make(chan bool),
		Dv_stop:          make(chan bool),
		DoorTimerAction:  make(chan timer.TimerAction),
		DoorTimerTimeout: make(chan bool),
	}
}

// InitLogger sets up global logging configuration with compact time format
func InitLogger() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		// Enable source code location
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Format timestamp
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.Format("15:04:05"))
				}
			}

			// Format source information as file:line
			if a.Key == slog.SourceKey {
				if source, ok := a.Value.Any().(*slog.Source); ok {
					// Extract just filename (not full path)
					file := source.File
					if lastSlash := strings.LastIndexByte(file, '/'); lastSlash >= 0 {
						file = file[lastSlash+1:]
					}

					// Format as file:line
					sourceInfo := fmt.Sprintf("%s:%d", file, source.Line)
					a.Value = slog.StringValue(sourceInfo)
				}
			}

			return a
		},
	})

	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func InitElevState(nodeID int) *types.ElevState {
	elevator := &ElevState{
		NodeID:    nodeID,
		Dir:       types.MD_Stop,
		Orders:    [config.NumFloors][config.NumButtons]bool{},
		Behaviour: types.Idle,
		CurrentBtnEvent: types.ButtonEvent{
			Floor:  -1,           // Initialize to invalid floor
			Button: types.BT_Cab, // Default to cab button
		},
	}
	slog.Debug("Elevator initialized", "nodeID", nodeID)
	return elevator
}

// InitElevPos initializes the elevator by moving it down to the first detected floor.
func InitElevPos(nodeID int) {
	elevator := InitElevState(nodeID)
	// If floor sensor returns -1, keep moving down until we reach the first detected floor.
	if elevio.GetFloor() == -1 {
		elevio.SetMotorDirection(types.MD_Down)
		slog.Debug("InitElevator: No floor detected, moving down to first floor sensor")
		for {
			floor := elevio.GetFloor()
			if floor != -1 {
				elevio.SetMotorDirection(types.MD_Stop)
				elevator.Floor = floor
				slog.Debug("InitElevator: Floor sensor triggered", "floor", floor)
				break
			}
		}
	}
}
