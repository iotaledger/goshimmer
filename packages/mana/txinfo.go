package mana

import (
	"time"

	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// TxInfo holds information related to the transaction which we are processing for mana calculation.
type TxInfo struct {
	// Timestamp is the timestamp of the transaction.
	TimeStamp time.Time
	// TransactionID is the ID of the transaction.
	TransactionID ledgerstate.TransactionID
	// TotalBalance is the amount of funds being transferred via the transaction.
	TotalBalance float64
	// PledgeID is a map of mana types and the node to which this transaction pledges its mana type to.
	PledgeID map[Type]identity.ID
	// InputInfos is a slice of InputInfo that holds mana related info about each input within the transaction.
	InputInfos []InputInfo
}

func (t *TxInfo) sumInputs() float64 {
	t.TotalBalance = 0
	for _, input := range t.InputInfos {
		t.TotalBalance += input.Amount
	}
	return t.TotalBalance
}

// InputInfo holds mana related info about an input within a transaction.
type InputInfo struct {
	// Timestamp is the timestamp of the transaction that created this output (input).
	TimeStamp time.Time
	// Amount is the balance of the input.
	Amount float64
	// PledgeID is a map of mana types and the node to which the transaction that created the output pledges its mana type to.
	PledgeID map[Type]identity.ID
	// InputID is the input consumed.
	InputID ledgerstate.OutputID
}

// SnapshotInfo defines the info for the mana snapshot.
type SnapshotInfo struct {
	Value float64
	TxID  ledgerstate.TransactionID
}
