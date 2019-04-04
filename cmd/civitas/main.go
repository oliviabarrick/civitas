package main

import (
	"flag"
	"github.com/justinbarrick/civitas/pkg/cluster"
	"github.com/justinbarrick/civitas/pkg/kubeadm"
	"github.com/justinbarrick/civitas/pkg/util"
	"log"
	"os"
	"strings"
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
	var mdnsService = flag.String("mdns-service", os.Getenv("MDNS_SERVICE"), "The mDNS service to broadcast.")
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

	discoveryConfig := flag.Args()
	envDiscovery := os.Getenv("DISCOVERY_CONFIG")
	if envDiscovery != "" {
		discoveryConfig = append(discoveryConfig, strings.Split(envDiscovery, "\n")...)
	}

	cluster := &cluster.Cluster{
		NodeName: *nodeName,
		Addr: *address,
		Port: *port,
		NumInitialNodes: *numInitialNodes,
		MDNSService: *mdnsService,
		DiscoveryConfig: discoveryConfig,
	}

	if err = cluster.Start(); err != nil {
		log.Fatal(err)
	}

	k := kubeadm.NewKubeadm(cluster, *controlPlaneIP)
	k.Controller(*numMasterNodes)

	select{ }
}
