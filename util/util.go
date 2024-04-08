package util

/*
 *	References:
 *		https://gist.github.com/upperwal/38cd0c98e4a6b34c061db0ff26def9b9
 *		https://ldej.nl/post/building-an-echo-application-with-libp2p/
 *		https://github.com/libp2p/go-libp2p/blob/master/examples/chat-with-rendezvous/chat.go
 *		https://github.com/libp2p/go-libp2p/blob/master/examples/pubsub/basic-chat-with-rendezvous/main.go
 */

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"os"
	"log"
	"errors"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	host "github.com/libp2p/go-libp2p/core/host"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/multiformats/go-multiaddr"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	"fmt"
	"time"
	"bufio"
)

/*
 *
 * Check a file for a 2048 bit RSA private key and load it or generate a new one.
 *
 * Parameters:
 *   path: The name of the file to load key from or file name to save new key to.
 *
 * Returns:
 *   If the file exists and is of correct format, a libp2p wrapped private key is returned.
 *   If the file does not exist, a new one is generated and saved to the specified file name.
 *   If the specified file exists but does not contain a valid RSA private key, an error is returned.
 *   Returns an error for any key generation error.
 * 
 * Author: Rushikesh 
 */
func CheckOrCreatePrivateKey(path string) (crypto.PrivKey, error) {
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

/*
 * Check for peers who have announced themselves on the DHT. 
 * If the DHT is running in server mode, then we will announce ourselves and check for 
 * others who have announced as well.
 *
 * Parameters:
 *   context: The context
 *   h: libp2p host
 *   kDHT: the libp2p ipfs DHT object to use
 *   advertise: the string to use to check for others who have announced themselves. If
 * 				DHT is in server mode then that string will be used to announce ourselves as well.
 *
 */
func DiscoverPeers(ctx context.Context, h host.Host, kDHT *dht.IpfsDHT, advertise string) {
	routingDiscovery := drouting.NewRoutingDiscovery(kDHT);
	if(kDHT.Mode() == dht.ModeServer){
		dutil.Advertise(ctx, routingDiscovery, advertise);
	}

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


/*
 * Searches for file bootstrap.peer and parses it to get multiaddrs of bootstrap peers.
 * Each line is a mutliaddr.
 *
 * Returns:
 *   A slice of libp2p multiaddrs
 * Author: Erick
 */
func ReadBootstrapPeers() []multiaddr.Multiaddr {
	peers := []multiaddr.Multiaddr{}

	file, err := os.Open("bootstrap.peers")
	if err != nil {
		panic(err)
	}

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		// might have to strip the newline char ?
		line := scanner.Text()

		multiadd, err := multiaddr.NewMultiaddr(line)
		if err != nil {
			panic(err)
		}
		peers = append(peers, multiadd)
	}

	return peers
}

/*
 * Convert a max 8 byte slice to its 64 bit int value.
 *
 * Parameters:
 *   value: The byte slice to convert to an int
 *
 * Returns:
 *   An unsigned 64 bit int
 * Author: Austin
 */
func ConvertBytesTo64BitInt(value []byte) uint64 {
	suppliedTime := uint64(0)
	shift := 7
	for i := 0; i < len(value); i++ {
		suppliedTime = suppliedTime | (uint64(value[i]) << (shift * 8))
		shift--;
	}
	return suppliedTime
}