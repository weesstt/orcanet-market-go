package main

import (
	"fmt"
	"flag"
	"context"
	"net/http"
	"time"
	"encoding/json"
	"io/ioutil"
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "github.com/weesstt/starfish-market"
	"google.golang.org/grpc/metadata"
)

type PublicIPResp struct {
	DNS string `json:"DNS"`
	EDNS string `json:"EDNS"`
	HTTP string `json:"HTTP"`
}

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

	status := 1

	fmt.Println("OrcaNet Market Server: Consumer")

	//Create client to interact with gRPC server
	client := pb.NewMarketClient(conn)
	pubIP, err := GetPublicIP()

	fmt.Println("1) Query the producer market for asks of specific data")
	fmt.Println("2) Initiate a transaction with a producer for specific data")
	fmt.Println("3) Exit")

	if(err != nil){
		fmt.Println("Could not start producer client because public IP could not be retrieved.\nPlease try again.")
		return 
	}

	md := metadata.Pairs(
		"pubIP", pubIP,
	)

	ctx := metadata.NewOutgoingContext(context.Background(), md)

	for {
		if(status == 0){
			break
		}

		fmt.Print("Please choose an option: ")

		var option int
		_, err = fmt.Scanf("%d", &option)
		if(err != nil){
			fmt.Println("Invalid option, please try again")
			continue
		}

		switch option {
		case 1: 
			
			fmt.Print("Please enter the data identifier you want to query the market for: ")
			var identifier string
			_, err = fmt.Scanf("%s", &identifier)
			if(err != nil){
				fmt.Println("Invalid identifier please try again")
				continue
			}

			//Prepare gRPC call to query market for asks
			marketQueryArgs := &pb.MarketQueryArgs{
				Identifier: identifier,
			}

			marketResp, err := client.ConsumerMarketQuery(ctx, marketQueryArgs)
			if(err != nil){
				fmt.Println("Error executing consumer market query request RPC:", err)
				continue
			}

			fmt.Println("Current market asks for " + identifier + ": ")
			list := marketResp.GetOffers()

			if(len(list) == 0){
				fmt.Println("There are currently no producers serving that data!")
				continue
			}

			for i := 0; i < len(list); i++ {
				ask := list[i]
				fmt.Printf("%s: %f\n", ask.GetProducerPubIP(), ask.GetBid())
			}

		case 2:
			fmt.Print("Please enter the identifier of the data you want to request: ")
			var identifier string
			_, err = fmt.Scanf("%s", &identifier)
			if(err != nil){
				fmt.Println("Invalid identifier please try again")
				continue
			}

			//Prepare gRPC call to query market for asks
			marketQueryArgs := &pb.MarketQueryArgs{
				Identifier: identifier,
			}

			marketResp, err := client.ConsumerMarketQuery(ctx, marketQueryArgs)
			if(err != nil){
				fmt.Println("Error executing consumer market query request RPC:", err)
				continue
			}

			fmt.Println("Current market asks for " + identifier + ": ")
			list := marketResp.GetOffers()

			if(len(list) == 0){
				fmt.Println("There are currently no producers serving that data!")
				continue
			}

			for i := 0; i < len(list); i++ {
				ask := list[i]
				fmt.Printf("%d) %s, %f\n", i, ask.GetProducerPubIP(), ask.GetBid())
			}

			var producerOption int
			for {
				fmt.Print("Please choose a producer to start a transaction with: ")
				_, err = fmt.Scanf("%d", &producerOption)
				if(err != nil || producerOption < 0 || producerOption > len(list)){
					fmt.Printf("Invalid option! Must be between 0 and %d\n", len(list) - 1)
					continue
				}
				break
			}

			chosenAsk := list[producerOption]
			chosenAsk.ConsumerPubIP = pubIP
			
			//Call InitiateMarketTransaction gRPC service
			fmt.Println("Waiting for producer to accept transaction, will timeout after 1 minute")
			resp, err := client.InitiateMarketTransaction(ctx, chosenAsk)
			if(err != nil){
				fmt.Println("Error executing initiate transaction request RPC:", err)
				continue
			}

			fmt.Printf("Please retrieve data from: %s\n", resp.GetURL())

			var transactionID string
			for {
				fmt.Print("Please send payment and enter the transaction ID on OrcaNet: ")
				_, err = fmt.Scanf("%s", &transactionID)
				if(err != nil){
					fmt.Printf("Invalid input, please try again!\n")
					continue
				}
				break
			}

			//Call FinalizeMarketTransaction gRPC
			receiptMD := metadata.Pairs(
				"transactionIdentifier", transactionID,
			)
		
			receiptCtx := metadata.NewOutgoingContext(context.Background(), receiptMD)

			_, err = client.FinalizeMarketTransaction(receiptCtx, chosenAsk)
			if(err != nil){
				fmt.Println("Error executing finalizing transaction request RPC:", err)
				continue
			}

			fmt.Println("Finalized transaction!")

		default:
			fmt.Println("Invalid option, please try again")
			continue
		}
	}
	
	return
}

//Makes a GET request to www.mapper.ntppool.org/json to get public ip address of consumer
func GetPublicIP() (string, error) {
	url := "http://www.mapper.ntppool.org/json"

	resp, err := http.Get(url)

	attempts := 0
	for {
		if(err != nil){
			if(attempts == 10){
				return "nil", errors.New("Could not query server for public IP!")
			}

			time.Sleep(time.Second * 1)
			resp, err = http.Get(url)
			attempts++
			continue
		}
		break
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "nil", errors.New("Error reading server response for public IP!")
	}

	var bodyJSON PublicIPResp
	err = json.Unmarshal(body, &bodyJSON)
	if err != nil {
		return "nil", errors.New("There was an error parsing json response from public ip retrieval")
	}

	return bodyJSON.HTTP, nil
}