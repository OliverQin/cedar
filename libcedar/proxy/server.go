package proxy

import (
	"log"
	"sync"

	"github.com/OliverQin/cedar/libcedar/bundle"
	"github.com/OliverQin/cedar/libcedar/socks"
)

type ProxyServer struct {
	mapLock sync.RWMutex
	servers map[uint32]*socks.Endpoint
	tunnel  *bundle.Endpoint
}

func NewProxyServer(password string, local string, bufferSize int) *ProxyServer {
	ret := new(ProxyServer)

	ret.servers = make(map[uint32]*socks.Endpoint)
	ret.tunnel = bundle.NewEndpoint(uint32(bufferSize), "server", local, password)
	ret.tunnel.SetOnReceived(ret.serverToSocks)
	ret.tunnel.SetOnBundleLost(ret.bundleLost)

	return ret
}

func (ps *ProxyServer) bundleLost(id uint32) {
	ps.mapLock.Lock()
	if _, ok := ps.servers[id]; ok {
		delete(ps.servers, id)
	}
	ps.mapLock.Unlock()

	log.Println("[ProxyServer.bundleLost]", id)

	return
}

func (ps *ProxyServer) serverToSocks(id uint32, msg []byte) {
	ps.mapLock.RLock()
	sv, ok := ps.servers[id]
	ps.mapLock.RUnlock()

	if !ok {
		sv = socks.NewServer()

		ps.mapLock.Lock()
		ps.servers[id] = sv
		ps.mapLock.Unlock()

		socksToServer := func(msg []byte) error {
			ps.tunnel.Write(id, msg)
			return nil
		}
		sv.OnCommandGenerated = socksToServer
	}

	sv.WriteCommand(msg)
}

/*
Run is an endless loop to run the server
*/
func (ps *ProxyServer) Run() {
	ps.tunnel.ServerStart()
}
