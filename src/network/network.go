package network

import (
	"log/slog"
	"time"

	"multivator/src/config"
	"multivator/src/elev"
	"multivator/src/types"
)

const (
	peerUpdateTimeout  = 100 * time.Millisecond
	broadcastPort      = 15657
	PeersPort          = 15658
)

type ElevStateWrapper struct {
	*elev.ElevState
}

type NetChannels struct {
	BidTx         chan types.Message
	BidRx         chan types.Message
	HallArrivalTx chan types.Message
	HallArrivalRx chan types.Message
	PeerUpdateCh  chan types.PeerUpdate
}

func InitChannels() NetChannels {
	return NetChannels{

		BidTx:         make(chan types.Message),
		BidRx:         make(chan types.Message),
		HallArrivalTx: make(chan types.Message),
		HallArrivalRx: make(chan types.Message),
		PeerUpdateCh:  make(chan types.PeerUpdate),
	}
}

func (elevator *ElevStateWrapper) handleMessageEvent(inMsg types.Message, elevInMsgCh, netOutMsgCh chan types.Message, lmChans *LightManagerChannels) {
	// Skip messages from self with exceptions for critical message types
	if inMsg.SenderID == elevator.GetState().NodeID &&
		inMsg.Type != types.LocalHallOrder &&
		inMsg.Type != types.NetHallOrder {
		slog.Debug("Skipping message from self", "type", inMsg.Type, "event", inMsg.Event, "senderID", inMsg.SenderID)
		return
	}

	// Single elevator system: convert message to local assignment
	if len(getCurrentPeers()) < 2 {
		slog.Debug("Single elevator mode - converting to local assignment", "event", inMsg.Event)

		// For hall orders and other relevant message types
		if inMsg.Type == types.LocalHallOrder || inMsg.Type == types.NetHallOrder {
			// Create a new local assignment message
			assignMsg := types.Message{
				Type:     types.LocalHallAssignment,
				Event:    inMsg.Event,
				SenderID: elevator.GetState().NodeID,
			}
			slog.Debug("Single elevator: Converting hall order to local assignment",
				"floor", inMsg.Event.Floor,
				"button", inMsg.Event.Button)
			elevInMsgCh <- assignMsg

			// Also turn on the light
			if inMsg.Event.Button != types.BT_Cab {
				lmChans.lightOnChan <- inMsg.Event
			}
		}
		return
	}

	switch inMsg.Type {
	case types.LocalHallOrder:
		// For LocalHallOrder, set the SenderID before processing
		if inMsg.SenderID == 0 { // If SenderID not set yet
			inMsg.SenderID = elevator.GetState().NodeID
			slog.Debug("Set SenderID for local hall order", "nodeID", inMsg.SenderID)
		}
		slog.Debug("Processing local hall order", "event", inMsg.Event)
		elevator.processHallOrder(inMsg.Event, netOutMsgCh)
	case types.NetHallOrder:
		slog.Debug("Received network hall order", "event", inMsg.Event, "from", inMsg.SenderID)
		elevator.broadcastBid(inMsg, netOutMsgCh)
	case types.Bid:
		slog.Debug("Received bid", "event", inMsg.Event, "from", inMsg.SenderID, "cost", inMsg.Cost)
		elevator.processBid(inMsg, elevInMsgCh, lmChans)
	}
}

func (elevator *ElevStateWrapper) ProcessHallOrder(event types.ButtonEvent, netOutMsgCh chan types.Message) {
	slog.Info("Processing hall order", "floor", event.Floor, "button", event.Button)

	// Clear any existing bids for this event
	elevator.clearCostCalc(event)

	// Only broadcast the hall order - do NOT calculate bid here or register event
	// (This will happen when we receive the NetHallOrder)
	msg := types.Message{
		Type:     types.NetHallOrder,
		Event:    event,
		SenderID: elevator.NodeID,
	}

	slog.Info("Broadcasting hall order to network", "floor", event.Floor, "button", event.Button, "nodeID", elevator.NodeID)

	// Send multiple times for reliability (using the configured repetition count)
	for i := 0; i < config.MsgRepetitions; i++ {
		netOutMsgCh <- msg
		time.Sleep(config.MsgInterval)
	}
}

func (elevator *ElevStateWrapper) clearCostCalc(hallEvent types.ButtonEvent) {
	for _, ebp := range elevator.EventBids {
		if ebp.Event.Floor == hallEvent.Floor && ebp.Event.Button == hallEvent.Button {
			elevator.UpdateEventBids(func(bids *[]types.EventBidsPair) {
				for i, b := range *bids {
					if b.Event == ebp.Event {
						*bids = append((*bids)[:i], (*bids)[i+1:]...)
						break
					}
				}
			})
			break
		}
	}
}

func sendMultipleMessages(msg types.Message, out chan<- types.Message) {
	for i := 0; i < config.MsgRepetitions; i++ {
		out <- msg
		time.Sleep(config.MsgInterval)
	}
}
