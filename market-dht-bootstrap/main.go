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
	"fmt"
	"log"
	"time"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"strings"
	"os"
	pb "orcanet/market"
	"github.com/golang/protobuf/proto"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	record "github.com/libp2p/go-libp2p-record"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/multiformats/go-multiaddr"
	"regexp"
	"errors"
)

//gRPC server 
type server struct {
	pb.UnimplementedMarketServer
}

// Create a record validator to store our own values within our defined protocol
type OrcaValidator struct{}

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
		suppliedTime = suppliedTime | uint64(value[len(value) - i]) << (i - 1)
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
				for i = 1; i < 5; i++ {
					suppliedTime = suppliedTime | uint64(value[i][len(value) - i]) << (i - 1)
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

func main() {
	var bootstrapPeer string
	flag.StringVar(&bootstrapPeer, "bootstrap", "", "Specify a bootstrap peer multiaddr")
	flag.Parse()

	ctx := context.Background()

	privKey, err := checkOrCreatePrivateKey("privateKey.pem");
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

	log.Printf("Host ID: %s", host.ID())
	log.Printf("Connect to me on:")
	for _, addr := range host.Addrs() {
		log.Printf("%s/p2p/%s", addr, host.ID())
	}

	// Start a DHT, for use in peer discovery. We can't just make a new DHT
	// client because we want each peer to maintain its own local copy of the
	// DHT, so that the bootstrapping node of the DHT can go down without
	// inhibiting future peer discovery.
	var validator record.Validator = OrcaValidator{}
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
	go discoverPeers(ctx, host, kDHT, "orcanet/market")

	select {}
}

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

func discoverPeers(ctx context.Context, h host.Host, kDHT *dht.IpfsDHT, advertise string) {
	routingDiscovery := drouting.NewRoutingDiscovery(kDHT)
	dutil.Advertise(ctx, routingDiscovery, advertise)

	// Look for others who have announced and attempt to connect to them
	for {
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
			}
		}
		time.Sleep(time.Second * 5)
	}
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
