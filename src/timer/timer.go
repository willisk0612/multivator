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

func Run(duration *time.Timer, timeout chan bool, action <-chan TimerAction) {
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
