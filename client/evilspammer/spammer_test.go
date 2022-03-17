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

	options := []Options{
		WithSpamDetails(5, time.Second, time.Minute, 100),
		WithSpammingFunc(ValueSpammingFunc),
		WithSpamWallet(evilWallet),
	}
	spammer := NewSpammer(options...)
	spammer.Spam()

}
