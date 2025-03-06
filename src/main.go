package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"multivator/lib/driver-go/elevio"
	"multivator/src/config"
	"multivator/src/elev"
	"multivator/src/network"
	"multivator/src/timer"
	"multivator/src/types"
	"os"
)

var cabOrders []int

func addCabOrder(floor int) {
	cabOrders = append(cabOrders, floor)
}

func remCabOrder(floor int) {
	var tmp []int
	for _, v := range cabOrders {
		if v != floor {
			tmp = append(tmp, v)
		}
	}
	cabOrders = tmp
}

func SaveCabOrders(elevator *types.ElevState) {
	file, err := os.Create(fmt.Sprintf("backup-node%d.json", elevator.NodeID))
	if err != nil {
		log.Printf("Error creating file: %v", err)
		return
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Printf("Error closing file: %v", err)
		}
	}(file)

	jsonEncoder := json.NewEncoder(file)
	if err := jsonEncoder.Encode(cabOrders); err != nil {
		log.Printf("Error encoding to JSON: %v", err)
	}
}

func RestoreCabOrders(elevator *types.ElevState) {
	file, err := os.Open(fmt.Sprintf("backup-node%d.json", elevator.NodeID))
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Printf("Error opening file: %v", err)
		return
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Printf("Error closing file: %v", err)
		}
	}(file)

	jsonDecoder := json.NewDecoder(file)
	if err := jsonDecoder.Decode(&cabOrders); err != nil {
		log.Printf("Error decoding JSON: %v", err)
	}
}

func main() {
	nodeID := flag.Int("id", 0, "Node ID of the elevator")
	flag.Parse()
	port := 15657 + *nodeID
	elev.InitLogger(*nodeID)

	elevio.Init(fmt.Sprintf("localhost:%d", port), config.NumFloors)
	elevator := elev.InitElevState(*nodeID)
	drv_buttons, drv_floors, drv_obstr, drv_stop := elev.InitHW()
	doorTimerTimeout, doorTimerAction := timer.Init()
	elev.InitElevPos(elevator)

	bidTxBuf, bidRx, hallArrivalTxBuf, hallArrivalRx, peerUpdateCh := network.Init(elevator)

	RestoreCabOrders(elevator)
	for _, v := range cabOrders {
		elev.MoveElevator(elevator, types.ButtonEvent{v, types.BT_Cab}, doorTimerAction)
	}
	for {
		select {

		// Elevator control
		case btn := <-drv_buttons:
			if btn.Button == types.BT_Cab {
				addCabOrder(btn.Floor)
				SaveCabOrders(elevator)
				elev.MoveElevator(elevator, btn, doorTimerAction)
			} else {
				network.HandleHallOrder(elevator, btn, doorTimerAction, bidTxBuf)
			}
		case floor := <-drv_floors:
			elev.HandleFloorArrival(elevator, floor, doorTimerAction)
			btnEvent := elevator.CurrentBtnEvent
			if btnEvent.Button != types.BT_Cab {
				network.TransmitHallArrival(elevator, btnEvent, hallArrivalTxBuf)
			} else {
				remCabOrder(floor)
				SaveCabOrders(elevator)
			}

		case obstruction := <-drv_obstr:
			elev.HandleObstruction(elevator, obstruction, doorTimerAction)
		case <-drv_stop:
			elev.HandleStop(elevator)
		case <-doorTimerTimeout:
			elev.HandleDoorTimeout(elevator, doorTimerAction)

		// Network communication
		case bid := <-bidRx:
			network.HandleBid(elevator, bid, bidTxBuf, hallArrivalTxBuf, doorTimerAction)
		case hallArrival := <-hallArrivalRx:
			network.HandleHallArrival(elevator, hallArrival)
		case update := <-peerUpdateCh:
			network.PeerUpdate.Peers = update.Peers
			slog.Info("Peer update", "peerUpdate", network.PeerUpdate)
		}
	}
}
