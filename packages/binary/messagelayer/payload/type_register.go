package payload

import (
	"sync"
)

// Unmarshaler takes some data and unmarshals it into a payload.
type Unmarshaler func(data []byte) (Payload, error)

// Definition defines the properties of a payload type.
type Definition struct {
	Name string
	Unmarshaler
}

var (
	typeRegister              = make(map[Type]Definition)
	typeRegisterMutex         sync.RWMutex
	genericUnmarshalerFactory func(payloadType Type) Unmarshaler
)

// RegisterType registers a payload type with the given unmarshaler.
func RegisterType(payloadType Type, payloadName string, unmarshaler Unmarshaler) {
	typeRegisterMutex.Lock()
	typeRegister[payloadType] = Definition{
		Name:        payloadName,
		Unmarshaler: unmarshaler,
	}
	typeRegisterMutex.Unlock()
}

// GetUnmarshaler returns the unmarshaler for the given type if known or
// the generic unmarshaler if the given payload type has no associated unmarshaler.
func GetUnmarshaler(payloadType Type) Unmarshaler {
	typeRegisterMutex.RLock()
	defer typeRegisterMutex.RUnlock()
	if definition, exists := typeRegister[payloadType]; exists {
		return definition.Unmarshaler
	}
	return genericUnmarshalerFactory(payloadType)
}

// SetGenericUnmarshalerFactory sets the generic unmarshaler.
func SetGenericUnmarshalerFactory(unmarshalerFactory func(payloadType Type) Unmarshaler) {
	genericUnmarshalerFactory = unmarshalerFactory
}

// Name returns the name of a given payload type.
func Name(payloadType Type) string {
	typeRegisterMutex.RLock()
	defer typeRegisterMutex.RUnlock()
	if definition, exists := typeRegister[payloadType]; exists {
		return definition.Name
	}
	return ObjectName
}
