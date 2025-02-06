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
	broadcastPort = 20000
	peersPort     = 20001
	nodeID        = "node-1" // change as needed
)

func main() {
	// Initialize channels for broadcasting and peer updates.
	msgCh := make(chan string)
	peerUpdateCh := make(chan peers.PeerUpdate)
	peerTxEnable := make(chan bool, 1)
	peerTxEnable <- true

	// Start the broadcast receiver and transmitter.
	go bcast.Receiver(broadcastPort, msgCh)
	go func() {
		bcast.Transmitter(broadcastPort, msgCh)
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

	// Read user input and broadcast messages.
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
			msgCh <- msg
		}
	}
}
