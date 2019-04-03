package main

import (
	"flag"
	"github.com/justinbarrick/zeroconf/pkg/cluster"
	"github.com/justinbarrick/zeroconf/pkg/kubeadm"
	"github.com/justinbarrick/zeroconf/pkg/util"
	"log"
	"os"
)

func main() {
	hostName, err := os.Hostname()
	if err != nil {
		log.Println("Warning: could not get hostname: ", err)
	}

	var numInitialNodes = flag.Int("initial-nodes", 3, "number of nodes to expect for bootstrapping")
	var numMasterNodes = flag.Int("master-nodes", 3, "number of master nodes to maintain")
	var address = flag.String("address", "", "the address of this node")
	var iface = flag.String("interface", "", "the interface to advertise and bind to")
	var port = flag.Int("port", 1234, "the port to bind to for p2p activity")
	var nodeName = flag.String("name", hostName, "the identifier to use for this node")
	var controlPlaneIP = flag.String("control-plane-ip", "127.0.13.37", "IP address to bind the control plane load balancer to on each node.")
	flag.Parse()

	if *iface != "" {
		ipStr, err := util.GetIPForInterface(*iface)
		if err != nil {
			log.Fatal(err)
		}
		address = &ipStr
	}

	if *address == "" {
		log.Fatal("address or interface must be specified.")
	}

	log.Println("joining cluster as", *nodeName, "advertising", *address)

	cluster := cluster.NewCluster(*nodeName, *address, *port, *numInitialNodes)

	if err = cluster.Start(flag.Args()); err != nil {
		log.Fatal(err)
	}

	k := kubeadm.NewKubeadm(cluster, *controlPlaneIP)
	k.Controller(*numMasterNodes)

	select{ }
}
