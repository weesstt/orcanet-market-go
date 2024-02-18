# OrcaNet Market Server 

This is a simple market server implemented in Go, utilizing the built-in Go RPC module.
Given that this is a prototype that will be implemented later with the blockchain, there is no
concept of a "user account". Instead, each consumer can input the hash digest of the file they want to request and their bid to retrieve said file. Whenever a bid is placed on a file, a UUID is created to identify said transation. The UUID is generated using the Go UUID Module and is guarenteed to be unique across space and time. Producers can then query the table to see if there are currently any bids for a certain hash digest that they may hold. 

# Server Usage

```bash
cd server/
go run server.go
```

# Consumer Usage

```bash
cd consumer/
go run server.go
```

# Producer Usage

```bash
cd consumer/
go run server.go
```