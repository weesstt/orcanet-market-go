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

	fmt.Println("OrcaNet Market Server: Producer")

	//Create client to interact with gRPC server
	client := pb.NewMarketClient(conn)
	pubIP, err := GetPublicIP()

	fmt.Println("1) Register a market ask for specific data")
	fmt.Println("2) Query the market to see incoming transactions for registered market asks")
	fmt.Println("4) Exit")

	if(err != nil){
		fmt.Println("Could not start producer client because public IP could not be retrieved.\nPlease try again.")
		return 
	}

	md := metadata.Pairs(
		"pubIP", pubIP,
		"webResource", pubIP + ":80",
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
			
			fmt.Print("Please enter the data identifier you want to register on the market: ")
			var identifier string
			_, err = fmt.Scanf("%s", &identifier)
			if(err != nil){
				fmt.Println("Invalid identifier please try again")
				continue
			}

			var bid float32
			for {
				fmt.Print("Please enter the asking price: ")
				_, err = fmt.Scanf("%f", &bid)
				if(err != nil || bid < 0){
					fmt.Println("Asking price must be greater than 0 and a float!")
					continue
				}
				break
			}

			//Prepare gRPC call to register market ask
			marketAskArgs := &pb.MarketAskArgs{
				Identifier: identifier,
				Bid: bid,
			}

			marketAskResp, err := client.RegisterMarketAsk(ctx, marketAskArgs)
			if(err != nil){
				fmt.Println("Error executing producer register request RPC:", err)
				return;
			}

			fmt.Println("Successfully registered market ask for data identifier " + marketAskResp.GetIdentifier())

		case 2:
			fmt.Print("Please enter the data identifier you want to query the market for: ")
			var identifier string
			_, err = fmt.Scanf("%s", &identifier)
			if(err != nil){
				fmt.Println("Invalid identifier please try again")
				continue
			}

			//Prepare gRPC call to producer query market 
			marketQueryArgs := &pb.MarketQueryArgs{
				Identifier: identifier,
			}

			marketQueryResp, err := client.ProducerMarketQuery(ctx, marketQueryArgs)
			if(err != nil){
				fmt.Println("Error executing producer market query request RPC:", err)
				return;
			}

			fmt.Println("Incoming transactions: ")
			list := marketQueryResp.GetOffers()

			if(len(list) == 0){
				fmt.Println("There are currently no consumers requesting that data!")
				continue
			}

			for i := 0; i < len(list); i++ {
				ask := list[i]
				fmt.Printf("%d) %s\n", i, ask.GetConsumerPubIP())
			}

			var consumerOption int
			for {
				fmt.Print("Please choose a consumer to serve (-1 to exit): ")
				_, err = fmt.Scanf("%d", &consumerOption)
				if(err != nil || consumerOption < -1 || consumerOption > len(list)){
					fmt.Printf("Invalid option! Must be between 0 and %d\n", len(list) - 1)
					continue
				}
				break
			}

			if(consumerOption == -1){continue}

			fmt.Print("Please enter the path where the data is stored: ") 

			var path string
			_, err = fmt.Scanf("%s", &path)
			if(err != nil){
				fmt.Printf("Invalid path!\n")
				continue
			}

			//TODO: Move specific data into web root of exposed web server 

			//Prepare gRPC call to accept transaction
			fmt.Println("Accepted transaction and waiting for receipt, this will timeout after 1 minute")
			receipt, err := client.ProducerAcceptTransaction(ctx, list[consumerOption])
			if(err != nil){
				fmt.Println("Error executing producer accept transaction request RPC:", err)
				return;
			}

			fmt.Println("Transaction finalized. OrcaNet transaction ID: " + receipt.GetIdentifier())

		default:
			fmt.Println("Invalid option, please try again")
			continue
		}
	}
	return
}

//Makes a GET request to www.mapper.ntppool.org/json to get public ip address of producer
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

