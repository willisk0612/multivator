package elev

import (
	"time"

	"multivator/src/config"
	"multivator/src/types"

	"github.com/tiendc/go-deepcopy"
)

// Cost uses several factors to determine the cost (in seconds) of an elevator taking a hall order.
func Cost(elevator *types.ElevState, btnEvent types.ButtonEvent) time.Duration {
	simElev := new(types.ElevState)
	if err := deepcopy.Copy(simElev, elevator); err != nil {
		panic(err)
	}

	// Base cost: distance to target floor
	distance := abs(elevator.Floor - btnEvent.Floor)
	cost := time.Duration(distance) * 2 * time.Second

	// Add penalty for door being open
	if simElev.Behaviour == types.DoorOpen {
		cost += config.DoorOpenDuration * time.Second
	}

	// Add significant penalty for each existing orders
	orderCount := 0
	for floor := range simElev.Orders[elevator.NodeID] {
		for button := range config.NumButtons {
			if simElev.Orders[elevator.NodeID][floor][button] {
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

	simElev.Orders[elevator.NodeID][btnEvent.Floor][btnEvent.Button] = true

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
