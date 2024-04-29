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
	"flag"
	"log"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	record "github.com/libp2p/go-libp2p-record"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"orcanet/util"
	"orcanet/validator"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
)

func main() {
	var bootstrapPeer string
	flag.StringVar(&bootstrapPeer, "bootstrap", "", "Specify a bootstrap peer multiaddr")
	flag.Parse()

	ctx := context.Background()

	privKey, err := util.CheckOrCreatePrivateKey("privateKey.pem");
	if(err != nil){
		panic(err);
	}

	//Construct multiaddr from string and create host to listen on it. Will listen on all interfaces
	sourceMultiAddr, _ := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/44981")
	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(sourceMultiAddr.String()),
		libp2p.Identity(privKey), //derive id from private key
	}
	host, err := libp2p.New(opts...)
	if err != nil {
		panic(err)
	}

	_, err = relay.New(host)
	if err != nil {
		log.Printf("Failed to instantiate the relay: %v", err)
		return
	}

	log.Printf("Host ID: %s", host.ID())
	log.Printf("Connect to me on:")
	for _, addr := range host.Addrs() {
		log.Printf("%s/p2p/%s", addr, host.ID())
	}

	// Start a DHT, for use in peer discovery. We can't just make a new DHT
	// client because we want each peer to maintain its own local copy of the
	// DHT, so that the bootstrapping node of the DHT can go down without
	// inhibiting future peer discovery.
	var validator record.Validator = validator.OrcaValidator{}
	var options []dht.Option
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

	if bootstrapPeer != "" {
		connectToBootstrapPeer(bootstrapPeer, host, ctx)
	} 
	go util.DiscoverPeers(ctx, host, kDHT, "orcanet/market")

	select {}
}

/*
 *
 * Connect to a bootstrap peer using the specified multiaddr string.
 *
 * Parameters:
 *   multiAddr: Multiaddr string of the peer you are trying to connect to.
 *   host: libp2p host
 *   ctx: the context
 * 
 */
func connectToBootstrapPeer(multiAddr string, host host.Host, ctx context.Context){
	peerAddr, err := multiaddr.NewMultiaddr(multiAddr)
	if err != nil {
		panic(err)
	}
	peerinfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
	go func() {
		if err := host.Connect(ctx, *peerinfo); err != nil {
			log.Println("WARNING: ", err)
		} else {
			log.Println("Connection established with bootstrap node:", *peerinfo)
		}
	}()
}