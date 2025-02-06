package config

import "time"

const (
	N_FLOORS           = 4
	N_BUTTONS          = 3
	SENSOR_POLLRATE    = 25 * time.Millisecond
	DOOR_OPEN_DURATION = 3 * time.Second
)
