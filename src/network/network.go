package network

import (
	"fmt"
	"log/slog"
	"time"

	//"multivator/lib/driver-go/elevio"
	"multivator/lib/driver-go/elevio"
	"multivator/lib/network-go/network/bcast"
	"multivator/lib/network-go/network/peers"

	//"multivator/src/config"
	"multivator/src/elev"
	"multivator/src/types"
)

const (
	peerUpdateTimeout  = 100 * time.Millisecond
	messageRepetitions = 1
	messageInterval    = 10 * time.Millisecond
	broadcastPort      = 15657
	PeersPort          = 15658
	lightBroadcastPort = 15659
)

// Channels for light manager state
type LightManagerChannels struct {
	lightOnChan       chan types.ButtonEvent
	lightOffChan      chan types.ButtonEvent
	bcastLightOffChan chan types.ButtonEvent
	lightBcastOutCh   chan types.Message
	// External channel for receiving light events from subsystems
	lightElevMsgCh chan types.Message
}

func newLightManagerChannels(lightElevMsgCh chan types.Message) *LightManagerChannels {
	return &LightManagerChannels{
		lightOnChan:       make(chan types.ButtonEvent),
		lightOffChan:      make(chan types.ButtonEvent),
		bcastLightOffChan: make(chan types.ButtonEvent),
		lightBcastOutCh:   make(chan types.Message),
		lightElevMsgCh:    lightElevMsgCh,
	}
}

type ElevStateMgrWrapper struct {
	*elev.ElevStateMgr
}

// Run starts the network subsystem and sends messages to the elevator subsystem in case of hall assignments.
func Run(elevMgr *elev.ElevStateMgr, elevInMsgCh, elevOutMsgCh chan types.Message, lightElevMsgCh chan types.Message) {
	// Separate channels for elevator and light messaging
	netInMsgCh := make(chan types.Message)
	netOutMsgCh := make(chan types.Message)
	lightInMsgCh := make(chan types.Message)

	// Initialize light manager channels
	lmChans := newLightManagerChannels(lightElevMsgCh)

	// Start broadcast routines for elevator and light messaging
	go bcast.Receiver(broadcastPort, netInMsgCh)
	go bcast.Transmitter(broadcastPort, netOutMsgCh)
	go bcast.Receiver(lightBroadcastPort, lightInMsgCh)
	go bcast.Transmitter(lightBroadcastPort, lmChans.lightBcastOutCh)

	peerUpdateCh := make(chan types.PeerUpdate)
	go peers.Transmitter(PeersPort, fmt.Sprintf("node-%d", elevMgr.GetState().NodeID), make(chan bool))
	go peers.Receiver(PeersPort, peerUpdateCh)
	go handlePeerUpdates(peerUpdateCh)
	go peerManager()
	go lightManager(lmChans)

	elevWrapper := ElevStateMgrWrapper{elevMgr}

	// Forward light events from other subsystems to the light manager
	go func() {
		for msg := range lightElevMsgCh {
			slog.Debug("Forwarding light message from elevator subsystem", "type", msg.Type, "event", msg.Event)
			elevWrapper.handleLightMessage(msg, lmChans)
		}
	}()

	for {
		select {
		case msg := <-elevOutMsgCh:
			slog.Debug("Received message in network from elevator subsystem", "type", msg.Type, "event", msg.Event)

			// Special handling for LocalHallOrder from elevator subsystem
			if msg.Type == types.LocalHallOrder {
				slog.Info("Processing local hall order from elevator", "floor", msg.Event.Floor, "button", msg.Event.Button)

				// In single elevator mode, directly send it back to elevator
				if len(getCurrentPeers()) < 2 {
					slog.Info("Single elevator mode - directly assigning hall order", "floor", msg.Event.Floor, "button", msg.Event.Button)
					elevInMsgCh <- types.Message{
						Type:     types.LocalHallAssignment,
						Event:    msg.Event,
						SenderID: elevMgr.GetState().NodeID,
					}

					// Turn on light
					if msg.Event.Button != types.BT_Cab {
						lmChans.lightOnChan <- msg.Event
					}
				} else {
					// In multi-elevator mode, process hall order normally
					elevWrapper.processHallOrder(msg.Event, netOutMsgCh)
				}
			} else {
				// Process other message types normally
				elevWrapper.handleMessageEvent(msg, elevInMsgCh, netOutMsgCh, lmChans)
			}

			// Always process light messages
			elevWrapper.handleLightMessage(msg, lmChans)
			slog.Debug("Finished processing message from elevator subsystem")

		case msg := <-netInMsgCh:
			elevWrapper.handleMessageEvent(msg, elevInMsgCh, netOutMsgCh, lmChans)

		case msg := <-lightInMsgCh:
			elevWrapper.handleLightMessage(msg, lmChans)
		}
	}
}

