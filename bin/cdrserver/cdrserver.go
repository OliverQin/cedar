package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/OliverQin/cedar/libcedar/proxy"
)

func PrintUsage() {
	fmt.Fprintf(os.Stderr, "Cedar is a faster encrypted proxy.\n")
	fmt.Fprintf(os.Stderr, "This is %s, remote part of Cedar.\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\n")

	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	var helpInfo bool
	var serviceString string
	var password string
	var bufferSize int

	flag.BoolVar(&helpInfo, "h", false, "Display help info.")
	flag.StringVar(&serviceString, "s", "127.0.0.1:41289", "Service string like \"127.0.0.1:41289\".")
	flag.StringVar(&password, "p", "123456", "Password for encryption.")
	flag.IntVar(&bufferSize, "b", 100, "Max number of buffers.")

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

	server := proxy.NewProxyServer(password, serviceString, bufferSize)
	server.Run()
}
