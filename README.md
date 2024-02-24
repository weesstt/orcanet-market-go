# OrcaNet Market Server 
This is a simple market server implemented in Go using gRPC.
Each consumer can input the hash digest of the data they want to request and see which producers are holding their file and what their asking transfer price is. Producers can see incoming requests for data they hold and choose to serve certain consumer requests. This prototype runs under the assumption that each producer/consumer has their own public IP address which will change in later versions. 

# System Flow
Assume that a producer has already been provided with data that a consumer has requested be stored
on the network. A producer first registers with the market the specific data that they have 
and their asking price for consumers to retrieve the data. A consumer can then query the market and see which
producers are holding specific data and their asking price. A consumer can call the InitiateMarketTransaction
gRPC method on the market server to start a request for specific data. The InitiateMarketTransaction method
will not return a MarketDataTransfer message until a producer agrees to serve the data or an error
occurs because of a timeout. While the consumer is waiting for a return from the server, a producer
will call the ProducerMarketQuery gRPC method to see which consumers want specific data. A producer can
call the ProducerAcceptTransaction method to accept to serve data to a consumer. The ProducerAcceptTransaction will not return a value until a FinalizeMarketTransaction gRPC call is made by the consumer indicating the end of a transaction or there is a timeout. When the ProducerAcceptTransaction method is received by the server, a MarketDataTransfer message will be returned to the consumer by the InitiateMarketTransaction method. At which time the consumer should send the transaction to the OrcaNet blockchain then call the FinalizeMarketTransaction with the transaction ID on the blockchain.

## TODO 
- Producer: Implement a CLI with the following options
    1) Register market ask (Calls RegisterMarketAsk on market server).
    2) List the incoming requests for specific data. 
    3) Accept a certain incoming request for specific data.
- Consumer: Implement a CLI with the following options
    1) Initiate Market Transaction to indicate to a specific producer data is being requested.
    2) Finalize Market Transaction once data is received by consumer and a transaction was done on the blockchain.
- Server 
    1) rpc MarketQuery(MarketQueryArgs) returns (MarketQueries) {}
    2) rpc RegisterMarketAsk(MarketAskArgs) returns (MarketAsk) {}
    3) rpc InitiateMarketTransaction(MarketAsk) returns (MarketDataTransfer) {}
    4) rpc ProducerMarketQuery(MarketQueryArgs) returns (MarketQueries)
    5) rpc ProducerAcceptTransaction(MarketAsk) returns (Receipt) {}
    6) rpc FinalizeMarketTransaction(MarketAsk) returns (Receipt) {} 

### Long Term
- Abstract away notion of single file transfer to be a stream of data. 
- Change process of consumer sending payment once file is received to sending payment for bytes received.
- Connect to blockchain to actually process payments when transactions are being done. 

## Important Best Practices!
gRPC Protocol Buffers can not have repeated field numbers! When removing a field number either by using a different number or removing a field entirely, add a reserved statement to the message with the field number
so that it cannot be reused. Other protobuf coding best practices can be found [here](https://protobuf.dev/programming-guides/dos-donts/).

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

## Usage
Server
```bash
$ cd market/
$ go run server/server.go
```

Consumer
```bash
$ cd market/
$ go run consumer/Consumer.go
```

Producer
```bash
$ cd market/
$ go run producer/producer.go
```