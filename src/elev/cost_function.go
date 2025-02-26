// Cost function is calculated and broadcasted every time a button event is stored in the local elevator.
package elev

import (
	"time"

	"multivator/src/types"
)

func (elevMgr *ElevStateMgr) TimeToServedOrder(btnEvent types.ButtonEvent) time.Duration {
	elevator := elevMgr.GetState()

	// Base cost: distance to target floor
	distance := abs(elevator.Floor - btnEvent.Floor)
	cost := time.Duration(distance) * 2 * time.Second

	// Add penalty for door being open
	if elevator.Behaviour == types.DoorOpen {
		cost += 3 * time.Second
	}

	// Add significant penalty for each existing order
	orderCount := 0
	for f := range elevator.Orders {
		for b := range elevator.Orders[f] {
			if elevator.Orders[f][b] {
				orderCount++
			}
		}
	}
	cost += time.Duration(orderCount) * 4 * time.Second

	// Add penalty if elevator needs to change direction
	if elevator.Dir != types.MD_Stop {
		targetDir := getDirection(elevator.Floor, btnEvent.Floor)
		if targetDir != elevator.Dir {
			cost += 4 * time.Second
		}
	}

	return cost
}

func getDirection(from, to int) types.MotorDirection {
	if from < to {
		return types.MD_Up
	}
	if from > to {
		return types.MD_Down
	}
	return types.MD_Stop
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
