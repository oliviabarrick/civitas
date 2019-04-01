package lock

import (
	"errors"
	"fmt"
	"github.com/minio/dsync"
	"sync"
)

type LockServer struct {
	mutex  sync.Mutex
	locked bool
}

func (ls *LockServer) Lock(args *dsync.LockArgs, reply *bool) error {
	ls.mutex.Lock()
	defer ls.mutex.Unlock()

	fmt.Println("lock (server)!")
	if !ls.locked {
		ls.locked = true
		*reply = true
	} else {
		*reply = false
	}

	return nil
}

func (ls *LockServer) Unlock(args *dsync.LockArgs, reply *bool) error {
	ls.mutex.Lock()
	defer ls.mutex.Unlock()

	ls.locked = false
	*reply = true

	return nil
}

func (ls *LockServer) RUnlock(args *dsync.LockArgs, reply *bool) error {
	return errors.New("RUnlock is not supported")
}

func (ls *LockServer) RLock(args *dsync.LockArgs, reply *bool) error {
	return errors.New("RLock is not supported")
}

func (ls *LockServer) ForceUnlock(args *dsync.LockArgs, reply *bool) error {
	return errors.New("ForceUnlock is not supported")
}
