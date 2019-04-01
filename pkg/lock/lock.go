package lock

import (
	"errors"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"github.com/minio/dsync"
)

type Lock struct {
	initialNodes int
	lockClients []dsync.NetLocker
	ds *dsync.Dsync
	dm *dsync.DRWMutex
}

func NewLock(rpcAddr string, initialNodes int) *Lock {
	go func() {
		lockServer := &LockServer{}
		rpcServer := rpc.NewServer()
		rpcServer.RegisterName("Dsync", lockServer)
		rpcServer.HandleHTTP("/", "/_debug")

		listener, err := net.Listen("tcp", rpcAddr)
		if err != nil {
			log.Fatal(err)
		}

		log.Println("LockServer listening at ", rpcAddr)
		http.Serve(listener, nil)
	} ()

	return &Lock{
		initialNodes: initialNodes,
	}
}

func (l *Lock) AddNode(node dsync.NetLocker) {
	for _, c := range l.lockClients {
		if c.ServerAddr() == node.ServerAddr() && c.ServiceEndpoint() == node.ServiceEndpoint() {
			return
		}
	}

	l.lockClients = append(l.lockClients, node)
}

func (l *Lock) Lock() (bool, error) {
	if len(l.lockClients) < l.initialNodes {
		return false, errors.New("not enough nodes")
	}

	var err error

	if l.ds == nil {
		l.ds, err = dsync.New(l.lockClients, 0)
		if err != nil {
			return false, err
		}
	}

	if l.dm == nil {
		l.dm = dsync.NewDRWMutex("leader", l.ds)
	}

	if l.dm.GetLockNonBlocking("leader", "leader") {
		return true, nil
	}

	return false, nil
}
