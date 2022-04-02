package utxo

import (
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

// Unmarshal unmarshals a TransactionID using a MarshalUtil (for easier unmarshalling).
func (t TransactionID) Unmarshal(marshalUtil *marshalutil.MarshalUtil) (txID TransactionID, err error) {
	err = txID.Identifier.FromMarshalUtil(marshalUtil)
	return
}

func (t TransactionID) String() (humanReadable string) {
	return "TransactionID(" + t.Alias() + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionIDs ///////////////////////////////////////////////////////////////////////////////////////////////

type TransactionIDs = *set.AdvancedSet[TransactionID]

func NewTransactionIDs(ids ...TransactionID) (new TransactionIDs) {
	return set.NewAdvancedSet[TransactionID](ids...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputID /////////////////////////////////////////////////////////////////////////////////////////////////////

const OutputIDLength = types.IdentifierLength

type OutputID struct {
	types.Identifier
}

func NewOutputID(txID TransactionID, index uint16, metadata []byte) OutputID {
	return OutputID{blake2b.Sum256(marshalutil.New().Write(txID).WriteUint16(index).WriteBytes(metadata).Bytes())}
}

// Unmarshal unmarshals an OutputID using a MarshalUtil (for easier unmarshalling).
func (t OutputID) Unmarshal(marshalUtil *marshalutil.MarshalUtil) (outputID OutputID, err error) {
	err = outputID.Identifier.FromMarshalUtil(marshalUtil)
	return
}

func (t OutputID) String() (humanReadable string) {
	return "OutputID(" + t.Alias() + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputIDs ////////////////////////////////////////////////////////////////////////////////////////////////////

type OutputIDs = *set.AdvancedSet[OutputID]

func NewOutputIDs(ids ...OutputID) (new OutputIDs) {
	return set.NewAdvancedSet[OutputID](ids...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
