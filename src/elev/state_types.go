// State types are defined in elev package to make method receivers possible in elev_state.go.
package elev



// ElevState represents the state of the elevator.
type ElevState struct {
	NodeID          int
	Floor           int
	BetweenFloors   bool
	Dir             types.MotorDirection
	Orders          [][][]bool // Cab, HallUp, HallDown
	Behaviour       types.ElevBehaviour
	Obstructed      bool
	EventBids       []types.EventBidsPair
	CurrentBtnEvent types.ButtonEvent
}
