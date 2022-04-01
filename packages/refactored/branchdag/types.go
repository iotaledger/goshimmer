package branchdag

import (
	"github.com/iotaledger/goshimmer/packages/refactored/types"
	"github.com/iotaledger/goshimmer/packages/refactored/types/utxo"
)

type ConflictID = utxo.OutputID

type ConflictIDs = utxo.OutputIDs

type BranchID = utxo.TransactionID

type BranchIDs = utxo.TransactionIDs

var NewConflictIDs = utxo.NewOutputIDs

var NewBranchIDs = utxo.NewTransactionIDs

var MasterBranchID BranchID

var BranchIDLength = types.IdentifierLength

var ConflictIDLength = types.IdentifierLength
