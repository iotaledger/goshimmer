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

// Unmarshal unmarshals an ID using a MarshalUtil (for easier unmarshalling).
func (t TransactionID) Unmarshal(marshalUtil *marshalutil.MarshalUtil) (txID TransactionID, err error) {
	idBytes, err := marshalUtil.ReadBytes(types.IdentifierLength)
	if err != nil {
		err = errors.Errorf("failed to parse ID (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	copy(txID.Identifier[:], idBytes)

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionIDs ///////////////////////////////////////////////////////////////////////////////////////////////

type TransactionIDs struct {
	*set.AdvancedSet[TransactionID]
}

func NewTransactionIDs(ids ...TransactionID) (new TransactionIDs) {
	return TransactionIDs{set.NewAdvancedSet[TransactionID](ids...)}
}

func (t TransactionIDs) AddAll(txIDs TransactionIDs) (added bool) {
	return t.AdvancedSet.AddAll(txIDs.AdvancedSet)
}

func (t TransactionIDs) DeleteAll(txIDs TransactionIDs) (removedTxIDs TransactionIDs) {
	return TransactionIDs{t.AdvancedSet.DeleteAll(txIDs.AdvancedSet)}
}

func (t TransactionIDs) Equal(txIDs TransactionIDs) (equal bool) {
	return t.AdvancedSet.Equal(txIDs.AdvancedSet)
}

func (t TransactionIDs) Intersect(other TransactionIDs) (intersection TransactionIDs) {
	return TransactionIDs{t.AdvancedSet.Intersect(other.AdvancedSet)}
}

func (t TransactionIDs) Filter(predicate func(element TransactionID) bool) (filtered TransactionIDs) {
	return TransactionIDs{t.AdvancedSet.Filter(predicate)}
}

func (t TransactionIDs) Clone() (txIDs TransactionIDs) {
	return TransactionIDs{t.AdvancedSet.Clone()}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputID /////////////////////////////////////////////////////////////////////////////////////////////////////

const OutputIDLength = types.IdentifierLength

type OutputID struct {
	types.Identifier
}

// Unmarshal unmarshals an ID using a MarshalUtil (for easier unmarshalling).
func (t OutputID) Unmarshal(marshalUtil *marshalutil.MarshalUtil) (outputID OutputID, err error) {
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionIDs ///////////////////////////////////////////////////////////////////////////////////////////////

type OutputIDs struct {
	*set.AdvancedSet[OutputID]
}

func NewOutputIDs(ids ...OutputID) (new OutputIDs) {
	return OutputIDs{set.NewAdvancedSet[OutputID](ids...)}
}

func (t OutputIDs) AddAll(txIDs OutputIDs) (added bool) {
	return t.AdvancedSet.AddAll(txIDs.AdvancedSet)
}

func (t OutputIDs) DeleteAll(txIDs OutputIDs) (removedTxIDs OutputIDs) {
	return OutputIDs{t.AdvancedSet.DeleteAll(txIDs.AdvancedSet)}
}

func (t OutputIDs) Equal(txIDs OutputIDs) (equal bool) {
	return t.AdvancedSet.Equal(txIDs.AdvancedSet)
}

func (t OutputIDs) Intersect(other OutputIDs) (intersection OutputIDs) {
	return OutputIDs{t.AdvancedSet.Intersect(other.AdvancedSet)}
}

func (t OutputIDs) Filter(predicate func(element OutputID) bool) (filtered OutputIDs) {
	return OutputIDs{t.AdvancedSet.Filter(predicate)}
}

func (t OutputIDs) Clone() (txIDs OutputIDs) {
	return OutputIDs{t.AdvancedSet.Clone()}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
