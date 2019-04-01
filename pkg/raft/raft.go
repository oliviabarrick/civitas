package raft

import (
	"io/ioutil"
	"errors"
	"fmt"
	"log"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/raft"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

type FSM struct {
	sync.Mutex
	logCh chan []byte
	logs [][]byte
}

type Snapshot struct {
	logs     [][]byte
	maxIndex int
}

func NewFSM() *FSM {
	return &FSM{
		logCh: make(chan []byte),
	}
}

func (m *FSM) Apply(log *raft.Log) interface{} {
	m.Lock()
	defer m.Unlock()
	m.logs = append(m.logs, log.Data)
	m.logCh <- log.Data
	return len(m.logs)
}

func (m *FSM) Snapshot() (raft.FSMSnapshot, error) {
	m.Lock()
	defer m.Unlock()
	return &Snapshot{m.logs, len(m.logs)}, nil
}

func (m *FSM) Restore(inp io.ReadCloser) error {
	m.Lock()
	defer m.Unlock()
	defer inp.Close()
	hd := codec.MsgpackHandle{}
	dec := codec.NewDecoder(inp, &hd)

	m.logs = nil
	return dec.Decode(&m.logs)
}

func (m *Snapshot) Persist(sink raft.SnapshotSink) error {
	hd := codec.MsgpackHandle{}
	enc := codec.NewEncoder(sink, &hd)
	if err := enc.Encode(m.logs[:m.maxIndex]); err != nil {
		sink.Cancel()
		return err
	}
	sink.Close()
	return nil
}

func (m *Snapshot) Release() {}

type Raft struct {
	Name       string
	ListenAddr string
	raft       *raft.Raft
	fsm *FSM
	notifyCh   chan bool
	added map[string]bool
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

	var logOutput io.Writer
	if os.Getenv("DEBUG") != "1" {
		logOutput = ioutil.Discard
	} else {
		logOutput = os.Stderr
	}

	t, err := raft.NewTCPTransport(r.ListenAddr, addr, 5, 5*time.Second, logOutput)
	if err != nil {
		return err
	}

	snapshotStore := raft.NewInmemSnapshotStore()
	store := raft.NewInmemStore()

	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(r.Name)
	raftConfig.LogOutput = logOutput

	r.notifyCh = make(chan bool)
	raftConfig.NotifyCh = r.notifyCh

	r.fsm = NewFSM()
	r.raft, err = raft.NewRaft(raftConfig, r.fsm, store, store, snapshotStore, t)

	r.added = map[string]bool{}

	log.Println("raft listening at:", r.ListenAddr)
	return err
}

func (r *Raft) LogChannel() (chan []byte) {
	return r.fsm.logCh
}

func (r *Raft) NotifyChannel() (chan bool) {
	return r.notifyCh
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
	if r.added[name] == false {
		log.Printf("added member to raft %s (%s:%d)\n", name, addr, port)
	}
	r.added[name] = true
	return err
}

func (r *Raft) Apply(data []byte) error {
	return r.raft.Apply(data, 5*time.Second).Error()
}

func (r *Raft) Leader() bool {
	return r.raft.State() == raft.Leader
}
