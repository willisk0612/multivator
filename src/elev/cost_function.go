// Cost function is calculated and broadcasted every time a button event is stored in the local elevator.
package elev

import (
	"context"
	"multivator/src/config"
	"multivator/src/types"
	"time"
)

// TimeToServedOrder calculates the time it takes for the elevator to serve an order. Its calculations are based on distance if the elevator is still, and fsm logic if the elevator is moving.
func (elevMgr *ElevStateMgr) TimeToServedOrder(btnEvent types.ButtonEvent) time.Duration {
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	costCh := make(chan time.Duration, 1)

	go func() {
		elevator := elevMgr.GetState()
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
				if simElev.shouldStop() {
					shouldClear := simElev.clearOrdersAtFloor()
					if simElev.Floor == btnEvent.Floor && shouldClear[btnEvent.Button] {
						costCh <- duration
						return
					}
					duration += config.DoorOpenDuration
					simElev.Dir = elevMgr.chooseDirInit().Dir
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
