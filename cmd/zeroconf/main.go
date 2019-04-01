package main

import (
	"flag"
	"github.com/justinbarrick/zeroconf/pkg/cluster"
	"github.com/justinbarrick/zeroconf/pkg/kubeadm"
	"log"
	"os"
	"time"
)

func main() {
	hostName, err := os.Hostname()
	if err != nil {
		log.Println("Warning: could not get hostname: ", err)
	}

	var numInitialNodes = flag.Int("initial-nodes", 3, "number of nodes to expect for bootstrapping")
	var numMasterNodes = flag.Int("master-nodes", 3, "number of master nodes to maintain")
	var address = flag.String("address", "10.0.0.155", "the address of this node")
	var port = flag.Int("port", 1234, "the port to bind to for p2p activity")
	var nodeName = flag.String("name", hostName, "the identifier to use for this node")
	flag.Parse()

	log.Println("joining cluster as", *nodeName)

	cluster := cluster.NewCluster(*nodeName, *address, *port, *numInitialNodes)

	if err = cluster.Start(flag.Args()); err != nil {
		log.Fatal(err)
	}

	k := kubeadm.NewKubeadm(cluster)
	k.Controller(*numMasterNodes)

	time.Sleep(600 * time.Second)
}
