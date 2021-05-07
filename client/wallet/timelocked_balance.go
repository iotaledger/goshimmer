package wallet

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"sort"
	"time"
)

type TimelockedBalance struct {
	Balance     map[ledgerstate.Color]uint64
	LockedUntil time.Time
}

type TimelockedBalanceSlice []*TimelockedBalance

func (t TimelockedBalanceSlice) Sort() {
	sort.Slice(t, func(i, j int) bool {
		return t[i].LockedUntil.Before(t[j].LockedUntil)
	})
}
