package config

import (
	"time"
)

var (
	NodeID    int
	BcastPort int
	PeersPort int
)

const (
	MsgRepetitions   = 5
	MsgInterval      = 10 * time.Millisecond
	BidTimeout       = 1 * time.Second
	NumElevators     = 3
	NumFloors        = 4
	NumButtons       = 3
	SensorPollRate   = 25 * time.Millisecond
	DoorOpenDuration = 3 * time.Second
	TravelDuration   = 2 * time.Second
	DirChangePenalty = 2 * time.Second
	BcastBasePort    = 16400
	PeersBasePort    = 17400
)
