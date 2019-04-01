package main

import (
	"strconv"
	"fmt"
	"log"
	"os"
	"time"
	"github.com/justinbarrick/zeroconf-k8s/pkg/cluster"
	"github.com/justinbarrick/zeroconf-k8s/pkg/kubeadm"
)

func main() {
	numInitialNodes := 3

	strPort := os.Args[2]
	port, err := strconv.ParseInt(strPort, 10, 32)
	if err != nil {
		log.Fatal(err)
	}

	nodeName := os.Args[1]

	cluster := cluster.NewCluster(nodeName, "10.0.0.155", int(port), numInitialNodes)

	if err = cluster.Start(os.Args[3:]); err != nil {
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
