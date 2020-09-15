package tangle

import (
	"sync"
)

// PayloadType represents the type id of a payload.
type PayloadType = uint32

// Unmarshaler takes some data and unmarshals it into a payload.
type Unmarshaler func(data []byte) (Payload, error)

// Definition defines the properties of a payload type.
type Definition struct {
	Name string
	Unmarshaler
}

var (
	typeRegister       = make(map[PayloadType]Definition)
	typeRegisterMutex  sync.RWMutex
	genericUnmarshaler Unmarshaler
)

func init() {
	// register the generic unmarshaler
	SetGenericUnmarshaler(GenericPayloadUnmarshaler)
}

// RegisterPayloadType registers a payload type with the given unmarshaler.
func RegisterPayloadType(payloadType PayloadType, payloadName string, unmarshaler Unmarshaler) {
	typeRegisterMutex.Lock()
	typeRegister[payloadType] = Definition{
		Name:        payloadName,
		Unmarshaler: unmarshaler,
	}
	typeRegisterMutex.Unlock()
}

// GetUnmarshaler returns the unmarshaler for the given type if known or
// the generic unmarshaler if the given payload type has no associated unmarshaler.
func GetUnmarshaler(payloadType PayloadType) Unmarshaler {
	typeRegisterMutex.RLock()
	defer typeRegisterMutex.RUnlock()

	if definition, exists := typeRegister[payloadType]; exists {
		return definition.Unmarshaler
	}

	return genericUnmarshaler
}

// SetGenericUnmarshaler sets the generic unmarshaler.
func SetGenericUnmarshaler(unmarshaler Unmarshaler) {
	genericUnmarshaler = unmarshaler
}

// Name returns the name of a given payload type.
func Name(payloadType PayloadType) string {
	typeRegisterMutex.RLock()
	defer typeRegisterMutex.RUnlock()
	if definition, exists := typeRegister[payloadType]; exists {
		return definition.Name
	}
	return ObjectName
}
