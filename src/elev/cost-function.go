package elev

import (
	"log/slog"
	"main/src/config"
	"main/src/types"
	"time"
)

func TimeToServedOrder(btnEvent types.ButtonEvent, elevator types.Elevator) time.Duration {
	elevCopy := DeepCopy(elevator)
	elevCopy.Orders[btnEvent.Floor][btnEvent.Button] = true

	duration := time.Duration(0)

	switch elevCopy.Behaviour {
	case types.Idle:
		if btnEvent.Button == types.BT_HallDown && elevCopy.Floor >= btnEvent.Floor {
			elevCopy.Dir = types.MD_Down
		} else if btnEvent.Button == types.BT_HallUp && elevCopy.Floor <= btnEvent.Floor {
			elevCopy.Dir = types.MD_UP
		} else {
			elevCopy.Dir = chooseDirInit(&elevCopy).Dir
		}
		slog.Debug("CostCalc (Idle): initial direction", "floor", elevCopy.Floor, "direction", elevCopy.Dir)
		if elevCopy.Dir == types.MD_Stop {
			slog.Debug("CostCalc: no movement needed", "duration", duration)
			return duration
		}
	case types.Moving:
		// Remove premature floor update to simulate from actual current floor.
		slog.Debug("CostCalc (Moving): starting from current floor", "floor", elevCopy.Floor, "direction", elevCopy.Dir, "duration", duration)
	}

	for {
		if elevCopy.Floor < 0 || elevCopy.Floor >= config.N_FLOORS {
			slog.Error("CostCalc: out-of-range floor", "floor", elevCopy.Floor)
			return duration
		}

		if shouldStop(&elevCopy, btnEvent.Button) {
			if elevCopy.Floor == btnEvent.Floor && elevCopy.Orders[btnEvent.Floor][btnEvent.Button] {
				slog.Debug("CostCalc: reached target floor", "finalFloor", elevCopy.Floor, "direction", elevCopy.Dir, "totalDuration", duration+config.DOOR_OPEN_DURATION)
				return duration + config.DOOR_OPEN_DURATION
			}

			elevCopy.Orders[elevCopy.Floor][btnEvent.Button] = false
			duration += config.DOOR_OPEN_DURATION
			slog.Debug("CostCalc: door open added", "floor", elevCopy.Floor, "newDuration", duration)

			elevCopy.Dir = chooseDirInit(&elevCopy).Dir
			slog.Debug("CostCalc: new direction re-computed", "floor", elevCopy.Floor, "direction", elevCopy.Dir)
			if elevCopy.Dir == types.MD_Stop {
				slog.Debug("CostCalc: no further movement needed", "finalDuration", duration)
				return duration
			}
		}

		elevCopy.Floor += int(elevCopy.Dir)
		duration += config.TRAVEL_DURATION
	}
}

func DeepCopy(elevator types.Elevator) types.Elevator {
	copy := types.Elevator{
		NodeID:    elevator.NodeID,
		Floor:     elevator.Floor,
		Dir:       elevator.Dir,
		Behaviour: elevator.Behaviour,
		Orders:    [config.N_FLOORS][config.N_BUTTONS]bool{},
	}
	return copy
}
