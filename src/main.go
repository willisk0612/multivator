// Main file for the elevator system. It contains two subsystems for single elevator control and network communication.
package main

import (
	"flag"
	"fmt"
	"time"
	"log/slog"

	"multivator/lib/driver-go/elevio"
	"multivator/lib/network-go/network/bcast"
	"multivator/lib/network-go/network/peers"
	"multivator/src/config"
	"multivator/src/elev"
	"multivator/src/network"
	"multivator/src/timer"
	"multivator/src/types"
)

func main() {
	// Initialize elevator subsystem
	nodeID := flag.Int("id", 0, "Node ID of the elevator")
	flag.Parse()
	port := 15657 + *nodeID
	elev.InitLogger()
	elevio.Init(fmt.Sprintf("localhost:%d", port), config.NumFloors)

	drv_buttons, drv_floors, drv_obstr, drv_stop := elev.InitHW()
	elevator := elev.InitElevState(*nodeID)
	elev.InitElevPos(elevator)

	hallOrderCh := make(chan types.ButtonEvent)

	doorTimerDuration := time.NewTimer(config.DoorOpenDuration)
	doorTimerTimeout := make(chan bool)
	doorTimerAction := make(chan timer.TimerAction)
	go timer.Timer(doorTimerDuration, doorTimerTimeout, doorTimerAction)

	// Initialize network subsystem
	bidTx, bidRx, hallOrderTx, hallOrderRx, hallArrivalTx, hallArrivalRx, peerUpdateCh := network.InitChannels[any]()

	go bcast.Transmitter(config.BcastPort, bidTx, hallOrderTx, hallArrivalTx)
	go bcast.Receiver(config.BcastPort, bidRx, hallOrderRx, hallArrivalRx)
	go peers.Transmitter(config.PeersPort, fmt.Sprintf("node-%d", elevator.NodeID), make(chan bool))
	go peers.Receiver(config.PeersPort, peerUpdateCh)
	slog.Info("Network subsystem initialized")

	for {
		select {
			
		// Elevator subsystem
		case btn := <-drv_buttons:
			slog.Info("New event:","Btn:", btn)
			elev.HandleButtonPress(elevator, btn, doorTimerAction, hallOrderCh, bidTx)
			slog.Info("HandleButtonPress done")
		case floor := <-drv_floors:
			elev.HandleFloorArrival(elevator, floor, doorTimerAction)
		case obstruction := <-drv_obstr:
			elev.HandleObstruction(elevator, obstruction, doorTimerAction)
		case <-drv_stop:
			elev.HandleStop(elevator)
		case <-doorTimerTimeout:
			elev.HandleDoorTimeout(elevator, doorTimerAction)

		// Network subsystem
		case bid := <-bidRx:
			network.HandleBid(elevator, bid)
		case hallArrival := <-hallArrivalRx:
			network.HandleHallArrival(elevator, hallArrival)
		case peerUpdate := <-peerUpdateCh:
			network.HandlePeerUpdates(peerUpdate)
		case hallOrder := <-hallOrderCh:
			network.HandleHallOrder(elevator, hallOrder, bidTx)
		}
	}
}
