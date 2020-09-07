package mana

import (
	"time"

	"github.com/iotaledger/hive.go/identity"
)

type TxInfo struct {
	TimeStamp    time.Time
	TotalBalance float64
	InputInfo    []InputInfo
}

func (t *TxInfo) sumInputs() float64 {
	t.TotalBalance = 0
	for _, input := range t.InputInfo {
		t.TotalBalance += input.Amount
	}
	return t.TotalBalance
}

type InputInfo struct {
	TimeStamp         time.Time
	Amount            float64
	AccessPledgeID    identity.ID
	ConsensusPledgeID identity.ID
	AccessRevokeID    identity.ID
	ConsensusRevokeID identity.ID
}
