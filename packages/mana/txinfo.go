package mana

import (
	"time"

	"github.com/iotaledger/hive.go/identity"
)

type TxInfo struct {
	TimeStamp    time.Time
	TotalBalance float64
	InputInfo    []inputInfo
}

func (t *TxInfo) sumInputs() float64 {
	t.TotalBalance = 0
	for _, input := range t.InputInfo {
		t.TotalBalance += input.Amount
	}
	return t.TotalBalance
}

type inputInfo struct {
	TimeStamp         time.Time
	Amount            float64
	AccessPledgeID    identity.ID
	ConsensusPledgeID identity.ID
}
