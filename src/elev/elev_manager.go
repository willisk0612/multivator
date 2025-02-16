package elev

import (
	"main/src/types"
	"reflect"
)

// ElevatorCmd encapsulates an operation on the elevator.
type ElevatorCmd struct {
	Exec func(elevator *types.Elevator)
}

// ElevatorManager owns the elevator and serializes its access.
type ElevatorManager struct {
	cmds chan ElevatorCmd
}

// StartElevatorManager starts the manager goroutine.
func StartElevatorManager(elevator *types.Elevator) *ElevatorManager {
	mgr := &ElevatorManager{cmds: make(chan ElevatorCmd)}
	go func() {
		for cmd := range mgr.cmds {
			cmd.Exec(elevator)
		}
	}()
	return mgr
}

// Execute sends a command to the manager.
func (mgr *ElevatorManager) Execute(cmd interface{}, args ...interface{}) {
	switch f := cmd.(type) {
	case ElevatorCmd:
		mgr.cmds <- f
	case func(*types.Elevator):
		mgr.cmds <- ElevatorCmd{Exec: f}
	default:
		// Create closure that captures the arguments
		mgr.cmds <- ElevatorCmd{
			Exec: func(e *types.Elevator) {
				// Use reflection to call the function with the correct arguments
				reflect.ValueOf(cmd).Call(append(
					[]reflect.Value{reflect.ValueOf(e)},
					reflectArgs(args)...,
				))
			},
		}
	}
}

func reflectArgs(args []interface{}) []reflect.Value {
	vals := make([]reflect.Value, len(args))
	for i, arg := range args {
		vals[i] = reflect.ValueOf(arg)
	}
	return vals
}
