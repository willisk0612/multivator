package network

import (
	"fmt"
	"log/slog"
	"time"

	"multivator/lib/network-go/network/bcast"
	"multivator/lib/network-go/network/peers"

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
	lightElevMsgCh    chan types.Message
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
	// Initialize channels and managers
	netChannels := initializeNetworkChannels(lightElevMsgCh)
	elevWrapper := ElevStateMgrWrapper{elevMgr}

	// Start communication modules
	startNetworkCommunication(elevMgr, netChannels)

	// Forward light events from other subsystems to the light manager
	go forwardLightMessages(&elevWrapper, netChannels.lmChans)

	// Main message processing loop
	for {
		select {
		case msg := <-elevOutMsgCh:
			slog.Debug("Received message in network from elevator subsystem", "type", msg.Type, "event", msg.Event)
			handleElevatorOutMessage(&elevWrapper, msg, elevInMsgCh, netChannels)

		case msg := <-netChannels.netInMsgCh:
			elevWrapper.handleMessageEvent(msg, elevInMsgCh, netChannels.netOutMsgCh, netChannels.lmChans)

		case msg := <-netChannels.lightInMsgCh:
			elevWrapper.handleLightMessage(msg, netChannels.lmChans)
		}
	}
}

// NetworkChannels holds all communication channels used by the network module
type NetworkChannels struct {
	netInMsgCh   chan types.Message
	netOutMsgCh  chan types.Message
	lightInMsgCh chan types.Message
	lmChans      *LightManagerChannels
}

// initializeNetworkChannels creates and returns all required communication channels
func initializeNetworkChannels(lightElevMsgCh chan types.Message) NetworkChannels {
	return NetworkChannels{
		netInMsgCh:   make(chan types.Message),
		netOutMsgCh:  make(chan types.Message),
		lightInMsgCh: make(chan types.Message),
		lmChans:      newLightManagerChannels(lightElevMsgCh),
	}
}

// startNetworkCommunication initializes all network communication modules
func startNetworkCommunication(elevMgr *elev.ElevStateMgr, channels NetworkChannels) {
	// Start broadcast routines for elevator and light messaging
	go bcast.Receiver(broadcastPort, channels.netInMsgCh)
	go bcast.Transmitter(broadcastPort, channels.netOutMsgCh)
	go bcast.Receiver(lightBroadcastPort, channels.lightInMsgCh)
	go bcast.Transmitter(lightBroadcastPort, channels.lmChans.lightBcastOutCh)

	// Start peer communication
	peerUpdateCh := make(chan types.PeerUpdate)
	go peers.Transmitter(PeersPort, fmt.Sprintf("node-%d", elevMgr.GetState().NodeID), make(chan bool))
	go peers.Receiver(PeersPort, peerUpdateCh)
	go handlePeerUpdates(peerUpdateCh)
	go peerManager()

	// Start light manager
	go lightManager(channels.lmChans)
}

// forwardLightMessages forwards light messages from elevator subsystem to the light manager
func forwardLightMessages(elevWrapper *ElevStateMgrWrapper, lmChans *LightManagerChannels) {
	for msg := range lmChans.lightElevMsgCh {
		slog.Debug("Forwarding light message from elevator subsystem", "type", msg.Type, "event", msg.Event)
		elevWrapper.handleLightMessage(msg, lmChans)
	}
}

// handleElevatorOutMessage processes messages from the elevator subsystem
func handleElevatorOutMessage(elevWrapper *ElevStateMgrWrapper, msg types.Message, elevInMsgCh chan types.Message, channels NetworkChannels) {
	// Special handling for LocalHallOrder from elevator subsystem
	if msg.Type == types.LocalHallOrder {
		handleLocalHallOrder(elevWrapper, msg, elevInMsgCh, channels)
	} else {
		// Process other message types normally
		elevWrapper.handleMessageEvent(msg, elevInMsgCh, channels.netOutMsgCh, channels.lmChans)
	}

	// Always process light messages
	elevWrapper.handleLightMessage(msg, channels.lmChans)
	slog.Debug("Finished processing message from elevator subsystem")
}

// handleLocalHallOrder processes hall orders from the local elevator
func handleLocalHallOrder(elevWrapper *ElevStateMgrWrapper, msg types.Message, elevInMsgCh chan types.Message, channels NetworkChannels) {
	slog.Info("Processing local hall order from elevator", "floor", msg.Event.Floor, "button", msg.Event.Button)

	// In single elevator mode, directly send it back to elevator
	if isSingleElevatorMode() {
		assignLocalHallOrder(elevWrapper, msg, elevInMsgCh, channels.lmChans)
	} else {
		// In multi-elevator mode, process hall order normally
		elevWrapper.processHallOrder(msg.Event, channels.netOutMsgCh)
	}
}

