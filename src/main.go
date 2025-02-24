// Main file for the elevator system. It contains two subsystems for single elevator control and network communication.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"multivator/lib/driver-go/elevio"
	"multivator/src/config"
	"multivator/src/elev"
	"multivator/src/network"
	"multivator/src/types"
)

func main() {
	nodeID := flag.Int("id", 0, "Node ID of the elevator")
	flag.Parse()
	port := 15657 + *nodeID

	elev.InitLogger()
	elevio.Init(fmt.Sprintf("localhost:%d", port), config.NumFloors)
	elevator := elev.InitElevState(*nodeID)

	elevInMsgCh := make(chan types.Message)
	elevOutMsgCh := make(chan types.Message)
	elevMgr := elev.StartStateMgr(elevator)

	go elev.Run(elevMgr, *nodeID, elevInMsgCh, elevOutMsgCh)
	go network.Run(elevMgr, elevInMsgCh, elevOutMsgCh)

	slog.Info("System initialized", "port", port, "nodeID", nodeID)
	select {} // Keep main running
}
