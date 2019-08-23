package peer

import (
	pb "github.com/wollac/autopeering/peer/proto"
)

// ServiceType defines the service type (e.g., TCP, UDP)
type ServiceType = int32

const (
	TCP ServiceType = iota
	UDP
)

// TypePort defines the tuple <Type, Port>, e.g, <TCP, 8000>
type TypePort struct {
	Type ServiceType
	Port uint16
}

// ServiceMap defines the mapping between a service ID and its tuple TypePort
// e.g., map[autopeering:&{TCP, 8000}]
type ServiceMap = map[string]*TypePort

// NewServiceMap initializes and returns an empty new ServiceMap
func NewServiceMap() ServiceMap {
	return make(ServiceMap)
}

// encodeService encodes a ServiceMap into a proto bufeer ServiceMap message
func encodeService(s ServiceMap) (result *pb.ServiceMap, err error) {
	result = &pb.ServiceMap{}
	result.Map = make(map[string]*pb.TypePort)

	for k, v := range s {
		result.Map[k] = &pb.TypePort{
			Type: pb.TypePort_Type(v.Type),
			Port: int32(v.Port),
		}
	}

	return
}

// decodeService decodes a proto bufeer ServiceMap message (in) into a ServiceMap (out)
// out MUST NOT be nil
func decodeService(in *pb.ServiceMap, out ServiceMap) (err error) {
	for k, v := range in.GetMap() {
		sp := &TypePort{
			Type: int32(v.GetType()),
			Port: uint16(v.GetPort()),
		}
		out[k] = sp
	}
	return
}
