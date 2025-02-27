package network

import (
	"log/slog"

	"multivator/lib/driver-go/elevio"
	"multivator/src/types"
)

// handleLightMessage processes light-related messages and routes them to the appropriate channel
func (elevMgr *ElevStateMgrWrapper) handleLightMessage(msg types.Message, lmChans *LightManagerChannels) {
	switch msg.Type {
	case types.LocalLightOn:
		if msg.Event.Button != types.BT_Cab { // Only handle hall button lights
			slog.Info("Setting hall light ON", "floor", msg.Event.Floor, "button", msg.Event.Button)
			lmChans.lightOnChan <- msg.Event
		}
	case types.LocalLightOff:
		if msg.Event.Button != types.BT_Cab { // Only handle hall button lights
			slog.Info("Setting hall light OFF locally", "floor", msg.Event.Floor, "button", msg.Event.Button)
			lmChans.lightOffChan <- msg.Event

			// This is a local light off request, so broadcast it to all nodes
			bcastMsg := types.Message{
				Type:     types.BcastLightOff,
				Event:    msg.Event,
				SenderID: elevMgr.GetState().NodeID,
			}
			slog.Debug("Broadcasting light OFF message to other nodes", "floor", msg.Event.Floor, "button", msg.Event.Button)
			lmChans.lightBcastOutCh <- bcastMsg
		}
	case types.BcastLightOff:
		if msg.Event.Button != types.BT_Cab {
			slog.Info("Received broadcast light OFF message", "floor", msg.Event.Floor, "button", msg.Event.Button)
			lmChans.bcastLightOffChan <- msg.Event
		}
	}
}

// lightManager handles all hall light events for synchronization across nodes.
func lightManager(lmChans *LightManagerChannels) {
	hallLightStates := make(map[types.ButtonEvent]bool)

	for {
		select {
		case event := <-lmChans.lightOnChan:
			if event.Button != types.BT_Cab {
				hallLightStates[event] = true
				slog.Debug("Hall light turned ON", "floor", event.Floor, "button", event.Button)
				elevio.SetButtonLamp(event.Button, event.Floor, true)
			}

		case event := <-lmChans.lightOffChan:
			if event.Button != types.BT_Cab {
				hallLightStates[event] = false
				slog.Debug("Hall light turned OFF locally", "floor", event.Floor, "button", event.Button)
				elevio.SetButtonLamp(event.Button, event.Floor, false)
			}

		case event := <-lmChans.bcastLightOffChan:
			if event.Button != types.BT_Cab {
				hallLightStates[event] = false
				slog.Debug("Hall light turned OFF (broadcast)", "floor", event.Floor, "button", event.Button)
				elevio.SetButtonLamp(event.Button, event.Floor, false)
			}
		}
	}
}
