package elev

import (
	"log/slog"
	"main/src/config"
	"main/src/types"
	"time"
)

func TimeToServedOrder(btnEvent types.ButtonEvent, elevCopy types.Elevator) time.Duration {
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
		slog.Debug("CostCalc (Moving): starting from current floor", "floor", elevCopy.Floor, "direction", elevCopy.Dir, "duration", duration)
	}

	for {
		if shouldStop(&elevCopy) {
			shouldClear := clearOrdersAtFloor(&elevCopy)
			if elevCopy.Floor == btnEvent.Floor && shouldClear[btnEvent.Button] {
				if duration < 0 {
					duration = 0
				}
				return duration
			}
			for b := 0; b < config.N_BUTTONS; b++ {
				if shouldClear[b] {
					elevCopy.Orders[elevCopy.Floor][b] = false
				}
			}
			duration += time.Duration(config.DOOR_OPEN_DURATION)
			elevCopy.Dir = chooseDirInit(&elevCopy).Dir
		}
		elevCopy.Floor += int(elevCopy.Dir)
		duration += time.Duration(config.TRAVEL_DURATION)
	}
}
