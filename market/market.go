/*
*	References:
*		https://gist.github.com/upperwal/38cd0c98e4a6b34c061db0ff26def9b9
*		https://ldej.nl/post/building-an-echo-application-with-libp2p/
*		https://github.com/libp2p/go-libp2p/blob/master/examples/chat-with-rendezvous/chat.go
*		https://github.com/libp2p/go-libp2p/blob/master/examples/pubsub/basic-chat-with-rendezvous/main.go
*/

package market

import (
	"context"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	record "github.com/libp2p/go-libp2p-record"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	"google.golang.org/protobuf/types/known/emptypb"
	"errors"
	"github.com/golang/protobuf/proto"
	"time"
)

type Server struct {
	UnimplementedMarketServer
	K_DHT *dht.IpfsDHT
	PrivKey crypto.PrivKey
	PubKey crypto.PubKey
	V record.Validator
}

/*
 * gRPC service to register a file on the DHT market.
 * 
 * Parameters:
 *   ctx: Context
 *   in: A protobuf RegisterFileRequest struct that represents the file/producer being registered.
 *
 * Returns:
 *   An empty protobuf struct
 *   An error, if any
 * Author: Austin
 */
func (s *Server) RegisterFile(ctx context.Context, in *RegisterFileRequest) (*emptypb.Empty, error) {
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
		user := &User{}

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

/*
 * gRPC service to check for producers who have registered a specific file.
 * 
 * Parameters:
 *   ctx: Context
 *   in: A protobuf CheckHoldersRequest struct that represents the file to look up.
 *
 * Returns:
 *   A HoldersResponse protobuf struct that represents the producers and their prices.
 *   An error, if any
 * Author: Austin
 */
func (s *Server) CheckHolders(ctx context.Context, in *CheckHoldersRequest) (*HoldersResponse, error) {
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

	users := make([]*User, 0)
	for i := 0; i < len(bestValue) - 4; i++ {
		value := bestValue;
		messageLength := uint16(value[i + 1]) << 8 | uint16(value[i])
		digitalSignatureLength := uint16(value[i + 3]) << 8 | uint16(value[i + 2])
		contentLength := messageLength + digitalSignatureLength
		user := &User{}

		err := proto.Unmarshal(value[i + 4:i + 4 + int(messageLength)], user) //will parse bytes only until user struct is filled out
		if err != nil {
			return nil, err
		}

		users = append(users, user);
		i = i + 4 + int(contentLength) - 1
	}

	return &HoldersResponse{Holders: users}, nil
}
