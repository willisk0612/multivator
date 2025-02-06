package network

import (
	"fmt"
	"log"
	"main/lib/driver-go/elevio"
	"main/lib/network-go/network/bcast"
	"main/lib/network-go/network/peers"
	"main/src/types"
	"time"
)

const (
	broadcastPort = 15647
	peersPort     = 15648
)

type networkChannels struct {
	incomingMsg chan types.Message
	outgoingMsg chan types.Message
	peerUpdate  chan peers.PeerUpdate
}

func initializeChannels() networkChannels {
	return networkChannels{
		incomingMsg: make(chan types.Message),
		outgoingMsg: make(chan types.Message),
		peerUpdate:  make(chan peers.PeerUpdate),
	}
}

func setupNetworkHandlers(channels networkChannels, nodeIDStr string) {
	go bcast.Receiver(broadcastPort, channels.incomingMsg)
	go bcast.Transmitter(broadcastPort, channels.outgoingMsg)
	go peers.Transmitter(peersPort, nodeIDStr, make(chan bool))
	go peers.Receiver(peersPort, channels.peerUpdate)
}

func handlePeerUpdates(peerUpdateCh <-chan peers.PeerUpdate) {
	for update := range peerUpdateCh {
		log.Printf("Peers update: New: %s, Lost: %v, All: %v",
			update.New, update.Lost, update.Peers)
	}
}

func handleIncomingMessages(incomingMsgCh <-chan types.Message, nodeID int) {
	for msg := range incomingMsgCh {
		if msg.SenderNodeID != nodeID {
			fmt.Print(formatButtonEvent(msg))
		}
	}
}

func forwardLocalEvents(eventCh <-chan types.ButtonEvent, outgoingMsgCh chan<- types.Message, nodeID int) {
	for event := range eventCh {
		outgoingMsgCh <- types.Message{
			SenderNodeID: nodeID,
			Event:        event,
		}
	}
}

func PollMessages(elevator *types.Elevator, eventCh <-chan types.ButtonEvent) {
	nodeIDStr := fmt.Sprintf("node-%d", elevator.NodeID)
	channels := initializeChannels()

	setupNetworkHandlers(channels, nodeIDStr)

	go handlePeerUpdates(channels.peerUpdate)
	go handleIncomingMessages(channels.incomingMsg, elevator.NodeID)
	go forwardLocalEvents(eventCh, channels.outgoingMsg, elevator.NodeID)

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
	peerUpdateCh := make(chan peers.PeerUpdate, 1)
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

func formatButtonEvent(m types.Message) string {
	buttonType := map[elevio.ButtonType]string{
		elevio.BT_HallUp:   "Hall up",
		elevio.BT_HallDown: "Hall down",
		elevio.BT_Cab:      "Cab",
	}[m.Event.Button]

	return fmt.Sprintf("\n[Node %d] Button press: %s at floor %d\n",
		m.SenderNodeID, buttonType, m.Event.Floor)
}
