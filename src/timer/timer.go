package timer

import (
	"log/slog"
	"time"

	"multivator/src/config"
)

type TimerAction int

const (
	Start TimerAction = iota
	Stop
)

func Timer(duration *time.Timer, timeout chan bool, action <-chan TimerAction) {
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
			slog.Debug("Timer timed out")
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
