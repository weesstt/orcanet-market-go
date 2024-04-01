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
 *	References:
 *		https://gist.github.com/upperwal/38cd0c98e4a6b34c061db0ff26def9b9
 *		https://ldej.nl/post/building-an-echo-application-with-libp2p/
 *		https://github.com/libp2p/go-libp2p/blob/master/examples/chat-with-rendezvous/chat.go
 *		https://github.com/libp2p/go-libp2p/blob/master/examples/pubsub/basic-chat-with-rendezvous/main.go
 */


package main

import (
	"context"
	"strings"
	"flag"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"

	pb "orcanet/market"
	"os"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	record "github.com/libp2p/go-libp2p-record"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	// dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
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
	K_DHT *dht.IpfsDHT
	PrivKey crypto.PrivKey
	PubKey crypto.PubKey
	V record.Validator
}

var (
	port = flag.Int("port", 50051, "The server port")
)

// map file hashes to supplied files + prices
//TODO: change this to be the DHT instance
var files = make(map[string][]*pb.RegisterFileRequest)

//Validates keys and values that are being put into the DHT.
//Keys must conform to a sha256 hash
//Values must conform the specification in /server/README.md
func (v OrcaValidator) Validate(key string, value []byte) error{
	// verify key is a sha256 hash
    hexPattern := "^[a-fA-F0-9]{64}$"
    regex := regexp.MustCompile(hexPattern)
    if !regex.MatchString(strings.Replace(key, "orcanet/market/", "", -1)) {
		return errors.New("Provided key is not in the form of a SHA-256 digest!")
	}

	pubKeySet := make(map[string] bool)

	for i := 0; i < len(value) - 4; i++ {
		messageLength := uint16(value[i + 1]) << 8 | uint16(value[i])
		digitalSignatureLength := uint16(value[i + 3]) << 8 | uint16(value[i + 2])
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

    currentTime := time.Now().UTC()
    unixTimestamp := currentTime.Unix()
    unixTimestampInt64 := uint64(unixTimestamp)

	suppliedTime := uint64(0)
	for i := 1; i < 5; i++ {
		suppliedTime = suppliedTime | (uint64(value[len(value) - i]) << (i - 1))
	}

	if(suppliedTime > unixTimestampInt64){
		return errors.New("Supplied time cannot be less than current time")
	}

	return nil
}

//We will select the best value based on the longest, latest, valid chain
func (v OrcaValidator) Select(key string, value [][]byte) (int, error){
	max := 0
	maxIndex := 0
	latestTime := uint64(0);
	for i := 0; i < len(value); i++ {
		if len(value[i]) > max {
			//validate chain 
			if v.Validate(key, value[i]) == nil {
				
				suppliedTime := uint64(0)
				for j := 1; j < 5; j++ {
					suppliedTime = suppliedTime | uint64(value[i][len(value[i]) - j]) << (j - 1)
				}

				if(suppliedTime > latestTime){
					max = len(value[i])
					latestTime = suppliedTime;
					maxIndex = i;
				}
			}
		}
	}

	return maxIndex, nil;
}


func checkOrCreatePrivateKey(path string) (crypto.PrivKey, error) {
	// Check if the privateKey.pem exists
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		// No private key file, so let's create one
		privKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, err
		}
		privKeyBytes := x509.MarshalPKCS1PrivateKey(privKey)
		privKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: privKeyBytes,
		})
		err = ioutil.WriteFile(path, privKeyPEM, 0600)
		if err != nil {
			return nil, err
		}
		log.Println("New private key generated and saved to", path)

		libp2pPrivKey, _, err := crypto.KeyPairFromStdKey(privKey);
		if(err != nil){
			return nil, err
		}

		return libp2pPrivKey, nil;
	} else if err != nil {
		// Some other error occurred when trying to read the file
		return nil, err
	}

	// Private key file exists, let's read it
	privKeyBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(privKeyBytes)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		log.Println("Private key file is of invalid format")
		return nil, errors.New("private key file is of invalid format")
	}
	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	log.Println("Existing private key loaded from", path)

	libp2pPrivKey, _, err := crypto.KeyPairFromStdKey(privKey);
	if(err != nil){
		return nil, err
	}

	return libp2pPrivKey, nil;
}

