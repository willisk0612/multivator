package timer

import (
	"time"

	"multivator/src/config"
)

type TimerAction int

const (
	Start TimerAction = iota
	Stop
)

func Init() (chan bool, chan TimerAction) {
	doorTimerTimeout := make(chan bool)
	doorTimerAction := make(chan TimerAction)
	doorTimerDuration := time.NewTimer(config.DoorOpenDuration)
	go runTimer(doorTimerDuration, doorTimerTimeout, doorTimerAction)
	return doorTimerTimeout, doorTimerAction
}

func runTimer(duration *time.Timer, timeout chan bool, action <-chan TimerAction) {
	for {
		select {
		case a := <-action:
			switch a {
			case Start:
				resetTimer(duration)
			case Stop:
				duration.Stop()
			}
		case <-duration.C:
			timeout <- true
		}
	}
}

// Stops the timer and resets it.
func resetTimer(t *time.Timer) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(config.DoorOpenDuration)
}
