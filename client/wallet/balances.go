package wallet

import (
	"sort"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// TimedBalance represents a balance that is time dependent.
type TimedBalance struct {
	Balance map[ledgerstate.Color]uint64
	Time    time.Time
}

// TimedBalanceSlice is a slice containing TimedBalances.
type TimedBalanceSlice []*TimedBalance

// Sort sorts the balances based on their Time.
func (t TimedBalanceSlice) Sort() {
	sort.Slice(t, func(i, j int) bool {
		return t[i].Time.Before(t[j].Time)
	})
}
