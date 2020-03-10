package id

import (
	"fmt"

	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
)

// Id represents the hash of a payload that is used to identify the given payload.
type Id [Length]byte

// New creates a payload id from a base58 encoded string.
func New(base58EncodedString string) (result Id, err error) {
	bytes, err := base58.Decode(base58EncodedString)
	if err != nil {
		return
	}

	if len(bytes) != Length {
		err = fmt.Errorf("length of base58 formatted payload id is wrong")

		return
	}

	copy(result[:], bytes)

	return
}

// Parse is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func Parse(marshalUtil *marshalutil.MarshalUtil) (Id, error) {
	if id, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return FromBytes(data) }); err != nil {
		return Id{}, err
	} else {
		return id.(Id), nil
	}
}

// FromBytes unmarshals a payload id from a sequence of bytes.
// It either creates a new payload id or fills the optionally provided object with the parsed information.
func FromBytes(bytes []byte, optionalTargetObject ...*Id) (result Id, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	var targetObject *Id
	switch len(optionalTargetObject) {
	case 0:
		targetObject = &result
	case 1:
		targetObject = optionalTargetObject[0]
	default:
		panic("too many arguments in call to FromBytes")
	}

	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	// read id from bytes
	idBytes, err := marshalUtil.ReadBytes(Length)
	if err != nil {
		return
	}
	copy(targetObject[:], idBytes)

	// copy result if we have provided a target object
	result = *targetObject

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// String returns a base58 encoded version of the payload id.
func (id Id) String() string {
	return base58.Encode(id[:])
}

func (id Id) Bytes() []byte {
	return id[:]
}

// Empty represents the id encoding the genesis.
var Genesis Id

// Length defined the amount of bytes in a payload id (32 bytes hash value).
const Length = 32
