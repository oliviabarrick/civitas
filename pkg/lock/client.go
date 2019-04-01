package lock

import (
	"net/rpc"
	"sync"

	"github.com/minio/dsync"
)

type ReconnectRPCClient struct {
	mutex sync.Mutex
	rpc   *rpc.Client
	addr  string
}

func NewClient(addr string) dsync.NetLocker {
	return &ReconnectRPCClient{
		addr: addr,
	}
}

func (rpcClient *ReconnectRPCClient) Close() error {
	rpcClient.mutex.Lock()
	defer rpcClient.mutex.Unlock()

	if rpcClient.rpc == nil {
		return nil
	}

	clnt := rpcClient.rpc
	rpcClient.rpc = nil

	return clnt.Close()
}

func (rpcClient *ReconnectRPCClient) Call(serviceMethod string, args interface{}, reply interface{}) (err error) {
	rpcClient.mutex.Lock()
	defer rpcClient.mutex.Unlock()

	dialCall := func() error {
		if rpcClient.rpc == nil {
			clnt, derr := rpc.DialHTTP("tcp", rpcClient.addr)
			if derr != nil {
				return derr
			}
			rpcClient.rpc = clnt
		}

		return rpcClient.rpc.Call(serviceMethod, args, reply)
	}

	if err = dialCall(); err == rpc.ErrShutdown {
		rpcClient.rpc.Close()
		rpcClient.rpc = nil
		err = dialCall()
	}

	return err
}

func (rpcClient *ReconnectRPCClient) RLock(args dsync.LockArgs) (status bool, err error) {
	err = rpcClient.Call("Dsync.RLock", &args, &status)
	return status, err
}

func (rpcClient *ReconnectRPCClient) Lock(args dsync.LockArgs) (status bool, err error) {
	err = rpcClient.Call("Dsync.Lock", &args, &status)
	return status, err
}

func (rpcClient *ReconnectRPCClient) RUnlock(args dsync.LockArgs) (status bool, err error) {
	err = rpcClient.Call("Dsync.RUnlock", &args, &status)
	return status, err
}

func (rpcClient *ReconnectRPCClient) Unlock(args dsync.LockArgs) (status bool, err error) {
	err = rpcClient.Call("Dsync.Unlock", &args, &status)
	return status, err
}

func (rpcClient *ReconnectRPCClient) ForceUnlock(args dsync.LockArgs) (status bool, err error) {
	err = rpcClient.Call("Dsync.ForceUnlock", &args, &status)
	return status, err
}

func (rpcClient *ReconnectRPCClient) ServerAddr() string {
	return rpcClient.addr
}

func (rpcClient *ReconnectRPCClient) ServiceEndpoint() string {
	return "/"
}
