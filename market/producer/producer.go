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

	//Get user input for market query
	var requestDigest string
	fmt.Print("Enter file digest to see requests: ")
	_, err = fmt.Scanln(&requestDigest)
    if err != nil {
        fmt.Println("Error reading file digest:", err)
        return
    }

	//Prepare RPC Producer Query Arguments
	query := &pb.MarketQueryArgs{
		FileDigest: requestDigest,
	}

	//Create client to interact with gRPC server
	client := pb.NewMarketClient(conn)
	queryResp, err := client.ProducerQuery(context.Background(), query)
	if(err != nil){
		fmt.Println("Error executing consumer retrieve request RPC:", err)
		return;
	}

	// Print the result
	resultList := queryResp.GetRequests()
	length := len(resultList)
	if length == 0 {
		fmt.Println("There are no requests for the specified file digest!")
	} else {
		fmt.Println("Requests for file digest: " + query.FileDigest)
		for i := 0; i < length; i++ {
			fmt.Printf("%d: Bid - %f\n", i, resultList[i].GetBid())
		}
	}
}