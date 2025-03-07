package network

import (
	"fmt"
	"time"

	"multivator/lib/network-go/network/bcast"
	"multivator/lib/network-go/network/peers"
	"multivator/src/config"
	"multivator/src/types"
)

var PeerUpdate types.PeerUpdate

func Init(elevator *types.ElevState) (
	chan types.Message[types.Bid],
	chan types.Message[types.Bid],
	chan types.Message[types.Sync],
	chan types.Message[types.Sync],
	chan types.PeerUpdate,
) {
	bidTx := make(chan types.Message[types.Bid])
	bidTxBuf := make(chan types.Message[types.Bid])
	bidRx := make(chan types.Message[types.Bid])
	syncTx := make(chan types.Message[types.Sync])
	syncTxBuf := make(chan types.Message[types.Sync])
	syncRx := make(chan types.Message[types.Sync])
	peerUpdateCh := make(chan types.PeerUpdate)

	hallOrders = make(map[types.ButtonEvent]map[int]types.Bid)

	go bcast.Transmitter(config.BcastPort, bidTx, syncTx)
	go bcast.Receiver(config.BcastPort, bidRx, syncRx)
	go peers.Transmitter(config.PeersPort, fmt.Sprintf("node-%d", elevator.NodeID), make(chan bool))
	go peers.Receiver(config.PeersPort, peerUpdateCh)
	go msgBufferTx(bidTxBuf, bidTx)
	go msgBufferTx(syncTxBuf, syncTx)

	return bidTxBuf, bidRx, syncTxBuf, syncRx, peerUpdateCh
}

// msgBufferTx listens for messages, and sends a burst of messages at a fixed interval
func msgBufferTx[T types.MsgContent](msgBufCh chan types.Message[T], msgTxCh chan types.Message[T]) {
	for msg := range msgBufCh {
		for range config.MsgRepetitions {
			msgTxCh <- msg
			time.Sleep(config.MsgInterval)
		}
	}
}

func getPeers() []string {
	result := make([]string, len(PeerUpdate.Peers))
	copy(result, PeerUpdate.Peers)
	return result
}
