package raft

import (
	"errors"
	"fmt"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/raft"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

type MockFSM struct {
	sync.Mutex
	logs [][]byte
}

type MockSnapshot struct {
	logs     [][]byte
	maxIndex int
}

func (m *MockFSM) Apply(log *raft.Log) interface{} {
	m.Lock()
	defer m.Unlock()
	fmt.Println("applying", log.Data)
	m.logs = append(m.logs, log.Data)
	return len(m.logs)
}

func (m *MockFSM) Snapshot() (raft.FSMSnapshot, error) {
	m.Lock()
	defer m.Unlock()
	return &MockSnapshot{m.logs, len(m.logs)}, nil
}

func (m *MockFSM) Restore(inp io.ReadCloser) error {
	m.Lock()
	defer m.Unlock()
	defer inp.Close()
	hd := codec.MsgpackHandle{}
	dec := codec.NewDecoder(inp, &hd)

	m.logs = nil
	return dec.Decode(&m.logs)
}

func (m *MockSnapshot) Persist(sink raft.SnapshotSink) error {
	hd := codec.MsgpackHandle{}
	enc := codec.NewEncoder(sink, &hd)
	if err := enc.Encode(m.logs[:m.maxIndex]); err != nil {
		sink.Cancel()
		return err
	}
	sink.Close()
	return nil
}

func (m *MockSnapshot) Release() {
}

type Raft struct {
	Name       string
	ListenAddr string
	raft       *raft.Raft
}

func NewRaft(name, listenAddr string, port int) (*Raft, error) {
	raft := &Raft{
		Name:       name,
		ListenAddr: fmt.Sprintf("%s:%d", listenAddr, port),
	}

	return raft, raft.Start()
}

func (r *Raft) Start() error {
	addr, err := net.ResolveTCPAddr("tcp", r.ListenAddr)
	if err != nil {
		return err
	}

	t, err := raft.NewTCPTransport(r.ListenAddr, addr, 5, 5*time.Second, os.Stderr)
	if err != nil {
		return err
	}

	snapshotStore := raft.NewInmemSnapshotStore()
	store := raft.NewInmemStore()

	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(r.Name)

	r.raft, err = raft.NewRaft(raftConfig, &MockFSM{}, store, store, snapshotStore, t)
	return err
}

func (r *Raft) Bootstrapped() bool {
	return r.raft.LastIndex() > 0
}

func (r *Raft) Bootstrap() error {
	if r.Bootstrapped() {
		return nil
	}

	err := r.raft.BootstrapCluster(raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      raft.ServerID(r.Name),
				Address: raft.ServerAddress(r.ListenAddr),
			},
		},
	}).Error()
	if err != nil {
		return err
	}

	acquiredLeader := <-r.raft.LeaderCh()
	if !acquiredLeader {
		return errors.New("error acquiring leader while bootstrapping")
	}

	return nil
}

func (r *Raft) AddNode(name string, addr net.IP, port uint16) error {
	memberAddr := raft.ServerAddress(fmt.Sprintf("%s:%d", addr, port))
	err := r.raft.AddVoter(raft.ServerID(name), memberAddr, 0, 5*time.Second).Error()
	fmt.Printf("ADDED MEMBER %s (%s:%d)\n", name, addr, port)
	return err
}

func (r *Raft) Apply(data []byte) error {
	return r.raft.Apply(data, 5*time.Second).Error()
}

func (r *Raft) Leader() bool {
	return r.raft.State() == raft.Leader
}
