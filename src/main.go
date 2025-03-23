package main

import (
	"flag"

	"multivator/src/config"
	"multivator/src/dispatcher"
	"multivator/src/executor"
	"multivator/src/types"
)

func main() {
	nodeID := flag.Int("id", 0, "Node ID of the elevator")
	flag.Parse()
	config.NodeID = *nodeID

	elevUpdateCh := make(chan types.ElevState)
	hallOrderCh := make(chan types.HallOrder)
	sendSyncCh := make(chan bool)
	orderUpdateCh := make(chan types.Orders, config.NumElevators)

	go dispatcher.Run(elevUpdateCh, orderUpdateCh, hallOrderCh, sendSyncCh)
	go executor.Run(elevUpdateCh, orderUpdateCh, hallOrderCh, sendSyncCh)
	select {}
}
