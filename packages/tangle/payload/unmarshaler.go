package payload

import "fmt"

// UnmarshalerFunc defines the function signature for functions that can unmarshal Payloads.
type UnmarshalerFunc func(data []byte) (Payload, error)

// Unmarshaler returns the UnmarshalerFunc for the given Type or the GenericDataPayloadUnmarshaler if the Type is unknown.
func Unmarshaler(payloadType Type) UnmarshalerFunc {
	typeRegisterMutex.RLock()
	defer typeRegisterMutex.RUnlock()

	if definition, exists := typeRegister[payloadType]; exists {
		fmt.Println("unmarshaller", definition.Name, payloadType)
		return definition.UnmarshalerFunc
	}

	return GenericDataPayloadUnmarshaler
}
