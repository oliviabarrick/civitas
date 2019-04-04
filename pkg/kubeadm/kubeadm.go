package kubeadm

import (
	"github.com/justinbarrick/civitas/pkg/proxy"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/justinbarrick/civitas/pkg/cluster"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	kubeadm "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta1"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"
)

func random(length int) string {
	bytes := make([]byte, length)

	for i := 0; i < length; i++ {
		bytes[i] = byte(97 + rand.Intn(123-97))
	}

	return string(bytes)
}

func writeConfig(objs ...runtime.Object) (string, error) {
	tmpfile, err := ioutil.TempFile("", "kubeadm*.yaml")
	if err != nil {
		return "", err
	}

	encoder := kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, nil, nil)

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
	log.Println("running command:", name, arg)
	cmd := exec.Command(name, arg...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type Kubeadm struct {
	Token          string
	CertificateKey string
	Masters        []string
	cluster        *cluster.Cluster
	proxy          *proxy.Proxy
	controlPlaneIP string
}

func NewKubeadm(cluster *cluster.Cluster, controlPlaneIP string) *Kubeadm {
	return &Kubeadm{
		cluster: cluster,
		proxy: proxy.NewProxy(fmt.Sprintf("%s:6444", controlPlaneIP)),
		controlPlaneIP: controlPlaneIP,
	}
}

func (k *Kubeadm) ClusterConfiguration() *kubeadm.ClusterConfiguration {
	return &kubeadm.ClusterConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterConfiguration",
			APIVersion: "kubeadm.k8s.io/v1beta1",
		},
		KubernetesVersion: "v1.14.0",
		APIServer: kubeadm.APIServer{
			CertSANs: []string{k.controlPlaneIP,},
		},
		ControlPlaneEndpoint: fmt.Sprintf("%s:6444", k.controlPlaneIP),
	}
}

func (k *Kubeadm) JoinConfiguration(master bool) *kubeadm.JoinConfiguration {
	joinConfig := kubeadm.JoinConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "JoinConfiguration",
			APIVersion: "kubeadm.k8s.io/v1beta1",
		},
		Discovery: kubeadm.Discovery{
			BootstrapToken: &kubeadm.BootstrapTokenDiscovery{
				APIServerEndpoint:        fmt.Sprintf("%s:6444", k.controlPlaneIP),
				Token:                    k.Token,
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
			Kind:       "InitConfiguration",
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
					ID:     token[0],
					Secret: token[1],
				},
				TTL: &metav1.Duration{},
			},
		},
		LocalAPIEndpoint: kubeadm.APIEndpoint{
			BindPort: 6443,
		},
	}
}

func (k *Kubeadm) Reset() error {
	return run("kubeadm", "reset", "--force")
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

	// TODO: make configurable
	for _, preflight := range []string{
		"FileContent--proc-sys-net-bridge-bridge-nf-call-iptables",
		"Swap", "SystemVerification",
	} {
		args = append(args, "--ignore-preflight-errors", preflight)
	}

	return run("kubeadm", args...)
}

func (k *Kubeadm) InitCluster() error {
	log.Println("initializing as Kubernetes bootstrap node.")

	return k.Kubeadm([]string{
		"init", "--experimental-upload-certs",
		"--certificate-key", k.CertificateKey,
	}, k.InitConfiguration(), k.ClusterConfiguration(),
	)
}

func (k *Kubeadm) InitMaster() error {
	log.Println("initializing as Kubernetes master.")

	return k.Kubeadm([]string{
		"join", "--certificate-key", k.CertificateKey,
	}, k.JoinConfiguration(true), k.ClusterConfiguration(),
	)
}

func (k *Kubeadm) InitWorker() error {
	log.Println("initializing as Kubernetes worker.")

	return k.Kubeadm([]string{
		"join",
	}, k.JoinConfiguration(false), k.ClusterConfiguration(),
	)
}

func (k *Kubeadm) GenerateBootstrapToken() string {
	return fmt.Sprintf("%s.%s", random(6), random(16))
}

func (k *Kubeadm) GenerateCertificateKey() string {
	h := sha256.New()
	h.Write([]byte(random(255)))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (k *Kubeadm) PickMaster() {
	members := k.cluster.Members()

	rand.Seed(time.Now().Unix())

	picked := map[string]bool{}
	for _, master := range k.Masters {
		picked[master] = true
	}

	for {
		master := members[rand.Intn(len(members))].Name
		if !picked[master] {
			k.Masters = append(k.Masters, master)
			return
		}
	}
}

func (k *Kubeadm) SetBootstrapToken(token string) {
	k.Token = token
}

func (k *Kubeadm) SetCertificateKey(certificateKey string) {
	k.CertificateKey = certificateKey
}

func (k *Kubeadm) SetCluster(cluster *cluster.Cluster) {
	k.cluster = cluster
}

func (k *Kubeadm) IsBootstrap() bool {
	return k.Masters[0] == k.cluster.NodeName
}

func (k *Kubeadm) IsMaster() bool {
	for _, master := range k.Masters {
		if master == k.cluster.NodeName {
			return true
		}
	}

	return false
}

func (k *Kubeadm) StartNode() error {
	if k.IsBootstrap() {
		return k.InitCluster()
	} else if k.IsMaster() {
		return k.InitMaster()
	} else {
		return k.InitWorker()
	}
}

func (k *Kubeadm) FilterMasters() {
	members := k.cluster.Members()

	knownMembers := map[string]bool{}
	for _, member := range members {
		knownMembers[member.Name] = true
	}

	masters := []string{}
	for _, master := range k.Masters {
		if knownMembers[master] {
			masters = append(masters, master)
		}
	}

	k.Masters = masters
}

func (k *Kubeadm) PickMasters(numMasterNodes int) {
	k.FilterMasters()

	for len(k.Masters) < numMasterNodes {
		k.PickMaster()
	}
}

func (k *Kubeadm) ClusterLeader(numMasterNodes int) error {
	<-k.cluster.NotifyChannel()

	log.Println("elected as cluster leader.")

	k.PickMasters(numMasterNodes)

	if k.Token == "" {
		k.Token = k.GenerateBootstrapToken()
	}

	if k.CertificateKey == "" {
		k.CertificateKey = k.GenerateCertificateKey()
	}

	if err := k.cluster.Send(&k); err != nil {
		return err
	}

	return nil
}

func (k *Kubeadm) WaitForClusterState() error {
	clusterStateBytes := <-k.cluster.LogChannel()

	if err := json.Unmarshal(clusterStateBytes, k); err != nil {
		return err
	}

	log.Println("got cluster state:", k)
	k.UpdateAPIProxy()

	return k.StartNode()
}

func (k *Kubeadm) Controller(numMasterNodes int) {
	if err := k.proxy.Run(); err != nil {
		log.Fatal("error starting api server proxy:", err)
	}

	go func() {
		for {
			if err := k.ClusterLeader(numMasterNodes); err != nil {
				log.Fatal("cluster leader error:", err)
			}
		}
	}()

	go func() {
		for {
			if err := k.WaitForClusterState(); err != nil {
				log.Fatal("error initializing cluster:", err)
			}
		}
	}()
}

func (k *Kubeadm) UpdateAPIProxy() {
	masterIPs := []string{}
	members := k.cluster.Members()
	for _, master := range k.Masters {
		for _, member := range members {
			if member.Name != master {
				continue
			}

			masterIPs = append(masterIPs, fmt.Sprintf("%s:6443", member.Addr.String()))
		}
	}

	k.proxy.Set(masterIPs)
}
