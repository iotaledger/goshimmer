package peer

import (
	pb "github.com/wollac/autopeering/peer/proto"
)

// TypePort defines the tuple <Type, Port>, e.g, <TCP, 8000>
type NetworkAddress struct {
	Network string
	Address string
}

// ServiceMap defines the mapping between a service ID and its tuple TypePort
// e.g., map[autopeering:&{TCP, 8000}]
type ServiceMap = map[string]*NetworkAddress

// NewServiceMap initializes and returns an empty new ServiceMap
func NewServiceMap() ServiceMap {
	return make(ServiceMap)
}

// encodeService encodes a ServiceMap into a proto bufeer ServiceMap message
func encodeService(s ServiceMap) (result *pb.ServiceMap, err error) {
	result = &pb.ServiceMap{}
	result.Map = make(map[string]*pb.NetworkAddress)

	for k, v := range s {
		result.Map[k] = &pb.NetworkAddress{
			Network: v.Network,
			Address: v.Address,
		}
	}

	return
}

// decodeService decodes a proto bufeer ServiceMap message (in) into a ServiceMap (out)
// out MUST NOT be nil
func decodeService(in *pb.ServiceMap, out ServiceMap) (err error) {
	for k, v := range in.GetMap() {
		sp := &NetworkAddress{
			Network: v.GetNetwork(),
			Address: v.GetAddress(),
		}
		out[k] = sp
	}
	return
}
