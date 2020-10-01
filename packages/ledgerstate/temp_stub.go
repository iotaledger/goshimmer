package ledgerstate

import (
	"github.com/iotaledger/hive.go/marshalutil"
)

// region TEMPORARY DEFINITIONS TO PREVENT ERRORS DUE TO UNMERGED MODELS ///////////////////////////////////////////////

// TransactionIDLength contains the amount of bytes that a marshaled version of the ID contains.
const TransactionIDLength = 32

// TransactionID is the type that represents the identifier of a Transaction.
type TransactionID [TransactionIDLength]byte

// OutputIDLength contains the amount of bytes that a marshaled version of the OutputID contains.
const OutputIDLength = TransactionIDLength + marshalutil.UINT16_SIZE

// OutputID is the data type that represents the identifier of an Output.
type OutputID [OutputIDLength]byte

// BranchIDLength contains the amount of bytes that a marshaled version of the BranchID contains.
const BranchIDLength = 32

// BranchID is the data type that represents the identifier of a Branch.
type BranchID [BranchIDLength]byte

// Bytes returns a marshaled version of this BranchID.
func (b BranchID) Bytes() []byte {
	return b[:]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
