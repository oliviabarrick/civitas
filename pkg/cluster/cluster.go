package cluster

import (
	"encoding/json"
	"fmt"
	"strings"
	hserf "github.com/hashicorp/serf/serf"
	"github.com/justinbarrick/civitas/pkg/lock"
	"github.com/justinbarrick/civitas/pkg/raft"
	"github.com/justinbarrick/civitas/pkg/serf"
	"log"
	"io/ioutil"
	"os"
	"time"
	"github.com/hashicorp/go-discover"
	"github.com/hashicorp/mdns"
)

type Cluster struct {
	Port            int
	Addr            string
	NodeName        string
	NumInitialNodes int
	MDNSService     string
	DiscoveryConfig []string
	raft            *raft.Raft
	serf            *serf.Serf
	lock            *lock.Lock
}

func (c *Cluster) Start() error {
	var err error

	serfPort := c.Port
	raftPort := c.Port + 1
	dsyncPort := c.Port + 2

	c.raft, err = raft.NewRaft(c.NodeName, c.Addr, int(raftPort))
	if err != nil {
		return err
	}

	c.serf = serf.NewSerf(c.NodeName, c.Addr, int(serfPort))
	c.serf.JoinCallback = c.JoinCallback

	rpcAddr := fmt.Sprintf("%s:%d", c.Addr, dsyncPort)

	c.lock = lock.NewLock(rpcAddr, c.NumInitialNodes)
	c.lock.AddNode(lock.NewClient(rpcAddr))

	if err := c.serf.Start(); err != nil {
		return err
	}

	if err := c.Announce(); err != nil {
		return err
	}

	go c.DiscoverNodes()
	go c.serf.Join()

	return nil
}

func (c *Cluster) JoinCallback(event hserf.MemberEvent) {
	if !c.raft.Bootstrapped() {
		for _, member := range c.serf.Members() {
			memberRpcAddr := fmt.Sprintf("%s:%d", member.Addr.String(), member.Port+2)
			c.lock.AddNode(lock.NewClient(memberRpcAddr))
		}

		lockAcquired, err := c.lock.Lock()
		if err != nil && err.Error() == "not enough nodes" {
			return
		} else if err != nil {
			log.Fatal(err)
		}

		if lockAcquired {
			if err = c.raft.Bootstrap(); err != nil {
				log.Fatal("could not bootstrap raft", err)
			}
		}
	}

	if c.raft.Bootstrapped() && c.raft.Leader() {
		for _, member := range c.serf.Members() {
			if member.Name == c.NodeName {
				continue
			}

			if err := c.raft.AddNode(member.Name, member.Addr, member.Port+1); err != nil {
				log.Fatal("error adding member", err)
			}
		}
	}
}

func (c *Cluster) Send(obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	return c.raft.Apply(data)
}

func (c *Cluster) LogChannel() chan []byte {
	return c.raft.LogChannel()
}

func (c *Cluster) NotifyChannel() chan bool {
	return c.raft.NotifyChannel()
}

func (c *Cluster) Members() []hserf.Member {
	return c.serf.Members()
}

func (c *Cluster) Announce() error {
	if c.MDNSService == "" {
		return nil
	}

	service, err := mdns.NewMDNSService(c.NodeName, c.MDNSService, "", "", c.Port, nil, []string{})
	if err != nil {
		return err
	}

	_, err = mdns.NewServer(&mdns.Config{Zone: service})
	return err
}

func (c *Cluster) DiscoverNodes() {
	logger := ioutil.Discard
	if os.Getenv("DEBUG") == "1" {
		logger = os.Stderr
	}

	l := log.New(logger, "", log.LstdFlags)

	d := discover.Discover{}

	discoveryConfig := c.DiscoveryConfig
	if c.MDNSService != "" {
		discoveryConfig = append(discoveryConfig, fmt.Sprintf("provider=mdns service=%s domain=local", c.MDNSService))
	}

	seenPeers := map[string]bool{}

	for {
		for _, cfg := range discoveryConfig {
			tmpAddrs, err := d.Addrs(cfg, l)
			if err != nil {
				log.Println(err)
				continue
			}

			for _, addr := range tmpAddrs {
				if seenPeers[addr] {
					continue
				}

				seenPeers[addr] = true

				if ! strings.Contains(addr, ":") {
					addr = fmt.Sprintf("%s:%d", addr, c.Port)
				}

				log.Println("Discovered peer:", addr)
				c.serf.AddNode(addr)
			}
		}

		time.Sleep(2 * time.Second)
	}
}
