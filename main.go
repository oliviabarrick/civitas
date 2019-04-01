package main

import (
	"strconv"
	"fmt"
	"log"
	"os"
	"time"
	hserf "github.com/hashicorp/serf/serf"
	"github.com/justinbarrick/zeroconf-k8s/pkg/lock"
	"github.com/justinbarrick/zeroconf-k8s/pkg/raft"
	"github.com/justinbarrick/zeroconf-k8s/pkg/kubeadm"
	"github.com/justinbarrick/zeroconf-k8s/pkg/serf"
)

func main() {
	numInitialNodes := 3

	strPort := os.Args[2]
	port, err := strconv.ParseInt(strPort, 10, 32)
	if err != nil {
		log.Fatal(err)
	}

	nodeName := os.Args[1]

	serfPort := port
	raftPort := port + 1
	dsyncPort := port + 2

	rpcAddr := fmt.Sprintf("10.0.0.155:%d", dsyncPort)

	leaderLock := lock.NewLock(rpcAddr, numInitialNodes)
	leaderLock.AddNode(lock.NewClient(rpcAddr))

	raft, err := raft.NewRaft(nodeName, "10.0.0.155", int(raftPort))
	if err != nil {
		log.Fatal(err)
	}

	serf := serf.NewSerf(nodeName, int(serfPort))
	serf.JoinCallback = func(event hserf.MemberEvent) {
		for _, member := range event.Members {
			fmt.Println("MEMBER JOINED", member.Name, member.Addr, member.Port)

			if member.Name == nodeName {
				continue
			}

			if ! raft.Bootstrapped() {
				memberRpcAddr := fmt.Sprintf("%s:%d", member.Addr.String(), member.Port + 2)
				leaderLock.AddNode(lock.NewClient(memberRpcAddr))
			} else if raft.Leader() {
				if err := raft.AddNode(member.Name, member.Addr, member.Port + 1); err != nil {
					log.Fatal("error adding member", err)
				}
			}
		}

		if raft.Bootstrapped() {
			return
		}

		lockAcquired, err := leaderLock.Lock()
		if err != nil && err.Error() == "not enough nodes" {
			return
		}

		if err != nil {
			log.Fatal(err)
		}

		if lockAcquired {
			if err = raft.Bootstrap(); err != nil {
				log.Fatal("could not bootstrap raft", err)
			}

			if ! raft.Leader() {
				return
			}

			fmt.Println("we are leader")

			for _, member := range serf.Members() {
				if member.Name == nodeName {
					continue
				}

				if err := raft.AddNode(member.Name, member.Addr, member.Port + 1); err != nil {
					log.Fatal("error adding member", err)
				}

				fmt.Println("ADDED MEMBER", member.Addr, member.Port, member.Name)
			}

			if err := raft.Apply([]byte("hello world")); err != nil {
				log.Fatal("error writing to raft", err)
			}
		}
	}

	if err := serf.Start(); err != nil {
		log.Fatal(err)
	}

	serf.Join(os.Args[3:])

	k := kubeadm.Kubeadm{
		APIServer: "k8s.example.com",
		Token: "abcdef.abcdef12abcdef12",
		CertificateKey: "abcd",
	}
	fmt.Println(k.InitWorker())

	time.Sleep(60 * time.Second)
}
