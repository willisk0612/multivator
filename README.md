## multivator
Real-time system for multiple elevators, programmed in Go.

## How to run

Modify ```src/config/config.go``` numPeers constant to the number of elevators desired.
Example terminal commands for two elevators:

```bash
cd lib/simulator && ./SimElevatorServer.exe --port 15657
```
```bash
cd lib/simulator && ./SimElevatorServer.exe --port 15658
```
```bash
go run src/main.go --id 0
```
```bash
go run src/main.go --id 1
```

## Description

### System Architecture

The system uses a peer to peer topology. It utilizes UDP broadcasting, with the possibility to send a burst of messages in case of packet loss. The  various parameters in ```src/config/config.go``` for configuring/tuning the system.

### Mechanisms
#### Bidding
1. On hall orders, initial bids (button events with costs) are broadcasted.
2. The other nodes respond with a reply bid.
3. Once the number of received bids are equal to the number of peers in the network, it locally chooses an assignee to take the hall order.

#### Synchronization

  - Orders are stored in a nested array in the ```ElevState``` struct in ```src/types/elev_types.go```. It stores all the cab/hall orders for all the elevators in the system.
  - On hall arrivals, hall and cab orders are synchronized. Cab orders are restored from the network upon reconnection.

