package network

import (
	"log/slog"
	"sync"

	"multivator/src/types"
)

var (
	peerUpdatesChan = make(chan []string)
	currentPeers    []string
	peersMutex      sync.RWMutex
)

func peerManager() {
	for update := range peerUpdatesChan {
		peersMutex.Lock()
		currentPeers = update
		peersMutex.Unlock()
	}
}

func getCurrentPeers() []string {
	peersMutex.RLock()
	defer peersMutex.RUnlock()
	result := make([]string, len(currentPeers))
	copy(result, currentPeers)
	return result
}

func handlePeerUpdates(peerUpdateCh <-chan types.PeerUpdate) {
	for update := range peerUpdateCh {
		if update.New != "" {
			slog.Info("New peer connected", "newPeer", update.New, "totalPeers", len(update.Peers))
		}
		if len(update.Lost) > 0 {
			slog.Info("Peer(s) lost", "lostPeers", update.Lost, "totalPeers", len(update.Peers))
		}
		slog.Info("Current peer list updated", "count", len(update.Peers), "peers", update.Peers)

		peerUpdatesChan <- update.Peers
	}
}
