package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/OliverQin/cedar/libcedar/proxy"
)

func PrintUsage() {
	fmt.Fprintf(os.Stderr, "Cedar is a faster encrypted proxy.\n")
	fmt.Fprintf(os.Stderr, "This is %s, local part of Cedar.\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\n")

	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	var helpInfo bool
	var localService string
	var remoteService string
	var password string
	var bufferSize int
	var numOfConns int

	flag.StringVar(&remoteService, "r", "127.0.0.1:41289", "Remote address string.")
	flag.BoolVar(&helpInfo, "h", false, "Display help info.")
	flag.StringVar(&localService, "s", "127.0.0.1:1080", "Local service string like \"127.0.0.1:1080\". It should not be used with local port (-l).")
	flag.StringVar(&password, "p", "123456", "Password for encryption")
	flag.IntVar(&bufferSize, "b", 100, "Max number of buffers.")
	flag.IntVar(&numOfConns, "n", 10, "Number of TCP connections.")

	flag.Parse()

	if helpInfo {
		PrintUsage()
		os.Exit(0)
	}

	fmt.Fprintln(os.Stderr, "Remote: ", remoteService)
	fmt.Fprintln(os.Stderr, "Local:", localService)
	fmt.Fprintln(os.Stderr, "Running...")

	clt := proxy.NewProxyLocal(password, remoteService, localService, bufferSize)
	clt.Run(numOfConns)

	blocker := make(chan int)
	<-blocker
}
