package main

import (
	"fmt"
	"flag"
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "github.com/weesstt/starfish-market"
)

var (
	serverAddr = flag.String("addr", "localhost:50051", "The server address in the format of host:port")
)

func main() {
	//Set up connection to gRPC server
	var opts []grpc.DialOption //Can be used to set up auth credentials in the future 
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials())) //we will just use tcp for now
	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		fmt.Println("Error connecting to server:", err)
		return
	}
	defer conn.Close()

	//Get user input for options
	var requestDigest string
	fmt.Print("Enter file digest to request: ")
	_, err = fmt.Scanln(&requestDigest)

    if err != nil {
        fmt.Println("Error reading file digest:", err)
        return
    }

	var bid float32
	fmt.Print("Enter bid to request file: ")
	_, err = fmt.Scanf("%f", &bid)

    if err != nil {
        fmt.Println("Error reading user bid:", err)
        return
    }

	//Prepare RPC ConsumerRetrieveRequest Argument
	request := &pb.MarketRequestArgs{
		Bid: bid,
		FileDigest: requestDigest,
	}

	//Create client to interact with gRPC server
	client := pb.NewMarketClient(conn)
	requestInfo, err := client.ConsumerRetrieveRequest(context.Background(), request)
	if(err != nil){
		fmt.Println("Error executing consumer retrieve request RPC:", err)
		return;
	}

	// Print the result
	fmt.Println("Transaction UUID: " + requestInfo.GetUuid())
}