// assignLocalHallOrder assigns a hall order to the local elevator in single elevator mode
func assignLocalHallOrder(elevWrapper *ElevStateMgrWrapper, msg types.Message, elevInMsgCh chan types.Message, lmChans *LightManagerChannels) {
	slog.Info("Single elevator mode - directly assigning hall order", "floor", msg.Event.Floor, "button", msg.Event.Button)

	elevInMsgCh <- types.Message{
		Type:     types.LocalHallAssignment,
		Event:    msg.Event,
		SenderID: elevWrapper.GetState().NodeID,
	}

	// Turn on light
	if msg.Event.Button != types.BT_Cab {
		lmChans.lightOnChan <- msg.Event
	}
}

var prevMsg types.Message

func isSingleElevatorMode() bool {
	return len(getCurrentPeers()) < 2
}

func (elevMgr *ElevStateMgrWrapper) handleMessageEvent(inMsg types.Message, elevInMsgCh, netOutMsgCh chan types.Message, lmChans *LightManagerChannels) {
	// Skip duplicate messages
	if inMsg == prevMsg {
		return
	}
	prevMsg = inMsg

	// Single elevator system handling
	if isSingleElevatorMode() {
		handleSingleElevatorMessage(elevMgr, inMsg, elevInMsgCh, lmChans)
		return
	}

	// Handle multi-elevator case
	handleMultiElevatorMessage(elevMgr, inMsg, elevInMsgCh, netOutMsgCh, lmChans)
}

// handleSingleElevatorMessage processes messages in single elevator mode
func handleSingleElevatorMessage(elevMgr *ElevStateMgrWrapper, inMsg types.Message, elevInMsgCh chan types.Message, lmChans *LightManagerChannels) {
	if inMsg.Type == types.LocalHallOrder || inMsg.Type == types.NetHallOrder {
		// Create a new local assignment message
		assignMsg := types.Message{
			Type:     types.LocalHallAssignment,
			Event:    inMsg.Event,
			SenderID: elevMgr.GetState().NodeID,
		}
		elevInMsgCh <- assignMsg

		// Also turn on the light
		if inMsg.Event.Button != types.BT_Cab {
			lmChans.lightOnChan <- inMsg.Event
		}
	}
}

// handleMultiElevatorMessage processes messages in multi-elevator mode
func handleMultiElevatorMessage(elevMgr *ElevStateMgrWrapper, inMsg types.Message, elevInMsgCh, netOutMsgCh chan types.Message, lmChans *LightManagerChannels) {
	switch inMsg.Type {
	case types.LocalHallOrder:
		// Set the SenderID before processing if needed
		ensureSenderID(elevMgr, &inMsg)
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

// ensureSenderID ensures the message has a valid sender ID
func ensureSenderID(elevMgr *ElevStateMgrWrapper, inMsg *types.Message) {
	if inMsg.SenderID == 0 { // If SenderID not set yet
		inMsg.SenderID = elevMgr.GetState().NodeID
		slog.Debug("Set SenderID for local hall order", "nodeID", inMsg.SenderID)
	}
}

func (elevMgr *ElevStateMgrWrapper) processHallOrder(event types.ButtonEvent, netOutMsgCh chan types.Message) {
	slog.Info("Processing hall order", "floor", event.Floor, "button", event.Button)

	// Clear any existing bids for this event
	elevMgr.clearCostCalc(event)

	// Only broadcast the hall order - do NOT calculate bid here or register event
	// (This will happen when we receive the NetHallOrder)
	msg := createHallOrderMessage(elevMgr, event)

	slog.Info("Broadcasting hall order to network", "floor", event.Floor, "button", event.Button, "nodeID", elevMgr.GetState().NodeID)

	// Send multiple times for reliability (using the configured repetition count)
	sendMultipleMessages(msg, netOutMsgCh)
}

// createHallOrderMessage creates a network hall order message
func createHallOrderMessage(elevMgr *ElevStateMgrWrapper, event types.ButtonEvent) types.Message {
	return types.Message{
		Type:     types.NetHallOrder,
		Event:    event,
		SenderID: elevMgr.GetState().NodeID,
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
		select {
		case out <- msg:
			// Message sent successfully
		default:
			// Channel is full or closed, log error
			slog.Error("Failed to send message, channel might be full or closed",
				"type", msg.Type, "event", msg.Event)
			return
		}
		time.Sleep(messageInterval)
	}
}
