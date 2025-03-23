package utils

import (
	"fmt"
	"slices"
	"strconv"

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

func FindLowestID(nodes []string) int {
	minID := len(nodes)
	for _, node := range nodes {
		nodeInt, _ := strconv.Atoi(node[5:])
		if nodeInt < minID {
			minID = nodeInt
		}
	}

	return minID
}

// PrintStatus is called when a PeerUpdate is received
func PrintStatus(peersList peers.PeerUpdate) {
	fmt.Printf("\rNode ID: %d | ", config.NodeID) // Note: removed \n, added space
	ownID := fmt.Sprintf("node-%d", config.NodeID)
	if slices.Contains(peersList.Peers, ownID) {
		fmt.Print("Status: Connected    \r") // Added padding spaces and \r
	} else {
		fmt.Print("Status: Disconnected\r") // Added \r
	}
}
