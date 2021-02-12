package payload

import (
	"encoding/binary"
	"strconv"
	"sync"

	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"golang.org/x/xerrors"
)

// TypeLength contains the amount of bytes of a marshaled Type.
const TypeLength = marshalutil.Uint32Size

// Type represents the Type of a payload.
type Type uint32

// typeMetadata holds additional information for registered Types.
type typeMetadata struct {
	Name string
	UnmarshalerFunc
}

var (
	// typeRegister contains a map of all Types that where registered by the node.
	typeRegister = make(map[Type]typeMetadata)

	// typeRegisterMutex is used to make synchronize the access to the previously defined map.
	typeRegisterMutex sync.RWMutex
)

// NewType creates and registers a new payload Type.
func NewType(typeNumber uint32, typeName string, typeUnmarshaler UnmarshalerFunc) (payloadType Type) {
	payloadType = Type(typeNumber)

	typeRegisterMutex.Lock()
	defer typeRegisterMutex.Unlock()

	if registeredType, typeRegisteredAlready := typeRegister[payloadType]; typeRegisteredAlready {
		panic("payload type " +
			typeName + "(" + strconv.FormatUint(uint64(typeNumber), 10) + ")" +
			" tries to overwrite previously created type " +
			registeredType.Name + "(" + strconv.FormatUint(uint64(typeNumber), 10) + ")")
	}

	typeRegister[payloadType] = typeMetadata{
		Name:            typeName,
		UnmarshalerFunc: typeUnmarshaler,
	}

	return
}

// TypeFromBytes unmarshals a Type from a sequence of bytes.
func TypeFromBytes(typeBytes []byte) (typeResult Type, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(typeBytes)
	if typeResult, err = TypeFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Type from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// TypeFromMarshalUtil unmarshals a Type using a MarshalUtil (for easier unmarshaling).
func TypeFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (typeResult Type, err error) {
	typeUint32, err := marshalUtil.ReadUint32()
	if err != nil {
		err = xerrors.Errorf("failed to parse type (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	typeResult = Type(typeUint32)

	return
}

// Bytes returns a marshaled version of the Type.
func (t Type) Bytes() (bytes []byte) {
	bytes = make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, uint32(t))

	return
}

// String returns a human readable version of the Type for debug purposes.
func (t Type) String() string {
	typeRegisterMutex.RLock()
	defer typeRegisterMutex.RUnlock()

	if definition, exists := typeRegister[t]; exists {
		return definition.Name + "(" + strconv.FormatUint(uint64(t), 10) + ")"
	}

	return "UnknownType(" + strconv.FormatUint(uint64(t), 10) + ")"
}
