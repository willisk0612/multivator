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

// Run starts the elevator subsystem and listens for events from the network subsystem.
func Run(elevMgr *ElevStateMgr, nodeID int, elevInMsgCh chan types.Message, elevOutMsgCh chan types.Message, lightElevMsgCh chan types.Message) {
	// Store the light message channel in the elevator manager
	elevMgr.SetLightMsgChannel(lightElevMsgCh)

	drv_buttons := make(chan types.ButtonEvent)
	drv_floors := make(chan int)
	drv_obstr := make(chan bool)
	drv_stop := make(chan bool)

	doorTimerDuration := time.NewTimer(config.DoorOpenDuration)
	doorTimerTimeout := make(chan bool, 1)
	doorTimerAction := make(chan timer.TimerAction, 1)

	go elevio.PollButtons(drv_buttons)
	go elevio.PollFloorSensor(drv_floors)
	go elevio.PollObstructionSwitch(drv_obstr)
	go elevio.PollStopButton(drv_stop)
	go timer.Timer(doorTimerDuration, doorTimerTimeout, doorTimerAction)

	InitLogger()
	InitElevPos(nodeID)

	for {
		select {
		case btn := <-drv_buttons:
			if btn.Button == types.BT_Cab || elevio.GetFloor() == -1 {
				elevMgr.MoveElevator(btn, doorTimerAction)
			} else {
				slog.Debug("Hall button press discovered in elevator. Sending to network")
				msg := types.Message{
					Type:     types.LocalHallOrder,
					Event:    btn,
					SenderID: elevMgr.GetState().NodeID,
				}
				elevOutMsgCh <- msg // Send hall order to network
				slog.Debug("Hall message sent to network", "floor", btn.Floor, "button", btn.Button, "nodeID", elevMgr.GetState().NodeID)
			}
		case floor := <-drv_floors:
			elevMgr.HandleFloorArrival(floor, doorTimerAction)
		case obstruction := <-drv_obstr:
			elevMgr.HandleObstruction(obstruction, doorTimerAction)
		case <-drv_stop:
			elevMgr.HandleStop()
		case <-doorTimerTimeout:
			elevMgr.HandleDoorTimeout(doorTimerAction)
		case msg := <-elevInMsgCh:
			slog.Debug("Received message in elevator subsystem", "type", msg.Type, "event", msg.Event)
			if msg.Type == types.LocalHallAssignment {
				slog.Info("Elevator received hall assignment - moving elevator to floor",
					"floor", msg.Event.Floor,
					"button", msg.Event.Button)
				elevMgr.MoveElevator(msg.Event, doorTimerAction)
			}
		}
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

func InitElevState(nodeID int) *ElevState {
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
