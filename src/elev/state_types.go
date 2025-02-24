// State types are defined in elev package to make method receivers possible in elev_state.go.
package elev

import (
	"multivator/src/config"
	"multivator/src/types"
)

// ElevState represents the state of the elevator.
type ElevState struct {
	NodeID        int
	Floor         int
	BetweenFloors bool
	Dir           types.MotorDirection
	Orders        [config.NumFloors][config.NumButtons]bool
	Behaviour     types.ElevBehaviour
	Obstructed    bool
	EventBids     []types.EventBidsPair
}

// StateCmd sends a command to the elevator manager.
type ElevStateCmd struct {
	Exec func(elevator *ElevState)
}

// StateMgr owns the elevator and serializes its access.
type ElevStateMgr struct {
	Cmds chan ElevStateCmd
}

type ElevMgrField string

const (
	ElevFloorField         ElevMgrField = "Floor"
	ElevBetweenFloorsField ElevMgrField = "BetweenFloors"
	ElevDirField           ElevMgrField = "Dir"
	ElevOrdersField        ElevMgrField = "Orders"
	ElevBehaviourField     ElevMgrField = "Behaviour"
	ElevObstructedField    ElevMgrField = "Obstructed"
	ElevEventBidsField     ElevMgrField = "EventBids"
)
