package config

import "time"

const (
	NumFloors        = 4
	NumButtons       = 3
	SensorPollRate   = 25 * time.Millisecond
	DoorOpenDuration = 3 * time.Second
	TravelDuration   = 2 * time.Second
	DirChangePenalty = 2 * time.Second
	MsgRepetitions   = 1
	MsgInterval      = 10 * time.Millisecond
)
