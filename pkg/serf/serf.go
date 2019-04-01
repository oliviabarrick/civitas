package serf

import (
	"github.com/hashicorp/serf/serf"
	"log"
	"os"
	"time"
)

type Serf struct {
	Name         string
	Port         int
	JoinCallback func(serf.MemberEvent)
	events       chan serf.Event
	serf         *serf.Serf
}

func NewSerf(name string, port int) *Serf {
	return &Serf{
		Name: name,
		Port: port,
	}
}

func (s *Serf) Start() (err error) {
	s.events = make(chan serf.Event)

	serfConfig := serf.DefaultConfig()
	serfConfig.MemberlistConfig.BindPort = s.Port
	serfConfig.MemberlistConfig.AdvertisePort = s.Port
	serfConfig.NodeName = s.Name
	serfConfig.EventCh = s.events

	s.serf, err = serf.Create(serfConfig)

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

func (s *Serf) Join(bootstrapAddrs []string) {
	for {
		if _, err := s.serf.Join(os.Args[3:], false); err != nil {
			log.Println(err)
			time.Sleep(2 * time.Second)
			continue
		}

		break
	}
}

func (s *Serf) Members() []serf.Member {
	return s.serf.Members()
}
