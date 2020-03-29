package payload

import (
	"crypto/rand"
	"fmt"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/mr-tron/base58"
)

// Id represents the hash of a payload that is used to identify the given payload.
type Id [IdLength]byte

// NewId creates a payload id from a base58 encoded string.
func NewId(base58EncodedString string) (result Id, err error) {
	bytes, err := base58.Decode(base58EncodedString)
	if err != nil {
		return
	}

	if len(bytes) != IdLength {
		err = fmt.Errorf("length of base58 formatted payload id is wrong")

		return
	}

	copy(result[:], bytes)

	return
}

// ParseId is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func ParseId(marshalUtil *marshalutil.MarshalUtil) (Id, error) {
	if id, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return IdFromBytes(data) }); err != nil {
		return Id{}, err
	} else {
		return id.(Id), nil
	}
}

// IdFromBytes unmarshals a payload id from a sequence of bytes.
// It either creates a new payload id or fills the optionally provided object with the parsed information.
func IdFromBytes(bytes []byte, optionalTargetObject ...*Id) (result Id, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	var targetObject *Id
	switch len(optionalTargetObject) {
	case 0:
		targetObject = &result
	case 1:
		targetObject = optionalTargetObject[0]
	default:
		panic("too many arguments in call to IdFromBytes")
	}

	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	// read id from bytes
	idBytes, err := marshalUtil.ReadBytes(IdLength)
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

// Random creates a random id which can for example be used in unit tests.
func RandomId() (id Id) {
	// generate a random sequence of bytes
	idBytes := make([]byte, IdLength)
	if _, err := rand.Read(idBytes); err != nil {
		panic(err)
	}

	// copy the generated bytes into the result
	copy(id[:], idBytes)

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
var GenesisId Id

// IdLength defined the amount of bytes in a payload id (32 bytes hash value).
const IdLength = 32
