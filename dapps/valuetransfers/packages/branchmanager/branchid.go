package branchmanager

import (
	"fmt"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/types"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

type BranchId [BranchIdLength]byte

var (
	UndefinedBranchId = BranchId{}
	MasterBranchId    = BranchId{1}
)

// NewBranchId creates a new BranchId from a transaction Id.
func NewBranchId(transactionId transaction.Id) (branchId BranchId) {
	copy(branchId[:], transactionId.Bytes())

	return
}

func BranchIdFromBytes(bytes []byte) (result BranchId, err error, consumedBytes int) {
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

func BranchIdFromBase58(base58String string) (branchId BranchId, err error) {
	// decode string
	bytes, err := base58.Decode(base58String)
	if err != nil {
		return
	}

	// sanitize input
	if len(bytes) != BranchIdLength {
		err = fmt.Errorf("base58 encoded string does not match the length of a BranchId")

		return
	}

	// copy bytes to result
	copy(branchId[:], bytes)

	return
}

func ParseBranchId(marshalUtil *marshalutil.MarshalUtil) (result BranchId, err error) {
	var branchIdBytes []byte
	if branchIdBytes, err = marshalUtil.ReadBytes(BranchIdLength); err != nil {
		return
	}

	copy(result[:], branchIdBytes)

	return
}

// Bytes marshals the BranchId into a sequence of bytes.
func (branchId BranchId) Bytes() []byte {
	return branchId[:]
}

// String creates a base58 encoded version of the BranchId.
func (branchId BranchId) String() string {
	return base58.Encode(branchId[:])
}

// BranchIdLength encodes the length of a branch identifier - since branches get created by transactions, it has the
// same length as a transaction Id.
const BranchIdLength = transaction.IdLength

type BranchIds map[BranchId]types.Empty

func (branchIds BranchIds) ToList() (result []BranchId) {
	result = make([]BranchId, len(branchIds))
	i := 0
	for branchId := range branchIds {
		result[i] = branchId
		i++
	}

	return
}
