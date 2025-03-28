## multivator
Real-time system for multiple elevators, programmed in Go.

## How to run

Modify ```src/config/config.go``` NumElevators constant to the number of elevators desired.
Example terminal commands for two elevators:

```bash
cd lib/simulator && ./SimElevatorServer.exe --port 17400
```
```bash
cd lib/simulator && ./SimElevatorServer.exe --port 17401
```
```bash
go run src/main.go --id 0
```
```bash
go run src/main.go --id 1
```

## Description

The system uses a peer to peer topology.

On hall orders:
  1. Executor receives button press and sends it to dispatcher
  2. If we are alone, send the order back to executor. Else compute cost, store it in a map, and broadcast it as a bid. Also store received bids from other nodes.
  3. Once number of stored bids are equal to number of connected peers, assign the order the the peer with the lowest bid. Send the order from the dispatcher back to the executor.

Examples of fault tolerance mechanisms (assuming at least one peer is connected):
  - Restore lost cab orders through the network.
  - Overtake hall orders if an assigned peer disconnects.
  - If all bids are not received within a specified time, assign the order to itself.
