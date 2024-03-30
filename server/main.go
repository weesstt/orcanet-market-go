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
	"sync"
	"time"

	pb "orcanet/market"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	record "github.com/libp2p/go-libp2p-record"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/multiformats/go-multiaddr"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"regexp"
	"errors"
	"github.com/golang/protobuf/proto"
)

// Create a record validator to store our own values within our defined protocol
type OrcaValidator struct{}

type server struct {
	pb.UnimplementedMarketServer
	kDHT dht.IpfsDHT
}

var (
	port = flag.Int("port", 50051, "The server port"),
)

// map file hashes to supplied files + prices
//TODO: change this to be the DHT instance
var files = make(map[string][]*pb.RegisterFileRequest)

func (v OrcaValidator) Validate(key string, value []byte) error{
	// verify key is a sha256 hash
    hexPattern := "^[a-fA-F0-9]{64}$"
    regex := regexp.MustCompile(hexPattern)
    if !regex.MatchString(key) {
		return errors.New("Provided key is not in the form of a SHA-256 digest!")
	}

	pubKeySet := make(map[string] bool)

	for i := 0; i < len(value); i++ {
		messageLength := uint16(value[1]) << 8 | uint16(value[0])
		digitalSignatureLength := uint16(value[3]) << 8 | uint16(value[2])
		contentLength := messageLength + digitalSignatureLength
		user := &pb.User{}

		err := proto.Unmarshal(value[i + 4:i + 4 + int(messageLength)], user) //will parse bytes only until user struct is filled out
		if err != nil {
			return err
		}

		if pubKeySet[string(user.GetId())] == true {
			return errors.New("Duplicate record for the same public key found!")
		}else{
			pubKeySet[string(user.GetId())] = true
		}

		userMessageBytes := value[i + 4:i + 4 + int(messageLength)]

		publicKey, err := crypto.UnmarshalRsaPublicKey(user.GetId())
		if err != nil{
			return err
		}

		signatureBytes := value[i + 4 + int(messageLength):i + 4 + int(contentLength)]
		valid, err := publicKey.Verify(userMessageBytes, signatureBytes) //this function will automatically compute hash of data to compare signauture
		
		if err != nil {
			return err
		}

		if !valid {
			return errors.New("Signature invalid!")
		}

		i = i + 4 + int(contentLength) - 1
	}

	return nil
}

//We will select the best value based on the longest, valid chain
func (v OrcaValidator) Select(key string, value [][]byte) (int, error){
	max := 0
	maxIndex := 0
	for i := 0; i < len(value); i++ {
		if len(value[i]) > max {
			//validate chain 
			if OrcaValidator.Validate(key, value[i]) == nil {
				max = len(value[i])
				maxIndex = i
			}
		}
	}

	return maxIndex, nil;
}

