package service

import (
	"fmt"
	"net"

	"github.com/golang/protobuf/proto"
	pb "github.com/iotaledger/goshimmer/packages/autopeering/peer/service/proto"
)

// Record defines the mapping between a service ID and its tuple TypePort
// e.g., map[autopeering:&{TCP, 8000}]
type Record struct {
	m map[string]*networkAddress
}

// networkAddress implements net.Addr
type networkAddress struct {
	network string
	address string
}

// Network returns the service's network name.
func (a *networkAddress) Network() string {
	return a.network
}

// String returns the service's address in string form.
func (a *networkAddress) String() string {
	return a.address
}

// New initializes and returns an empty new Record
func New() *Record {
	return &Record{
		m: make(map[string]*networkAddress),
	}
}

// Get returns the network end point address of the service with the given name.
func (s *Record) Get(key Key) net.Addr {
	val, ok := s.m[string(key)]
	if !ok {
		return nil
	}
	return val
}

// CreateRecord creates a modifyable Record from the services.
func (s *Record) CreateRecord() *Record {
	result := New()
	for k, v := range s.m {
		result.m[k] = v
	}
	return result
}

// Update adds a new service to the map.
func (s *Record) Update(key Key, network string, address string) {
	s.m[string(key)] = &networkAddress{
		network: network, address: address,
	}
}

// String returns a string representation of the service record.
func (s *Record) String() string {
	return fmt.Sprintf("%v", s.m)
}

// FromProto creates a Record from the provided protobuf struct.
func FromProto(in *pb.ServiceMap) (*Record, error) {
	m := in.GetMap()
	if m == nil {
		return nil, nil
	}

	services := New()
	for service, addr := range m {
		services.m[service] = &networkAddress{
			network: addr.GetNetwork(),
			address: addr.GetAddress(),
		}
	}
	return services, nil
}

// ToProto returns the corresponding protobuf struct.
func (s *Record) ToProto() *pb.ServiceMap {
	if len(s.m) == 0 {
		return nil
	}

	services := make(map[string]*pb.NetworkAddress, len(s.m))
	for service, addr := range s.m {
		services[service] = &pb.NetworkAddress{
			Network: addr.network,
			Address: addr.address,
		}
	}

	return &pb.ServiceMap{
		Map: services,
	}
}

// Marshal serializes a given Peer (p) into a slice of bytes.
func (s *Record) Marshal() ([]byte, error) {
	return proto.Marshal(s.ToProto())
}

// Unmarshal de-serializes a given slice of bytes (data) into a Peer.
func Unmarshal(data []byte) (*Record, error) {
	s := &pb.ServiceMap{}
	if err := proto.Unmarshal(data, s); err != nil {
		return nil, err
	}
	return FromProto(s)
}
