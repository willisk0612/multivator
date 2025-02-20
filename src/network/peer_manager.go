package network

import (
	"log/slog"
	"main/src/types"
)

var (
	peerUpdatesChan = make(chan []string)
	getPeersChan    = make(chan chan []string)
)

func peerManager() {
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

func handlePeerUpdates(peerUpdateCh <-chan types.PeerUpdate) {
	for update := range peerUpdateCh {
		peerUpdatesChan <- update.Peers
		if update.New != "" {
			slog.Info("New peer connected", "newPeer", update.New, "totalPeers", len(update.Peers))
		}
		if len(update.Lost) > 0 {
			slog.Info("Peer(s) lost", "lostPeers", update.Lost, "totalPeers", len(update.Peers))
		}
	}
}
