package branchmanager

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

type ConflictId = transaction.OutputId

var (
	ParseConflictId     = transaction.ParseOutputId
	ConflictIdFromBytes = transaction.OutputIdFromBytes
)

const ConflictIdLength = transaction.OutputIdLength
