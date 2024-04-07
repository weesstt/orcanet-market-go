# Go Market Server

## Team Sea Chicken 🐔 and Starfish ⭐️

A DHT implementation of the OrcaNet market server, built using Go and [gRPC](https://grpc.io/docs/languages/go/quickstart).

## Setup

1. Install [Go](https://go.dev/doc/install)
   * Ensure Go executables are available on PATH, (e.g. at `~/go/bin`)
2. Install protoc:

   `apt install protobuf-compiler`

   (May require more a [more recent version](https://grpc.io/docs/protoc-installation/#install-pre-compiled-binaries-any-os))
3. Install protoc-gen-go:

   `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest`

4. Install protoc-gen-go-grpc:

   `go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest`

## Running

To run the market server:

```Shell
go run server/main.go
```

To run a test client:

```Shell
go run test_client/main.go
```

To compile the protobuf at `market/market.proto`:

```Shell
protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    ./market.proto
```
