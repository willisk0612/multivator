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

func PollMessages(elevator *types.Elevator, eventCh <-chan types.ButtonEvent) {
	nodeIDStr := fmt.Sprintf("node-%d", elevator.NodeID)
	var incomingMsgCh = make(chan types.Message)
	var outgoingMsgCh = make(chan types.Message)

	go bcast.Receiver(broadcastPort, incomingMsgCh)
	go bcast.Transmitter(broadcastPort, outgoingMsgCh)

	// Set up peer handling
	peerUpdateCh := make(chan peers.PeerUpdate)
	go peers.Transmitter(peersPort, nodeIDStr, make(chan bool))
	go peers.Receiver(peersPort, peerUpdateCh)

	// Handle peer updates
	go func() {
		for update := range peerUpdateCh {
			log.Printf("Peers update: New: %s, Lost: %v, All: %v",
				update.New, update.Lost, update.Peers)
		}
	}()

	// Handle incoming messages
	go func() {
		for msg := range incomingMsgCh {
			if msg.SenderNodeID != elevator.NodeID {
				fmt.Print(formatButtonEvent(msg))
			}
		}
	}()

	// Forward local events to network
	go func() {
		for event := range eventCh {
			outgoingMsgCh <- types.Message{
				SenderNodeID: elevator.NodeID,
				Event:       event,
			}
		}
	}()

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
