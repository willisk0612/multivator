package network

import (
	"multivator/src/types"
	"log/slog"
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

func HandlePeerUpdates(peerUpdate types.PeerUpdate) {
		if peerUpdate.New != "" {
			slog.Info("New peer connected", "newPeer", peerUpdate.New, "totalPeers", len(peerUpdate.Peers))
		}
		if len(peerUpdate.Lost) > 0 {
			slog.Info("Peer(s) lost", "lostPeers", peerUpdate.Lost, "totalPeers", len(peerUpdate.Peers))
		}
		slog.Info("Current peer list updated", "count", len(peerUpdate.Peers), "peers", peerUpdate.Peers)

		peerUpdatesChan <- peerUpdate.Peers
}

func getPeers() []string {
	replyChan := make(chan []string)
	getPeersChan <- replyChan
	return <-replyChan
}
