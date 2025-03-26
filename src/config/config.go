package config

import (
	"time"
)

var NodeID int

const (
	MsgRepetitions   = 5
	MsgInterval      = 10 * time.Millisecond
	BidTimeout       = 1 * time.Second
	NumElevators     = 3
	NumFloors        = 4
	NumButtons       = 3
	BtnPressInterval = 75 * time.Millisecond
	SensorPollRate   = 25 * time.Millisecond
	DoorOpenDuration = 3 * time.Second
	TravelDuration   = 2 * time.Second
	DirChangePenalty = 2 * time.Second
	BcastPort        = 16400
	PeersPort        = 17400
)
