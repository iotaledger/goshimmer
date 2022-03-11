package utxo

import (
	"encoding/binary"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"
)

const OutputIDLength = 32

type OutputID [OutputIDLength]byte

// NewOutputID returns a new OutputID from the given details.
func NewOutputID(transactionID TransactionID, outputIndex uint16, outputMetadata []byte) (outputID OutputID) {
	serializedOutputIndex := make([]byte, 2)
	binary.LittleEndian.PutUint16(serializedOutputIndex, outputIndex)

	return blake2b.Sum256(byteutils.ConcatBytes(transactionID.Bytes(), serializedOutputIndex, outputMetadata))
}

// OutputIDFromBytes unmarshals an OutputID from a sequence of bytes.
func OutputIDFromBytes(bytes []byte) (outputID OutputID, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if outputID, err = OutputIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse OutputID from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// OutputIDFromBase58 creates an OutputID from a base58 encoded string.
func OutputIDFromBase58(base58String string) (outputID OutputID, err error) {
	decodedBytes, err := base58.Decode(base58String)
	if err != nil {
		err = errors.Errorf("error while decoding base58 encoded OutputID (%v): %w", err, cerrors.ErrBase58DecodeFailed)
		return
	}

	if outputID, _, err = OutputIDFromBytes(decodedBytes); err != nil {
		err = errors.Errorf("failed to parse OutputID from bytes: %w", err)
		return
	}

	return
}

// OutputIDFromMarshalUtil unmarshals an OutputID using a MarshalUtil (for easier unmarshaling).
func OutputIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (outputID OutputID, err error) {
	outputIDBytes, err := marshalUtil.ReadBytes(OutputIDLength)
	if err != nil {
		err = errors.Errorf("failed to parse OutputID (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	copy(outputID[:], outputIDBytes)

	return
}

// Bytes returns a marshaled version of the OutputID.
func (o OutputID) Bytes() (serializedTransaction []byte) {
	return o[:]
}

// String creates a human-readable version of the OutputID.
func (o OutputID) String() (humanReadableOutputID string) {
	return "OutputID(" + base58.Encode(o[:]) + ")"
}
