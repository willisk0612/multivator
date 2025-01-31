package timer

import (
	"time"
)

type TimerAction int

const (
	Start TimerAction = iota
	Stop
)

const (
	DOOR_OPEN_DURATION = 3 * time.Second
)

// Starts, stops or resets a door timer for a specified time
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
		}
	}
}

// Stops the timer and resets it
func resetTimer(t *time.Timer) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(DOOR_OPEN_DURATION)
}
