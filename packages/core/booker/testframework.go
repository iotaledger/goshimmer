package booker

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
)

type TestFramework struct {
	Ledger *ledger.Ledger
	Booker *Booker

	*tangle.TestFramework
}

func NewTestFramework(t *testing.T) (newTestFramework *TestFramework) {
	newTestFramework = &TestFramework{
		Ledger:        ledger.New(),
		TestFramework: tangle.NewTestFramework(t),
	}
	newTestFramework.Booker = New(newTestFramework.Tangle, newTestFramework.Ledger)

	return
}
