package mana

import (
	"time"

	"github.com/iotaledger/hive.go/identity"
)

// TxInfo holds information related to the transaction which we are processing for mana calculation.
type TxInfo struct {
	// Timestamp is the timestamp of the transaction.
	TimeStamp time.Time
	// TotalBalance is the amount of funds being transferred via the transaction.
	TotalBalance float64
	// AccessPledgeID is the node to which this transaction pledges its access mana to.
	AccessPledgeID identity.ID
	// ConsensusPledgeID is the node to which this transaction pledges its consensus mana to.
	ConsensusPledgeID identity.ID
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

//  InputInfo holds mana related info about an input within a transaction.
type InputInfo struct {
	// Timestamp is the timestamp of the transaction that created this output (input).
	TimeStamp time.Time
	// Amount is the balance of the input.
	Amount float64
	// AccessPledgeID is the node to which the tx that created this output pledged access mana to.
	AccessPledgeID identity.ID
	// Consensus is the node to which the tx that created this output pledged consensus mana to.
	ConsensusPledgeID identity.ID
}
