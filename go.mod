module github.com/justinbarrick/civitas

go 1.12

replace github.com/minio/dsync => github.com/justinbarrick/dsync v0.0.0-20190331203947-a9d0969e8479

replace github.com/google/tcpproxy => github.com/yangchenyun/tcpproxy v0.0.0-20180611030643-2041ee5cacf9

require (
	github.com/gogo/protobuf v1.2.1 // indirect
	github.com/google/tcpproxy v0.0.0-00010101000000-000000000000
	github.com/hashicorp/go-discover v0.0.0-20190403160810-22221edb15cd
	github.com/hashicorp/go-msgpack v0.5.3
	github.com/hashicorp/mdns v1.0.0
	github.com/hashicorp/raft v1.0.0
	github.com/hashicorp/serf v0.8.2
	github.com/hkwi/nlgo v0.0.0-20170629055117-dbae43f4fc47 // indirect
	github.com/json-iterator/go v1.1.6 // indirect
	github.com/minio/dsync v0.0.0-20190131060523-fb604afd87b2
	github.com/mqliang/libipvs v0.0.0-20181031074626-20f197c976a3
	github.com/pkg/errors v0.8.1 // indirect
	golang.org/x/net v0.0.0-20190328230028-74de082e2cca // indirect
	gopkg.in/yaml.v2 v2.2.2 // indirect
	k8s.io/api v0.0.0-20190327184913-92d2ee7fc726 // indirect
	k8s.io/apimachinery v0.0.0-20190328224500-e508a7b04a89
	k8s.io/cluster-bootstrap v0.0.0-20190313124217-0fa624df11e9 // indirect
	k8s.io/component-base v0.0.0-20190313120452-4727f38490bc // indirect
	k8s.io/klog v0.2.0 // indirect
	k8s.io/kubernetes v1.14.0
	sigs.k8s.io/yaml v1.1.0 // indirect
)
