package peer

// NetworkAddress defines the tuple <Type, Port>, e.g, <TCP, 8000>
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
