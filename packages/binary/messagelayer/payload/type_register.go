package payload

import (
	"sync"
)

// Unmarshaler takes some data and unmarshals it into a payload.
type Unmarshaler func(data []byte) (Payload, error)

var (
	typeRegister              = make(map[Type]Unmarshaler)
	typeRegisterMutex         sync.RWMutex
	genericUnmarshalerFactory func(payloadType Type) Unmarshaler
)

// RegisterType registers a payload type with the given unmarshaler.
func RegisterType(payloadType Type, unmarshaler Unmarshaler) {
	typeRegisterMutex.Lock()
	typeRegister[payloadType] = unmarshaler
	typeRegisterMutex.Unlock()
}

// GetUnmarshaler returns the unmarshaler for the given type if known or
// the generic unmarshaler if the given payload type has no associated unmarshaler.
func GetUnmarshaler(payloadType Type) Unmarshaler {
	typeRegisterMutex.RLock()
	defer typeRegisterMutex.RUnlock()
	if unmarshaler, exists := typeRegister[payloadType]; exists {
		return unmarshaler
	}
	return genericUnmarshalerFactory(payloadType)
}

// SetGenericUnmarshalerFactory sets the generic unmarshaler.
func SetGenericUnmarshalerFactory(unmarshalerFactory func(payloadType Type) Unmarshaler) {
	genericUnmarshalerFactory = unmarshalerFactory
}
