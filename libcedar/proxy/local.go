package proxy

import (
	"github.com/OliverQin/cedar/libcedar/bundle"
	"github.com/OliverQin/cedar/libcedar/socks"
)

type ProxyLocal struct {
	tunnel *bundle.Endpoint
	client *socks.Endpoint
}

func NewProxyLocal(password string, remote string, local string, bufferSize int) *ProxyLocal {
	ret := new(ProxyLocal)
	ret.tunnel = bundle.NewEndpoint(uint32(bufferSize), "client", remote, password)
	ret.client = socks.NewClient(local)

	remoteToSocks := func(id uint32, msg []byte) {
		ret.client.WriteCommand(msg)
	}

	socksToRemote := func(msg []byte) error {
		ret.tunnel.Write(0, msg)
		return nil //TODO: signature not good, add error
	}

	ret.tunnel.SetOnReceived(remoteToSocks)
	ret.client.OnCommandGenerated = socksToRemote

	return ret
}

func (pl *ProxyLocal) Run(numOfConns int) {
	pl.tunnel.CreateConnection(numOfConns)

	err := pl.client.StartAsync()
	if err != nil {
		panic("cannot start socks service")
	}
}
