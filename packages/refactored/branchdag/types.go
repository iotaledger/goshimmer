package branchdag

import (
	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

type ConflictID = utxo.OutputID

type ConflictIDs = utxo.OutputIDs

type BranchID = utxo.TransactionID

type BranchIDs = utxo.TransactionIDs

var NewConflictIDs = utxo.NewOutputIDs

var NewBranchIDs = utxo.NewTransactionIDs

var MasterBranchID = utxo.EmptyTransactionID

var BranchIDLength = utxo.TransactionIDLength

var ConflictIDLength = utxo.OutputIDLength
