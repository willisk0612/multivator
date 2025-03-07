package main

import (
	"flag"
	"fmt"
	"log/slog"

	"multivator/lib/driver-go/elevio"
	"multivator/src/config"
	"multivator/src/elev"
	"multivator/src/network"
	"multivator/src/timer"
	"multivator/src/types"
)

func main() {
	nodeID := flag.Int("id", 0, "Node ID of the elevator")
	flag.Parse()
	port := 15657 + *nodeID
	elev.InitLogger(*nodeID)

	elevio.Init(fmt.Sprintf("localhost:%d", port), config.NumFloors)
	elevator := elev.InitElevState(*nodeID)
	drv_buttons, drv_floors, drv_obstr := elev.InitDriver()
	doorTimerTimeout, doorTimerAction := timer.Init()
	elev.InitElevPos(elevator)

	bidTxBuf, bidRx, syncTxBuf, syncRx, peerUpdateCh := network.Init(elevator)
	for {
		select {

		// Elevator control

		case btn := <-drv_buttons:
			slog.Debug("Button press received", "button", elev.FormatBtnEvent(btn))
			if btn.Button == types.BT_Cab {
				elev.MoveElevator(elevator, btn, doorTimerAction)
				network.TransmitOrderSync(elevator, syncTxBuf, false)
			} else {
				network.HandleHallOrder(elevator, btn, doorTimerAction, bidTxBuf)
				slog.Debug("Exited HandleHallOrder")
			}
		case floor := <-drv_floors:
			elev.HandleFloorArrival(elevator, floor, doorTimerAction)
			network.TransmitOrderSync(elevator, syncTxBuf, false)
		case obstruction := <-drv_obstr:
			elevator.Obstructed = obstruction
			doorTimerAction <- timer.Start
		case <-doorTimerTimeout:
			elev.HandleDoorTimeout(elevator, doorTimerAction)

		// Network communication

		case bid := <-bidRx:
			network.HandleBid(elevator, bid, bidTxBuf, syncTxBuf, doorTimerAction)
		case sync := <-syncRx:
			// If we just connected, sync with restoring cab orders. Otherwise, sync without restoring cab orders.
			network.HandleSync(elevator, sync)
		case update := <-peerUpdateCh:
			network.PeerUpdate.Lost = update.Lost
			network.PeerUpdate.Peers = update.Peers
			slog.Info("Peer update", "peerUpdate", network.PeerUpdate)
			// If a node different from our own connects, sync state with restoring cab orders.
			if update.New != fmt.Sprintf("node-%d", elevator.NodeID) && update.New != "" {
				network.TransmitOrderSync(elevator, syncTxBuf, true)
			}
		}
	}
}
