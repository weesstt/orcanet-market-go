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
 *
 */
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	pb "orcanet/market"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	port = flag.Int("port", 50051, "The server port")
)

// I think we can delete this
// maps file hashes to a list of requests
var requests = make(map[string][]*pb.FileRequest)

// map file hashes to supplied files + prices
var files = make(map[string][]*pb.SupplyFile)

// print the current requests map
func printRequestsMap() {
	for hash, users := range requests {
		fmt.Printf("\nFile Hash: %s\n", hash)

		for _, req := range users {
			user := req.GetUser()
			fmt.Println("Username: ", user.GetName())
		}
	}
}

// print the current holders map
func printHoldersMap() {
	for hash, holders := range files {
		fmt.Printf("\nFile Hash: %s\n", hash)

		for _, holder := range holders {
			user := holder.GetUser()
			fmt.Printf("Username: %s, Price: %d\n", user.GetName(), user.GetPrice())
		}

	}
}

type server struct {
	pb.UnimplementedMarketServer
}

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterMarketServer(s, &server{})
	log.Printf("Server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Error %v", err)
	}
}

// I think we can modify this instead of adding to the map 
// Add a request that a user with userId wants file with a hash
func (s *server) RequestFile(ctx context.Context, in *pb.FileRequest) (*pb.FileResponse, error) {
	hash := in.GetFileHash()

	// Check if file is held by anyone; I hate Go
	if _, ok := files[hash]; !ok {
		return &pb.FileResponse{Exists: false, Message: "File not found"}, nil
	}

	requests[hash] = append(requests[hash], in)

	return &pb.FileResponse{Exists: true, Message: "OK"}, nil
}

// I think we can delete this
// Get a list of userIds who are requesting a file with a hash
func (s *server) CheckRequests(ctx context.Context, in *pb.CheckRequest) (*pb.Requests, error) {
	hash := in.GetFileHash()
	printRequestsMap()

	reqs := requests[hash]
	return &pb.Requests{Requests: reqs}, nil
}

// register that the a user holds a file, then add the user to the list of file holders
func (s *server) RegisterFile(ctx context.Context, in *pb.SupplyFile) (*emptypb.Empty, error) {
	hash := in.GetFileHash()

	/*
		idk if we need this since many people should be able to hold the same file

		// Check if file is held by anyone, don't do anything
		// TODO: perform blockchain transaction here
		if _, ok := supplies[fileId]; ok {
			return &emptypb.Empty{}, nil
		}

	*/

	files[hash] = append(files[hash], in)
	fmt.Printf("Num of registered files: %d\n", len(files[hash]))
	return &emptypb.Empty{}, nil
}

// CheckHolders returns a list of user names holding a file with a hash
func (s *server) CheckHolders(ctx context.Context, in *pb.CheckHolder) (*pb.Holders, error) {
	hash := in.GetFileHash()

	holders := files[hash]
	printHoldersMap()

	return &pb.Holders{Holders: holders}, nil
}
