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
	fmt.Print("Enter file digest to see requests: ")
	_, err = fmt.Scanln(&requestDigest)

    if err != nil {
        fmt.Println("Error reading file digest:", err)
        return
    }

	query := service.MarketQueryArgs{
		FileDigest: requestDigest,
	}

	var queryResp service.MarketQueryResp

	err = client.Call("Market.ProducerQuery", &query, &queryResp)
	if err != nil {
		fmt.Println("Error querying market:", err)
		return
	}

	// Print the result
	length := len(queryResp.Result)
	if length == 0 {
		fmt.Println("There are no requests for the specified file digest!")
	} else {
		fmt.Println("Requests for file digest: " + query.FileDigest)
		for i := 0; i < length; i++ {
			fmt.Printf("%d: Bid - %f\n", i, queryResp.Result[i].Bid)
		}
	}


}