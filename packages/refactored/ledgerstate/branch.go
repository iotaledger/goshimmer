package ledgerstate

import (
	"github.com/iotaledger/hive.go/marshalutil"
)

const BranchIDLength = 32

type BranchID [BranchIDLength]byte

func BranchIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (branchID BranchID, err error) {
	return
}

// Bytes returns a marshaled version of this BranchID.
func (b BranchID) Bytes() []byte {
	return b[:]
}
