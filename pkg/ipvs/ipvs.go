package ipvs

import (
	"github.com/mqliang/libipvs"
	"net"
	"syscall"
)

type IPVS struct {
	ipvs libipvs.IPVSHandle
}

func NewIPVS() (*IPVS, error) {
	ipvs, err := libipvs.NewIPVSHandle(libipvs.IPVSHandleParams{})
	if err != nil {
		return nil, err
	}

	return &IPVS{
		ipvs: ipvs,
	}, nil
}

func (i *IPVS) Set(service string, ip string, port int, upstreams []string, upstreamPort int) error {
	s, err := i.SetService(service, ip, port)
	if err != nil {
		return err
	}

	return i.SetDestinations(s, upstreams, upstreamPort)
}

func (i *IPVS) SetService(service string, ip string, port int) (*libipvs.Service, error) {
	svcs, err := i.ipvs.ListServices()
	if err != nil {
		return nil, err
	}

	exists := false
	for _, svc := range svcs {
		if svc.Address.String() != "k8s-api" {
			continue
		}

		exists = true
	}

	k8s := &libipvs.Service{
		Address: net.ParseIP(ip),
		AddressFamily: syscall.AF_INET,
		Protocol: libipvs.Protocol(syscall.IPPROTO_TCP),
		Port: 6443,
		SchedName: libipvs.RoundRobin,
	}

	ipvsCommand := i.ipvs.NewService
	if exists {
		ipvsCommand = i.ipvs.UpdateService
	}

	return k8s, ipvsCommand(k8s)
}

func (i *IPVS) SetDestinations(service *libipvs.Service, upstreams []string, port int) error {
	expected := []*libipvs.Destination{}

	for _, upstream := range upstreams {
		expected = append(expected, &libipvs.Destination{
			Address: net.ParseIP(upstream),
			AddressFamily: syscall.AF_INET,
			Port: uint16(port),
			Weight: 10,
		})
	}

	destinations, err := i.ipvs.ListDestinations(service)
	if err != nil {
		return err
	}

	found := map[string]bool{}

	for _, destination := range destinations {
		destFound := false

		for _, expect := range expected {
			if expect.Address.String() == destination.Address.String() {
				destFound = true
				found[expect.Address.String()] = true
			}
		}

		if ! destFound {
			if err := i.ipvs.DelDestination(service, destination); err != nil {
				return err
			}
		}
	}

	for _, expect := range expected {
		destMethod := i.ipvs.NewDestination
		if found[expect.Address.String()] {
			destMethod = i.ipvs.UpdateDestination
		}

		if err := destMethod(service, expect); err != nil {
			return err
		}
	}

	return nil
}
