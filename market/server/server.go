package main

import (
	"fmt"
	"net"
	"net/rpc"
	"market/service"
)

func main() {
	marketService := new(service.Market)
	marketService.RequestMap = make(map[string][]*service.MarketRequestInfo)
	rpc.Register(marketService)

	// Create a listener on port 1234
	listener, err := net.Listen("tcp", ":1234")
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}

	defer listener.Close()

	fmt.Println("Server is listening on port 1234...")

	// Accept and handle incoming RPC connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}
		go rpc.ServeConn(conn)
	}
}