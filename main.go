package main

import (
	"strconv"
	"github.com/hashicorp/go-msgpack/codec"
	"sync"
	"io"
	"io/ioutil"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
	kubeadm "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/runtime"
	"github.com/hashicorp/serf/serf"
	"github.com/hashicorp/raft"
	"github.com/justinbarrick/zeroconf-k8s/pkg/lock"
)

func writeConfig(objs... runtime.Object) (string, error) {
	tmpfile, err := ioutil.TempFile("", "kubeadm*.yaml")
	if err != nil {
		return "", err
	}

	encoder := json.NewYAMLSerializer(json.DefaultMetaFactory, nil, nil)

	for _, obj := range objs {
		if err := encoder.Encode(obj, tmpfile); err != nil {
			return "", err
		}

		tmpfile.Write([]byte("---\n"))
	}

	tmpfile.Close()
	return tmpfile.Name(), nil
}

func run(name string, arg ...string) error {
	fmt.Println(name, arg)
	return nil
	cmd := exec.Command(name, arg...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type Kubeadm struct {
	APIServer string
	Token string
	CertificateKey string
}

func (k *Kubeadm) ClusterConfiguration() *kubeadm.ClusterConfiguration {
	return &kubeadm.ClusterConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterConfiguration",
			APIVersion: "kubeadm.k8s.io/v1beta1",
		},
		KubernetesVersion: "v1.14.0",
		APIServer: kubeadm.APIServer{
			CertSANs: []string{k.APIServer},
		},
		ControlPlaneEndpoint: fmt.Sprintf("%s:6443", k.APIServer),
	}
}

func (k *Kubeadm) JoinConfiguration(master bool) *kubeadm.JoinConfiguration {
	joinConfig := kubeadm.JoinConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind: "JoinConfiguration",
			APIVersion: "kubeadm.k8s.io/v1beta1",
		},
		Discovery: kubeadm.Discovery{
			BootstrapToken: &kubeadm.BootstrapTokenDiscovery{
				APIServerEndpoint: fmt.Sprintf("%s:6443", k.APIServer),
				Token: k.Token,
				UnsafeSkipCAVerification: true,
			},
		},
	}

	if master {
		joinConfig.ControlPlane = &kubeadm.JoinControlPlane{
			LocalAPIEndpoint: kubeadm.APIEndpoint{
				BindPort: 6443,
			},
		}
	}

	return &joinConfig
}

func (k *Kubeadm) InitConfiguration() *kubeadm.InitConfiguration {
	token := strings.Split(k.Token, ".")

	return &kubeadm.InitConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind: "InitConfiguration",
			APIVersion: "kubeadm.k8s.io/v1beta1",
		},
		BootstrapTokens: []kubeadm.BootstrapToken{
			kubeadm.BootstrapToken{
				Groups: []string{
					"system:bootstrappers:kubeadm:default-node-token",
				},
				Usages: []string{
					"signing",
					"authentication",
				},
				Token: &kubeadm.BootstrapTokenString{
					ID: token[0],
					Secret: token[1],
				},
				TTL: &metav1.Duration{},
			},
		},
	}
}

func (k *Kubeadm) Reset() error {
	return run("kubeadm", "reset")
}

func (k *Kubeadm) Kubeadm(args []string, configObjs ...runtime.Object) error {
	configPath, err := writeConfig(configObjs...)
	if err != nil {
		return err
	}

	if err := k.Reset(); err != nil {
		return err
	}

	// defer os.Remove(configPath)

	args = append(args, "--config", configPath)

	return run("kubeadm", args...)
}

func (k *Kubeadm) InitCluster() error {
	return k.Kubeadm([]string{
			"init", "--experimental-upload-certs",
			"--certificate-key", k.CertificateKey,
		}, k.InitConfiguration(), k.ClusterConfiguration(),
	)
}

func (k *Kubeadm) InitMaster() error {
	return k.Kubeadm([]string{
			"join", "--certificate-key", k.CertificateKey,
		}, k.JoinConfiguration(true), k.ClusterConfiguration(),
	)
}

func (k *Kubeadm) InitWorker() error {
	return k.Kubeadm([]string{
			"join",
		}, k.JoinConfiguration(false), k.ClusterConfiguration(),
	)
}

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

func addVoter() {

}

