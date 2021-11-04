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

// SnapshotNode defines the record for the mana snapshot of one node.
type SnapshotNode struct {
	AccessMana       AccessManaSnapshot
	SortedTxSnapshot SortedTxSnapshot
}

// AccessManaSnapshot defines the record for the aMana snapshot of one node.
type AccessManaSnapshot struct {
	Value     float64
	Timestamp time.Time
}

// TxSnapshot defines the record of one transaction.
type TxSnapshot struct {
	Value     float64
	TxID      ledgerstate.TransactionID
	Timestamp time.Time
}

// SortedTxSnapshot defines a list of SnapshotInfo sorted by timestamp.
type SortedTxSnapshot []*TxSnapshot

func (s SortedTxSnapshot) Len() int           { return len(s) }
func (s SortedTxSnapshot) Less(i, j int) bool { return s[i].Timestamp.Before(s[j].Timestamp) }
func (s SortedTxSnapshot) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
