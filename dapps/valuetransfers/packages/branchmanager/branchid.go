package branchmanager

import (
	"fmt"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/types"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

// BranchID represents an identifier of a Branch.
type BranchID [BranchIdLength]byte

var (
	// UndefinedBranchID is the zero value of a BranchID and represents a branch that has not been set.
	UndefinedBranchID = BranchID{}

	// MasterBranchID is the identifier of the MasterBranch (root of the Branch DAG).
	MasterBranchID = BranchID{1}
)

// NewBranchId creates a new BranchID from a transaction ID.
func NewBranchId(transactionId transaction.Id) (branchId BranchID) {
	copy(branchId[:], transactionId.Bytes())

	return
}

func BranchIdFromBytes(bytes []byte) (result BranchID, err error, consumedBytes int) {
	// parse the bytes
	marshalUtil := marshalutil.New(bytes)
	branchIdBytes, idErr := marshalUtil.ReadBytes(BranchIdLength)
	if idErr != nil {
		err = idErr

		return
	}
	copy(result[:], branchIdBytes)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func BranchIdFromBase58(base58String string) (branchId BranchID, err error) {
	// decode string
	bytes, err := base58.Decode(base58String)
	if err != nil {
		return
	}

	// sanitize input
	if len(bytes) != BranchIdLength {
		err = fmt.Errorf("base58 encoded string does not match the length of a BranchID")

		return
	}

	// copy bytes to result
	copy(branchId[:], bytes)

	return
}

func ParseBranchId(marshalUtil *marshalutil.MarshalUtil) (result BranchID, err error) {
	var branchIdBytes []byte
	if branchIdBytes, err = marshalUtil.ReadBytes(BranchIdLength); err != nil {
		return
	}

	copy(result[:], branchIdBytes)

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

// BranchIdLength encodes the length of a branch identifier - since branches get created by transactions, it has the
// same length as a transaction ID.
const BranchIdLength = transaction.IdLength

type BranchIds map[BranchID]types.Empty

func (branchIds BranchIds) ToList() (result []BranchID) {
	result = make([]BranchID, len(branchIds))
	i := 0
	for branchId := range branchIds {
		result[i] = branchId
		i++
	}

	return
}
