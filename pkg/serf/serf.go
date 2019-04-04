package serf

import (
	"github.com/hashicorp/serf/serf"
	"io/ioutil"
	"log"
	"os"
	"time"
)

type Serf struct {
	Name         string
	Addr         string
	Port         int
	JoinCallback func(serf.MemberEvent)
	bootstrapAddrs []string
	events       chan serf.Event
	serf         *serf.Serf
}

func NewSerf(name string, addr string, port int) *Serf {
	return &Serf{
		Name: name,
		Addr: addr,
		Port: port,
		bootstrapAddrs: []string{},
	}
}

func (s *Serf) Start() (err error) {
	s.events = make(chan serf.Event)

	serfConfig := serf.DefaultConfig()
	serfConfig.MemberlistConfig.BindPort = s.Port
	serfConfig.MemberlistConfig.BindAddr = s.Addr
	serfConfig.MemberlistConfig.AdvertisePort = s.Port
	serfConfig.MemberlistConfig.AdvertiseAddr = s.Addr
	serfConfig.NodeName = s.Name
	serfConfig.EventCh = s.events

	if os.Getenv("DEBUG") != "1" {
		serfConfig.LogOutput = ioutil.Discard
		serfConfig.MemberlistConfig.LogOutput = ioutil.Discard
	}

	s.serf, err = serf.Create(serfConfig)

	log.Printf("serf listening at: %s:%d\n", s.Addr, s.Port)

	go func() {
		for event := range s.events {
			switch event.EventType() {
			case serf.EventMemberJoin:
				s.JoinCallback(event.(serf.MemberEvent))
			default:
				continue
			}
		}
	}()

	return
}

func (s *Serf) AddNode(addr string) {
	s.bootstrapAddrs = append(s.bootstrapAddrs, addr)
}

func (s *Serf) Join() {
	for {
		if _, err := s.serf.Join(s.bootstrapAddrs, false); err != nil {
			log.Println(err)
		}

		time.Sleep(2 * time.Second)
	}
}

func (s *Serf) Members() []serf.Member {
	return s.serf.Members()
}
