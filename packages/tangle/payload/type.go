package payload

import (
	"encoding/binary"
	"strconv"
	"sync"
)

// Type represents the Type of a payload.
type Type uint32

var (
	// typeRegister contains a map of all Types that where registered by the node.
	typeRegister = make(map[Type]string)

	// typeRegisterMutex is used to make synchronize the access to the previously defined map.
	typeRegisterMutex sync.RWMutex
)

// NewType creates and registers a new payload Type.
func NewType(typeNumber uint32, typeName string) (payloadType Type) {
	payloadType = Type(typeNumber)

	typeRegisterMutex.Lock()
	defer typeRegisterMutex.Unlock()

	if registeredTypeName, typeRegisteredAlready := typeRegister[payloadType]; typeRegisteredAlready {
		panic("payload type " +
			typeName + "(" + strconv.FormatUint(uint64(typeNumber), 10) + ")" +
			" tries to overwrite previously created type " +
			registeredTypeName + "(" + strconv.FormatUint(uint64(typeNumber), 10) + ")")
	}

	typeRegister[payloadType] = typeName

	return
}

// Bytes returns a marshaled version of the Type.
func (t Type) Bytes() (bytes []byte) {
	bytes = make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, uint32(t))

	return
}

// String returns a human-readable version of the Type for debug purposes.
func (t Type) String() string {
	typeRegisterMutex.RLock()
	defer typeRegisterMutex.RUnlock()

	if typeName, exists := typeRegister[t]; exists {
		return typeName + "(" + strconv.FormatUint(uint64(t), 10) + ")"
	}

	return "UnknownType(" + strconv.FormatUint(uint64(t), 10) + ")"
}
