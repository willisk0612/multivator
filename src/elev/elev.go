package elev

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"multivator/lib/driver-go/elevio"
	"multivator/src/config"
	"multivator/src/timer"
	"multivator/src/types"
)

func InitHW() (chan types.ButtonEvent, chan int, chan bool, chan bool) {
	drv_buttons := make(chan types.ButtonEvent, 10)
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
func InitLogger(nodeID int) {
	logFile, err := os.OpenFile(fmt.Sprintf("node%d.log", nodeID), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		panic(err)
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)

	handler := slog.NewTextHandler(multiWriter, &slog.HandlerOptions{
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
					a.Value = slog.StringValue(fmt.Sprintf("%s:%d", file, source.Line))
				}
			}
			return a
		},
	})

	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func InitElevState(nodeID int) *types.ElevState {
	elevator := &types.ElevState{
		NodeID:    nodeID,
		Dir:       types.MD_Stop,
		Orders:    make([][][]bool, config.NumPeers),
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
	floor := elevio.GetFloor()
	if floor == -1 {
		slog.Debug("Moving down to find first floor")
		elevio.SetMotorDirection(types.MD_Down)
		for {
			floor = elevio.GetFloor()
			if floor != -1 {
				elevio.SetMotorDirection(types.MD_Stop)
				elevator.Floor = floor
				break
			}
		}
	}
}

// Move elevator to floor, set order and lamp
func MoveElevator(elevator *types.ElevState, btn types.ButtonEvent, timerAction chan timer.TimerAction) {
	slog.Debug("Moving elevator", "from", elevator.Floor, " to ", btn.Floor)
	if elevator.Floor == btn.Floor {
		slog.Debug("Elevator already at floor")
		OpenDoor(elevator, timerAction)
	} else {
		elevator.Orders[elevator.NodeID][btn.Floor][btn.Button] = true
		elevio.SetButtonLamp(btn.Button, btn.Floor, true)
		elevator.Dir = chooseDirIdle(elevator).Dir
		elevator.CurrentBtnEvent = btn
		moveMotor(elevator)
	}
}

// Move motor with safety check to avoid moving while door is open.
func moveMotor(elevator *types.ElevState) {
	if elevator.Behaviour == types.DoorOpen {
		slog.Debug("Cannot move while door is open")
		return
	}
	elevator.Behaviour = types.Moving
	elevio.SetMotorDirection(elevator.Dir)
}
