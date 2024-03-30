package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"fmt" // Keep this for normal usage
	pb "github.com/weesstt/starfish-market"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"io/ioutil"
	"log" // Only need to import once
	"net"
	"os"
	"time"
)

// Command line argument to specify the port to run the RPC server on.
var (
	port = flag.Int("port", 50051, "The server port")
)

// Define a custom type for the enum
type status int

// Enum values using iota
const (
	PendingProducerAcceptance status = iota
	PendingReceipt
	Finalized
)

// Number of seconds to wait before timing out an operation
const TIMEOUT = 60

type Transaction struct {
	Status        status
	Bid           float32
	Identifier    string
	ProducerPubIP string
	ConsumerPubIP string
	DataTransfer  string
	Receipt       string
}

// marketServer is used to implement market.MarketServer.
type marketServer struct {
	pb.UnimplementedMarketServer
	//This is a map where the keys are data identifiers and values are another map.
	//The second map has keys of public ip addr of producers and the values are MarketAsk structs
	ProducerAsks map[string]map[string]pb.MarketAsk

	//This is a map where the keys are the producer's public IP address, and the values are
	//maps that have consumers public ip address as keys and Transaction struct pointers as values
	Transactions map[string]map[string]*Transaction
}

const privateKeyPath = "privateKey.pem"

func checkOrCreatePrivateKey(path string) (*rsa.PrivateKey, error) {
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
		return privKey, nil
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
	return privKey, nil
}
func getTLSCertificate(certPath, keyPath string) (tls.Certificate, error) {
	// Load the certificate
	certPEM, err := ioutil.ReadFile(certPath)
	if err != nil {
		return tls.Certificate{}, err
	}

	// Load the private key
	keyPEM, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return tls.Certificate{}, err
	}

	// Create the TLS certificate
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, err
	}
	return cert, nil
}

func main() {
	flag.Parse()

	// Use the getTLSCertificate function to load the TLS certificate and key from files
	cert, err := getTLSCertificate("path/to/cert.pem", "path/to/key.pem")
	if err != nil {
		log.Fatalf("Failed to get TLS certificate: %v", err)
	}

	tlsCreds := credentials.NewServerTLSFromCert(&cert)

	opts := []grpc.ServerOption{
		grpc.Creds(tlsCreds),
	}

	grpcServer := grpc.NewServer(opts...)

	pb.RegisterMarketServer(grpcServer, &marketServer{
		ProducerAsks: make(map[string]map[string]pb.MarketAsk),
		Transactions: make(map[string]map[string]*Transaction),
	})

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Printf("Serving gRPC on localhost:%d", *port)
	log.Fatal(grpcServer.Serve(lis))
}

func (m *marketServer) ConsumerMarketQuery(ctx context.Context, args *pb.MarketQueryArgs) (*pb.MarketQuery, error) {
	producerMap, exists := m.ProducerAsks[args.GetIdentifier()]

	//If data identifier not in market server table then return an empty list.
	if !exists {
		resp := &pb.MarketQuery{
			Offers: []*pb.MarketAsk{},
		}

		return resp, nil
	}

	asks := []*pb.MarketAsk{}

	for _, value := range producerMap {
		asks = append(asks, &value)
	}

	resp := &pb.MarketQuery{
		Offers: asks,
	}

	return resp, nil
}

// Register a producers asking price for certain data with the market server.
// If producer previous registered data then the previous one will be deleted.
func (m *marketServer) RegisterMarketAsk(ctx context.Context, args *pb.MarketAskArgs) (*pb.MarketAsk, error) {
	md, _ := metadata.FromIncomingContext(ctx) //Producer must add their public ip address to context of grpc call

	pubIPs := md.Get("pubIP") //returns an array since a key can have multiple values, we later retrieve the first value

	if len(pubIPs) == 0 {
		return nil, errors.New("Producer Public IP address must be included in context of gRPC call.")
	}

	pubIP := pubIPs[0]

	if !isIPv4(pubIP) {
		return nil, errors.New("Public IP address in gRPC context must be of ipv4 format!")
	}

	if args.GetBid() <= 0 {
		return nil, errors.New("Asking transfer price must be greater than 0 OrcaCoins.")
	}

	ask := pb.MarketAsk{
		Bid:           args.GetBid(),
		Identifier:    args.GetIdentifier(),
		ProducerPubIP: pubIP,
	}

	_, exists := m.ProducerAsks[args.GetIdentifier()]

	if !exists {
		m.ProducerAsks[args.GetIdentifier()] = make(map[string]pb.MarketAsk)
	}

	m.ProducerAsks[args.GetIdentifier()][pubIP] = ask

	fmt.Println("Registered market ask for data identifier: " + ask.Identifier)
	fmt.Println("Bid: " + fmt.Sprintf("%f", ask.Bid) + ", Producer Public IP: " + ask.ProducerPubIP)
	fmt.Println("----------------------------------")

	return &ask, nil
}

