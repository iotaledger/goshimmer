package signatures

import (
	"github.com/iotaledger/goshimmer/packages/binary/datastructure/orderedmap"
	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
)

// Signatures represents a container for the address signatures of a value transfer.
// It internally manages the list of signatures as an ordered map, so that the serialization order is deterministic and
// produces the same sequence of bytes during marshaling and unmarshaling.
type Signatures struct {
	orderedMap *orderedmap.OrderedMap
}

// New creates an empty container for the address signatures of a value transfer.
func New() *Signatures {
	return &Signatures{
		orderedMap: orderedmap.New(),
	}
}

// FromBytes unmarshals a container with signatures from a sequence of bytes.
// It either creates a new container or fills the optionally provided container with the parsed information.
func FromBytes(bytes []byte, optionalTargetObject ...*Signatures) (result *Signatures, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Signatures{orderedMap: orderedmap.New()}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to FromBytes")
	}

	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	// read version
	versionByte, err := marshalUtil.ReadByte()
	if err != nil {
		return
	}

	// 0 byte encodes the end of the signatures
	for versionByte != 0 {
		// perform signature scheme specific decoding
		switch versionByte {
		case VERSION_ED25519:
			marshalUtil.ReadSeek(-1)
			signature, signatureErr := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return ed25519SignatureFromBytes(data) })
			if signatureErr != nil {
				err = signatureErr

				return
			}
			typeCastedSignature := signature.(Signature)

			result.orderedMap.Set(typeCastedSignature.Address(), typeCastedSignature)
		}

		// read version of next signature
		if versionByte, err = marshalUtil.ReadByte(); err != nil {
			return
		}
	}

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (signatures *Signatures) Add(address address.Address, signature Signature) {
	signatures.orderedMap.Set(address, signature)
}

func (signatures *Signatures) Get(address address.Address) (Signature, bool) {
	signature, exists := signatures.orderedMap.Get(address)
	if !exists {
		return nil, false
	}

	return signature.(Signature), exists
}

// Size returns the amount of signatures in this container.
func (signatures *Signatures) Size() int {
	return signatures.orderedMap.Size()
}

// ForEach iterates through all signatures, calling the consumer for every found entry.
// The iteration can be aborted by the consumer returning false
func (signatures *Signatures) ForEach(consumer func(address address.Address, signature Signature) bool) {
	signatures.orderedMap.ForEach(func(key, value interface{}) bool {
		return consumer(key.(address.Address), value.(Signature))
	})
}

// Bytes marshals the signatures into a sequence of bytes.
func (signatures *Signatures) Bytes() []byte {
	// initialize helper
	marshalUtil := marshalutil.New()

	// iterate through signatures and dump them
	signatures.ForEach(func(address address.Address, signature Signature) bool {
		marshalUtil.WriteBytes(signature.Bytes())

		return true
	})

	// trailing 0 to indicate the end of signatures
	marshalUtil.WriteByte(0)

	// return result
	return marshalUtil.Bytes()
}
