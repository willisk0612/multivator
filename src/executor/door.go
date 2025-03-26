package executor

import (
	"time"

	"multivator/lib/driver/elevio"
	"multivator/src/config"
	"multivator/src/types"
)

type DoorTimer struct {
	timer     *time.Timer
	timeoutCh chan bool
}

// openDoor modifies elevator state, sets door lamp and starts the door timer
func openDoor(elevator *types.ElevState, doorTimer DoorTimer) {
	elevator.Behaviour = types.DoorOpen
	elevio.SetDoorOpenLamp(true)
	if doorTimer.timer != nil {
		(doorTimer.timer).Reset(config.DoorOpenDuration)
	} else {
		doorTimer.timer = time.AfterFunc(config.DoorOpenDuration, func() {
			doorTimer.timeoutCh <- true
		})
	}
}