func (m *marketServer) InitiateMarketTransaction(ctx context.Context, args *pb.MarketAsk) (*pb.MarketDataTransfer, error) {
	//Validate that this MarketAsk is registered
	requestedIdentifier := args.GetIdentifier()
	producerPubIP := args.GetProducerPubIP()
	consumerPubIP := args.GetConsumerPubIP()

	registeredAsk, exists := m.ProducerAsks[requestedIdentifier][producerPubIP]

	if !exists {
		return nil, errors.New("There is currently no registered producers to serve data with identifier " + requestedIdentifier)
	}

	_, exists = m.Transactions[producerPubIP][consumerPubIP]

	if exists {
		return nil, errors.New("There is already an active transaction between the provided consumer and producer!")
	}

	if registeredAsk.Bid != args.GetBid() {
		return nil, errors.New("The current asking price for producer " + producerPubIP + " does not match the provided price.")
	}

	if !isIPv4(producerPubIP) || !isIPv4(consumerPubIP) {
		return nil, errors.New("Public IP address must be in ipv4 format!")
	}

	//Create transaction struct
	transaction := new(Transaction)
	transaction.Status = PendingProducerAcceptance
	transaction.Bid = registeredAsk.Bid
	transaction.Identifier = registeredAsk.Identifier
	transaction.ConsumerPubIP = consumerPubIP
	transaction.ProducerPubIP = producerPubIP

	_, exists = m.Transactions[producerPubIP]

	if !exists {
		m.Transactions[producerPubIP] = make(map[string]*Transaction)
	}

	m.Transactions[producerPubIP][consumerPubIP] = transaction

	//Begin loop to wait for producer to accept transaction
	timeout := TIMEOUT * time.Second
	channel := time.After(timeout)

	for {
		select {
		case <-channel:
			return nil, errors.New("Timeout reached for transaction. Try again")

		default:
			transaction = m.Transactions[producerPubIP][consumerPubIP]

			if transaction.Status == PendingReceipt {
				dataTransfer := &pb.MarketDataTransfer{
					URL:        transaction.DataTransfer,
					Identifier: transaction.Identifier,
				}
				return dataTransfer, nil
			}
		}

		time.Sleep(1 * time.Second)
	}
}

func (m *marketServer) ProducerMarketQuery(ctx context.Context, args *pb.MarketQueryArgs) (*pb.MarketQuery, error) {
	md, _ := metadata.FromIncomingContext(ctx) //Producer must add their public IP address to context

	pubIPs := md.Get("pubIP")

	if len(pubIPs) == 0 {
		return nil, errors.New("Producer Public IP address must be included in context of gRPC call.")
	}

	pubIP := pubIPs[0]

	if !isIPv4(pubIP) {
		return nil, errors.New("Public IP address in gRPC context must be of ipv4 format!")
	}

	transactionMap, exists := m.Transactions[pubIP]

	if !exists {
		resp := &pb.MarketQuery{
			Offers: []*pb.MarketAsk{},
		}
		return resp, nil
	}

	asks := []*pb.MarketAsk{}

	for consumerPubIP, transaction := range transactionMap {
		ask := new(pb.MarketAsk)
		ask.Bid = transaction.Bid
		ask.Identifier = transaction.Identifier
		ask.ConsumerPubIP = consumerPubIP
		ask.ProducerPubIP = pubIP

		asks = append(asks, ask)
	}

	resp := &pb.MarketQuery{
		Offers: asks,
	}

	return resp, nil
}

func (m *marketServer) ProducerAcceptTransaction(ctx context.Context, args *pb.MarketAsk) (*pb.Receipt, error) {
	md, _ := metadata.FromIncomingContext(ctx) //Producer must add the address of the exposed web server where consumer can reach the requested resource

	webResources := md.Get("webResource")

	if len(webResources) == 0 {
		return nil, errors.New("Producer must provide webResource to access requested data within context of gRPC call")
	}

	webResource := webResources[0]

	consumerPubIP := args.GetConsumerPubIP()
	producerPubIP := args.GetProducerPubIP()

	transaction, exists := m.Transactions[producerPubIP][consumerPubIP]

	if !exists {
		return nil, errors.New("No transaction exists with the provided market ask message!")
	}

	if transaction.Bid != args.GetBid() {
		return nil, errors.New("The market transaction bid does not match the provided bid!")
	}

	if transaction.Identifier != args.GetIdentifier() {
		return nil, errors.New("The market transaction data identifier does not match the provided identifier!")
	}

	//provide transaction with web server so that consumer can continue transaction
	transaction.DataTransfer = webResource

	//change status of transaction
	transaction.Status = PendingReceipt

	//Loop until consumer approves transaction with receipt
	timeout := TIMEOUT * time.Second
	channel := time.After(timeout)

	for {
		select {
		case <-channel:
			return nil, errors.New("Timeout reached for transaction. Try again")

		default:
			transaction := m.Transactions[producerPubIP][consumerPubIP]

			if transaction.Status == Finalized {
				receipt := &pb.Receipt{
					Identifier: transaction.Receipt,
				}

				delete(m.Transactions, producerPubIP)

				return receipt, nil
			}

			time.Sleep(1 * time.Second)
		}
	}
}

func (m *marketServer) FinalizeMarketTransaction(ctx context.Context, args *pb.MarketAsk) (*pb.Receipt, error) {
	md, _ := metadata.FromIncomingContext(ctx) //Consumer must add the identifier of the blockchain transaction

	transactionIDs := md.Get("transactionIdentifier")

	if len(transactionIDs) == 0 {
		return nil, errors.New("Consumer blockchain transaction ID must be provided within the context of the gRPC call")
	}

	transactionID := transactionIDs[0]

	consumerPubIP := args.GetConsumerPubIP()
	producerPubIP := args.GetProducerPubIP()

	transaction, exists := m.Transactions[producerPubIP][consumerPubIP]

	if !exists {
		return nil, errors.New("No transaction exists with the provided market ask message!")
	}

	if transaction.Bid != args.GetBid() {
		return nil, errors.New("The market transaction bid does not match the provided bid!")
	}

	if transaction.Identifier != args.GetIdentifier() {
		return nil, errors.New("The market transaction data identifier does not match the provided identifier!")
	}

	transaction.Receipt = transactionID
	transaction.Status = Finalized

	resp := &pb.Receipt{
		Identifier: transactionID,
	}

	return resp, nil
}

func isIPv4(ipString string) bool {
	ip := net.ParseIP(ipString)
	return ip != nil && ip.To4() != nil
}
