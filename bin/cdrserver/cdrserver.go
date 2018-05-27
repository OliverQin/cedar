package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
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

type cedarServerConfig struct {
	Remote     string
	Password   string
	BufferSize int
}

func main() {
	var helpInfo bool
	var remoteAddr string
	var password string
	var bufferSize int
	var configFilename string

	flag.BoolVar(&helpInfo, "h", false, "Display help info.")
	flag.StringVar(&remoteAddr, "s", "127.0.0.1:41289", "Remote (cdrserver) address and port, like \"127.0.0.1:41289\".")
	flag.StringVar(&password, "p", "123456", "Password for encryption.")
	flag.StringVar(&configFilename, "c", "", "Filename of config file. It overwrites command line parameters.")
	flag.IntVar(&bufferSize, "b", 100, "Max number of buffers.")

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

		conf := cedarServerConfig{}
		json.Unmarshal(data, &conf)

		if conf.Password != "" {
			password = conf.Password
		}
		if conf.Remote != "" {
			remoteAddr = conf.Remote
		}
		if conf.BufferSize != 0 {
			bufferSize = conf.BufferSize
		}
	}

	if remoteAddr == "" {
		fmt.Fprintf(os.Stderr, "Error: serviceString is empty.\n")
		fmt.Fprintf(os.Stderr, "Try using \"-s <service string>\" flag.\n")
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "Service string:", remoteAddr)
	/*go func() {
		time.Sleep(400 * time.Second)
		pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
		panic("stop")
	}()*/

	server := proxy.NewProxyServer(password, remoteAddr, bufferSize)
	server.Run()
}
