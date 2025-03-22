package dispatcher

import (
	"fmt"
	"sync/atomic"
	"time"

	"multivator/src/config"
)

// msgBufferTx is called as a goroutine multiple times for each message type
//   - increments monotonic counter for each input message
func msgBufferTx[T MsgContent](msgBufTxCh chan Msg[T], msgTxCh chan Msg[T], atomicCounter *atomic.Uint64) {
	for msgBufTx := range msgBufTxCh {
		msgBufTx.Counter = atomicCounter.Add(1)
		for range config.MsgRepetitions {
			msgTxCh <- msgBufTx
			time.Sleep(config.MsgInterval)
		}
	}
}

// msgBufferRx is called as a goroutine multiple times for each message type
//   - ignores own messages
//   - implements lamport timestamp for causal ordering
//   - stores seen messages in a map with id based on sender and counter
func msgBufferRx[T MsgContent](msgBufRxCh chan Msg[T], msgRxCh chan Msg[T], atomicCounter *atomic.Uint64) {
	seenMsgs := make(map[string]bool)
	recentMsgIDs := make([]string, config.MsgRepetitions)
	var nextIndex atomic.Uint32

	for msgRx := range msgRxCh {
		if msgRx.SenderID != config.NodeID {
			msgID := fmt.Sprintf("%d-%d", msgRx.SenderID, msgRx.Counter)

			if !seenMsgs[msgID] {
				seenMsgs[msgID] = true

				// Update Lamport timestamp
				for {
					localTime := atomicCounter.Load()
					newTime := max(localTime, msgRx.Counter) + 1
					if atomicCounter.CompareAndSwap(localTime, newTime) {
						break
					}
				}

				// Delete old message from seen messages
				index := int((nextIndex.Add(1) - 1) % config.MsgRepetitions)
				oldMsgID := recentMsgIDs[index]
				if oldMsgID != "" {
					delete(seenMsgs, oldMsgID)
				}
				recentMsgIDs[index] = msgID
				msgBufRxCh <- msgRx
			}
		}
	}
}

func max(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}
