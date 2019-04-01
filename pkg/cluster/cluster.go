package cluster

import (
	"fmt"
	"log"
	hserf "github.com/hashicorp/serf/serf"
	"github.com/justinbarrick/zeroconf/pkg/lock"
	"github.com/justinbarrick/zeroconf/pkg/raft"
	"github.com/justinbarrick/zeroconf/pkg/serf"
)

type Cluster struct {
	port int
	addr string
	nodeName string
	numInitialNodes int
	raft *raft.Raft
	serf *serf.Serf
	lock *lock.Lock
}

func NewCluster(nodeName, addr string, port int, numInitialNodes int) *Cluster {
	return &Cluster{
		nodeName: nodeName,
		port: port,
		addr: addr,
		numInitialNodes: numInitialNodes,
	}
}

func (c *Cluster) Start(bootstrapAddrs []string) error {
	var err error

	serfPort := c.port
	raftPort := c.port + 1
	dsyncPort := c.port + 2

	c.raft, err = raft.NewRaft(c.nodeName, c.addr, int(raftPort))
	if err != nil {
		return err
	}

	c.serf = serf.NewSerf(c.nodeName, int(serfPort))
	c.serf.JoinCallback = c.JoinCallback

	rpcAddr := fmt.Sprintf("%s:%d", c.addr, dsyncPort)

	c.lock = lock.NewLock(rpcAddr, c.numInitialNodes)
	c.lock.AddNode(lock.NewClient(rpcAddr))

	if err := c.serf.Start(); err != nil {
		return err
	}

	c.serf.Join(bootstrapAddrs)
	return nil
}

func (c *Cluster) JoinCallback(event hserf.MemberEvent) {
	if ! c.raft.Bootstrapped() {
		for _, member := range c.serf.Members() {
			memberRpcAddr := fmt.Sprintf("%s:%d", member.Addr.String(), member.Port + 2)
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

			if ! c.raft.Leader() {
				return
			}

			if err := c.raft.Apply([]byte("hello world")); err != nil {
				log.Fatal("error writing to raft", err)
			}
		}
	}

	if c.raft.Bootstrapped() && c.raft.Leader() {
		for _, member := range c.serf.Members() {
			if member.Name == c.nodeName {
				continue
			}

			if err := c.raft.AddNode(member.Name, member.Addr, member.Port + 1); err != nil {
				log.Fatal("error adding member", err)
			}

			fmt.Println("ADDED MEMBER", member.Addr, member.Port, member.Name)
		}
	}
}
