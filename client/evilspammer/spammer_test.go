package evilspammer

import (
	"github.com/iotaledger/goshimmer/client/evilwallet"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestSpamTransactions(t *testing.T) {
	evilWallet := evilwallet.NewEvilWallet()

	err := evilWallet.RequestFreshBigFaucetWallet()
	require.NoError(t, err)

	outWallet := evilWallet.NewWallet(evilwallet.Reuse)

	scenario := evilwallet.NewEvilScenario(evilwallet.SingleTransactionBatch(), false, outWallet)
	options := []Options{
		WithSpamDetails(5, time.Second, time.Second*10, 10),
		WithSpammingFunc(ValueSpammingFunc),
		WithSpamWallet(evilWallet),
		WithEvilScenario(scenario),
	}
	spammer := NewSpammer(options...)
	spammer.Spam()
}

func TestSpamDoubleSpend(t *testing.T) {
	evilWallet := evilwallet.NewEvilWallet()

	err := evilWallet.RequestFreshBigFaucetWallet()
	require.NoError(t, err)

	outWallet := evilWallet.NewWallet(evilwallet.Reuse)

	scenario := evilwallet.NewEvilScenario(evilwallet.DoubleSpendBatch(5), false, outWallet)
	options := []Options{
		WithSpamDetails(5, time.Second, time.Second*10, 10),
		WithSpammingFunc(CustomConflictSpammingFunc),
		WithSpamWallet(evilWallet),
		WithEvilScenario(scenario),
	}
	spammer := NewSpammer(options...)
	spammer.Spam()
}
