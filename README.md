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

The system uses a peer to peer topology. On hall orders, it computes a local cost and stores it in a map, and broadcasts a bid. Received bids are stored in the same map. Once the number of stored bids are equal to the number of elevators, the elevator takes the order if it has the lowest cost. It implements fault tolerant mechanisms such as restoring lost cab orders, overtaking of lost hall orders and a bid timeout mechanism.
