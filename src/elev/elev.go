package elev

import (
	"log/slog"
	"multivator/lib/driver-go/elevio"
	"multivator/src/config"
	"multivator/src/timer"
	"multivator/src/types"
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

func InitElevState(nodeID int) *ElevState {
	elevator := &ElevState{
		NodeID:    nodeID,
		Dir:       types.MD_Stop,
		Orders:    [config.NumFloors][config.NumButtons]bool{},
		Behaviour: types.Idle,
	}
	slog.Debug("Elevator initialized", "nodeID", nodeID)
	return elevator
}

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

// Run starts the elevator subsystem and listens for events from the network subsystem.
func Run(elevMgr *ElevStateMgr, nodeID int, elevInMsgCh chan types.Message, elevOutMsgCh chan types.Message) {
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

	InitElevPos(nodeID)

	for {
		select {
		case btn := <-drv_buttons:
			if btn.Button == types.BT_Cab || elevio.GetFloor() == -1 {
				elevMgr.MoveElevator(btn, doorTimerAction)
			} else {
				slog.Debug("Hall button press discovered in elevator. Sending to network")
				msg := types.Message{
					Type:  types.LocalHallOrder,
					Event: btn,
				}
				elevOutMsgCh <- msg // Send hall order to network
			}
		case floor := <-drv_floors:
			elevMgr.HandleFloorArrival(floor, doorTimerAction)
		case obstruction := <-drv_obstr:
			elevMgr.HandleObstruction(obstruction, doorTimerAction)
		case <- drv_stop:
			elevMgr.HandleStop()
		case <- doorTimerTimeout:
			elevMgr.HandleDoorTimeout(doorTimerAction)
		case msg := <- elevInMsgCh:
			slog.Debug("Received message in elevator subsystem")
			if msg.Type == types.LocalHallAssignment {
				elevMgr.MoveElevator(msg.Event, doorTimerAction)
			}
		}

	}
}
