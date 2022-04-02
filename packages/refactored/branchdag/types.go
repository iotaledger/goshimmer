package branchdag

import (
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/marshalutil"

	"github.com/iotaledger/goshimmer/packages/refactored/types"
	"github.com/iotaledger/goshimmer/packages/refactored/types/utxo"
)

// region BranchID /////////////////////////////////////////////////////////////////////////////////////////////////////

const BranchIDLength = types.IdentifierLength

type BranchID struct {
	types.Identifier
}

func NewBranchID(txID utxo.TransactionID) (new BranchID) {
	return BranchID{txID.Identifier}
}

// Unmarshal unmarshals a BranchID using a MarshalUtil (for easier unmarshalling).
func (t BranchID) Unmarshal(marshalUtil *marshalutil.MarshalUtil) (txID BranchID, err error) {
	err = txID.Identifier.FromMarshalUtil(marshalUtil)
	return
}

func (t BranchID) String() (humanReadable string) {
	return "BranchID(" + t.Alias() + ")"
}

var MasterBranchID BranchID

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchIDs ////////////////////////////////////////////////////////////////////////////////////////////////////

type BranchIDs = *set.AdvancedSet[BranchID]

func NewBranchIDs(ids ...BranchID) (new BranchIDs) {
	return set.NewAdvancedSet[BranchID](ids...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

type ConflictID = utxo.OutputID

type ConflictIDs = utxo.OutputIDs

var NewConflictIDs = utxo.NewOutputIDs

var ConflictIDLength = types.IdentifierLength
