package network

import (
	"log/slog"

	"multivator/lib/driver-go/elevio"
	"multivator/src/elev"
	"multivator/src/timer"
	"multivator/src/types"
)

// HandleHallOrder creates a bid for a hall order and sends it to the network
func HandleHallOrder(elevator *types.ElevState, btnEvent types.ButtonEvent, doorTimerAction chan timer.TimerAction, txBuffer chan types.Message[types.Bid]) {
	// If single elevator, move elevator and return
	if len(getPeers()) < 2 {
		slog.Debug("SINGLE ELEVATOR")
		elev.MoveElevator(elevator, btnEvent, doorTimerAction)
		return
	}
	slog.Debug("MULTIPLE ELEVATORS")
	cost := elev.Cost(elevator, btnEvent)
	slog.Debug("Initial bid", "cost", cost)
	msg := types.Message[types.Bid]{
		Type:      types.BidMsg,
		Content:   types.Bid{BtnEvent: btnEvent, Cost: cost},
		SenderID:  elevator.NodeID,
		LoopCount: 0,
	}

	// Send initial bid
	if HallOrders[btnEvent] == nil {
		HallOrders[btnEvent] = make(map[int]types.Bid)
	}
	HallOrders[btnEvent][elevator.NodeID] = types.Bid{BtnEvent: btnEvent, Cost: cost}
	slog.Debug("Sending initial bid")
	txBuffer <- msg
}

// HandleHallArrival processes notifications that an elevator has arrived at a hall call
func HandleHallArrival(elevator *types.ElevState, msg types.Message[types.HallArrival]) {
	// Ignore own hall arrivals
	if msg.SenderID == elevator.NodeID {
		return
	}

	// If order is within bounds, clear it and turn off button lamp
	if msg.SenderID < len(elevator.Orders) &&
		msg.Content.BtnEvent.Floor < len(elevator.Orders[msg.SenderID]) {
		elevator.Orders[msg.SenderID][msg.Content.BtnEvent.Floor][msg.Content.BtnEvent.Button] = false
		elevio.SetButtonLamp(msg.Content.BtnEvent.Button, msg.Content.BtnEvent.Floor, false)
	}
}

// TransmitHallArrival sends a message to the network that the elevator has arrived at a hall call.
func TransmitHallArrival(elevator *types.ElevState, btnEvent types.ButtonEvent, txBuffer chan types.Message[types.HallArrival]) {
	msg := types.Message[types.HallArrival]{
		Type:      types.HallArrivalMsg,
		Content:   types.HallArrival{BtnEvent: btnEvent},
		SenderID:  elevator.NodeID,
		LoopCount: 0,
	}
	txBuffer <- msg
}
