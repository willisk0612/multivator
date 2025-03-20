package dispatcher

import (
	"sync/atomic"
	"time"

	"multivator/src/config"
)

// msgBufferTx sends a burst of messages to bcast.Transmitter
//  - implements lamport timestamp for outgoing messages
func msgBufferTx[T MsgContent](msgBufTxCh chan Msg[T], msgTxCh chan Msg[T], atomicCounter *atomic.Uint64) {
	for msgBufTx := range msgBufTxCh {
		atomicCounter.Add(1)
		msgBufTx.Counter = atomicCounter.Load()
		for range config.MsgRepetitions {
			msgTxCh <- msgBufTx
			time.Sleep(config.MsgInterval)
		}
	}
}

// msgBufferRx receives a burst of messages from bcast.Receiver
//  - ignores own messages
//  - implements lamport timestamp for incoming messages
func msgBufferRx[T MsgContent](msgBufRxCh chan Msg[T], msgRxCh chan Msg[T], atomicCounter *atomic.Uint64) {
	for msgRx := range msgRxCh {
		if msgRx.SenderID != config.NodeID {
			localCounter := atomicCounter.Load()
			if msgRx.Counter > localCounter {
				atomicCounter.Store(msgRx.Counter)
			}
			atomicCounter.Add(1)
			msgBufRxCh <- msgRx
		}
	}
}
