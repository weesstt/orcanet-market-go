package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"github.com/google/uuid"
	"errors"
	"google.golang.org/grpc"
	pb "github.com/weesstt/starfish-market"
)

//Command line argument to specify the port to run the RPC server on.
var (
	port = flag.Int("port", 50051, "The server port")
)

// marketServer is used to implement market.MarketServer.
type marketServer struct {
	pb.UnimplementedMarketServer
	RequestMap map[string][]*pb.MarketRequestInfo
}

func main(){
	//Set up RPC server to listen on localhost with specified port
	flag.Parse();
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterMarketServer(grpcServer, &marketServer{
		RequestMap: make(map[string][]*pb.MarketRequestInfo),
	})
	log.Println(fmt.Sprintf("Serving gRPC on 0.0.0.0:%d", *port))
	log.Fatal(grpcServer.Serve(lis))
}


//RPC Consumer request to the market to retrieve a file
func (m *marketServer) ConsumerRetrieveRequest(ctx context.Context, in *pb.MarketRequestArgs) (*pb.MarketRequestInfo, error) {
	if in.Bid <= 0 {
		return nil, errors.New("Market bid must be greater than 0 orca coins")
	}

	//Set up response to client with information regarding the new request
	resp := &pb.MarketRequestInfo{
		Bid: in.GetBid(),
		FileDigest: in.GetFileDigest(),
		Uuid: uuid.New().String(),
		PubKey: "TODO",
	}

	//Add the new request to the market server table
	_, exists := m.RequestMap[in.GetFileDigest()]
	if !exists {
		m.RequestMap[in.GetFileDigest()] = []*pb.MarketRequestInfo{}
	} 
	m.RequestMap[in.GetFileDigest()] = append(m.RequestMap[in.GetFileDigest()], resp)

	fmt.Println("Received File Request for digest: " + resp.FileDigest)
	fmt.Printf("Bid: %f\n", resp.Bid)
	fmt.Printf("PubKey: %s\n", resp.PubKey)
	fmt.Printf("UUID: %s\n", resp.Uuid)
	fmt.Println();

	return resp, nil
}

//RPC Producer query to the market to see requests for a specific file
func (m *marketServer) ProducerQuery(ctx context.Context, in *pb.MarketQueryArgs) (*pb.MarketQueryList, error) {
	list, exists := m.RequestMap[in.GetFileDigest()]
	
	//If file digest not in market server table then return an empty list.
	if !exists {
		resp := &pb.MarketQueryList{
			Requests: []*pb.MarketRequestInfo{},
		}
		return resp, nil
	} 

	resp := &pb.MarketQueryList{
		Requests: list,
	}

	return resp, nil
}