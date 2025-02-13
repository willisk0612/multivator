package network

import (
	"fmt"
	"log/slog"
	"main/lib/network-go/network/bcast"
	"main/lib/network-go/network/peers"
	"main/src/elev"
	"main/src/types"
	"sync"
	"time"
)

const (
	broadcastPort = 15647
	peersPort     = 15648
	ackTimeout    = 500 * time.Millisecond
)

var (
	eventBids   []types.EventBidsPair
	pendingAcks sync.Map
	// Remove the global connectedPeers and use channels instead.
	peerUpdatesChan = make(chan []string)
	getPeersChan    = make(chan chan []string)
)

// peerManager owns the peers state.
func peerManager() {
	var currentPeers []string
	for {
		select {
		case newList := <-peerUpdatesChan:
			currentPeers = newList
		case replyChan := <-getPeersChan:
			replyChan <- currentPeers
		}
	}
}

// Helper to get a snapshot of current peers.
func getCurrentPeers() []string {
	reply := make(chan []string)
	getPeersChan <- reply
	return <-reply
}

func init() {
	go peerManager()
}

func broadcastAssignment(assignment types.OrderAssignment, outMsgCh chan<- types.Message, assignmentCh chan<- types.OrderAssignment) {
	outMsgCh <- types.Message{
		Type:     types.Assignment,
		Event:    assignment.Event,
		Cost:     assignment.Cost,
		SenderID: assignment.AssignedTo,
	}
	assignmentCh <- assignment
}

func HandleMessageEvent(elevator *types.Elevator, inMsg types.Message, outMsgCh chan<- types.Message, assignmentCh chan<- types.OrderAssignment) {
	switch inMsg.Type {
	case types.HallOrder:
		if inMsg.SenderID == elevator.NodeID {
			return
		}
		// Use a shallow copy.
		localElev := *elevator
		bid := elev.TimeToServedOrder(inMsg.Event, localElev)
		slog.Debug("Computed cost for hall order", "nodeID", elevator.NodeID, "event", inMsg.Event, "cost", bid)
		outMsgCh <- types.Message{
			Type:     types.Bid,
			Event:    inMsg.Event,
			Cost:     bid,
			SenderID: elevator.NodeID,
		}
		outMsgCh <- types.Message{
			Type:  types.Ack,
			Event: inMsg.Event,
		}

	case types.Assignment:
		assignment := types.OrderAssignment{
			Event:      inMsg.Event,
			AssignedTo: inMsg.SenderID,
			Cost:       inMsg.Cost,
			IsLocal:    inMsg.SenderID == elevator.NodeID,
		}
		assignmentCh <- assignment

	case types.Bid:
		peersCopy := getCurrentPeers()
		for i := range eventBids {
			if eventBids[i].Event == inMsg.Event {
				eventBids[i].Bids = append(eventBids[i].Bids, types.BidEntry{
					NodeID: inMsg.SenderID,
					Cost:   inMsg.Cost,
				})
				slog.Debug("Received bid", "nodeID", inMsg.SenderID, "event", inMsg.Event, "cost", inMsg.Cost)
				slog.Debug("Event bids state", "event", inMsg.Event, "bids", eventBids[i].Bids)
				if len(eventBids[i].Bids) == len(peersCopy) {
					assignment := findBestBid(eventBids[i], elevator.NodeID)
					broadcastAssignment(assignment, outMsgCh, assignmentCh)
					eventBids = append(eventBids[:i], eventBids[i+1:]...)
				}
				break
			}
		}

	case types.Ack:
		if val, ok := pendingAcks.Load(inMsg.Event); ok {
			ackCh := val.(chan struct{})
			select {
			case ackCh <- struct{}{}:
			default:
			}
		}
	}
}

func findBestBid(ebp types.EventBidsPair, localNodeID int) types.OrderAssignment {
	// ...existing code...
	if len(ebp.Bids) == 0 {
		return types.OrderAssignment{
			Event:      ebp.Event,
			AssignedTo: -1,
			Cost:       0,
			IsLocal:    false,
		}
	}

	bestBid := ebp.Bids[0]
	slog.Debug("Initial bid", "nodeID", bestBid.NodeID, "cost", bestBid.Cost)
	for _, bid := range ebp.Bids {
		slog.Debug("Comparing bid", "nodeID", bid.NodeID, "cost", bid.Cost)
		if bid.Cost < bestBid.Cost {
			bestBid = bid
		}
	}
	slog.Debug("Best bid selected", "nodeID", bestBid.NodeID, "cost", bestBid.Cost)
	return types.OrderAssignment{
		Event:      ebp.Event,
		AssignedTo: bestBid.NodeID,
		Cost:       bestBid.Cost,
		IsLocal:    bestBid.NodeID == localNodeID,
	}
}

