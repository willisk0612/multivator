package config

import (
	"log/slog"
	"time"
)

var NodeID int

const (
	LogLevel         = slog.LevelDebug
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
	BcastPort        = 15657
	PeersPort        = 15658
)