func (elevMgr *ElevStateMgrWrapper) handleMessageEvent(inMsg types.Message, elevInMsgCh, netOutMsgCh chan types.Message, lmChans *LightManagerChannels) {
	// Skip messages from self with exceptions for critical message types
	if inMsg.SenderID == elevMgr.GetState().NodeID &&
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
				SenderID: elevMgr.GetState().NodeID,
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
			inMsg.SenderID = elevMgr.GetState().NodeID
			slog.Debug("Set SenderID for local hall order", "nodeID", inMsg.SenderID)
		}
		slog.Debug("Processing local hall order", "event", inMsg.Event)
		elevMgr.processHallOrder(inMsg.Event, netOutMsgCh)
	case types.NetHallOrder:
		slog.Debug("Received network hall order", "event", inMsg.Event, "from", inMsg.SenderID)
		elevMgr.broadcastBid(inMsg, netOutMsgCh)
	case types.Bid:
		slog.Debug("Received bid", "event", inMsg.Event, "from", inMsg.SenderID, "cost", inMsg.Cost)
		elevMgr.processBid(inMsg, elevInMsgCh, lmChans)
	}
}

func (elevMgr *ElevStateMgrWrapper) processHallOrder(event types.ButtonEvent, netOutMsgCh chan types.Message) {
	slog.Info("Processing hall order", "floor", event.Floor, "button", event.Button)

	// Clear any existing bids for this event
	elevMgr.clearCostCalc(event)

	// Only broadcast the hall order - do NOT calculate bid here or register event
	// (This will happen when we receive the NetHallOrder)
	msg := types.Message{
		Type:     types.NetHallOrder,
		Event:    event,
		SenderID: elevMgr.GetState().NodeID,
	}

	slog.Info("Broadcasting hall order to network", "floor", event.Floor, "button", event.Button, "nodeID", elevMgr.GetState().NodeID)

	// Send multiple times for reliability (using the configured repetition count)
	for i := 0; i < messageRepetitions; i++ {
		netOutMsgCh <- msg
		time.Sleep(messageInterval)
	}
}

// handleLightMessage processes light-related messages and routes them to the appropriate channel
func (elevMgr *ElevStateMgrWrapper) handleLightMessage(msg types.Message, lmChans *LightManagerChannels) {
	switch msg.Type {
	case types.LocalLightOn:
		if msg.Event.Button != types.BT_Cab { // Only handle hall button lights
			slog.Info("Setting hall light ON", "floor", msg.Event.Floor, "button", msg.Event.Button)
			lmChans.lightOnChan <- msg.Event
		}
	case types.LocalLightOff:
		if msg.Event.Button != types.BT_Cab { // Only handle hall button lights
			slog.Info("Setting hall light OFF locally", "floor", msg.Event.Floor, "button", msg.Event.Button)
			lmChans.lightOffChan <- msg.Event

			// This is a local light off request, so broadcast it to all nodes
			bcastMsg := types.Message{
				Type:     types.BcastLightOff,
				Event:    msg.Event,
				SenderID: elevMgr.GetState().NodeID,
			}
			slog.Debug("Broadcasting light OFF message to other nodes", "floor", msg.Event.Floor, "button", msg.Event.Button)
			lmChans.lightBcastOutCh <- bcastMsg
		}
	case types.BcastLightOff:
		if msg.Event.Button != types.BT_Cab { // Only handle hall button lights
			slog.Info("Received broadcast light OFF message", "floor", msg.Event.Floor, "button", msg.Event.Button)
			lmChans.bcastLightOffChan <- msg.Event
			// DO NOT re-broadcast a received broadcast message
		}
	}
}

// lightManager handles all hall button lights in a distributed system
func lightManager(lmChans *LightManagerChannels) {
	hallLightStates := make(map[types.ButtonEvent]bool)

	for {
		select {
		case event := <-lmChans.lightOnChan:
			if event.Button != types.BT_Cab { // Safety check - only handle hall lights
				hallLightStates[event] = true
				slog.Debug("Hall light turned ON", "floor", event.Floor, "button", event.Button)
				elevio.SetButtonLamp(event.Button, event.Floor, true)
			}

		case event := <-lmChans.lightOffChan:
			if event.Button != types.BT_Cab { // Safety check - only handle hall lights
				hallLightStates[event] = false
				slog.Debug("Hall light turned OFF locally", "floor", event.Floor, "button", event.Button)
				elevio.SetButtonLamp(event.Button, event.Floor, false)
			}

		case event := <-lmChans.bcastLightOffChan:
			if event.Button != types.BT_Cab { // Safety check - only handle hall lights
				hallLightStates[event] = false
				slog.Debug("Hall light turned OFF (broadcast)", "floor", event.Floor, "button", event.Button)
				elevio.SetButtonLamp(event.Button, event.Floor, false)
			}
		}
	}
}

func (elevMgr *ElevStateMgrWrapper) clearCostCalc(hallEvent types.ButtonEvent) {
	for _, ebp := range elevMgr.GetState().EventBids {
		if ebp.Event.Floor == hallEvent.Floor && ebp.Event.Button == hallEvent.Button {
			elevMgr.UpdateEventBids(func(bids *[]types.EventBidsPair) {
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
	for i := 0; i < messageRepetitions; i++ {
		out <- msg
		time.Sleep(messageInterval)
	}
}
