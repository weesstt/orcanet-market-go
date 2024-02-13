package main

import (
	"fmt"
	"net/rpc"
	"market/service"
)

func main() {
	// Dial the RPC server
	client, err := rpc.Dial("tcp", "localhost:1234")
	if err != nil {
		fmt.Println("Error connecting to server:", err)
		return
	}
	defer client.Close()

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

	request := service.MarketRequestArgs{
		Bid: bid,
		FileDigest: requestDigest,
	}

	var requestInfo service.MarketRequestInfo

	err = client.Call("Market.ConsumerRequest", &request, &requestInfo)
	if err != nil {
		fmt.Println("Error creating consumer request:", err)
		return
	}

	// Print the result
	fmt.Println("Transaction UUID: " + requestInfo.Identifier.String())
}