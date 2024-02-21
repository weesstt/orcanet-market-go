# OrcaNet Market Server 
This is a simple market server implemented in Go using gRPC.
Given that this is a prototype that will be implemented later with the blockchain, there is no
concept of a "user account". Instead, each consumer can input the hash digest of the file they want to request and their bid to retrieve said file. Whenever a bid is placed on a file, a UUID is created to identify said transation. The UUID is generated using the Go UUID Module and is guarenteed to be unique across space and time. Producers can then query the table to see if there are currently any bids for a certain hash digest that they may hold. 

## TODO 
- Set up functionality for consumer to specify public key to identify themselves when requesting a file.
- Set up a gRPC service to initiate a file transfer on behalf of the producer and consumers whenever a producer wants to accept a transaction. 

## Needed Clarifications
- This build assumes that multiple, unique people can place bids on the same file and producers can choose which person they ultimately want to serve. Since only the file digest is needed to identify a transaction, anyone with the file digest can request anyone elses file. Is this behavior correct? 

## Important!
- gRPC Protocol Buffers can not have repeated field numbers! When removing a field number either by using a different number or removing a field entirely, add a reserved statement to the message with the field number
so that it cannot be reused. Other protobuf coding best practices can be found [here](https://protobuf.dev/programming-guides/dos-donts/).

## Usage
- Server
```bash
$ cd market/
$ go run server/server.go
```

- Consumer
```bash
$ cd market/
$ go run consumer/Consumer.go
```

- Producer
```bash
$ cd market/
$ go run producer/producer.go
```

## Prerequisites
+ Go [(Installation 📎)](https://go.dev/doc/install)
+ Protocol buffer compiler, protoc. [(Installation 📎)](https://grpc.io/docs/protoc-installation/)
+ Go plugins for protocol buffer. Update path so protoc can find Go plugins.
    ```bash
    $ go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
    $ go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2
    $ export PATH="$PATH:$(go env GOPATH)/bin"
    ```

## Compiling gRPC Protobuf Go Code
```bash
protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    ./market.proto
```
- Must be done every time there is an update to the market.proto file to correctly access stubs within Go.

## RPC Documentation (TODO)
```protobuf
service Market {
    //A simple RPC to make a request to the server to retrieve a file.
    //Message MarketRequestArgs contains the information for the request.
    //Message MarketRequestInfo is returned to indicate that the request was received and contains information
    //about the request such as the bid, the file digest, the uuid of the transaction, and the public key of the requesting person.
    rpc ConsumerRetrieveRequest(MarketRequestArgs) returns (MarketRequestInfo) {}

    //A simple RPC to make a query to the market for a specific file. 
    //Message MarketQueryArgs contains the specific file digest to query. 
    //Message MarketQueryList contains an 'array' of MarketRequestInfo messages which are the current requests.
    rpc ProducerQuery(MarketQueryArgs) returns (MarketQueryList) {}
}

//A message that contains the arguments to make a request to retrieve a file. 
message MarketRequestArgs {
    //The number of OrcaCoins to offer for the transaction
    float bid = 1;

    //The file digest of the file that is desired.
    string fileDigest = 2;
}

//A message that contains the information of a specific market file request. Returned by the ConsumerRetrieveRequest rpc.
message MarketRequestInfo {
    //The number of OrcaCoins offered for the transaction.
    float bid = 1;
    
    //The file digest of the file requested for the transaction.
    string fileDigest = 2;

    //The UUID associated with the transaction.
    string uuid = 3;

    //The public key of the consumer requesting the file. 
    string pubKey = 4;
}

//A message that contains the arguments to query the server to see bids for a specific file. 
message MarketQueryArgs {
    //The digest of the file to query for requests. 
    string fileDigest = 1;
}

//A message that contains the list of market requests. Returned by the ProducerQuery rpc. 
message MarketQueryList {
    //A list of market requests.
    repeated MarketRequestInfo requests = 1;
}
```