func handlePeerUpdates(peerUpdateCh <-chan types.PeerUpdate) {
	for update := range peerUpdateCh {
		// Send the new peers list through the update channel.
		peerUpdatesChan <- update.Peers
		if update.New != "" {
			slog.Info("New peer connected", "newPeer", update.New, "totalPeers", len(update.Peers))
		}
		if len(update.Lost) > 0 {
			slog.Info("Peer(s) lost", "lostPeers", update.Lost, "totalPeers", len(update.Peers))
		}
		slog.Info("Peer update", "new", update.New, "lost", update.Lost, "peers", len(update.Peers))
	}
}

// Change createBidMsg to accept a getter function instead of a fixed elevator pointer.
func createBidMsg(btnEventCh <-chan types.ButtonEvent, outgoingMsgCh chan<- types.Message, getElev func() *types.Elevator, assignmentCh chan<- types.OrderAssignment) {
	for event := range btnEventCh {
		// Get the current elevator state each time.
		currentElev := getElev()
		if event.Button == types.BT_Cab {
			continue
		}
		peersCopy := getCurrentPeers()
		slog.Info("Received hall call", "event", event, "connectedPeers", len(peersCopy))
		cost := elev.TimeToServedOrder(event, *currentElev)
		slog.Info("Cost", "nodeID", currentElev.NodeID, "event", event, "cost", cost)
		eventBids = append(eventBids, types.EventBidsPair{
			Event: event,
			Bids: []types.BidEntry{
				{NodeID: currentElev.NodeID, Cost: cost},
			},
		})
		msg := types.Message{
			Type:     types.HallOrder,
			Event:    event,
			Cost:     cost,
			SenderID: currentElev.NodeID,
		}
		if len(peersCopy) == 0 {
			assignment := types.OrderAssignment{
				Event:      event,
				AssignedTo: currentElev.NodeID,
				Cost:       cost,
				IsLocal:    true,
			}
			slog.Debug("Assigning hall call locally", "event", event, "cost", cost)
			outgoingMsgCh <- msg
			broadcastAssignment(assignment, outgoingMsgCh, assignmentCh)
		} else {
			ackCh := make(chan struct{}, len(peersCopy)-1)
			pendingAcks.Store(event, ackCh)
			outgoingMsgCh <- msg
			go waitForAcks(event, ackCh)
		}
	}
}

// Update PollMessages to pass the getter without invoking it.
func PollMessages(getElev func() *types.Elevator, btnEventCh <-chan types.ButtonEvent, assignmentCh chan<- types.OrderAssignment) {
	nodeIDStr := fmt.Sprintf("node-%d", getElev().NodeID)
	incomingMsg := make(chan types.Message)
	outgoingMsg := make(chan types.Message)
	peerUpdate := make(chan types.PeerUpdate)
	go bcast.Receiver(broadcastPort, incomingMsg)
	go bcast.Transmitter(broadcastPort, outgoingMsg)
	go peers.Transmitter(peersPort, nodeIDStr, make(chan bool))
	go peers.Receiver(peersPort, peerUpdate)
	go handlePeerUpdates(peerUpdate)
	go createBidMsg(btnEventCh, outgoingMsg, getElev, assignmentCh)
	slog.Debug("PollMessages: starting message loop", "connectedPeers", getCurrentPeers())
	for msg := range incomingMsg {
		HandleMessageEvent(getElev(), msg, outgoingMsg, assignmentCh)
	}
}

func AssignNodeID() int {
	peerUpdateCh := make(chan types.PeerUpdate, 1)
	go peers.Receiver(peersPort, peerUpdateCh)
	deadline := time.After(2 * time.Second)
	var peersList []string
Loop:
	for {
		select {
		case update := <-peerUpdateCh:
			peersList = update.Peers
		case <-deadline:
			break Loop
		}
	}
	if len(peersList) == 0 {
		return 0
	}
	return len(peersList)
}

func waitForAcks(event types.ButtonEvent, ackCh chan struct{}) {
	peersCopy := getCurrentPeers()
	expectedAcks := len(peersCopy) - 1 // remote nodes only
	if expectedAcks < 0 {
		expectedAcks = 0
	}
	slog.Debug("waitForAcks: expected acks", "event", event, "expectedAcks", expectedAcks)
	receivedAcks := 0
	timeout := time.After(ackTimeout)
	for receivedAcks < expectedAcks {
		select {
		case <-ackCh:
			receivedAcks++
			slog.Debug("Received ack", "event", event, "received", receivedAcks, "expected", expectedAcks)
		case <-timeout:
			slog.Warn("Acknowledgment timeout", "event", event, "received", receivedAcks, "expected", expectedAcks)
			goto cleanup
		}
	}
	slog.Info("Message fully acknowledged", "event", event)
cleanup:
	pendingAcks.Delete(event)
	close(ackCh)
}
