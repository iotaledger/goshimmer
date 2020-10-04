package ledgerstate

import (
	"github.com/iotaledger/hive.go/marshalutil"
	"golang.org/x/xerrors"
)

// region TEMPORARY DEFINITIONS TO PREVENT ERRORS DUE TO UNMERGED MODELS ///////////////////////////////////////////////

// TransactionIDLength contains the amount of bytes that a marshaled version of the ID contains.
const TransactionIDLength = 32

// TransactionID is the type that represents the identifier of a Transaction.
type TransactionID [TransactionIDLength]byte

// TransactionIDFromBytes unmarshals a TransactionID from a sequence of bytes.
func TransactionIDFromBytes(bytes []byte) (result TransactionID, consumedBytes int, err error) {
	// parse the bytes
	marshalUtil := marshalutil.New(bytes)
	idBytes, idErr := marshalUtil.ReadBytes(TransactionIDLength)
	if idErr != nil {
		err = idErr

		return
	}
	copy(result[:], idBytes)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// TransactionIDFromMarshalUtil is a wrapper for simplified unmarshaling of TransactionIDs from a byte stream using the
// marshalUtil package.
func TransactionIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (TransactionID, error) {
	id, err := marshalUtil.Parse(func(data []byte) (interface{}, int, error) { return TransactionIDFromBytes(data) })
	if err != nil {
		return TransactionID{}, err
	}

	return id.(TransactionID), nil
}

// Bytes marshals the ID into a sequence of bytes.
func (i TransactionID) Bytes() []byte {
	return i[:]
}

// Transaction represents a value transfer.
type Transaction struct{}

// UnsignedBytes returns the unsigned bytes of the Transaction.
func (t *Transaction) UnsignedBytes() []byte {
	return nil
}

// BranchIDLength contains the amount of bytes that a marshaled version of the BranchID contains.
const BranchIDLength = 32

// BranchID is the data type that represents the identifier of a Branch.
type BranchID [BranchIDLength]byte

// BranchIDFromMarshalUtil unmarshals a BranchID using a MarshalUtil (for easier unmarshaling).
func BranchIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (branchID BranchID, err error) {
	branchIDBytes, err := marshalUtil.ReadBytes(BranchIDLength)
	if err != nil {
		err = xerrors.Errorf("failed to parse BranchID (%v): %w", err, ErrParseBytesFailed)
		return
	}
	copy(branchID[:], branchIDBytes)

	return
}

// Bytes returns a marshaled version of this BranchID.
func (b BranchID) Bytes() []byte {
	return b[:]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
