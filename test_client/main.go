/*
 *
 * Copyright 2015 gRPC authors.
 *
 * Modified by Stony Brook University students
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	pb "orcanet/market"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	addr = flag.String("addr", "localhost:50051", "the address to connect to")
)

func main() {
	flag.Parse()
	// Set up a connection to the server.
	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	defer conn.Close()
	c := pb.NewMarketClient(conn)

	// Prompt for username in terminal
	var username string
	fmt.Print("Enter username: ")
	fmt.Scanln(&username)

	// Generate a random ID for new user
	rand.Seed(time.Now().UnixNano())
	userID := fmt.Sprintf("user%d", rand.Intn(10000))

	// Create a User struct with the provided username and generated ID
	user := &pb.User{
		Id:   userID,
		Name: username,
	}

	for {
		fmt.Println("---------------------------------")
		fmt.Println("1. Request a file")
		fmt.Println("2. Register a file")
		fmt.Println("3. Check requests for a file")
		fmt.Println("4. Check holders for a file")
		fmt.Println("5. Exit")
		fmt.Print("Option: ")
		var choice int
		fmt.Scanln(&choice)

		fmt.Print("Enter a fileId: ")
		var fileId string
		fmt.Scanln(&fileId)
		switch choice {
		case 1:
			fmt.Print("Enter a bid: ")
			var bid int
			fmt.Scanln(&bid)

			createRequest(c, user, fileId, bid)
		case 2:
			registerRequest(c, user, fileId)
		case 3:
			checkRequests(c, fileId)
		case 4:
			checkHolders(c, fileId)
		case 5:
			return
		}

		fmt.Println("\n\n")
	}
}

// creates a request that a user with userId wants a file with fileId
func createRequest(c pb.MarketClient, user *pb.User, fileId string, bid int) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r, err := c.RequestFile(ctx, &pb.FileRequest{User: user, FileId: fileId, Bid: int32(bid)})
	if err != nil {
		log.Fatalf("Error: %v", err)
	} else {
		log.Printf("Result: %t, %s", r.GetExists(), r.GetMessage())
	}
}

// get all users who wants a file with fileId
func checkRequests(c pb.MarketClient, fileId string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	reqs, err := c.CheckRequests(ctx, &pb.CheckRequest{FileId: fileId})
	if err != nil {
		log.Fatalf("Error: %v", err)
	} else {
		for _, req := range reqs.GetRequests() {
			user := req.GetUser()
			log.Printf("Username: %s, Bid: %d", user.GetName(), req.GetBid())
		}
	}
}

// print all users who are holding a file with fileId
func checkHolders(c pb.MarketClient, fileId string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	holders, err := c.CheckHolders(ctx, &pb.CheckHolder{FileId: fileId})
	if err != nil {
		log.Fatalf("Error: %v", err)
	} else {
		log.Printf("Holders: %s", holders.GetStrings())
	}
}

func registerRequest(c pb.MarketClient, user *pb.User, fileId string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := c.RegisterFile(ctx, &pb.RegisterRequest{User: user, FileId: fileId})
	if err != nil {
		log.Fatalf("Error: %v", err)
	} else {
		log.Printf("Success")
	}
}
