package kubeadm

import (
	"io/ioutil"
	"fmt"
	"os"
	"os/exec"
	"strings"
	kubeadm "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/runtime"
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
