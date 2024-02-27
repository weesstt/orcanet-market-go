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
	"cmp"
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"slices"
	"strconv"
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
	userID := fmt.Sprintf("user%d", rand.Intn(10000))

	fmt.Print("Enter a price for supplying files: ")
	var price int32
	_, err = fmt.Scanln(&price)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	// Create a User struct with the provided username and generated ID
	user := &pb.User{
		Id:    userID,
		Name:  username,
		Ip:    "localhost",
		Port:  416320,
		Price: price,
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
		_, err := fmt.Scanln(&choice)
		if err != nil {
			fmt.Println("Error: ", err)
			continue
		}

		if choice == 5 {
			return
		}

		fmt.Print("Enter a file hash: ")
		var fileHash string
		_, err = fmt.Scanln(&fileHash)
		if err != nil {
			fmt.Println("Error: ", err)
			continue
		}

		switch choice {
		case 1:
			createRequest(c, user, fileHash)
		case 2:
			registerRequest(c, user, fileHash)
		case 3:
			checkRequests(c, fileHash)
		case 4:
			checkHolders(c, user, fileHash)
		case 5:
			return
		default:
			fmt.Println("Unknown option: ", choice)
		}

		fmt.Println()
	}
}

// creates a request that a user with userId wants a file with fileHash
func createRequest(c pb.MarketClient, user *pb.User, fileHash string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r, err := c.RequestFile(ctx, &pb.FileRequest{User: user, FileHash: fileHash})
	if err != nil {
		log.Fatalf("Error: %v", err)
	} else {
		log.Printf("Result: %t, %s", r.GetExists(), r.GetMessage())
	}
}

// I think we can delete this
// get all users who wants a file with fileHash
func checkRequests(c pb.MarketClient, fileHash string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	reqs, err := c.CheckRequests(ctx, &pb.CheckRequest{FileHash: fileHash})
	if err != nil {
		log.Fatalf("Error: %v", err)
	} else {
		for _, req := range reqs.GetRequests() {
			user := req.GetUser()
			log.Printf("Username: %s", user.GetName())
		}
	}
}

// print all users who are holding a file with fileHash
func checkHolders(c pb.MarketClient, user *pb.User, fileHash string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	holders, err := c.CheckHolders(ctx, &pb.CheckHolder{FileHash: fileHash})
	if err != nil {
		log.Fatalf("Error: %v", err)
		return
	}
	supply_files := holders.GetHolders()
	slices.SortFunc(supply_files, func(a, b *pb.SupplyFile) int {
		return cmp.Compare(a.GetUser().GetPrice(), b.GetUser().GetPrice())
	})
	for idx, holder := range supply_files {
		user := holder.GetUser()
		fmt.Printf("(%d) Username: %s, Price: %d\n", idx, user.GetName(), user.GetPrice())
	}

	fmt.Println("Choose which supplier to get file from, or 'n' to cancel:")
	var choice string
	_, err = fmt.Scanln(&choice)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	idx, err := strconv.ParseInt(choice, 10, 32)
	if err != nil {
		return
	}
	if idx < 0 || int(idx) > len(supply_files) {
		fmt.Println("Invalid index chosen")
		return
	}
	fmt.Printf("%v chosen, requesting file\n", idx)
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	r, err := c.RequestFile(ctx, &pb.FileRequest{User: user, FileHash: fileHash})
	if err != nil {
		log.Fatalf("Error: %v", err)
	} else {
		log.Printf("Result: %t, %s", r.GetExists(), r.GetMessage())
	}

}

func registerRequest(c pb.MarketClient, user *pb.User, fileHash string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := c.RegisterFile(ctx, &pb.SupplyFile{User: user, FileHash: fileHash})
	if err != nil {
		log.Fatalf("Error: %v", err)
	} else {
		log.Printf("Success")
	}
}
