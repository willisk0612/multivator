// Main file for the elevator system. It contains two subsystems for single elevator control and network communication.
package main

import (
	"flag"
	"fmt"

	"multivator/lib/driver-go/elevio"
	"multivator/src/config"
	"multivator/src/elev"
	//"multivator/src/network"
	"multivator/src/timer"
	"multivator/src/types"
)

	func main() {
	nodeID := flag.Int("id", 0, "Node ID of the elevator")
	flag.Parse()
	port := 15657 + *nodeID

	elev.InitLogger()

	elevio.Init(fmt.Sprintf("localhost:%d", port), config.NumFloors)
	elevator := elev.InitElevState(*nodeID)

	drv_buttons, drv_floors, drv_obstr, drv_stop := elev.InitHW()
	doorTimerTimeout, doorTimerAction := timer.Init()
	elev.InitElevPos(elevator)

	// 8. Initialize network last
	//bidTx, bidRx, hallArrivalTx, hallArrivalRx, peerUpdateCh := network.Init(elevator)

	for {
		select {

		// Elevator
		case btn := <-drv_buttons:
			if btn.Button == types.BT_Cab {
				elev.MoveElevator(elevator, btn, doorTimerAction)
			} //else {
				//network.HandleHallOrder(elevator, btn, doorTimerAction, bidTx)
			//}
		case floor := <-drv_floors:
			elev.HandleFloorArrival(elevator, floor, doorTimerAction)
		case obstruction := <-drv_obstr:
			elev.HandleObstruction(elevator, obstruction, doorTimerAction)
		case <-drv_stop:
			elev.HandleStop(elevator)
		case <-doorTimerTimeout:
			elev.HandleDoorTimeout(elevator, doorTimerAction)

		// Network
		// case bid := <-bidRx:
		// 	network.HandleBid(elevator, bid)
		// case hallArrival := <-hallArrivalRx:
		// 	network.HandleHallArrival(elevator, hallArrival, hallArrivalTx)
		// case peerUpdate := <-peerUpdateCh:
		// 	network.HandlePeerUpdates(peerUpdate)
		}
	}
}
