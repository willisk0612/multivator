// Main file for the elevator system. It contains two subsystems for single elevator control and network communication.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"time"

	"multivator/lib/driver-go/elevio"
	"multivator/lib/network-go/network/bcast"
	"multivator/lib/network-go/network/peers"
	"multivator/src/config"
	"multivator/src/elev"
	"multivator/src/network"
	"multivator/src/timer"
	"multivator/src/types"
)

const (
	broadcastPort = 15657
	PeersPort     = 15658
)

func main() {

	// Initialize elevator subsystem

	nodeID := flag.Int("id", 0, "Node ID of the elevator")
	flag.Parse()
	port := 15657 + *nodeID
	elevio.Init(fmt.Sprintf("localhost:%d", port), config.NumFloors)
	elev.InitLogger()
	elevChs := elev.InitChannels()
	doorTimerDuration := time.NewTimer(config.DoorOpenDuration)
	elevator := elev.InitElevState(*nodeID)

	go elevio.PollButtons(elevChs.Drv_buttons)
	go elevio.PollFloorSensor(elevChs.Drv_floors)
	go elevio.PollObstructionSwitch(elevChs.Drv_obstr)
	go elevio.PollStopButton(elevChs.Dv_stop)
	go timer.Timer(doorTimerDuration, elevChs.DoorTimerTimeout, elevChs.DoorTimerAction)

	elev.InitElevPos(*nodeID)

	// Initialize network subsystem

	netChs := network.InitChannels()

	go bcast.Transmitter(broadcastPort, netChs.BidTx, netChs.HallOrderTx, netChs.HallArrivalTx)
	go bcast.Receiver(broadcastPort, netChs.BidRx, netChs.HallOrderRx, netChs.HallArrivalRx)
	go peers.Transmitter(PeersPort, fmt.Sprintf("node-%d", elevator.NodeID), make(chan bool))
	go peers.Receiver(PeersPort, netChs.PeerUpdateCh)

	for {
		select {
		// Elevator subsystem

		case btn := <-elevChs.Drv_buttons:
			if btn.Button == types.BT_Cab || elevio.GetFloor() == -1 {
				elev.MoveElevator(elevator, btn, elevChs.DoorTimerAction)
			} else {
				slog.Debug("Hall button press discovered in elevator. Sending to network")
				msg := types.Message{
					Type:     types.LocalHallOrder,
					Event:    btn,
					SenderID: elevator.NodeID,
				}
				sendMultipleMessages(msg, netChs.HallOrderTx)
			}
		case floor := <-elevChs.Drv_floors:
			elev.HandleFloorArrival(elevator, floor, elevChs.DoorTimerAction)
		case obstruction := <-elevChs.Drv_obstr:
			elev.HandleObstruction(elevator, obstruction, elevChs.DoorTimerAction)
		case <-elevChs.Dv_stop:
			elev.HandleStop(elevator)
		case <-elevChs.DoorTimerTimeout:
			elev.HandleDoorTimeout(elevator, elevChs.DoorTimerAction)

		// Network subsystem
		case bid := <-netChs.BidRx:
			// Store bid and check if all bids are received. If so, assign order
			// TODO: Broadcast hallOrderTx
		case hallArrival := <-netChs.HallArrivalRx:
			// Modify order matrix and turn off light

		// TODO: Implement sync hall lights when 


		}
	}
}

func sendMultipleMessages(msg types.Message, out chan<- types.Message) {
	for i := 0; i < config.MsgRepetitions; i++ {
		out <- msg
		time.Sleep(config.MsgInterval)
	}
}
