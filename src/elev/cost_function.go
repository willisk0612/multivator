package elev

import (
	"context"
	"main/src/config"
	"main/src/types"
	"time"
)

// TimeToServedOrder calculates the time it takes for the elevator to serve an order. It runs concurrently with a context based timeout.
func TimeToServedOrder(elevMgr *types.ElevatorManager, btnEvent types.ButtonEvent) time.Duration {
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	costCh := make(chan time.Duration, 1)

	go func() {
		elevator := GetElevState(elevMgr)
		simElev := *elevator
		simElev.Orders[btnEvent.Floor][btnEvent.Button] = true

		baseCost := time.Duration(0)
		if simElev.Behaviour == types.DoorOpen {
			baseCost += config.DoorOpenDuration
			simElev.Behaviour = types.Idle
		}

		// For idle elevator, calculate direct path cost
		if simElev.Behaviour == types.Idle {
			distance := abs(simElev.Floor - btnEvent.Floor)
			select {
			case costCh <- baseCost + (time.Duration(distance) * config.TravelDuration):
			case <-ctx.Done():
				return
			}
			return
		}

		// For moving elevator, simulate the time using the existing fsm logic
		duration := baseCost
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if simElev.Floor < 0 || simElev.Floor >= config.NumFloors {
					simElev.Dir = -simElev.Dir
				}
				if shouldStop(&simElev) {
					shouldClear := clearOrdersAtFloor(&simElev)
					if simElev.Floor == btnEvent.Floor && shouldClear[btnEvent.Button] {
						costCh <- duration
						return
					}
					duration += config.DoorOpenDuration
					simElev.Dir = chooseDirInit(elevMgr).Dir
				}

				simElev.Floor += int(simElev.Dir)
				duration += config.TravelDuration
			}
		}
	}()

	select {
	case cost := <-costCh:
		return cost
	case <-ctx.Done():
		return 24 * time.Hour // Return max cost on timeout
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
