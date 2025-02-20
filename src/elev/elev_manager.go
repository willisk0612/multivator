package elev

import (
	"main/src/types"
	"main/src/config"
	"reflect"
)

// StartElevatorManager starts the manager goroutine.
func StartElevatorManager(elevator *types.Elevator) *types.ElevatorManager {
	elevMgr := &types.ElevatorManager{Cmds: make(chan types.ElevatorCmd)}
	go func() {
		for cmd := range elevMgr.Cmds {
			cmd.Exec(elevator)
		}
	}()
	return elevMgr
}

// GetElevState creates a shallow copy of the elevator state.
func GetElevState(elevMgr *types.ElevatorManager) *types.Elevator {
	reply := make(chan *types.Elevator)
	elevMgr.Cmds <- types.ElevatorCmd{
		Exec: func(elevator *types.Elevator) {
			clone := *elevator // shallow copy
			// NOTE: We are not deep copying the EventBids field!
			reply <- &clone
		},
	}
	return <-reply
}

// UpdateState safely updates the elevator state using reflection.
func UpdateState(elevMgr *types.ElevatorManager, field types.ElevMgrField, value interface{}) {
	elevMgr.Cmds <- types.ElevatorCmd{
		Exec: func(e *types.Elevator) {
			v := reflect.ValueOf(e).Elem()
			f := v.FieldByName(string(field))
			if !f.IsValid() || !f.CanSet() {
				return
			}
			val := reflect.ValueOf(value)
			if val.Type().ConvertibleTo(f.Type()) {
				f.Set(val.Convert(f.Type()))
			}
		},
	}
}

// UpdateOrders applies an update function on the orders array.
func UpdateOrders(elevMgr *types.ElevatorManager, updateFunc func(orders *[config.NumFloors][config.NumButtons]bool)) {
	elevMgr.Cmds <- types.ElevatorCmd{
		Exec: func(elevator *types.Elevator) {
			updateFunc(&elevator.Orders)
		},
	}
}

// UpdateEventBids applies an update function on the EventBids slice.
func UpdateEventBids(elevMgr *types.ElevatorManager, updateFunc func(bids *[]types.EventBidsPair)) {
	elevMgr.Cmds <- types.ElevatorCmd{
		Exec: func(elevator *types.Elevator) {
			updateFunc(&elevator.EventBids)
		},
	}
}