func main() {
	//TODO: read from file bootstrap.peers to get peers
	bootstrapPeer := ""
	ctx := context.Background()

	//Generate private key for peer, 
	//TODO: add flag option to allow user to specify public/private key files instead of generating one
	privKey, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	if err != nil {
		panic(err)
	}

	//Construct multiaddr from string and create host to listen on it
	sourceMultiAddr, _ := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/44981")
	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(sourceMultiAddr.String()),
		libp2p.Identity(privKey), //derive id from private key
	}
	host, err := libp2p.New(opts...)
	if err != nil {
		panic(err)
	}

	log.Printf("Host ID: %s", host.ID())
	log.Printf("Connect to me on:")
	for _, addr := range host.Addrs() {
		log.Printf("%s/p2p/%s", addr, host.ID())
	}

	//An array if we want to expand to a more stable peer list instead of providing in args
	bootstrapPeers := []string{
		bootstrapPeer,
	}

	// Start a DHT, for use in peer discovery. We can't just make a new DHT
	// client because we want each peer to maintain its own local copy of the
	// DHT, so that the bootstrapping node of the DHT can go down without
	// inhibiting future peer discovery.
	var validator record.Validator = OrcaValidator{}
	var options []dht.Option
	// no need for if statement to check if client is peer ? unless the testclient is also
	// supposed to be removed
	options = append(options, dht.Mode(dht.ModeServer))
	options = append(options, dht.ProtocolPrefix("orcanet/market"), dht.Validator(validator))
	kDHT, err := dht.New(ctx, host, options...)
	if err != nil {
		panic(err)
	}

	// Bootstrap the DHT. In the default configuration, this spawns a Background
	// thread that will refresh the peer table every five minutes.
	log.Println("Bootstrapping the DHT")
	if err = kDHT.Bootstrap(ctx); err != nil {
		panic(err)
	}

	// Let's connect to the bootstrap nodes first. They will tell us about the
	// other nodes in the network.
	var wg sync.WaitGroup
	for _, peerAddrString := range bootstrapPeers {
		if peerAddrString == "" {
			continue
		}
		peerAddr, err := multiaddr.NewMultiaddr(peerAddrString)
		if err != nil {
			panic(err)
		}
		peerinfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := host.Connect(ctx, *peerinfo); err != nil {
				log.Println("WARNING: ", err)
			} else {
				log.Println("Connection established with bootstrap node:", *peerinfo)
			}
		}()
	}
	wg.Wait()

	go discoverPeers(ctx, host, kDHT, "orcanet/market")
	time.Sleep(time.Second * 5)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(err)
	}

	s := grpc.NewServer()
	serverStruct = server{}
	serverStruct.kDHT = kDHT;
	pb.RegisterMarketServer(s, &serverStruct)
	log.Printf("Server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Error %v", err)
	}

	select {}

}

// register that the a user holds a file, then add the user to the list of file holders
func (s *server) RegisterFile(ctx context.Context, in *pb.RegisterFileRequest) (*emptypb.Empty, error) {
	hash := in.GetFileHash()
	holders, err := server.kDHT.SearchValue(ctx, "orcanet/market/" + hash);
	if(err != nil){
		return nil, errors.New("Could not retrieve holders: " + err);
	}

	files[hash] = append(files[hash], in)
	fmt.Printf("Num of registered files: %d\n", len(files[hash]))
	return &emptypb.Empty{}, nil
}

// CheckHolders returns a list of user names holding a file with a hash
func (s *server) CheckHolders(ctx context.Context, in *pb.CheckHoldersRequest) (*pb.HoldersResponse, error) {
	hash := in.GetFileHash()
	holders, err := server.kDHT.SearchValue(ctx, "orcanet/market/" + hash);
	if(err != nil){
		return nil, errors.New("Could not retrieve holders: " + err);
	}

	users := make([]*pb.User, len(holders))
	for i := 0; i < len(holders); i++ {
		messageLength := uint16(value[1]) << 8 | uint16(value[0])
		digitalSignatureLength := uint16(value[3]) << 8 | uint16(value[2])
		contentLength := messageLength + digitalSignatureLength
		user := &pb.User{}

		err := proto.Unmarshal(value[i + 4:i + 4 + int(messageLength)], user) //will parse bytes only until user struct is filled out
		if err != nil {
			return err
		}

		users = append(users, user);
		i = i + 4 + int(contentLength) - 1
	}

	return &pb.HoldersResponse{Holders: users}, nil
}

func discoverPeers(ctx context.Context, h host.Host, kDHT *dht.IpfsDHT, advertise string) {
	routingDiscovery := drouting.NewRoutingDiscovery(kDHT)
	dutil.Advertise(ctx, routingDiscovery, advertise)

	// Look for others who have announced and attempt to connect to them
	for {
		fmt.Println("Searching for peers...")
		peerChan, err := routingDiscovery.FindPeers(ctx, advertise)
		if err != nil {
			panic(err)
		}
		for peer := range peerChan {
			if peer.ID == h.ID() {
				continue // No self connection
			}
			err := h.Connect(ctx, peer)
			if err != nil {
				fmt.Printf("Failed connecting to %s, error: %s\n", peer.ID, err)
			} else {
				fmt.Println("Connected to:", peer.ID)
			}
		}
		time.Sleep(time.Second * 10)
	}
}
