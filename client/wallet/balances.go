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

type ConditionalBalance struct {
	Balance          map[ledgerstate.Color]uint64
	FallbackDeadline time.Time
}

type ConditionalBalanceSlice []*ConditionalBalance

func (c ConditionalBalanceSlice) Sort() {
	sort.Slice(c, func(i, j int) bool {
		return c[i].FallbackDeadline.Before(c[j].FallbackDeadline)
	})
}
