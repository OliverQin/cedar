package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/OliverQin/cedar/libcedar/proxy"
	"github.com/OliverQin/cedar/libcedar/socks"
)

func PrintUsage() {
	fmt.Fprintf(os.Stderr, "Cedar is a faster encrypted proxy.\n")
	fmt.Fprintf(os.Stderr, "This is %s, local part of Cedar.\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\n")

	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	flag.PrintDefaults()
}

type cedarClientConfig struct {
	Local      string
	Remote     string
	Password   string
	BufferSize int
	NumOfConns int
}

func main() {
	var helpInfo bool
	var localAddr string
	var remoteAddr string
	var password string
	var bufferSize int
	var configFilename string
	var numOfConns int

	flag.StringVar(&remoteAddr, "r", "127.0.0.1:41289", "Remote (cdrserver) address and port, like \"127.0.0.1:41289\".")
	flag.BoolVar(&helpInfo, "h", false, "Display help info.")
	flag.StringVar(&localAddr, "s", "127.0.0.1:1080", "Local address and port like \"127.0.0.1:1080\".")
	flag.StringVar(&password, "p", "123456", "Password for encryption")
	flag.StringVar(&configFilename, "c", "", "Filename of config file. It overwrites command line parameters.")
	flag.IntVar(&bufferSize, "b", 100, "Max number of buffers. Size of each buffer is "+strconv.Itoa(socks.DefaultBufferLength)+"B.")
	flag.IntVar(&numOfConns, "n", 10, "Number of TCP connections.")

	flag.Parse()

	if helpInfo {
		PrintUsage()
		os.Exit(0)
	}

	if configFilename != "" {
		data, err := ioutil.ReadFile(configFilename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot load config file.\n")
			os.Exit(0)
		}

		conf := cedarClientConfig{}
		json.Unmarshal(data, &conf)

		if conf.Password != "" {
			password = conf.Password
		}
		if conf.Remote != "" {
			remoteAddr = conf.Remote
		}
		if conf.Local != "" {
			localAddr = conf.Local
		}
		if conf.BufferSize != 0 {
			bufferSize = conf.BufferSize
		}
		if conf.NumOfConns != 0 {
			numOfConns = conf.NumOfConns
		}
	}

	fmt.Fprintln(os.Stderr, "Remote: ", remoteAddr)
	fmt.Fprintln(os.Stderr, "Local:", localAddr)
	fmt.Fprintln(os.Stderr, "Running...")

	clt := proxy.NewProxyLocal(password, remoteAddr, localAddr, bufferSize)
	clt.Run(numOfConns)

	blocker := make(chan int)
	<-blocker
}
