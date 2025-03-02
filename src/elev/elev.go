package elev

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"multivator/lib/driver-go/elevio"
	"multivator/src/config"
	"multivator/src/types"
)

func InitHW() (chan types.ButtonEvent, chan int, chan bool, chan bool) {
	drv_buttons := make(chan types.ButtonEvent)
	drv_floors := make(chan int)
	drv_obstr := make(chan bool)
	drv_stop := make(chan bool)

	go elevio.PollButtons(drv_buttons)
	go elevio.PollFloorSensor(drv_floors)
	go elevio.PollObstructionSwitch(drv_obstr)
	go elevio.PollStopButton(drv_stop)

	return drv_buttons, drv_floors, drv_obstr, drv_stop
}

// InitLogger sets up global logging configuration with compact time format
func InitLogger() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.Format("15:04:05"))
				}
			}

			if a.Key == slog.SourceKey {
				if source, ok := a.Value.Any().(*slog.Source); ok {
					file := source.File
					if lastSlash := strings.LastIndexByte(file, '/'); lastSlash >= 0 {
						file = file[lastSlash+1:]
					}
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
	// Ensure at least one entry
	maxNodes := nodeID + 1
	if maxNodes < 1 {
		maxNodes = 1
	}
	elevator := &types.ElevState{
		NodeID:    nodeID,
		Dir:       types.MD_Stop,
		Orders:    make([][][]bool, maxNodes),
		Behaviour: types.Idle,
	}

	for nodeIndex := range elevator.Orders {
		elevator.Orders[nodeIndex] = make([][]bool, config.NumFloors)
		for floorNum := range elevator.Orders[nodeIndex] {
			elevator.Orders[nodeIndex][floorNum] = make([]bool, config.NumButtons)
		}
	}

	return elevator
}

// InitElevPos moves down until the first floor is detected, then stops
func InitElevPos(elevator *types.ElevState) {
	// If floor sensor returns -1, keep moving down until we reach the first detected floor.
	if elevio.GetFloor() == -1 {
		slog.Debug("Moving down to find first floor")
		elevio.SetMotorDirection(types.MD_Down)
		for {
			floor := elevio.GetFloor()
			if floor != -1 {
				elevio.SetMotorDirection(types.MD_Stop)
				elevator.Floor = floor
				break
			}
		}
	}
}
