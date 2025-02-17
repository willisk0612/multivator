package timer

import (
	"main/src/config"
	"time"
)

type TimerAction int

const (
	Start TimerAction = iota
	Stop
)

// Store start time to calculate remaining duration
var startTime time.Time
var isRunning bool

func Timer(duration *time.Timer, timeout chan bool, action <-chan TimerAction) {
	for {
		select {
		case a := <-action:
			switch a {
			case Start:
				startTime = time.Now()
				isRunning = true
				resetTimer(duration)
			case Stop:
				isRunning = false
				duration.Stop()
			}
		case <-duration.C:
			isRunning = false
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
