package network

import (
	"multivator/src/types"
)

var (
	peerUpdatesChan = make(chan []string)
	getPeersChan    = make(chan chan []string)
)

func PeerManager() {
	var currentPeers []string
	for {
		select {
		case newList := <-peerUpdatesChan:
			currentPeers = newList
		case replyChan := <-getPeersChan:
			replyChan <- currentPeers
		}
	}
}

func getCurrentPeers() []string {
	reply := make(chan []string)
	getPeersChan <- reply
	return <-reply
}

func HandlePeerUpdates(peerUpdateCh <-chan types.PeerUpdate) {
	for update := range peerUpdateCh {
		peerUpdatesChan <- update.Peers
	}
}
