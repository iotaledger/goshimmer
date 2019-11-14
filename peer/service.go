package peer

import (
	pb "github.com/wollac/autopeering/peer/proto"
)

// NetworkAddress defines the tuple <Type, Port>, e.g, <TCP, 8000>
type NetworkAddress struct {
	Network string
	Address string
}

// ServiceMap defines the mapping between a service ID and its tuple TypePort
// e.g., map[autopeering:&{TCP, 8000}]
type ServiceMap = map[string]NetworkAddress

// NewServiceMap initializes and returns an empty new ServiceMap
func NewServiceMap() ServiceMap {
	return make(ServiceMap)
}

// NewServiceMapFromProto creates a ServiceMap from the provided protobuf struct.
func NewServiceMapFromProto(in *pb.ServiceMap) ServiceMap {
	m := in.GetMap()
	if m == nil {
		return nil
	}

	services := make(ServiceMap, len(m))
	for service, addr := range m {
		services[service] = NetworkAddress{Network: addr.GetNetwork(), Address: addr.GetAddress()}
	}

	return services
}

// ServiceMapToProto returns the corresponding protobuf struct.
func ServiceMapToProto(m ServiceMap) *pb.ServiceMap {
	if len(m) == 0 {
		return nil
	}

	services := make(map[string]*pb.NetworkAddress, len(m))
	for service, addr := range m {
		services[service] = &pb.NetworkAddress{
			Network: addr.Network,
			Address: addr.Address,
		}
	}

	return &pb.ServiceMap{
		Map: services,
	}
}
