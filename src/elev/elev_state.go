package elev

import (
	"reflect"

	"multivator/src/config"
	"multivator/src/types"
)

// StartStateMgr starts the elevator state manager goroutine that serializes access to the elevator state.
func StartStateMgr(elevator *ElevState) *ElevStateMgr {
	elevMgr := &ElevStateMgr{
		Cmds: make(chan ElevStateCmd),
	}
	go func() {
		for cmd := range elevMgr.Cmds {
			cmd.Exec(elevator)
		}
	}()
	return elevMgr
}

// GetState creates a shallow copy of the elevator state.
func (elevMgr *ElevStateMgr) GetState() *ElevState {
	reply := make(chan *ElevState)
	elevMgr.Cmds <- ElevStateCmd{
		Exec: func(elevator *ElevState) {
			clone := *elevator
			reply <- &clone
		},
	}
	return <-reply
}

// UpdateState updates boolean, integer and enum fields in the elevator struct using cmd channel.
func (elevMgr *ElevStateMgr) UpdateState(field ElevMgrField, value interface{}) {
	elevMgr.Cmds <- ElevStateCmd{
		Exec: func(e *ElevState) {
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

// UpdateOrders updates the Orders field using cmd channel.
func (elevMgr *ElevStateMgr) UpdateOrders(updateFunc func(orders *[config.NumFloors][config.NumButtons]bool)) {
	elevMgr.Cmds <- ElevStateCmd{
		Exec: func(elevator *ElevState) {
			updateFunc(&elevator.Orders)
		},
	}
}

// UpdateEventBids updates the EventBids field using cmd channel.
func (elevMgr *ElevStateMgr) UpdateEventBids(updateFunc func(bids *[]types.EventBidsPair)) {
	elevMgr.Cmds <- ElevStateCmd{
		Exec: func(elevator *ElevState) {
			updateFunc(&elevator.EventBids)
		},
	}
}
