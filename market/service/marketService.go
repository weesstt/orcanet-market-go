package service

import (
	"github.com/google/uuid"
	"errors"
	"fmt"
)

//Consumer request to the market to retrieve a file
func (m *Market) ConsumerRequest(args *MarketRequestArgs, response *MarketRequestInfo) error {
	if args.Bid <= 0 {
		return errors.New("Market bid must be greater than 0 rat coins")
	}

	info := MarketRequestInfo{
		Bid: args.Bid,
		FileDigest: args.FileDigest,
		Identifier: uuid.New(),
	}

	_, exists := m.RequestMap[args.FileDigest]
	if !exists {
		m.RequestMap[args.FileDigest] = []*MarketRequestInfo{}
	} 

	m.RequestMap[args.FileDigest] = append(m.RequestMap[args.FileDigest], &info)
	
	response.Bid = args.Bid
	response.FileDigest = args.FileDigest
	response.Identifier = info.Identifier

	fmt.Println("Received File Request for digest: " + args.FileDigest)
	fmt.Printf("Bid: %f\n", args.Bid)

	return nil
}

//Producer query to the market to see requests for a specific file hash
func (m *Market) ProducerQuery(args *MarketQueryArgs, response *MarketQueryResp) error {
	_, exists := m.RequestMap[args.FileDigest]
	if !exists {
		response.Result = nil
	} else {
		response.Result = m.RequestMap[args.FileDigest]
	}

	return nil
}