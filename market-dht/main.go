/*
	References:
		https://gist.github.com/upperwal/38cd0c98e4a6b34c061db0ff26def9b9
		https://ldej.nl/post/building-an-echo-application-with-libp2p/
		https://github.com/libp2p/go-libp2p/blob/master/examples/chat-with-rendezvous/chat.go

*/

///ip4/54.174.250.68/tcp/44981/p2p/QmVicAxtGGNXW4oU3f4686py2sB7csAr6wuUyhR1YXe99a boostrap node

package main

import (
	"context"
	"fmt"
	"bufio"
	"log"
	"sync"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
	// "github.com/libp2p/go-libp2p-net"
	// "github.com/ipfs/go-datastore"
	// "github.com/ipfs/go-ipfs-addr"
	// "github.com/libp2p/go-libp2p-peerstore"
	// "github.com/ipfs/go-cid"
	// "github.com/multiformats/go-multihash"
	"github.com/multiformats/go-multiaddr"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"flag"
)

func main() {
	var bootstrapPeer string
	flag.StringVar(&bootstrapPeer, "b", "", "Specify a bootstrap peer multiaddr")
	flag.Parse()
	if(bootstrapPeer == ""){
		panic("A bootstrap peer must be provided!");
	}

	ctx := context.Background()
	
	//Generate private key for peer
	privKey, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	if err != nil {
		panic(err)
	}

	//Construct multiaddr from string and create host to listen on it
	sourceMultiAddr, _ := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/0")
	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(sourceMultiAddr.String()),
		libp2p.Identity(privKey), //derive id from private key
	}
	host, err := libp2p.New(opts...);
	if err != nil {
		panic(err)
	}
	host.SetStreamHandler("orcanet/1.0.0", handleStream)
	log.Printf("Host ID: %s", host.ID())
	log.Printf("Connect to me on:")
	for _, addr := range host.Addrs() {
		log.Printf("  %s/p2p/%s", addr, host.ID())
	}

	//An array if we want to expand to a more stable peer list instead of providing in args
	bootstrapPeers := []string {
		bootstrapPeer,
	}

	// Start a DHT, for use in peer discovery. We can't just make a new DHT
	// client because we want each peer to maintain its own local copy of the
	// DHT, so that the bootstrapping node of the DHT can go down without
	// inhibiting future peer discovery.
	kDHT, err := dht.New(ctx, host)
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

	select {}
}

func handleStream(stream network.Stream) {
	log.Println("Got a new stream!")

	// Create a buffer stream for non-blocking read and write.
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

	go readData(rw)

	// 'stream' will stay open until you close it (or the other side closes it).
}

func readData(rw *bufio.ReadWriter) {
	for {
		str, _ := rw.ReadString('\n')
		if str == "" {
			return
		}
		if str != "\n" {
			// Green console colour: 	\x1b[32m
			// Reset console colour: 	\x1b[0m
			fmt.Printf("\x1b[32m%s\x1b[0m> ", str)
		}
	}
}