package utxo

import (
	"crypto/rand"
	"encoding/binary"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"
)

// OutputID is the type that represents the identifier of an Output.
type OutputID [OutputIDLength]byte

// NewOutputID creates a new OutputID from the given execution results.
func NewOutputID(transactionID TransactionID, outputIndex uint16, outputMetadata []byte) (new OutputID) {
	serializedOutputIndex := make([]byte, 2)
	binary.LittleEndian.PutUint16(serializedOutputIndex, outputIndex)

	return blake2b.Sum256(byteutils.ConcatBytes(transactionID.Bytes(), serializedOutputIndex, outputMetadata))
}

// FromRandomness fills the OutputID with random information.
func (o *OutputID) FromRandomness() (err error) {
	_, err = rand.Read((*o)[:])

	return
}

// FromBytes unmarshals an OutputID from a sequence of bytes.
func (o *OutputID) FromBytes(bytes []byte) (consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if err = o.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse OutputID from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// FromBase58 creates an OutputID from a base58 encoded string.
func (o *OutputID) FromBase58(base58String string) (err error) {
	decodedBytes, err := base58.Decode(base58String)
	if err != nil {
		err = errors.Errorf("error while decoding base58 encoded OutputID (%v): %w", err, cerrors.ErrBase58DecodeFailed)
		return
	}

	if _, err = o.FromBytes(decodedBytes); err != nil {
		err = errors.Errorf("failed to parse OutputID from bytes: %w", err)
		return
	}

	return
}

// FromMarshalUtil unmarshals an OutputID using a MarshalUtil (for easier unmarshalling).
func (o *OutputID) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	outputIDBytes, err := marshalUtil.ReadBytes(OutputIDLength)
	if err != nil {
		err = errors.Errorf("failed to parse OutputID (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	copy((*o)[:], outputIDBytes)

	return
}

func (o OutputID) RegisterAlias(alias string) {
	_outputIDAliasesMutex.Lock()
	defer _outputIDAliasesMutex.Unlock()

	_outputIDAliases[o] = alias
}

func (o OutputID) UnregisterAlias() {
	_outputIDAliasesMutex.Lock()
	defer _outputIDAliasesMutex.Unlock()

	delete(_outputIDAliases, o)
}

// Bytes returns a marshaled version of the OutputID.
func (o OutputID) Bytes() (serialized []byte) {
	return o[:]
}

// Base58 returns a base58 encoded version of the OutputID.
func (o OutputID) Base58() string {
	return base58.Encode(o[:])
}

// String creates a human-readable version of the OutputID.
func (o OutputID) String() (humanReadable string) {
	_outputIDAliasesMutex.RLock()
	defer _outputIDAliasesMutex.RUnlock()

	if alias, exists := _outputIDAliases[o]; exists {
		return alias
	}

	return "OutputID(" + base58.Encode(o[:]) + ")"
}

// OutputIDLength contains the byte size of an OutputID.
const OutputIDLength = 32

var (
	_outputIDAliases      = make(map[OutputID]string)
	_outputIDAliasesMutex = sync.RWMutex{}
)
