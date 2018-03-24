package main

import (
	"github.com/OliverQin/cedar/libcedar/socks"
)

const (
	proxyTestAddr = "127.0.0.1:1082"
)

func main() {
	// Typical way to use socks service
	ssServer := socks.NewServer()
	ssClient := socks.NewClient(proxyTestAddr)

	ssServer.OnCommandGenerated = ssClient.WriteCommand
	ssClient.OnCommandGenerated = ssServer.WriteCommand

	err := ssClient.StartAsync()
	if err != nil {
		panic("cannot start socks service")
	}

	a := make(chan int)
	<-a

	return
}
