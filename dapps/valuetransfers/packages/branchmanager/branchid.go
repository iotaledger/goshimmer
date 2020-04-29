package branchmanager

import (
	"fmt"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/types"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

// BranchID represents an identifier of a Branch.
type BranchID [BranchIDLength]byte

var (
	// UndefinedBranchID is the zero value of a BranchID and represents a branch that has not been set.
	UndefinedBranchID = BranchID{}

	// MasterBranchID is the identifier of the MasterBranch (root of the Branch DAG).
	MasterBranchID = BranchID{1}
)

// NewBranchID creates a new BranchID from a transaction ID.
func NewBranchID(transactionID transaction.ID) (branchID BranchID) {
	copy(branchID[:], transactionID.Bytes())

	return
}

// BranchIDFromBytes unmarshals a BranchID from a sequence of bytes.
func BranchIDFromBytes(bytes []byte) (result BranchID, consumedBytes int, err error) {
	// parse the bytes
	marshalUtil := marshalutil.New(bytes)
	branchIDBytes, idErr := marshalUtil.ReadBytes(BranchIDLength)
	if idErr != nil {
		err = idErr

		return
	}
	copy(result[:], branchIDBytes)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// BranchIDFromBase58 creates a new BranchID from a base58 encoded string.
func BranchIDFromBase58(base58String string) (branchID BranchID, err error) {
	// decode string
	bytes, err := base58.Decode(base58String)
	if err != nil {
		return
	}

	// sanitize input
	if len(bytes) != BranchIDLength {
		err = fmt.Errorf("base58 encoded string does not match the length of a BranchID")

		return
	}

	// copy bytes to result
	copy(branchID[:], bytes)

	return
}

// ParseBranchID unmarshals a BranchID using the given marshalUtil (for easier marshaling/unmarshaling).
func ParseBranchID(marshalUtil *marshalutil.MarshalUtil) (result BranchID, err error) {
	var branchIDBytes []byte
	if branchIDBytes, err = marshalUtil.ReadBytes(BranchIDLength); err != nil {
		return
	}

	copy(result[:], branchIDBytes)

	return
}

// Bytes marshals the BranchID into a sequence of bytes.
func (branchId BranchID) Bytes() []byte {
	return branchId[:]
}

// String creates a base58 encoded version of the BranchID.
func (branchId BranchID) String() string {
	return base58.Encode(branchId[:])
}

// BranchIDLength encodes the length of a branch identifier - since branches get created by transactions, it has the
// same length as a transaction ID.
const BranchIDLength = transaction.IDLength

// BranchIds represents a collection of BranchIds.
type BranchIds map[BranchID]types.Empty

// ToList create a slice of BranchIDs from the collection.
func (branchIDs BranchIds) ToList() (result []BranchID) {
	result = make([]BranchID, len(branchIDs))
	i := 0
	for branchID := range branchIDs {
		result[i] = branchID
		i++
	}

	return
}
