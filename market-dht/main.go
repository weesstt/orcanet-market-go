/*
	References:
		https://gist.github.com/upperwal/38cd0c98e4a6b34c061db0ff26def9b9
		https://ldej.nl/post/building-an-echo-application-with-libp2p/
		https://github.com/libp2p/go-libp2p/blob/master/examples/chat-with-rendezvous/chat.go
		https://github.com/libp2p/go-libp2p/blob/master/examples/pubsub/basic-chat-with-rendezvous/main.go
*/

package main

import (
	"context"
	"fmt"
	// "bufio"
	"time"
	"log"
	"sync"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/host"
	// "github.com/libp2p/go-libp2p-net"
	// "github.com/ipfs/go-datastore"
	// "github.com/ipfs/go-ipfs-addr"
	// "github.com/libp2p/go-libp2p-peerstore"
	// "github.com/ipfs/go-cid"
	// "github.com/multiformats/go-multihash"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/multiformats/go-multiaddr"
	"github.com/libp2p/go-libp2p/core/crypto"
	// "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p-record"
	"flag"
)

//Create a record validator to store our own values within our defined protocol
type OrcaValidator struct {}

//testing, will actually validate later
func (v OrcaValidator) Validate(key string, value []byte) error{
	return nil; 
}

func (v OrcaValidator) Select(key string, value [][]byte) (int, error){
	return 0, nil; 
}

func main() {
	var bootstrapPeer string
	var searchKey string
	var putKey string
	var putValue string 
	var isClient bool
	flag.StringVar(&bootstrapPeer, "bootstrap", "", "Specify a bootstrap peer multiaddr")
	flag.StringVar(&searchKey, "searchKey", "", "Search for a key repeatedly until found")
	flag.StringVar(&putKey, "putKey", "", "Put a value into the DHT with this key, must specify value too")
	flag.StringVar(&putValue, "putValue", "", "Put a value into the DHT, must specify key too")
	flag.BoolVar(&isClient, "clientMode", false, "Specify client or server, true for client")
	flag.Parse()

	ctx := context.Background()
	
	//Generate private key for peer
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
	host, err := libp2p.New(opts...);
	if err != nil {
		panic(err)
	}

	log.Printf("Host ID: %s", host.ID())
	log.Printf("Connect to me on:")
	for _, addr := range host.Addrs() {
		log.Printf("%s/p2p/%s", addr, host.ID())
	}

	//An array if we want to expand to a more stable peer list instead of providing in args
	bootstrapPeers := []string {
		bootstrapPeer,
	} 

	// Start a DHT, for use in peer discovery. We can't just make a new DHT
	// client because we want each peer to maintain its own local copy of the
	// DHT, so that the bootstrapping node of the DHT can go down without
	// inhibiting future peer discovery.
	var validator record.Validator = OrcaValidator{}
	var options []dht.Option
	if(isClient){ //if no bootstrap peer, go into server mode
		options = append(options, dht.Mode(dht.ModeClient))
	}else{
		options = append(options, dht.Mode(dht.ModeServer))
	}
	options = append(options, dht.ProtocolPrefix("orcanet/market"), dht.Validator(validator));
	kDHT, err := dht.New(ctx, host, options...)
	if(err != nil){
		panic(err);
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
		if(peerAddrString == ""){continue;}
		peerAddr, err := multiaddr.NewMultiaddr(peerAddrString)
		if(err != nil){
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
	
	go discoverPeers(ctx, host, kDHT, "orcanet/market");
	time.Sleep(5 * time.Second)

	if(putKey != ""){
		for {
			err = kDHT.PutValue(ctx, "orcanet/market/" + putKey, []byte(putValue))
			if(err != nil){
				fmt.Println("Error: ", err)
				time.Sleep(5 * time.Second)
				continue;
			}
			fmt.Println("Put key: ", putKey + " Value: " + putValue)
			break;
		}
	}
	
	if(searchKey != ""){
		for {
			valueStream, err := kDHT.SearchValue(ctx, "orcanet/market/" + searchKey)
			fmt.Println("Searching for " + searchKey)
			if(err != nil){
				fmt.Println("Error: ", err)
				time.Sleep(5 * time.Second)
				continue;
			}
			time.Sleep(5 * time.Second)
			for byteArray := range valueStream {
				fmt.Println(string(byteArray));
			}
		}
	}

	select {}
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