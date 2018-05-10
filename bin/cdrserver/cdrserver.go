package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/pprof"
	"time"

	"github.com/OliverQin/cedar/libcedar/bundle"
	"github.com/OliverQin/cedar/libcedar/socks"
)

func PrintUsage() {
	fmt.Fprintf(os.Stderr, "Cedar is a faster encrypted proxy.\n")
	fmt.Fprintf(os.Stderr, "This is %s, local part of Cedar.\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\n")

	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	flag.PrintDefaults()
}

func RunServer(password string, local string) {
	//Conns := 20
	BufSize := uint32(500)

	ssServer := socks.NewServer()

	sv := bundle.NewEndpoint(BufSize, "server", local, password)

	serverToSocks := func(id uint32, msg []byte) {
		//FIXME: id is not used, it's wrong. Now only one-client supported. Currying is needed.
		//fmt.Println("sv to socks:", len(msg), msg)
		copiedMsg := make([]byte, len(msg))
		copy(copiedMsg, msg)
		ssServer.WriteCommand(copiedMsg)
	}

	socksToServer := func(msg []byte) error {
		copiedMsg := make([]byte, len(msg))
		copy(copiedMsg, msg)
		sv.Write(0, copiedMsg)
		//fmt.Println("socks to sv", len(msg), msg)
		return nil
	}

	ssServer.OnCommandGenerated = socksToServer
	sv.SetOnReceived(serverToSocks)

	fmt.Println("Running...")
	sv.ServerStart()
}

func main() {
	/*go func() {
		time.Sleep(20 * time.Second)
		pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
		panic("wwwww")
	}()*/
	go func() {
		time.Sleep(400 * time.Second)
		pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
		panic("stop")
	}()
	RunServer("test_password", "127.0.0.1:27968")
}
