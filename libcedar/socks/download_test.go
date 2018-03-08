package socks

import (
	"log"
	"net/http"
	"strings"
	"testing"

	"golang.org/x/net/proxy"
)

const (
	proxyTestAddr = "127.0.0.1:9549"
	urlTest       = "https://en.wikipedia.org/wiki/Main_Page"
)

func TestProxy(t *testing.T) {
	ssServer := NewServer()
	ssClient := NewClient(proxyTestAddr)

	ssServer.OnCommandGenerated = ssClient.WriteCommand
	ssClient.OnCommandGenerated = ssServer.WriteCommand

	err := ssClient.StartAsync()
	if err != nil {
		panic("cannot start socks service")
	}

	// create a socks5 dialer
	dialer, err := proxy.SOCKS5("tcp", proxyTestAddr, nil, proxy.Direct)
	if err != nil {
		log.Panicln("can not connect to the proxy", err)
	}
	// setup a http client
	httpTransport := &http.Transport{}
	httpTransport.Dial = dialer.Dial
	httpClient := &http.Client{Transport: httpTransport}
	// set our socks5 as the dialer

	// create a request
	req, err := httpClient.Get(urlTest)
	log.Println("running")
	if err != nil {
		log.Panicln("get failed", err)
	}
	buf := make([]byte, 1024*1024*1) //1MiB
	n, err := req.Body.Read(buf)

	respStr := string(buf[:n])
	if strings.Count(respStr, "free encyclopedia") < 1 {
		log.Println(respStr)
		panic("content error")
	}

	return
}
