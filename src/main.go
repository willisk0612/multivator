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
	drv_buttons, drv_floors, drv_obstr, drv_stop := elev.InitDriver()
	doorTimerTimeout, doorTimerAction := timer.Init()
	elev.InitElevPos(elevator)

	bidTxBuf, bidRx, syncOrdersTxBuf, syncOrdersRx, peerUpdateCh := network.Init(elevator)
	for {
		select {

		// Elevator control

		case btn := <-drv_buttons:
			slog.Debug("Button press received", "button", elev.FormatBtnEvent(btn))
			if btn.Button == types.BT_Cab {
				elev.MoveElevator(elevator, btn, doorTimerAction)
			} else {
				network.HandleHallOrder(elevator, btn, doorTimerAction, bidTxBuf)
				slog.Debug("Exited HandleHallOrder")
			}
		case floor := <-drv_floors:
			elev.HandleFloorArrival(elevator, floor, doorTimerAction)
			btnEvent := elevator.CurrentBtnEvent
			if btnEvent.Button != types.BT_Cab && elevator.Behaviour == types.DoorOpen {
				network.TransmitOrderSync(elevator, syncOrdersTxBuf)
			}
		case obstruction := <-drv_obstr:
			elev.HandleObstruction(elevator, obstruction, doorTimerAction)
		case <-drv_stop:
			elev.HandleStop(elevator)
		case <-doorTimerTimeout:
			elev.HandleDoorTimeout(elevator, doorTimerAction)

		// Network communication

		case bid := <-bidRx:
			network.HandleBid(elevator, bid, bidTxBuf, syncOrdersTxBuf, doorTimerAction)
		case syncOrders := <-syncOrdersRx:
			network.HandleSyncOrders(elevator, syncOrders)
		case update := <-peerUpdateCh:
			network.PeerUpdate.Peers = update.Peers
			slog.Info("Peer update", "peerUpdate", network.PeerUpdate)
			// If a node different from our own connects, send a sync message
			if update.New != fmt.Sprintf("node-%d", elevator.NodeID) {
				network.TransmitOrderSync(elevator, syncOrdersTxBuf)
			}
		}
	}
}
