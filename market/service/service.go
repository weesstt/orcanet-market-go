package service

import (
	"github.com/google/uuid"
)

type NoArgs struct {}
type NoResponse struct {}

//Keys are string digests, values are pointers to MarketRequestInfo structs
type RequestMap map[string][]*MarketRequestInfo

type Market struct {
	RequestMap RequestMap
}

type MarketRequestArgs struct {
	Bid float32
	FileDigest string
}

type MarketRequestInfo struct {
	Bid float32
	FileDigest string
	Identifier uuid.UUID
}

type MarketQueryArgs struct {
	FileDigest string
}

type MarketQueryResp struct {
	Result []*MarketRequestInfo
}

