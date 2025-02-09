package network

import (
	"fmt"
	"log/slog"
	"main/lib/network-go/network/bcast"
	"main/lib/network-go/network/peers"
	"main/src/types"
	"time"
)

const (
	broadcastPort = 15647
	peersPort     = 15648
	maxRetries    = 3
	ackTimeout    = 500 * time.Millisecond
	bufferSize    = 100
)

var (
	outgoingBuffer = make(chan types.BufferEntry, bufferSize)
	ackChannel     = make(chan int64, bufferSize)
)

func handlePeerUpdates(peerUpdateCh <-chan types.PeerUpdate) {
	for update := range peerUpdateCh {
		slog.Info("Peer update received",
			"new", update.New,
			"lost", update.Lost,
			"peers", update.Peers)
	}
}

func createMessage(msgType types.MessageType, senderID int, event types.ButtonEvent, ackID int64) types.Message {
	return types.Message{
		BufferID:  time.Now().UnixNano(),
		Type:      msgType,
		SenderID:  senderID,
		Event:     event,
		AckID:     ackID,
		Timestamp: time.Now(),
	}
}

func handleIncomingMessages(incomingMsgCh <-chan types.Message, nodeID int, outgoingMsgCh chan<- types.Message, mainEventCh chan<- types.Message) {
	for msg := range incomingMsgCh {
		if msg.SenderID != nodeID {
			if msg.Type == types.MsgAcknowledge {
				ackChannel <- msg.AckID
				continue
			}
			ack := createMessage(types.MsgAcknowledge, nodeID, types.ButtonEvent{}, msg.BufferID)
			outgoingMsgCh <- ack
			mainEventCh <- msg
		}
	}
}

func forwardLocalEvents(eventCh <-chan types.ButtonEvent, outgoingMsgCh chan<- types.Message, nodeID int) {
	for event := range eventCh {
		msg := createMessage(types.MsgButtonEvent, nodeID, event, 0)
		outgoingMsgCh <- msg
	}
}

func PollMessages(elevator *types.Elevator, eventCh <-chan types.ButtonEvent, networkEventCh chan<- types.Message) {
	nodeIDStr := fmt.Sprintf("node-%d", elevator.NodeID)

	incomingMsg := make(chan types.Message)
	outgoingMsg := make(chan types.Message)
	peerUpdate := make(chan types.PeerUpdate)

	go bcast.Receiver(broadcastPort, incomingMsg)
	go bcast.Transmitter(broadcastPort, outgoingMsg)
	go peers.Transmitter(peersPort, nodeIDStr, make(chan bool))
	go peers.Receiver(peersPort, peerUpdate)

	go handlePeerUpdates(peerUpdate)
	go handleIncomingMessages(incomingMsg, elevator.NodeID, outgoingMsg, networkEventCh)
	go forwardLocalEvents(eventCh, outgoingMsg, elevator.NodeID)
	go handleRetransmissions(outgoingMsg)

	select {}
}

func AssignNodeID() int {
	peerCount := numConnectedPeers()
	if peerCount == 0 {
		return 0
	}
	return peerCount
}

func numConnectedPeers() int {
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
	return len(peersList)
}

// Helper function to handle network events
func HandleMessageEvent(msg types.Message) {
	if msg.Type == types.MsgButtonEvent {
		buttonType := map[types.ButtonType]string{
			types.BT_HallUp:   "Hall up",
			types.BT_HallDown: "Hall down",
			types.BT_Cab:      "Cab",
		}[msg.Event.Button]

		slog.Info("Button event received",
			"node", msg.SenderID,
			"type", buttonType,
			"floor", msg.Event.Floor)
	}
}

func handleRetransmissions(outgoingMsgCh chan<- types.Message) {
	for buf := range outgoingBuffer {
		for attempt := 0; attempt <= maxRetries; attempt++ {
			outgoingMsgCh <- buf.Msg

			select {
			case <-buf.Done:
				return
			case <-time.After(ackTimeout):
				if attempt == maxRetries {
					close(buf.Done)
					return
				}
				continue
			}
		}
	}
}
