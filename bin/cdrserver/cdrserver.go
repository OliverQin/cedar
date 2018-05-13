package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/OliverQin/cedar/libcedar/bundle"
	"github.com/OliverQin/cedar/libcedar/socks"
)

func PrintUsage() {
	fmt.Fprintf(os.Stderr, "Cedar is a faster encrypted proxy.\n")
	fmt.Fprintf(os.Stderr, "This is %s, remote part of Cedar.\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\n")

	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	flag.PrintDefaults()
}

func RunServer(password string, local string) {
	BufSize := uint32(100)

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
	var helpInfo bool
	var serviceString string
	var password string

	flag.BoolVar(&helpInfo, "h", false, "Display help info.")
	flag.StringVar(&serviceString, "s", "", "Service string like \"127.0.0.1:41289\". ")
	flag.StringVar(&password, "p", "123456", "Password for encryption")

	flag.Parse()

	if helpInfo {
		PrintUsage()
		os.Exit(0)
	}
	if serviceString == "" {
		fmt.Fprintf(os.Stderr, "Error: serviceString is empty.\n")
		fmt.Fprintf(os.Stderr, "Try using \"-s <service string>\" flag.\n")
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "Service string:", serviceString)
	/*go func() {
		time.Sleep(400 * time.Second)
		pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
		panic("stop")
	}()*/
	RunServer(password, serviceString)
}
