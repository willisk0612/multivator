package config

import "time"

const (
	NumFloors        = 4
	NumButtons       = 3
	SensorPollRate   = 25 * time.Millisecond
	DoorOpenDuration = 3 * time.Second
	TravelDuration   = 2 * time.Second
	DirectionChangePenalty = 2 * time.Second
)
