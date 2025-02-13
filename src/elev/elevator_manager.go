package elev

import "main/src/types"

// ElevatorCmd encapsulates an operation on the elevator.
type ElevatorCmd struct {
	Exec func(elevator *types.Elevator)
	// Optional reply channel for getters if needed.
	Reply chan *types.Elevator
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
			if cmd.Reply != nil {
				// Return the elevator pointer (caller must copy if needed)
				cmd.Reply <- elevator
			}
		}
	}()
	return mgr
}

// Execute sends a command to the manager.
func (mgr *ElevatorManager) Execute(cmd ElevatorCmd) {
	mgr.cmds <- cmd
}

// Get returns a safe copy of the current elevator state by copying it inside the manager goroutine.
func (mgr *ElevatorManager) Get() *types.Elevator {
	reply := make(chan *types.Elevator)
	mgr.cmds <- ElevatorCmd{
		Exec: func(e *types.Elevator) {
			// Make a copy inside the manager goroutine.
			cpy := *e
			reply <- &cpy
		},
	}
	return <-reply
}
