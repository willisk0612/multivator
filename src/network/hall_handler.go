package network

import (
	"log/slog"

	"multivator/src/elev"
	"multivator/src/timer"
	"multivator/src/types"
)

var hallOrders map[types.ButtonEvent]map[int]types.Bid

// HandleHallOrder creates a bid for a hall order and sends it to the network
func HandleHallOrder(elevator *types.ElevState, btnEvent types.ButtonEvent, doorTimerAction chan timer.TimerAction, txBuffer chan types.Message[types.Bid]) {
	slog.Debug("Entering HandleHallOrder")
	// If single elevator, move elevator and return
	if len(getPeers()) < 2 {
		slog.Debug("Single elevator")
		elev.MoveElevator(elevator, btnEvent, doorTimerAction)
		return
	}
	// Store and transmit initial bid
	cost := elev.TimeToServeOrder(elevator, btnEvent)
	slog.Debug("Initial bid", "Cost", cost)
	msg := types.Message[types.Bid]{
		Type:      types.BidMsg,
		Content:   types.Bid{BtnEvent: btnEvent, Cost: cost},
		SenderID:  elevator.NodeID,
		LoopCount: 0,
	}
	storeBid(msg)
	slog.Debug("Sending initial bid")
	txBuffer <- msg
	slog.Debug("Initial bid sent")
}
