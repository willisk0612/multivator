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
	chan types.Message[types.SyncOrders],
	chan types.Message[types.SyncOrders],
	chan types.PeerUpdate,
) {
	bidTx := make(chan types.Message[types.Bid])
	bidTxBuf := make(chan types.Message[types.Bid])
	bidRx := make(chan types.Message[types.Bid])
	syncOrdersTx := make(chan types.Message[types.SyncOrders])
	syncOrdersTxBuf := make(chan types.Message[types.SyncOrders])
	syncOrdersRx := make(chan types.Message[types.SyncOrders])
	peerUpdateCh := make(chan types.PeerUpdate)

	hallOrders = make(map[types.ButtonEvent]map[int]types.Bid)

	go bcast.Transmitter(config.BcastPort, bidTx, syncOrdersTx)
	go bcast.Receiver(config.BcastPort, bidRx, syncOrdersRx)
	go peers.Transmitter(config.PeersPort, fmt.Sprintf("node-%d", elevator.NodeID), make(chan bool))
	go peers.Receiver(config.PeersPort, peerUpdateCh)
	go msgBufferTx(bidTxBuf, bidTx)
	go msgBufferTx(syncOrdersTxBuf, syncOrdersTx)

	return bidTxBuf, bidRx, syncOrdersTxBuf, syncOrdersRx, peerUpdateCh
}

// msgBufferTx listens for messages, and sends a burst of messages at a fixed interval
func msgBufferTx[T types.MsgContent](msgBuf chan types.Message[T], msgTx chan types.Message[T]) {
	for msg := range msgBuf {
		for range config.MsgRepetitions {
			msgTx <- msg
			time.Sleep(config.MsgInterval)
		}
	}
}

func getPeers() []string {
	result := make([]string, len(PeerUpdate.Peers))
	copy(result, PeerUpdate.Peers)
	return result
}
