package payload

import (
	"sync"
)

type Unmarshaler func(data []byte) (Payload, error)

var (
	typeRegister              = make(map[Type]Unmarshaler)
	typeRegisterMutex         sync.RWMutex
	genericUnmarshalerFactory func(payloadType Type) Unmarshaler
)

func RegisterType(payloadType Type, unmarshaler Unmarshaler) {
	typeRegisterMutex.Lock()
	typeRegister[payloadType] = unmarshaler
	typeRegisterMutex.Unlock()
}

func GetUnmarshaler(payloadType Type) Unmarshaler {
	typeRegisterMutex.RLock()
	if unmarshaler, exists := typeRegister[payloadType]; exists {
		typeRegisterMutex.RUnlock()

		return unmarshaler
	} else {
		typeRegisterMutex.RUnlock()

		return genericUnmarshalerFactory(payloadType)
	}
}

func SetGenericUnmarshalerFactory(unmarshalerFactory func(payloadType Type) Unmarshaler) {
	genericUnmarshalerFactory = unmarshalerFactory
}
