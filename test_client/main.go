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
	var price int64
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
		fmt.Println("1. Register a file")
		fmt.Println("2. Check holders for a file")
		fmt.Println("3. Exit")
		fmt.Print("Option: ")
		var choice int
		_, err := fmt.Scanln(&choice)
		if err != nil {
			fmt.Println("Error: ", err)
			continue
		}

		if choice == 3 {
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
			registerFile(c, user, fileHash)
		case 2:
			checkHolders(c, user, fileHash)
		case 3:
			return
		default:
			fmt.Println("Unknown option: ", choice)
		}

		fmt.Println()
	}
}

// print all users who are holding a file with fileHash
func checkHolders(c pb.MarketClient, user *pb.User, fileHash string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	holders, err := c.CheckHolders(ctx, &pb.CheckHoldersRequest{FileHash: fileHash})
	if err != nil {
		log.Fatalf("Error: %v", err)
		return
	}
	supply_files := holders.GetHolders()
	slices.SortFunc(supply_files, func(a, b *pb.User) int {
		return cmp.Compare(a.GetPrice(), b.GetPrice())
	})
	for idx, holder := range supply_files {
		fmt.Printf("(%d) Username: %s, Price: %d\n", idx, holder.GetName(), holder.GetPrice())
	}

}

func registerFile(c pb.MarketClient, user *pb.User, fileHash string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := c.RegisterFile(ctx, &pb.RegisterFileRequest{User: user, FileHash: fileHash})
	if err != nil {
		log.Fatalf("Error: %v", err)
	} else {
		log.Printf("Success")
	}
}
