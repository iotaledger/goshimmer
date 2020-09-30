package ledgerstate

import (
	"strings"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"github.com/mr-tron/base58"
	"golang.org/x/xerrors"
)

// region BranchID /////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	// UndefinedBranchID is the zero value of a BranchID and represents a branch that has not been set.
	UndefinedBranchID = BranchID{}

	// MasterBranchID is the identifier of the MasterBranch (root of the Branch DAG).
	MasterBranchID = BranchID{1}
)

// BranchIDLength contains the amount of bytes that a marshaled version of the BranchID contains.
const BranchIDLength = 32

// BranchID is the data type that represents the identifier of a Branch.
type BranchID [BranchIDLength]byte

// NewBranchID creates a new BranchID from a TransactionID.
func NewBranchID(transactionID TransactionID) (branchID BranchID) {
	copy(branchID[:], transactionID[:])

	return
}

// BranchIDFromBytes unmarshals a BranchID from a sequence of bytes.
func BranchIDFromBytes(bytes []byte) (branchID BranchID, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if branchID, err = BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse BranchID from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// BranchIDFromBase58 creates a BranchID from a base58 encoded string.
func BranchIDFromBase58(base58String string) (branchID BranchID, err error) {
	bytes, err := base58.Decode(base58String)
	if err != nil {
		err = xerrors.Errorf("error while decoding base58 encoded BranchID (%v): %w", err, ErrBase58DecodeFailed)
		return
	}

	if branchID, _, err = BranchIDFromBytes(bytes); err != nil {
		err = xerrors.Errorf("failed to parse BranchID from bytes: %w", err)
		return
	}

	return
}

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

// Base58 returns a base58 encoded version of the BranchID.
func (b BranchID) Base58() string {
	return base58.Encode(b.Bytes())
}

// String returns a human readable version of the BranchID.
func (b BranchID) String() string {
	switch b {
	case UndefinedBranchID:
		return "BranchID(UndefinedBranchID)"
	case MasterBranchID:
		return "BranchID(MasterBranchID)"
	default:
		return "BranchID(" + b.Base58() + ")"
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchIDs ////////////////////////////////////////////////////////////////////////////////////////////////////

// BranchIDs represents a collection of BranchIDs.
type BranchIDs map[BranchID]types.Empty

// NewBranchIDs creates a new collection of BranchIDs from the given BranchIDs.
func NewBranchIDs(branches ...BranchID) (branchIDs BranchIDs) {
	branchIDs = make(BranchIDs)
	for _, branchID := range branches {
		branchIDs[branchID] = types.Void
	}

	return
}

// BranchIDsFromBytes unmarshals a collection of BranchIDs from a sequence of bytes.
func BranchIDsFromBytes(bytes []byte) (branchIDs BranchIDs, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if branchIDs, err = BranchIDsFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse BranchIDs from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// BranchIDsFromMarshalUtil unmarshals a collection of BranchIDs using a MarshalUtil (for easier unmarshaling).
func BranchIDsFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (branchIDs BranchIDs, err error) {
	branchIDsCount, err := marshalUtil.ReadUint64()
	if err != nil {
		err = xerrors.Errorf("failed to parse BranchIDs count (%v): %w", err, ErrParseBytesFailed)
		return
	}

	branchIDs = make(BranchIDs)
	for i := uint64(0); i < branchIDsCount; i++ {
		branchID, branchIDErr := BranchIDFromMarshalUtil(marshalUtil)
		if branchIDErr != nil {
			err = xerrors.Errorf("failed to parse BranchID: %w", branchIDErr)
			return
		}

		branchIDs[branchID] = types.Void
	}

	return
}

// Slice creates a slice of BranchIDs from the collection.
func (branchIDs BranchIDs) Slice() (list []BranchID) {
	list = make([]BranchID, len(branchIDs))
	i := 0
	for branchID := range branchIDs {
		list[i] = branchID
		i++
	}

	return
}

// Bytes returns a marshaled version of the BranchIDs.
func (branchIDs BranchIDs) Bytes() []byte {
	marshalUtil := marshalutil.New(marshalutil.INT64_SIZE + len(branchIDs)*BranchIDLength)
	marshalUtil.WriteUint64(uint64(len(branchIDs)))
	for branchID := range branchIDs {
		marshalUtil.WriteBytes(branchID.Bytes())
	}

	return marshalUtil.Bytes()
}

// String returns a human readable version of the BranchIDs.
func (branchIDs BranchIDs) String() string {
	if len(branchIDs) == 0 {
		return "BranchIDs{}"
	}

	result := "BranchIDs{\n"
	for branchID := range branchIDs {
		result += strings.Repeat(" ", stringify.INDENTATION_SIZE) + branchID.String() + ",\n"
	}
	result += "}"

	return result
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Branch ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Branch represents a container for Transactions and Outputs representing a certain perception of the ledger state.
type Branch struct {
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
