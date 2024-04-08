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
	"flag"
	"fmt"
	"log"
	"net"
	"sync"
	pb "orcanet/market"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	record "github.com/libp2p/go-libp2p-record"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"google.golang.org/grpc"
	"orcanet/util"
	"orcanet/market"
	"orcanet/validator"
)

var (
	port = flag.Int("port", 50051, "The server port")
)

func main() {
	flag.Parse()
	ctx := context.Background()

	//Generate or load private key for libp2p host, 
	privKey, err := util.CheckOrCreatePrivateKey("privateKey.pem");
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

	bootstrapPeers := util.ReadBootstrapPeers()

	// Start a DHT, for now we will start in client mode until we can implement a way to 
	// detect if we are behind a NAT or not to run in server mode.
	var validator record.Validator = validator.OrcaValidator{}
	var options []dht.Option
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
	for _, peerAddr := range bootstrapPeers {
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

	go util.DiscoverPeers(ctx, host, kDHT, "orcanet/market")

	//Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(err)
	}

	s := grpc.NewServer()
	serverStruct := market.Server{}
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