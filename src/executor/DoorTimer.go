package executor

import (
	"multivator/lib/driver/elevio"
	"multivator/src/config"
	"time"
)

// DoorTimer
// `Start()` starts or restarts the door timer
// When the timer expires, it sends a signal through the timeout channel.
type DoorTimer struct {
	timer          *time.Timer
	timeoutChannel chan bool
}

func (doorTimer *DoorTimer) Start() {
	elevio.SetDoorOpenLamp(true)
	if doorTimer.timer != nil {
		(doorTimer.timer).Reset(config.DoorOpenDuration)
	} else {
		doorTimer.timer = time.AfterFunc(config.DoorOpenDuration, func() {
			doorTimer.timeoutChannel <- true
		})
	}
}
