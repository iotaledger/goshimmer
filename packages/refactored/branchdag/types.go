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
func (t BranchID) Unmarshal(marshalUtil *marshalutil.MarshalUtil) (branchID BranchID, err error) {
	err = branchID.Identifier.FromMarshalUtil(marshalUtil)
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

// region ConflictID ///////////////////////////////////////////////////////////////////////////////////////////////////

const ConflictIDLength = types.IdentifierLength

type ConflictID struct {
	types.Identifier
}

func NewConflictID(outputID utxo.OutputID) (new ConflictID) {
	return ConflictID{outputID.Identifier}
}

// Unmarshal unmarshals a ConflictID using a MarshalUtil (for easier unmarshalling).
func (t ConflictID) Unmarshal(marshalUtil *marshalutil.MarshalUtil) (conflictID ConflictID, err error) {
	err = conflictID.Identifier.FromMarshalUtil(marshalUtil)
	return
}

func (t ConflictID) String() (humanReadable string) {
	return "ConflictID(" + t.Alias() + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConflictIDs //////////////////////////////////////////////////////////////////////////////////////////////////

type ConflictIDs = *set.AdvancedSet[ConflictID]

func NewConflictIDs(ids ...ConflictID) (new ConflictIDs) {
	return set.NewAdvancedSet[ConflictID](ids...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
