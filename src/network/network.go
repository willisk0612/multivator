package main

import (
	"main/lib/network-go/network/bcast"
	"main/lib/network-go/network/peers"
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

const (
	nodeID        = "node-1" // change as needed
)

func main() {
	mode := "Simulation"
	var broadcastPort, peersPort int

	if mode == "Simulation" {
		// set broadcastPort and peersPort to 20000 and 20001
		broadcastPort = 20000
		peersPort = 20001
	} else if mode == "Server" {
		// set broadcastPort and peersPort to 15647
		broadcastPort = 15647
		peersPort = 15647
	}

	// Initialize channels for incoming and outgoing messages and peer updates.
	incomingMsgCh := make(chan string)
	outgoingMsgCh := make(chan string)
	peerUpdateCh := make(chan peers.PeerUpdate)
	peerTxEnable := make(chan bool, 1)
	peerTxEnable <- true

	// Start the broadcast receiver and transmitter.
	go bcast.Receiver(broadcastPort, incomingMsgCh)
	go func() {
		bcast.Transmitter(broadcastPort, outgoingMsgCh)
	}()

	// Add a goroutine to print received messages.
	go func() {
		for msg := range incomingMsgCh {
			fmt.Printf("Received: %s\n", msg)
		}
	}()

	// Start peers (node discovery) transmitter and receiver.
	go func() {
		peers.Transmitter(peersPort, nodeID, peerTxEnable)
	}()
	go func() {
		peers.Receiver(peersPort, peerUpdateCh)
	}()

	// Monitor peer updates.
	go func() {
		for update := range peerUpdateCh {
			log.Printf("Peers update: New: %s, Lost: %v, All: %v", update.New, update.Lost, update.Peers)
		}
	}()

	// Read user input and send messages for broadcast.
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter message: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("Error reading input: %v", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if msg := strings.TrimSpace(input); msg != "" {
			outgoingMsgCh <- msg
		}
	}
}
