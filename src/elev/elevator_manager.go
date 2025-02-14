package elev

import "main/src/types"

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
func (mgr *ElevatorManager) Execute(cmd ElevatorCmd) {
	mgr.cmds <- cmd
}
