package dispatcher

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"multivator/src/config"
)

// msgBufferTx sends a burst of messages to bcast.Transmitter
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

// msgBufferRx receives a burst of messages from bcast.Receiver
//   - ignores own messages
//   - implements lamport timestamp for causal ordering
//   - stores seen messages in a map to avoid duplicates
func msgBufferRx[T MsgContent](msgBufRxCh chan Msg[T], msgRxCh chan Msg[T], atomicCounter *atomic.Uint64) {
	var seenMsgs sync.Map
	recentMsgIDs := make([]string, config.MsgRepetitions)
	var nextIndex atomic.Uint32

	for msgRx := range msgRxCh {
		if msgRx.SenderID != config.NodeID {
			msgID := fmt.Sprintf("%d-%d", msgRx.SenderID, msgRx.Counter)

			if _, seen := seenMsgs.LoadOrStore(msgID, true); !seen {
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
					seenMsgs.Delete(oldMsgID)
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
