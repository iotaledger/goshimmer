package evilspammer

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/client/evilwallet"
	"github.com/stretchr/testify/require"
)

func TestSpamTransactions(t *testing.T) {
	evilWallet := evilwallet.NewEvilWallet()

	err := evilWallet.RequestFreshBigFaucetWallet()
	require.NoError(t, err)

	scenario := evilwallet.NewEvilScenario(nil, 0, false)
	options := []Options{
		WithSpamDetails(5, time.Second, time.Minute, 100),
		WithSpammingFunc(ValueSpammingFunc),
		WithSpamWallet(evilWallet),
		WithEvilScenario(scenario),
	}
	spammer := NewSpammer(options...)
	spammer.Spam()

}

func TestDoubleSpam(t *testing.T) {
	evilWallet := evilwallet.NewEvilWallet()

	err := evilWallet.RequestFreshBigFaucetWallet()
	require.NoError(t, err)

	scenario := evilwallet.NewEvilScenario(evilwallet.DoubleSpendBatch(2), 0, false)
	options := []Options{
		WithSpamDetails(5, time.Second, time.Minute, 100),
		WithSpammingFunc(DoubleSpendSpammingFunc),
		WithSpamWallet(evilWallet),
		WithEvilScenario(scenario),
	}
	spammer := NewSpammer(options...)
	spammer.Spam()

}
