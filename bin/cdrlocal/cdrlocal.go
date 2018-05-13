package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

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

func RunLocal(password string, remote string, local string) {
	Conns := 2
	BufSize := uint32(100)

	ssClient := socks.NewClient(local)

	clt := bundle.NewEndpoint(BufSize, "client", remote, password)

	remoteToSocks := func(id uint32, msg []byte) {
		copiedMsg := make([]byte, len(msg))
		copy(copiedMsg, msg)
		ssClient.WriteCommand(copiedMsg)
		//fmt.Println("rmt to socks:", len(msg), msg)
	}

	socksToRemote := func(msg []byte) error {
		copiedMsg := make([]byte, len(msg))
		copy(copiedMsg, msg)
		clt.Write(0, copiedMsg)
		//fmt.Println("socks to rmt:", len(msg), msg)
		return nil //TODO: signature not good, add error
	}

	clt.SetOnReceived(remoteToSocks)
	ssClient.OnCommandGenerated = socksToRemote

	clt.CreateConnection(Conns)

	err := ssClient.StartAsync()
	if err != nil {
		panic("cannot start socks service")
	}
}

func main() {
	var helpInfo bool
	var localPort uint64
	var localAddress string
	var localService string
	var remoteService string
	var password string

	flag.StringVar(&remoteService, "r", "", "Remote address string.")
	flag.BoolVar(&helpInfo, "h", false, "Display help info.")
	flag.Uint64Var(&localPort, "l", 0, "Local port of SOCKS server.")
	flag.StringVar(&localAddress, "a", "127.0.0.1", "Local addr of SOCKS server. ")
	flag.StringVar(&localService, "s", "", "Local service string like \"127.0.0.1:1080\". It should not be used with local port (-l).")
	flag.StringVar(&password, "p", "123456", "Password for encryption")

	flag.Parse()

	if helpInfo {
		PrintUsage()
		os.Exit(0)
	}
	if localPort > 0 && localService != "" {
		fmt.Fprintf(os.Stderr, "Error: using both local port and local service.\n")
		fmt.Fprintf(os.Stderr, "Do NOT use -l and -s at the same time.\n")
		os.Exit(1)
	}
	if (localPort > 65535 || localPort == 0) && localService == "" {
		fmt.Fprintf(os.Stderr, "Error: the port should be in 1-65535, but it's %d!\n", localPort)
		fmt.Fprintf(os.Stderr, "Try using \"-l [port]\" flag.\n")
		os.Exit(1)
	}

	if localService != "" {
		//localService = localService
	} else {
		localService = localAddress + ":" + strconv.FormatUint(localPort, 10)
	}
	fmt.Fprintln(os.Stderr, "Remote: ", remoteService)
	fmt.Fprintln(os.Stderr, "Local:", localService)
	fmt.Fprintln(os.Stderr, "Running...")

	RunLocal(password, remoteService, localService)
	//RunLocal("test_password", "127.0.0.1:27968", "127.0.0.1:1082")

	/*go func() {
		time.Sleep(20 * time.Second)
		pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
		panic("wwwww")
	}()*/

	blocker := make(chan int)
	<-blocker
}
