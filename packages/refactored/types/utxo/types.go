package utxo

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/marshalutil"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/refactored/types"
)

// region TransactionID ////////////////////////////////////////////////////////////////////////////////////////////////

const TransactionIDLength = types.IdentifierLength

type TransactionID struct {
	types.Identifier
}

var EmptyTransactionID TransactionID

// NewFromMarshalUtil unmarshals an ID using a MarshalUtil (for easier unmarshalling).
func (t TransactionID) NewFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (txID TransactionID, err error) {
	idBytes, err := marshalUtil.ReadBytes(types.IdentifierLength)
	if err != nil {
		err = errors.Errorf("failed to parse ID (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	copy(txID.Identifier[:], idBytes)

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

type TransactionIDs = *set.AdvancedSet[TransactionID]

func NewTransactionIDs(ids ...TransactionID) (new TransactionIDs) {
	return set.NewAdvancedSet[TransactionID](ids...)
}

const OutputIDLength = types.IdentifierLength

type OutputID struct {
	types.Identifier
}

// NewFromMarshalUtil unmarshals an ID using a MarshalUtil (for easier unmarshalling).
func (t OutputID) NewFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (outputID OutputID, err error) {
	idBytes, err := marshalUtil.ReadBytes(types.IdentifierLength)
	if err != nil {
		err = errors.Errorf("failed to parse ID (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	copy(outputID.Identifier[:], idBytes)

	return
}

func NewOutputID(txID TransactionID, index uint16, metadata []byte) OutputID {
	return OutputID{blake2b.Sum256(marshalutil.New().Write(txID).WriteUint16(index).WriteBytes(metadata).Bytes())}
}

type OutputIDs = *set.AdvancedSet[OutputID]

func NewOutputIDs(ids ...OutputID) (new OutputIDs) {
	return set.NewAdvancedSet[OutputID](ids...)
}
