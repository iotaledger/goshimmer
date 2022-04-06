package utxo

import (
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/types"
	"golang.org/x/crypto/blake2b"
)

// region TransactionID ////////////////////////////////////////////////////////////////////////////////////////////////

// TransactionID is a unique identifier for a Transaction.
type TransactionID struct {
	types.Identifier
}

// NewTransactionID returns a new TransactionID for the given data.
func NewTransactionID(txData []byte) (new TransactionID) {
	return TransactionID{
		types.NewIdentifier(txData),
	}
}

// Unmarshal un-serializes a TransactionID using a MarshalUtil.
func (t TransactionID) Unmarshal(marshalUtil *marshalutil.MarshalUtil) (txID TransactionID, err error) {
	err = txID.Identifier.FromMarshalUtil(marshalUtil)
	return
}

// String returns a human-readable version of the TransactionID.
func (t TransactionID) String() (humanReadable string) {
	return "TransactionID(" + t.Alias() + ")"
}

// EmptyTransactionID contains the null-value of the TransactionID type.
var EmptyTransactionID TransactionID

// TransactionIDLength contains the byte length of a serialized TransactionID.
const TransactionIDLength = types.IdentifierLength

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionIDs ///////////////////////////////////////////////////////////////////////////////////////////////

// TransactionIDs represents a collection of TransactionIDs.
type TransactionIDs = *set.AdvancedSet[TransactionID]

// NewTransactionIDs returns a new TransactionID collection with the given elements.
func NewTransactionIDs(ids ...TransactionID) (new TransactionIDs) {
	return set.NewAdvancedSet[TransactionID](ids...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputID /////////////////////////////////////////////////////////////////////////////////////////////////////

// OutputID is a unique identifier for an Output.
type OutputID struct {
	types.Identifier
}

// NewOutputID returns a new OutputID for the given details.
func NewOutputID(txID TransactionID, index uint16, metadata []byte) OutputID {
	return OutputID{
		blake2b.Sum256(marshalutil.New().Write(txID).WriteUint16(index).WriteBytes(metadata).Bytes()),
	}
}

// Unmarshal un-serializes a OutputID using a MarshalUtil.
func (t OutputID) Unmarshal(marshalUtil *marshalutil.MarshalUtil) (outputID OutputID, err error) {
	err = outputID.Identifier.FromMarshalUtil(marshalUtil)
	return
}

// String returns a human-readable version of the OutputID.
func (t OutputID) String() (humanReadable string) {
	return "OutputID(" + t.Alias() + ")"
}

// EmptyOutputID contains the null-value of the OutputID type.
var EmptyOutputID OutputID

// OutputIDLength contains the byte length of a serialized OutputID.
const OutputIDLength = types.IdentifierLength

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputIDs ////////////////////////////////////////////////////////////////////////////////////////////////////

// OutputIDs represents a collection of OutputIDs.
type OutputIDs = *set.AdvancedSet[OutputID]

// NewOutputIDs returns a new OutputID collection with the given elements.
func NewOutputIDs(ids ...OutputID) (new OutputIDs) {
	return set.NewAdvancedSet[OutputID](ids...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