func main() {
	flag.Parse()
	//TODO: read from file bootstrap.peers to get peers
	ctx := context.Background()

	//Generate private key for peer, 
	privKey, err := checkOrCreatePrivateKey("privateKey.pem");
	if(err != nil){
		panic(err);
	}

	pubKey := privKey.GetPublic();

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
		"/ip4/194.113.73.99/tcp/44981/p2p/QmUreqMsLeKVuoCJCQEGLwKZ8M5cogoPsQQcEYUj26kefi",
		"/ip4/209.151.148.27/tcp/44981/p2p/QmTnq5S4WZxoz9p1yP7JJBLXorgDYDqCnaKi97pTWUA3kQ",
		"/ip4/209.151.155.108/tcp/44981/p2p/Qmb9ibW4nVcma6wEd35MzbhPDgk4tPbp8gtxv8ESig918x",
	}

	// Start a DHT, for use in peer discovery. We can't just make a new DHT
	// client because we want each peer to maintain its own local copy of the
	// DHT, so that the bootstrapping node of the DHT can go down without
	// inhibiting future peer discovery.
	var validator record.Validator = OrcaValidator{}
	var options []dht.Option
	// no need for if statement to check if client is peer ? unless the testclient is also
	// supposed to be removed
	options = append(options, dht.Mode(dht.ModeClient))
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

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(err)
	}

	s := grpc.NewServer()
	serverStruct := server{}
	serverStruct.K_DHT = kDHT;
	serverStruct.PrivKey = privKey;
	serverStruct.PubKey = pubKey;
	serverStruct.V = validator

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
	pubKeyBytes, err := s.PubKey.Raw()
	if(err != nil){
		return nil, err
	}
	in.GetUser().Id = pubKeyBytes;

	holders, err := s.K_DHT.SearchValue(ctx, "orcanet/market/" + hash);
	if(err != nil){
		return nil, errors.New("Could not retrieve holders: " + err.Error());
	}

	bestValue := make([]byte, 0)
	for value := range holders {
		if len(value) > len(bestValue) {
			bestValue = value;
		}
	}

	//remove record for id if it already exists
	for i := 0; i < len(bestValue) - 4; i++ {
		value := bestValue
		messageLength := uint16(value[i + 1]) << 8 | uint16(value[i])
		digitalSignatureLength := uint16(value[i + 3]) << 8 | uint16(value[i + 2])
		contentLength := messageLength + digitalSignatureLength
		user := &pb.User{}

		err := proto.Unmarshal(value[i + 4:i + 4 + int(messageLength)], user) //will parse bytes only until user struct is filled out
		if err != nil {
			return nil, err
		}

		if(len(user.GetId()) == len(in.GetUser().GetId())){
			recordExists := true
			for i := range in.GetUser().GetId() {
				if user.GetId()[i] != in.GetUser().GetId()[i] {
					recordExists = false
					break
				}
			}

			if(recordExists){
				bestValue = append(bestValue[:i], bestValue[i + 4 + int(contentLength):]...);
				break;
			}
		}

		i = i + 4 + int(contentLength) - 1;
	}

	record := make([]byte, 0);
	userProtoBytes, err := proto.Marshal(in.GetUser());
	if(err != nil){
		return nil, err
	}
	userProtoSize := len(userProtoBytes);
	signature, err := s.PrivKey.Sign(userProtoBytes);
	if(err != nil){
		return nil, err
	}
	signatureLength := len(signature);
	record = append(record, byte(userProtoSize));
	record = append(record, byte(userProtoSize >> 8));
	record = append(record, byte(signatureLength));
	record = append(record, byte(signatureLength >> 8));
	record = append(record, userProtoBytes...);
	record = append(record, signature...);

	currentTime := time.Now().UTC()
    unixTimestamp := currentTime.Unix()
    unixTimestampInt64 := uint64(unixTimestamp)

	for i := 3; i >= 0; i-- {
		record = append(record, byte(unixTimestampInt64 >> i))
	}
	if(len(bestValue) != 0){
		bestValue = bestValue[:len(bestValue) - 4] //get rid of previous values timestamp
	}
	bestValue = append(bestValue, record...);
	err = s.K_DHT.PutValue(ctx, "orcanet/market/" + in.GetFileHash(), bestValue);
	if(err != nil){
		return nil, err;
	}
	return &emptypb.Empty{}, nil
}

// CheckHolders returns a list of user names holding a file with a hash
func (s *server) CheckHolders(ctx context.Context, in *pb.CheckHoldersRequest) (*pb.HoldersResponse, error) {
	hash := in.GetFileHash()
	valueStream, err := s.K_DHT.SearchValue(ctx, "orcanet/market/" + hash);
	if(err != nil){
		return nil, errors.New("Could not retrieve holders: " + err.Error());
	}

	bestValue := make([]byte, 0)
	for value := range valueStream {
		if len(value) > len(bestValue) {
			bestValue = value;
		}
	}

	users := make([]*pb.User, 0)
	for i := 0; i < len(bestValue) - 4; i++ {
		value := bestValue;
		messageLength := uint16(value[i + 1]) << 8 | uint16(value[i])
		digitalSignatureLength := uint16(value[i + 3]) << 8 | uint16(value[i + 2])
		contentLength := messageLength + digitalSignatureLength
		user := &pb.User{}

		err := proto.Unmarshal(value[i + 4:i + 4 + int(messageLength)], user) //will parse bytes only until user struct is filled out
		if err != nil {
			return nil, err
		}

		users = append(users, user);
		i = i + 4 + int(contentLength) - 1
	}

	return &pb.HoldersResponse{Holders: users}, nil
}

func discoverPeers(ctx context.Context, h host.Host, kDHT *dht.IpfsDHT, advertise string) {
	routingDiscovery := drouting.NewRoutingDiscovery(kDHT);
	// dutil.Advertise(ctx, routingDiscovery, advertise)

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
