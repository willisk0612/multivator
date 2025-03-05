package network

import (
	"fmt"
	//"log/slog"
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
	chan types.Message[types.HallArrival],
	chan types.Message[types.HallArrival],
	chan types.PeerUpdate,
) {
	bidTx := make(chan types.Message[types.Bid], 10)
	bidTxBuf := make(chan types.Message[types.Bid], 10)
	bidRx := make(chan types.Message[types.Bid], 10)
	bidRxBuf := make(chan types.Message[types.Bid], 10)
	hallArrivalTx := make(chan types.Message[types.HallArrival], 10)
	hallArrivalTxBuf := make(chan types.Message[types.HallArrival], 10)
	hallArrivalRx := make(chan types.Message[types.HallArrival], 10)
	hallArrivalRxBuf := make(chan types.Message[types.HallArrival], 10)
	peerUpdateCh := make(chan types.PeerUpdate)

	hallOrders = make(map[types.ButtonEvent]map[int]types.Bid)

	go bcast.Transmitter(config.BcastPort, bidTx, hallArrivalTx)
	go bcast.Receiver(config.BcastPort, bidRx, hallArrivalRx)
	go peers.Transmitter(config.PeersPort, fmt.Sprintf("node-%d", elevator.NodeID), make(chan bool))
	go peers.Receiver(config.PeersPort, peerUpdateCh)
	go msgBufferTx(bidTxBuf, bidTx)
	go msgBufferTx(hallArrivalTxBuf, hallArrivalTx)
	go msgBufferRx(bidRx, bidRxBuf)
	go msgBufferRx(hallArrivalRx, hallArrivalRxBuf)

	return bidTxBuf, bidRxBuf, hallArrivalTxBuf, hallArrivalRxBuf, peerUpdateCh
}

// msgBufferTx listens for messages, and sends a burst of messages at a fixed interval
func msgBufferTx[T types.MsgContent](msgBuf chan types.Message[T], msgTx chan types.Message[T]) {
	for msg := range msgBuf {
		// slog.Debug("Detected message in buffer", "type", msg.Type)
		burstTransmit(msg, msgTx)
	}
}

// msgBufferRx listens for messages, and forwards them to a buffer
func msgBufferRx[T types.MsgContent](msgRx chan types.Message[T], msgRxBuf chan types.Message[T]) {
	for msg := range msgRx {
		// slog.Debug("Buffering received message", "type", msg.Type)
		msgRxBuf <- msg
	}
}

func burstTransmit[T types.MsgContent](msg types.Message[T], tx chan<- types.Message[T]) {
	for i := 0; i < config.MsgRepetitions; i++ {
		tx <- msg
		time.Sleep(config.MsgInterval)
	}
}

func getPeers() []string {
	result := make([]string, len(PeerUpdate.Peers))
	copy(result, PeerUpdate.Peers)
	return result
}
