package utils

import (
	"fmt"
	"slices"

	"multivator/lib/network/peers"
	"multivator/src/config"
	"multivator/src/types"
)

// ForEachOrder is a helper function that reduces indentation when performing an action on all orders
func ForEachOrder(orders types.Orders, action func(node, floor, btn int)) {
	for node := range orders {
		for floor := range orders[node] {
			for btn := range orders[node][floor] {
				action(node, floor, btn)
			}
		}
	}
}

// PrintStatus is called when a PeerUpdate is received
func PrintStatus(peerUpdate peers.PeerUpdate) {
	fmt.Printf("\rNode ID: %d | ", config.NodeID)
	ownID := fmt.Sprintf("node-%d", config.NodeID)
	if slices.Contains(peerUpdate.Peers, ownID) {
		fmt.Print("Status: Connected    \r")
	} else {
		fmt.Print("Status: Disconnected\r")
	}
}
