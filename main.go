package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"
	"github.com/justinbarrick/zeroconf-k8s/pkg/cluster"
	"github.com/justinbarrick/zeroconf-k8s/pkg/kubeadm"
)

func main() {
	hostName, err := os.Hostname()
	if err != nil {
		log.Println("Warning: could not get hostname: ", err)
	}

	var numInitialNodes = flag.Int("initial-nodes", 3, "number of nodes to expect for bootstrapping")
	var address = flag.String("address", "10.0.0.155", "the address of this node")
	var port = flag.Int("port", 1234, "the port to bind to for p2p activity")
	var nodeName = flag.String("name", hostName, "the identifier to use for this node")
	flag.Parse()

	cluster := cluster.NewCluster(*nodeName, *address, *port, *numInitialNodes)

	if err = cluster.Start(flag.Args()); err != nil {
		log.Fatal(err)
	}

	k := kubeadm.Kubeadm{
		APIServer: "k8s.example.com",
		Token: "abcdef.abcdef12abcdef12",
		CertificateKey: "abcd",
	}

	fmt.Println(k.InitWorker())
	time.Sleep(60 * time.Second)
}
