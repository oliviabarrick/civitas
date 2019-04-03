package proxy

import (
	"github.com/google/tcpproxy"
	"log"
)

type Proxy struct {
	proxy tcpproxy.Proxy
	upstreams map[string]int
	address string
}

func NewProxy(address string) *Proxy {
	p := &Proxy{
		address: address,
		proxy: tcpproxy.Proxy{},
		upstreams: map[string]int{},
	}

	p.Set([]string{"0.0.0.0",})
	return p
}

func (p *Proxy) Set(upstreams []string) {
	desiredUpstreams := map[string]bool{}

	for _, upstream := range upstreams {
		if p.upstreams[upstream] == 0 {
			log.Printf("Adding route %s -> %s\n", p.address, upstream)
			p.upstreams[upstream] = p.proxy.AddRoute(p.address, tcpproxy.To(upstream))
		}

		desiredUpstreams[upstream] = true
	}

	for upstream, routeId := range p.upstreams {
		if ! desiredUpstreams[upstream] {
			log.Printf("Removing route %s -> %s\n", p.address, upstream)
			p.proxy.RemoveRoute(p.address, routeId)
			delete(p.upstreams, upstream)
		}
	}
}

func (p *Proxy) Run() error {
	log.Println("Starting API server load balancer on:", p.address)
	return p.proxy.Start()
}