func main() {
	numInitialNodes := 3

	strPort := os.Args[2]
	port, err := strconv.ParseInt(strPort, 10, 32)
	if err != nil {
		log.Fatal(err)
	}

	nodeName := os.Args[1]

	serfPort := port
	raftPort := port + 1
	dsyncPort := port + 2

	rpcAddr := fmt.Sprintf("10.0.0.155:%d", dsyncPort)
	raftAddr := fmt.Sprintf("10.0.0.155:%d", raftPort)

	leaderLock := lock.NewLock(rpcAddr, numInitialNodes)
	leaderLock.AddNode(lock.NewClient(rpcAddr))

	addr, err := net.ResolveTCPAddr("tcp", raftAddr)
	if err != nil {
		log.Fatal(err)
	}

	t, err := raft.NewTCPTransport(raftAddr, addr, 5, 5 * time.Second, os.Stderr)
	if err != nil {
		log.Fatal("creating transport: ", err)
	}

	snapshotStore := raft.NewInmemSnapshotStore()
	store := raft.NewInmemStore()

	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(nodeName)

	r, err := raft.NewRaft(raftConfig, &MockFSM{}, store, store, snapshotStore, t)
	if err != nil {
		log.Fatal("bootstrapping raft: ", err)
	}

	events := make(chan serf.Event)
	serfConfig := serf.DefaultConfig()
	serfConfig.MemberlistConfig.BindPort = int(serfPort)
	serfConfig.MemberlistConfig.AdvertisePort = int(serfPort)
	serfConfig.NodeName = nodeName
	serfConfig.EventCh = events

	s, err := serf.Create(serfConfig)
	if err != nil {
		log.Fatal(err)
	}

	bootstrapped := false
	leader := false

	go func() {
		for event := range events {
			if event.EventType() != serf.EventMemberJoin {
				continue
			}

			for _, member := range event.(serf.MemberEvent).Members {
				fmt.Println("MEMBER JOINED", member.Name, member.Addr, member.Port)

				if member.Name == nodeName {
					continue
				}

				if ! bootstrapped {
					memberRpcAddr := fmt.Sprintf("%s:%d", member.Addr.String(), member.Port + 2)
					leaderLock.AddNode(lock.NewClient(memberRpcAddr))
				} else if leader {
					memberAddr := raft.ServerAddress(fmt.Sprintf("%s:%d", member.Addr, member.Port + 1))
					if err := r.AddVoter(raft.ServerID(member.Name), memberAddr, 0, 5 * time.Second).Error(); err != nil {
						log.Fatal("error adding member", err)
					}
				}
			}

			if bootstrapped {
				continue
			}

			lockAcquired, err := leaderLock.Lock()
			if err != nil {
				log.Fatal(err)
			}

			if lockAcquired {
				err = r.BootstrapCluster(raft.Configuration{
					Servers: []raft.Server{
						{
							ID:      raft.ServerID(nodeName),
							Address: raft.ServerAddress(fmt.Sprintf("10.0.0.155:%d", raftPort)),
						},
					},
				}).Error()
				if err != nil {
					log.Fatal("error bootstrapping initial leader", err)
				}

				acquiredLeader := <-r.LeaderCh()
				if ! acquiredLeader {
					log.Fatal("!!! SOMETHING BAD HAPPENED")
				}

				fmt.Println("we are leader")

				for _, member := range s.Members() {
					if member.Name == nodeName {
						continue
					}

					memberAddr := raft.ServerAddress(fmt.Sprintf("%s:%d", member.Addr, member.Port + 1))
					if err := r.AddVoter(raft.ServerID(member.Name), memberAddr, 0, 5 * time.Second).Error(); err != nil {
						log.Fatal("error adding member", err)
					}

					fmt.Println("ADDED MEMBER", member.Addr, member.Port, member.Name)
				}

				if err := r.Apply([]byte("hello world"), 5 * time.Second).Error(); err != nil {
					log.Fatal("error writing to raft", err)
				}

				leader = true
			} else {
				fmt.Println("we are follower")
			}

			bootstrapped = true
		}
	}()

	for {
		if _, err := s.Join(os.Args[3:], false); err != nil {
			log.Println(err)
			time.Sleep(2 * time.Second)
			continue
		}

		break
	}

/*
	k := Kubeadm{
		APIServer: "k8s.example.com",
		Token: "abcdef.abcdef12abcdef12",
		CertificateKey: "abcd",
	}
	fmt.Println(k.InitCluster())
	fmt.Println(k.InitMaster())
	fmt.Println(k.InitWorker())
*/

	time.Sleep(60 * time.Second)
}
